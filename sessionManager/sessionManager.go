package main

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"net/http"
	"context"

	// The Docker management library - originally docker/docker, but now called "moby".
	"github.com/moby/moby/client"
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
			fmt.Println("Container: " + item.Names[0])
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
			fmt.Println("Starting session for user: ", username)
			// First, find an available port number.
			for possibleVNCPort = 5901; slices.Contains(VNCPorts, possibleVNCPort) && possibleVNCPort <= 5920; possibleVNCPort = possibleVNCPort + 1 {
			}
			// If no free port found, return an error.
			if possibleVNCPort == 0 {
				http.Error(w, "No free sessions.", http.StatusInternalServerError)
				return
			}
			// Start the container
			// ContainerStartOptions is usually empty unless you are using Checkpoints
			ctx := context.Background()
			// containerStartResult, = 
			_, containerStartErr := cli.ContainerStart(ctx, "desktop-" + username, client.ContainerStartOptions{})
			if containerStartErr != nil {
				http.Error(w, "Error starting container for user " + username + ", " + containerStartErr.Error(), http.StatusInternalServerError)
				return
			}
			// "docker", "run", "--detach", "--name", "desktop-" + username, "--expose", desktopPort, "--network", "pangolin_main", "sansay.co.uk-dockerdesktop:0.1-beta.3", "bash", "/home/desktopuser/startup.sh", "bananas", String.valueOf(vncDisplay));
		}
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"portNumber\":\"%d\", \"password\":\"vncpassword\"}", VNCPort)
	})

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
