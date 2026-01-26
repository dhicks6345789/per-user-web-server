/*
	The session manager for the per-user-web-server (PUWS) project. Sits on the main host server and communicates with / manages Docker containers, in particular the desktop instances of users who are connecting via Guacamole.
*/
package main

import (
	"os"
	"os/exec"
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
	"github.com/alexedwards/argon2id"

	// The Docker management library - originally docker/docker, but now called "moby".
	"github.com/moby/moby/client"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	//"github.com/moby/moby/api/types/mount"
)

func main() {
	// We want each desktop instance to have a separate, un-guessable VNC password. However, we also want that password to be consistant so we can easily reconnect a user to their session.
	// Rather than hold session password in memory, we use a hash function to generate a password for each session from the username, port number and a secret seed value.
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
		VNCPassword, VNCPasswordErr := argon2id.CreateHash(string(randomSeed)+username, argon2id.DefaultParams)
		if VNCPasswordErr != nil {
			http.Error(httpResponse, "Error generating VNC session password for user " + username + ", " + VNCPasswordErr.Error(), http.StatusInternalServerError)
			return
		}
		VNCPassword = strings.Split(VNCPassword, "$")[5]

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
			
			// Mount the user's Google Drive home to /mnt in the container host, ready to be passed to the user's desktop container.
			// To do: unmount or re-use any existing user mount, make sure we don't double-up.
			rcloneCmd := exec.Command("rclone", "mount", "gdrive:", "/mnt/" + username, "--allow-other", "--vfs-cache-mode", "writes", "--drive-impersonate", username + "@knightsbridgeschool.com")
			rcloneErr := rcloneCmd.Start()
			if rcloneErr != nil {
				http.Error(httpResponse, "Running rclone failed: " + rcloneErr.Error(), http.StatusInternalServerError)
				return
			}
			
			// Create the container that holds the user's desktop session.
			containerContext := context.Background()
			exposedPort, _ := network.ParsePort(strconv.Itoa(int(VNCPort)) + "/TCP")
			resp, containerCreateErr := cli.ContainerCreate(containerContext, client.ContainerCreateOptions{
				Config: &container.Config{
					// Expose the VNC port number we want to use to connect to the VNC instance running in this container.
					ExposedPorts: network.PortSet{exposedPort:{}},
					// 1. Define the VOLUME inside the container
					Volumes: map[string]struct{}{
						"/home/desktopuser/Documents": {},
					},
					Cmd: []string{"bash", "/home/desktopuser/startup.sh", VNCPassword, strconv.Itoa(VNCDisplay)},
					Tty: false,
				},
				NetworkingConfig: &network.NetworkingConfig{
					// Join the container to the main network group so the Guacamole gateway can see the VNC instance.
					EndpointsConfig: map[string]*network.EndpointSettings{
						"pangolin_main": &network.EndpointSettings{},
					},
				},
				HostConfig: &container.HostConfig{
					// 2. Bind the host path to that container path.
					Binds: []string{
						"/mnt/d.hicks":"/home/desktopuser/Documents",
					},
				},
				// We use our own container image.
				Image: "sansay.co.uk-dockerdesktop:0.1-beta.3",
				// Use a consistant name we can use later for management.
				Name: "desktop-" + username,
				//Volumes: map[string]struct{}{
						//"Type": {mount.TypeBind},
						//"Source": {"/mnt/" + username},
						//"Target": {"/home/desktopuser/Documents"},
						//"ReadOnly": {false},
					//},
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
		fmt.Fprintf(httpResponse, "{\"portNumber\":\"%d\", \"password\":\"" + VNCPassword + "\"}", VNCPort)
	})

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
