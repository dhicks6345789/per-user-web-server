package main

import (
	"io"
	"os"
	"fmt"
	"log"
	"time"
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
			// Start a Docker container - see for example code:
			// https://docs.docker.com/reference/api/engine/sdk/examples/
			// And for create options:
			// https://github.com/moby/moby/blob/master/api/types/container/config.go
			
			fmt.Println("Starting session for user: ", username)
			// First, find an available port number.
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
			// docker run "--detach", "--name", "desktop-" + username, "--expose", desktopPort, "--network", "pangolin_main", "sansay.co.uk-dockerdesktop:0.1-beta.3", "bash", "/home/desktopuser/startup.sh", "bananas", String.valueOf(vncDisplay)
			ctx := context.Background()
			exposedPort, _ := network.ParsePort(strconv.Itoa(int(VNCPort)) + "/TCP")
			resp, containerCreateErr := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
				Config: &container.Config{
					ExposedPorts: network.PortSet{exposedPort:{}},
					Cmd: []string{"bash", "/home/desktopuser/startup.sh", "vncpassword", strconv.Itoa(VNCDisplay)},
					Tty: false,
				},
				NetworkingConfig : &network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{
						"pangolin_main": &network.EndpointSettings{},
					},
				},
				Image: "sansay.co.uk-dockerdesktop:0.1-beta.3",
				Name: "desktop-" + username,
			})
			if containerCreateErr != nil {
				http.Error(w, "Error creating container for user " + username + ", " + containerCreateErr.Error(), http.StatusInternalServerError)
				return
			}
			
			options := client.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true, // Set to true to stream logs in real-time
				Timestamps: true,
				Tail:       "all",
			}
			
			// Start the container.
			_, containerStartErr := cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{})
			if containerStartErr != nil {
				http.Error(w, "Error starting container for user " + username + ", " + containerStartErr.Error(), http.StatusInternalServerError)
				return
			}
			
			// Get the reader.
			reader, err := cli.ContainerLogs(ctx, resp.ID, options)
			if err != nil {
				http.Error(w, "Error getting reader from container for user " + username + ", " + err.Error(), http.StatusInternalServerError)
				return
			}
			// defer reader.Close()

			io.Copy(os.Stdout, reader)

			// Wait for the container to be ready.
			time.Sleep(2 * time.Second)

			// Read the logs.
			bodyBytes, err := io.ReadAll(reader)
			if err != nil {
				http.Error(w, "Error reading logs from reader for user " + username + ", " + containerStartErr.Error(), http.StatusInternalServerError)
				return
			}
			// Convert bytes to string.
			fmt.Println(string(bodyBytes))

			reader.Close()
		}
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"portNumber\":\"%d\", \"password\":\"vncpassword\"}", VNCPort)
	})

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
