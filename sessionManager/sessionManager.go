/*
	The session manager for the per-user-web-server (PUWS) project. Sits on the main host server and communicates with / manages Docker containers,
	in particular the desktop instances of users who are connecting via Guacamole.
*/
package main

import (
	"os"
	"os/exec"
	"os/user"
	"fmt"
	"log"
	"time"
	"bufio"
	"slices"
	"strings"
	"strconv"
	"net/http"
	"context"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"

	// The Argon2 hashing library, used to produce passwords for VNC sessions.
	"golang.org/x/crypto/argon2"

	// The Docker management library - originally docker/docker, but now called "moby".
	"github.com/moby/moby/client"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/mount"
)

func runShellCommand(command string, args ...string) string {
	shellCmd := exec.Command(command, args...)
	cmdOutput, _ := shellCmd.CombinedOutput()
	return strings.TrimSpace(string(cmdOutput))
}

func main() {
	// We want each desktop instance to have a separate, un-guessable VNC password. However, we also want that password to be consistant so we can easily reconnect a user to their session.
	// Rather than hold session passwords in memory, we use a hash function to generate a password for each session from the username and a secret seed value.
	// That seed value is a simple string, stored in a text file at /etc/puws/seed.txt. If that path doesn't already exist, we create it now.
	seedPath := "/etc/puws/seed.txt"
	seedDir := filepath.Dir(seedPath)
	seedDirErr := os.MkdirAll(seedDir, 0755)
	if seedDirErr != nil {
		fmt.Println("Error creating directories: " + seedDirErr.Error())
		return
	}

	// Check if the seed value file exists, creating it if not.
	_, seedFileErr := os.Stat(seedPath)
	if os.IsNotExist(seedFileErr) {
		newSeedFile, seedFileCreateErr := os.Create(seedPath)
		if seedFileCreateErr != nil {
			fmt.Println("Failed to create file: " + seedPath + ", " + seedFileCreateErr.Error())
			return
		}
		// Generate a random 32-character hexadecimal string...
		seedBytes := make([]byte, 16)
		if _, seedBytesErr := rand.Read(seedBytes); seedBytesErr != nil {
			fmt.Println("Failed to generate random seed file: " + seedPath + ", " + seedBytesErr.Error())
			return
		}
		// ...and write it to the seed file.
		fmt.Fprintf(newSeedFile, hex.EncodeToString(seedBytes))
		newSeedFile.Close()
	} else if seedFileErr != nil {
		fmt.Println("An error occurred while checking the file: " + seedPath + ", " + seedFileErr.Error())
		return
	}

	// Get the random seed value.
	randomSeed, randomSeedErr := os.ReadFile(seedPath)
    if randomSeedErr != nil {
		fmt.Println("Error reading random seed value from file: " + seedPath + ", " + randomSeedErr.Error())
        return
    }
	
	// Initialize the Docker client. It automatically looks for the Docker socket (unix:///var/run/docker.sock).
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer cli.Close()

	// To do: somewhere, add a periodic function that can do things like close sessions that have been disconnected from for a set time.
	
	// Endpoint connectOrStartSession - returns a port number and password to connect with VNC.
	// Usage: POST /connectOrStartSession?username=USERNAME
	// Returns: JSON { portNumber, password }
	// If an existing session already exists for the user it returns the details for that, otherwise it starts a new desktop session (container).
	http.HandleFunc("/connectOrStartSession", func(httpResponse http.ResponseWriter, r *http.Request) {
		// Parse the HTTP GET/POST request form data.
		if err := r.ParseForm(); err != nil {
			http.Error(httpResponse, "Error parsing form", http.StatusBadRequest)
			return
		}
		// Get any passed variables using FormValue or PostForm.
		username := strings.TrimSpace(r.FormValue("username"))
		if username == "" {
			http.Error(httpResponse, "Missing 'username' parameter", http.StatusBadRequest)
			return
		}

		fmt.Println("Looking for session for user: ", username)

		// Get a list of existing containers from Docker.
		containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
		if err != nil {
			http.Error(httpResponse, err.Error(), http.StatusInternalServerError)
			return
		}

		var VNCPorts []uint16
		var VNCPort uint16 = 0
		var possibleVNCPort uint16 = 0
		// Go through the list of containers looking for any where the image used matches our "dockerdesktop" image.
		for _, item := range containers.Items {
			if strings.HasPrefix(item.Names[0], "/desktop-") {
				VNCPorts = append(VNCPorts, item.Ports[0].PrivatePort)
				if strings.TrimPrefix(item.Names[0], "/desktop-") == username {
					VNCPort = item.Ports[0].PrivatePort
					fmt.Printf("Found on port: %d", VNCPort)
				}
			}
		}

		// Generate a unique password for this session, a hash of the random seed and the username.
		// Generate the Argon2-hashed password. Parameters are: time (in iterations), memory (in bytes), threads, key length.
		VNCPassword := hex.EncodeToString(argon2.IDKey([]byte(username), randomSeed, 1, 64*1024, 4, 32))

		// If no existing session found, start one.
		if VNCPort == 0 {
			// Start an instance of our "desktop" Docker container - see for example code:
			// https://docs.docker.com/reference/api/engine/sdk/examples/
			// And for create options:
			// https://github.com/moby/moby/blob/master/api/types/container/config.go
			fmt.Println("Starting desktop session for user: ", username)
			
			// First, find an available VNC port number.
			for possibleVNCPort = 5901; slices.Contains(VNCPorts, possibleVNCPort) && possibleVNCPort <= 5920; possibleVNCPort = possibleVNCPort + 1 {
			}
			// If no free port found, return an error.
			if possibleVNCPort == 0 {
				http.Error(httpResponse, "No free sessions.", http.StatusInternalServerError)
				return
			}
			VNCPort = possibleVNCPort
			VNCDisplay := int(VNCPort) - 5900

			// Make sure there is a user with that username on the host machine so that when we create folders to mount in their desktop image they have the appropriate ownership and permissions.
			userUIDStr := ""
			userGIDStr := ""
			userTryCount := 0
			userCreateOutput := ""
			for userUIDStr == "" && userTryCount < 2 {
				desktopUser, desktopUserError := user.Lookup(username)
				if desktopUserError == nil {
					userUIDStr = desktopUser.Uid
					userGIDStr = desktopUser.Gid
				} else {
					// The user wasn't found - the user doesn't exist, therefore create it.
					userCreateOutput = runShellCommand("useradd", "-m", "-s", "/bin/bash", username)
					userTryCount = userTryCount + 1
				}
			}
			if userTryCount == 2 {
				http.Error(httpResponse, "Error creating user on host for user " + username + ": " + userCreateOutput, http.StatusInternalServerError)
				return
			}
			userUID, userUIDErr := strconv.Atoi(userUIDStr)
			if userUIDErr != nil {
				http.Error(httpResponse, "Error getting user UID: " + userUIDErr.Error(), http.StatusInternalServerError)
				return
			}
			userGID, userGIDErr := strconv.Atoi(userGIDStr)
			if userGIDErr != nil {
				http.Error(httpResponse, "Error getting user GID: " + userGIDErr.Error(), http.StatusInternalServerError)
				return
			}
			
			// We're about to create a container that mounts the user's /var/www/username folder.
			// First, make sure that folder exists, and that it is owned by the appropriate user.
			userWWWDirErr := os.MkdirAll("/var/www/" + username, 0755)
			if userWWWDirErr != nil {
				http.Error(httpResponse, "Error creating directory: " + userWWWDirErr.Error(), http.StatusInternalServerError)
				return
			}
			userChownErr := os.Chown("/var/www/" + username, userUID, userGID)
			if userChownErr != nil {
				http.Error(httpResponse, "Error assigning directory /var/www/" + username + " to user: " + userChownErr.Error(), http.StatusInternalServerError)
				return
			}
			
			// Create the container that holds the user's desktop session.
			containerContext := context.Background()
			exposedPort, _ := network.ParsePort(strconv.Itoa(int(VNCPort)) + "/TCP")
			resp, containerCreateErr := cli.ContainerCreate(containerContext, client.ContainerCreateOptions{
				Config: &container.Config{
					// Expose the VNC port number we want to use to connect to the VNC instance running in this container.
					ExposedPorts: network.PortSet{exposedPort:{}},
					// Pass in the VNC password and display number to the custom startup script that runs inside the container.
					// Cmd: []string{"bash", "/home/desktopuser/startup.sh", VNCPassword, strconv.Itoa(VNCDisplay)},
					//User: userUIDStr + ":" + userGIDStr,
					Cmd: []string{"bash", "/root/docker-desktop-root-startup.sh", username, userUIDStr, userGIDStr, VNCPassword, strconv.Itoa(VNCDisplay)},
					Tty: false,
				},
				NetworkingConfig: &network.NetworkingConfig{
					// Join the container to the main network group so the Guacamole gateway can see the VNC instance.
					EndpointsConfig: map[string]*network.EndpointSettings{
						"pangolin_main": &network.EndpointSettings{},
					},
				},
				// Set up mount points in the container. Confusingly, these mount points, in /home/username, will be created before the actual user inside the container.
				// Therefore, there is a startup script (that runs as root) inside the container that sets up the named user, matching UIDs with the host.
				HostConfig: &container.HostConfig{
					// We use the rclone Docker plugin to mount the user's Google Drive home folder as their "Documents" folder in their new desktop container.
					Mounts: []mount.Mount{
						mount.Mount{
							Type: mount.TypeVolume,
							Target: "/home/" + username + "/Documents",
							VolumeOptions: &mount.VolumeOptions{
								DriverConfig: &mount.Driver{
									Name: "rclone",
									Options: map[string]string{
										"remote": "gdrive:",
										"allow_other": "true",
										"vfs-cache-mode": "full",
										"drive-impersonate": username + "@knightsbridgeschool.com",
									},
								},
							},
						},
						// We mount the host www folder into the container. We have to match up the UIDs for the host and containers, hence us having to pass in the
						// host user's UID to the container's startup script.
						mount.Mount{
							Type: mount.TypeBind,
							Source: "/var/www/" + username,
							Target: "/home/" + username + "/www",
							ReadOnly: false,
						},
					},
				},
				// We use our own container image.
				Image: "sansay.co.uk-dockerdesktop:0.1-beta.3",
				// Use a consistant name we can use later for management.
				Name: "desktop-" + username,
			})
			// Check the container create process worked okay.
			if containerCreateErr != nil {
				http.Error(httpResponse, "Error creating container for user " + username + ", " + containerCreateErr.Error(), http.StatusInternalServerError)
				return
			}

			// Start the newly-create container, report any errors.
			_, containerStartErr := cli.ContainerStart(containerContext, resp.ID, client.ContainerStartOptions{})
			if containerStartErr != nil {
				http.Error(httpResponse, "Error starting container for user " + username + ", " + containerStartErr.Error(), http.StatusInternalServerError)
				return
			}
			
			// Get a reader object to read the container logs so we can check to see when the VNC server has started up.
			logReader, logReaderErr := cli.ContainerLogs(containerContext, resp.ID, client.ContainerLogsOptions{ShowStdout:true, ShowStderr:true, Follow:true, Timestamps:true, Tail:"all"})
			if logReaderErr != nil {
				http.Error(httpResponse, "Error getting reader from container, " + err.Error(), http.StatusInternalServerError)
				return
			}
			defer logReader.Close()
			
			// Create a new buffered scanner object se we can read the container logs a line at a time, looping until we see the "Starting VNC server" message.
			logScanner := bufio.NewScanner(logReader)
			logLine := ""
			// Note that, unless the container terminates early due to some error, logScanner.Scan() should always return true.
			for logScanner.Scan() && !strings.Contains(logLine, "Starting VNC server") {
				logLine = logScanner.Text()
				fmt.Println(logLine)
				time.Sleep(1 * time.Second)
			}
			
			// Report any errors during the log reading process.
			if logScannerErr := logScanner.Err(); logScannerErr != nil {
				http.Error(httpResponse, "Error getting reader from container, " + logScannerErr.Error(), http.StatusInternalServerError)
				return
			}
		}

		// If we've got to this point, we should have a running "desktop" container with a VNC session started up on a known port and with a known password.
		httpResponse.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(httpResponse, "{\"portNumber\":\"" + strconv.Itoa(int(VNCPort)) + "\", \"password\":\"" + VNCPassword + "\"}")
	})

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
