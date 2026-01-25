package main

import (
	"fmt"
	"log"
	"time"
	"bufio"
	"slices"
	"strings"
	"strconv"
	"net/http"
	"context"

	// The Docker management library - originally docker/docker, but now called "moby".
	"github.com/moby/moby/client"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
)

func main() {
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
	http.HandleFunc("/connectOrStartSession", func(w http.ResponseWriter, r *http.Request) {
		// Parse the HTTP GET/POST request form data.
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}
		// Get any passed variables using FormValue or PostForm.
		username := strings.TrimSpace(r.FormValue("username"))
		if username == "" {
			http.Error(w, "Missing 'username' parameter", http.StatusBadRequest)
			return
		}

		fmt.Println("Looking for session for user: ", username)

		// Get a list of existing containers from Docker.
		containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
				http.Error(w, "No free sessions.", http.StatusInternalServerError)
				return
			}
			VNCPort = possibleVNCPort
			VNCDisplay := int(VNCPort) - 5900
			
			// To do: unmount or re-use any existing user mount, make sure we don't double-up.
			// Mount the user's Google Drive home to /mnt in the container host, ready to be passed to the user's desktop container.
			// "rclone", "mount", "gdrive:", "/mnt/" + username, "--allow-other", "--vfs-cache-mode", "writes", "--drive-impersonate", username + "@knightsbridgeschool.com", "&"

			// Create the container that holds the user's desktop session.
			containerContext := context.Background()
			exposedPort, _ := network.ParsePort(strconv.Itoa(int(VNCPort)) + "/TCP")
			resp, containerCreateErr := cli.ContainerCreate(containerContext, client.ContainerCreateOptions{
				Config: &container.Config{
					// Expose the VNC port number we want to use to connect to the VNC instance running in this container.
					ExposedPorts: network.PortSet{exposedPort:{}},
					Cmd: []string{"bash", "/home/desktopuser/startup.sh", "vncpassword", strconv.Itoa(VNCDisplay)},
					Tty: false,
				},
				NetworkingConfig : &network.NetworkingConfig{
					// Join the container to the main network group so the Guacamole gateway can see the VNC instance.
					EndpointsConfig: map[string]*network.EndpointSettings{
						"pangolin_main": &network.EndpointSettings{},
					},
				},
				// We use our own container image.
				Image: "sansay.co.uk-dockerdesktop:0.1-beta.3",
				// Use a consistant name we can use later for management.
				Name: "desktop-" + username,
			})
			// Check the container create process worked okay.
			if containerCreateErr != nil {
				http.Error(w, "Error creating container for user " + username + ", " + containerCreateErr.Error(), http.StatusInternalServerError)
				return
			}

			// Start the newly-create container, report any errors.
			_, containerStartErr := cli.ContainerStart(containerContext, resp.ID, client.ContainerStartOptions{})
			if containerStartErr != nil {
				http.Error(w, "Error starting container for user " + username + ", " + containerStartErr.Error(), http.StatusInternalServerError)
				return
			}
			
			// Get a reader object to read the container logs so we can check to see when the VNC server has started up.
			logReader, logReaderErr := cli.ContainerLogs(containerContext, resp.ID, client.ContainerLogsOptions{ShowStdout:true, ShowStderr:true, Follow:true, Timestamps:true, Tail:"all"})
			if logReaderErr != nil {
				http.Error(w, "Error getting reader from container, " + err.Error(), http.StatusInternalServerError)
				return
			}
			defer logReader.Close()
			
			// Create a new buffered scanner object se we can read the container logs a line at a time.
			logScanner := bufio.NewScanner(logReader)
			logLine := ""
			
			// Read the container's log a line at a time, looping until we see the "Starting VNC server" message.
			// Note that, unless the container terminates early due to some error, logScanner.Scan() should always return true.
			for logSanner.Scan() && !strings.Contains(line, "Starting VNC server") {
				logLine = logScanner.Text()
				fmt.Println(logLine)
				time.Sleep(1 * time.Second)
			}
			
			// Report any errors during the scan process.
			if logScannerErr := logScanner.Err(); logScannerErr != nil {
				http.Error(w, "Error getting reader from container, " + logScannerErr.Error(), http.StatusInternalServerError)
				return
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Printf("{\"portNumber\":\"%d\", \"password\":\"vncpassword\"}", VNCPort)
		fmt.Fprintf(w, "{\"portNumber\":\"%d\", \"password\":\"vncpassword\"}", VNCPort)
	})

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
