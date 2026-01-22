package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/moby/moby/client"
)

func main() {
	// Initialize the Docker client. It automatically looks for the Docker socket (unix:///var/run/docker.sock).
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer cli.Close()

	// 1. Endpoint to List Containers
	//http.HandleFunc("/containers", func(w http.ResponseWriter, r *http.Request) {
		//fmt.Println("Called container.")
		//containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
		//if err != nil {
			//http.Error(w, err.Error(), http.StatusInternalServerError)
			//return
		//}

		//w.Header().Set("Content-Type", "application/json")
		//json.NewEncoder(w).Encode(containers)
	//})

	// 2. Endpoint to Stop a Container
	// Usage: POST /stop?id=container_id_here
	//http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		//if r.Method != http.MethodPost {
			//http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
			//return
		//}

		//containerID := r.URL.Query().Get("id")
		//if containerID == "" {
			//http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
			//return
		//}

		// Stop the container with a default timeout
		//_, err := cli.ContainerStop(context.Background(), containerID, client.ContainerStopOptions{})
		//if err != nil {
			//http.Error(w, fmt.Sprintf("Failed to stop: %v", err), http.StatusInternalServerError)
			//return
		//}

		//fmt.Fprintf(w, "Container %s stopped successfully", containerID)
	})

	// Endpoint connectOrStartSession - returns a port number and password to connect with VNC.
	// Usage: POST /connectOrStartSession?username=USERNAME
	// Returns: JSON {errorCode, portNumber, password}
	// If an existing session already exists for the user it returns the details for that, otherwise it starts a new desktop session (container).
	http.HandleFunc("/connectOrStartSession", func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		if username == "" {
			http.Error(w, "Missing 'username' parameter", http.StatusBadRequest)
			return
		}

		fmt.Println("Looking for session for user: ", username)
	}

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
