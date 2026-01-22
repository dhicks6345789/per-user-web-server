package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func main() {
	// Initialize the Docker client
	// It automatically looks for the Docker socket (unix:///var/run/docker.sock)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer cli.Close()

	// 1. Endpoint to List Containers
	http.HandleFunc("/containers", func(w http.ResponseWriter, r *http.Request) {
		containers, err := cli.ContainerList(context.Background(), container.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(containers)
	})

	// 2. Endpoint to Stop a Container
	// Usage: POST /stop?id=container_id_here
	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
			return
		}

		containerID := r.URL.Query().Get("id")
		if containerID == "" {
			http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
			return
		}

		// Stop the container with a default timeout
		err := cli.ContainerStop(context.Background(), containerID, container.StopOptions{})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to stop: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Container %s stopped successfully", containerID)
	})

	fmt.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
