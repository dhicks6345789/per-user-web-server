package main

import (
	"fmt"
	"log"
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
	// Returns: JSON { errorCode, portNumber, password }
	// If an existing session already exists for the user it returns the details for that, otherwise it starts a new desktop session (container).
	http.HandleFunc("/connectOrStartSession", func(w http.ResponseWriter, r *http.Request) {
		username := strings.TrimSpace(r.URL.Query().Get("username"))
		if username == "" {
			http.Error(w, "Missing 'username' parameter", http.StatusBadRequest)
			return
		}

		fmt.Println("Looking for session for user: ", username)

		containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		VNCPort := 0
		for _, item := range containers.Items {
			if strings.HasPrefix(item.Image, "sansay.co.uk-dockerdesktop-") {
				if strings.TrimPrefix(item.Image, "sansay.co.uk-dockerdesktop-") == username {
					fmt.Printf("Found - port: ")
					fmt.Printf(item.Ports[0])
				}
			}
		}

		// If no existing session found, start one.
		if VNCPort == 0 {
			fmt.Println("Starting session for user: ", username)
		}
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"errorCode\":\"\", \"portNumber\":\"\", \"password\":\"\"}")
	})

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
