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
	"strings"
	"strconv"
	"net/http"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"syscall"
	"path/filepath"

	// Only needed if we need to map a container's port to the host for VNC debugging purposes.
	//"net/netip"

	// The YAML package, used for config data.
	"gopkg.in/yaml.v3"

	// The Argon2 hashing library, used to produce passwords for VNC sessions.
	"golang.org/x/crypto/argon2"

	// The Docker management library - originally docker/docker, but now called "moby".
	"github.com/moby/moby/client"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/mount"
)

// Define structs to hold YAML config data.
type RcloneMount struct {
	Username string `yaml:"username"`
	Local    string `yaml:"local"`
	Remote   string `yaml:"remote"`
}
type Config struct {
	RcloneMounts []RcloneMount `yaml:"rcloneMounts"`
	// A shared key used to protect the admin-only endpoints (used by the admin control panel). If empty, admin endpoints are disabled.
	AdminKey string `yaml:"adminKey"`
}

// An entry in the session auto-start list - a user session (Docker container) that should be
// started automatically when the server (re)boots, without the user needing to log in to a
// "/desktop" or "/ssh" endpoint first. The list is managed by the admin control panel.
type AutoStartEntry struct {
	Username string `yaml:"username" json:"username"`
	Image    string `yaml:"image" json:"image"`
}

// The structure of the auto-start list config file.
type AutoStartConfig struct {
	Sessions []AutoStartEntry `yaml:"sessions" json:"sessions"`
}

func runShellCommand(command string, args ...string) string {
	fmt.Println("runShellCommand: " + command + " " + strings.Join(args, " "))
	shellCmd := exec.Command(command, args...)
	cmdOutput, _ := shellCmd.CombinedOutput()
	return strings.TrimSpace(string(cmdOutput))
}

func startShellCommand(command string, args ...string) string {
	fmt.Println("startShellCommand: " + command + " " + strings.Join(args, " "))
	shellCmd := exec.Command(command, args...)
	shellErr := shellCmd.Start()
	if shellErr != nil {
		return "Error starting process: " + shellErr.Error()
	}
	return ""
}

func mkdirChown(theFolder string, theUserUID int, theUserGID int) string {
	userDirErr := os.MkdirAll(theFolder, 0700)
	if userDirErr != nil {
		return "Error creating directory" + theFolder + ": " + userDirErr.Error()
	}
	userChownErr := os.Chown(theFolder, theUserUID, theUserGID)
	if userChownErr != nil {
		return "Error assigning directory " + theFolder + " to user: " + userChownErr.Error()
	}
	return ""
}

// A helper function to check the admin key presented by a caller (the admin control panel) against the key stored in the config file.
// A timing-safe comparison is used so the two keys can't be guessed by measuring how long the comparison takes.
func isValidAdminKey(r *http.Request, configKey string) bool {
	// If no admin key is set in the config file, the admin endpoints are disabled - fail closed.
	if configKey == "" {
		return false
	}
	// Get the key the caller passed in the header.
	requestKey := r.Header.Get("X-Admin-Key")
	if requestKey == "" {
		return false
	}
	// Compare the two keys in a timing-safe way.
	return subtle.ConstantTimeCompare([]byte(requestKey), []byte(configKey)) == 1
}

// Reads the total and available memory and swap from /proc/meminfo, returning the values in kilobytes.
func readMemoryInfo() (int64, int64, int64, int64, error) {
	memInfoFile, openErr := os.Open("/proc/meminfo")
	if openErr != nil {
		return 0, 0, 0, 0, openErr
	}
	defer memInfoFile.Close()

	var memTotal int64 = 0
	var memAvailable int64 = 0
	var swapTotal int64 = 0
	var swapFree int64 = 0
	memScanner := bufio.NewScanner(memInfoFile)
	for memScanner.Scan() {
		// Each line looks like "MemTotal:       16299248 kB" - split into the label and the value.
		memFields := strings.Fields(memScanner.Text())
		if len(memFields) < 2 {
			continue
		}
		memValue, parseErr := strconv.ParseInt(memFields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch memFields[0] {
		case "MemTotal:":
			memTotal = memValue
		case "MemAvailable:":
			memAvailable = memValue
		case "SwapTotal:":
			swapTotal = memValue
		case "SwapFree:":
			swapFree = memValue
		}
	}
	if memScanner.Err() != nil {
		return 0, 0, 0, 0, memScanner.Err()
	}
	return memTotal, memAvailable, swapTotal, swapFree, nil
}

// Reads the total and available disk space on the root file system, returning the values in bytes.
func readDiskInfo() (uint64, uint64, error) {
	var diskStat syscall.Statfs_t
	statfsErr := syscall.Statfs("/", &diskStat)
	if statfsErr != nil {
		return 0, 0, statfsErr
	}
	// Note: "Bavail" is the space available to unprivileged users, so it doesn't include the blocks reserved for root.
	return diskStat.Blocks * uint64(diskStat.Bsize), diskStat.Bavail * uint64(diskStat.Bsize), nil
}

// The path of the session auto-start list config file.
const autoStartPath = "/etc/puws/autostart.yml"

// loadAutoStart reads the session auto-start list from its config file. A missing file simply
// means no sessions are marked for auto-start, so an empty list is returned.
func loadAutoStart() ([]AutoStartEntry, error) {
	autoStartData, autoStartErr := os.ReadFile(autoStartPath)
	if autoStartErr != nil {
		if os.IsNotExist(autoStartErr) {
			return []AutoStartEntry{}, nil
		}
		return nil, autoStartErr
	}
	var autoStartConfig AutoStartConfig
	if unmarshalErr := yaml.Unmarshal(autoStartData, &autoStartConfig); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	return autoStartConfig.Sessions, nil
}

// saveAutoStart writes the session auto-start list to its config file.
func saveAutoStart(sessions []AutoStartEntry) error {
	autoStartData, marshalErr := yaml.Marshal(AutoStartConfig{Sessions: sessions})
	if marshalErr != nil {
		return marshalErr
	}
	return os.WriteFile(autoStartPath, autoStartData, 0600)
}

// Checks whether a session (an image / username pair) appears in the auto-start list.
func isAutoStartSession(autoStartSessions []AutoStartEntry, imageName string, username string) bool {
	for _, entry := range autoStartSessions {
		if entry.Image == imageName && entry.Username == username {
			return true
		}
	}
	return false
}

// findSession looks for an existing container (running or stopped) for the given image and username,
// named "imageName-username". Returns nil if no matching container is found.
func findSession(cli *client.Client, imageName string, username string) (*container.Summary, error) {
	containers, containersErr := cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if containersErr != nil {
		return nil, containersErr
	}
	for _, item := range containers.Items {
		if strings.TrimPrefix(item.Names[0], "/") == imageName+"-"+username {
			return &item, nil
		}
	}
	return nil, nil
}

// startSession makes sure a session (Docker container) for the given user and image exists and is
// running, creating and starting it if necessary. Used both when a user connects to a "/desktop" or
// "/ssh" endpoint and when automatically starting sessions marked for auto-start.
// Returns an empty string on success, or an error message.
func startSession(cli *client.Client, config Config, randomSeed []byte, username string, imageName string) string {
	// Generate a unique password for this session, a hash of the random seed and the username.
	// Generate the Argon2-hashed password. Parameters are: time (in iterations), memory (in bytes), threads, key length.
	VNCPassword := hex.EncodeToString(argon2.IDKey([]byte(username), randomSeed, 1, 64*1024, 4, 32))
	VNCPort := 5901
	VNCDisplay := 1

	// If a container already exists for this session (for example, it was stopped), just start it again.
	existingSession, existingErr := findSession(cli, imageName, username)
	if existingErr != nil {
		return "Error listing containers: " + existingErr.Error()
	}
	if existingSession != nil {
		fmt.Println("Starting existing " + imageName + " session for user: ", username)
		_, containerStartErr := cli.ContainerStart(context.Background(), existingSession.ID, client.ContainerStartOptions{})
		if containerStartErr != nil {
			return "Error starting container for user " + username + ": " + containerStartErr.Error()
		}
		return ""
	}

	fmt.Println("Starting " + imageName + " session for user: ", username)

	// Make sure there is a user with that username on the host machine so that when we create folders to mount in their Docker image they have the appropriate ownership and permissions.
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
		return "Error creating user on host for user " + username + ": " + userCreateOutput
	}
	userUID, userUIDErr := strconv.Atoi(userUIDStr)
	if userUIDErr != nil {
		return "Error getting user UID: " + userUIDErr.Error()
	}
	userGID, userGIDErr := strconv.Atoi(userGIDStr)
	if userGIDErr != nil {
		return "Error getting user GID: " + userGIDErr.Error()
	}
	
	// We're about to create a container that mounts the user's /var/www/username and /etc/webconsole/tasks/username folders.
	// First, make sure those folders exist, and that they are owned by the matching user and have
	// permissions of 711 (drwx--x--x) so that other users won't be able to access the folders.
	mkdirErr := mkdirChown("/var/www/" + username, userUID, userGID)
	if mkdirErr != "" {
		return mkdirErr
	}
	mkdirErr = mkdirChown("/etc/webconsole/tasks/" + username, userUID, userGID)
	if mkdirErr != "" {
		return mkdirErr
	}
	
	// Go through the config (which is simply empty by default) and use rclone to mount any remote folders.
	for _, rcloneOptions := range config.RcloneMounts {
		// First, set up the values used in the rclone commands.
		rcloneUsername := strings.ReplaceAll(rcloneOptions.Username, "{{USERNAME}}", username)
		rcloneDriveImpersonate := []string{}
		if rcloneUsername != "" {
			rcloneDriveImpersonate = []string{"--drive-impersonate", rcloneUsername}
		}
		rcloneLocal := strings.ReplaceAll(rcloneOptions.Local, "{{USERNAME}}", username)
		rcloneRemote := strings.ReplaceAll(rcloneOptions.Remote, "{{USERNAME}}", username)

		// Make sure the local folder isn't already being used as a mount point.
		umountOutput := runShellCommand("umount", rcloneLocal)
		if umountOutput != "" {
			fmt.Println("umountOutput: " + umountOutput)
		}
		
		// Make sure the local folder exists and is owned by the user.
		mkdirErr = mkdirChown(rcloneLocal, userUID, userGID)
		if mkdirErr != "" {
			return mkdirErr
		}
		
		// Make sure the remote destination exists - create a new, empty folder (using rclone) if not.
		rcloneMkdirOutput := startShellCommand("rclone", append(append([]string{"mkdir"}, rcloneDriveImpersonate...), []string{rcloneUsername, rcloneRemote}...)...)
		if rcloneMkdirOutput != "" {
			fmt.Println("rcloneMkdirOutput: " + rcloneMkdirOutput)
		}
		
		// Mount the remote folder using rclone.
		rcloneMountOutput := startShellCommand("rclone", append(append([]string{"mount"}, rcloneDriveImpersonate...), []string{"--vfs-cache-mode", "full", "--allow-other", rcloneRemote, rcloneLocal}...)...)
		if rcloneMountOutput != "" {
			fmt.Println("rcloneMountOutput: " + rcloneMountOutput)
		}

		// Wait for the mount operation to complete.
		rcloneFolderMounted := false
		for rcloneFolderMounted == false {
			// Run "df -h" to see if the folder is mounted okay.
			for _, line := range strings.Split(runShellCommand("df", "-h"), "\n") {
				if strings.Contains(line, rcloneLocal) {
					rcloneFolderMounted = true
				}
			}
			fmt.Println("Waiting for rclone mount " + rcloneLocal + " to complete...")
			// Pause to make sure(ish) the mount operation is complete.
			time.Sleep(1 * time.Second)
		}
	}
	
	// Create the container that holds the user's VNC session.
	containerContext := context.Background()
	exposedPort, _ := network.ParsePort(strconv.Itoa(int(VNCPort)) + "/TCP")
	resp, containerCreateErr := cli.ContainerCreate(containerContext, client.ContainerCreateOptions{
		Config: &container.Config{
			// Expose the VNC port number we want to use to connect to the VNC instance running in this container.
			ExposedPorts: network.PortSet{exposedPort:{}},
			// Pass in the VNC password and display number to the custom startup script that runs inside the container.
			Cmd: []string{"bash", "/root/docker-" + imageName + "-root-startup.sh", username, userUIDStr, userGIDStr, VNCPassword, strconv.Itoa(VNCDisplay)},
			Tty: false,
		},
		NetworkingConfig: &network.NetworkingConfig{
			// Join the container to the main network group so the Guacamole gateway can see the VNC instance.
			EndpointsConfig: map[string]*network.EndpointSettings{
				"pangolin_main": &network.EndpointSettings{},
			},
		},
		HostConfig: &container.HostConfig{
			// Set up mount points in the container. Confusingly, these mount points, in /home/username, will be created before the actual user inside the container.
			// Therefore, there is a startup script (that runs as root) inside the container that sets up the named user, matching UIDs with the host.
			Mounts: []mount.Mount{
				// We mount the host's user's home folder into the container. We have to match up the UIDs for the host and containers, hence us having to pass in the
				// host user's UID to the container's startup script.
				mount.Mount{
					Type: mount.TypeBind,
					Source: "/home/" + username,
					Target: "/home/" + username,
					ReadOnly: false,
				},
				// We mount the host www folder into the container. This is separate from the user's main home folder, we have a (custom) web server in a separate container
				// that serves user websites. This means a user doesn't have to have an active desktop session running for their website files to be served.
				mount.Mount{
					Type: mount.TypeBind,
					Source: "/var/www/" + username,
					Target: "/home/" + username + "/www",
					ReadOnly: false,
				},
				// We mount the host /etc/webconsole/tasks folder into the container. This lets the user create and edit Web Console Tasks.
				mount.Mount{
					Type: mount.TypeBind,
					Source: "/etc/webconsole/tasks/" + username,
					Target: "/home/" + username + "/webconsole",
					ReadOnly: false,
				},
			},
		},
		// We use our own container image.
		Image: "sansay.co.uk-docker" + imageName + ":0.1-beta.3",
		// Use a consistant name we can use later for management.
		Name: imageName + "-" + username,
	})
	// Check the container create process worked okay.
	if containerCreateErr != nil {
		return "Error creating container for user " + username + ", " + containerCreateErr.Error()
	}

	// Start the newly-created container, report any errors.
	_, containerStartErr := cli.ContainerStart(containerContext, resp.ID, client.ContainerStartOptions{})
	if containerStartErr != nil {
		return "Error starting container for user " + username + ", " + containerStartErr.Error()
	}
	
	// Get a reader object to read the container logs so we can check to see when the VNC server has started up.
	logReader, logReaderErr := cli.ContainerLogs(containerContext, resp.ID, client.ContainerLogsOptions{ShowStdout:true, ShowStderr:true, Follow:true, Timestamps:true, Tail:"all"})
	if logReaderErr != nil {
		return "Error getting reader from container, " + logReaderErr.Error()
	}
	defer logReader.Close()
	
	// Create a new buffered scanner object so we can read the container logs a line at a time, looping until we see the "Starting VNC server" message.
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
		return "Error getting reader from container, " + logScannerErr.Error()
	}
	return ""
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
		newSeedFile.WriteString(hex.EncodeToString(seedBytes))
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
	
	// Create an empty instance of the Config struct...
	var config Config
	// ...and read config data - just skip if there's no config file, the config variable will simply remain empty.
	configPath := "/etc/puws/config.yml"
	configFile, err := os.ReadFile(configPath)
	if err == nil {
		// Parse the YAML bytes into the Config struct instance.
		err = yaml.Unmarshal(configFile, &config)
		if err != nil {
			log.Fatalf("Failed to parse YAML config: %v", err)
		}
		fmt.Println("Config data loaded from " + configPath)
	} else {
		fmt.Println("No config file found at " + configPath + ", using default values.")
	}
	
	// Initialize the Docker client. It automatically looks for the Docker socket (unix:///var/run/docker.sock).
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer cli.Close()

	// To do: somewhere, add a periodic function that can do things like close sessions that have been disconnected from for a set time.
	
	// Endpoint connectToSession - returns a port number and password to connect with VNC.
	// Usage: POST /connectToSession?username=USERNAME&image=IMAGENAME
	// Returns: JSON { portNumber, password }
	// If an existing session already exists for the user it returns the details for that, otherwise it starts a new session (container).
	http.HandleFunc("/connectToSession", func(httpResponse http.ResponseWriter, r *http.Request) {
		// Parse the HTTP GET/POST request form data.
		if err := r.ParseForm(); err != nil {
			http.Error(httpResponse, "Error parsing form", http.StatusBadRequest)
			return
		}
		// Get any passed variables using FormValue or PostForm.
		username := strings.TrimSpace(r.FormValue("username"))
		imageName := strings.TrimSpace(r.FormValue("image"))
		startIfNotRunning := strings.TrimSpace(r.FormValue("start"))
		if username == "" {
			http.Error(httpResponse, "Missing 'username' parameter", http.StatusBadRequest)
			return
		}
		if imageName == "" {
			http.Error(httpResponse, "Missing 'image' parameter", http.StatusBadRequest)
			return
		}

		fmt.Println("Looking for session for user: ", username)

		// Look for an existing (running or stopped) session container for this user.
		existingSession, existingErr := findSession(cli, imageName, username)
		if existingErr != nil {
			http.Error(httpResponse, existingErr.Error(), http.StatusInternalServerError)
			return
		}

		// Generate a unique password for this session, a hash of the random seed and the username.
		// Generate the Argon2-hashed password. Parameters are: time (in iterations), memory (in bytes), threads, key length.
		VNCPassword := hex.EncodeToString(argon2.IDKey([]byte(username), randomSeed, 1, 64*1024, 4, 32))

		// If no running session exists, possibly start one.
		if existingSession == nil || existingSession.State != "running" {
			// A session isn't running, but we don't want to start one, so return to the caller.
			if startIfNotRunning != "true" {
				httpResponse.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(httpResponse, "{\"portNumber\":\"0\", \"password\":\"\"}")
				return
			}
			// Start the session - this creates a new container if one doesn't already exist.
			if startErr := startSession(cli, config, randomSeed, username, imageName); startErr != "" {
				http.Error(httpResponse, startErr, http.StatusInternalServerError)
				return
			}
		}

		// If we've got to this point, we should have a running container with a VNC session started up on a known port and with a known password.
		httpResponse.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(httpResponse, "{\"portNumber\":\"%s\", \"password\":\"%s\"}", strconv.Itoa(5901), VNCPassword)
	})

	// The following endpoints provide a "control panel" for system administrators, used by the web-based admin panel.
	// The endpoints are protected by a shared admin key, set in the config file, which the admin panel presents via the "X-Admin-Key" header.

	// Endpoint /admin/status - returns the status of the server: sessions (Docker containers) and host resource usage.
	// Usage: GET /admin/status
	// Returns: JSON with a list of sessions (including stopped ones, marked if they're selected for auto-start) and host resource usage values.
	http.HandleFunc("/admin/status", func(httpResponse http.ResponseWriter, r *http.Request) {
		// Check the caller is presenting the correct admin key.
		if !isValidAdminKey(r, config.AdminKey) {
			http.Error(httpResponse, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Build up the response data, which we'll encode to JSON before sending back.
		responseData := make(map[string]any)

		// The hostname of the machine the Session Manager is running on.
		hostname, hostnameErr := os.Hostname()
		if hostnameErr != nil {
			http.Error(httpResponse, "Error getting hostname: " + hostnameErr.Error(), http.StatusInternalServerError)
			return
		}
		responseData["hostname"] = hostname

		// Get a list of the existing containers (sessions) from Docker, including any that are stopped.
		containers, containersErr := cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
		if containersErr != nil {
			http.Error(httpResponse, "Error listing containers: " + containersErr.Error(), http.StatusInternalServerError)
			return
		}

		// Load the current auto-start list, so we can mark which sessions have been selected for it.
		autoStartSessions, autoStartErr := loadAutoStart()
		if autoStartErr != nil {
			http.Error(httpResponse, "Error loading auto-start list: " + autoStartErr.Error(), http.StatusInternalServerError)
			return
		}

		// Go through the containers, adding the important details of each one to our response.
		var sessions []map[string]string
		for _, item := range containers.Items {
			// The container name is "imageName-username" - split it into its two parts.
			sessionName := strings.TrimPrefix(item.Names[0], "/")
			sessionParts := strings.SplitN(sessionName, "-", 2)
			imageName := ""
			username := ""
			if len(sessionParts) == 2 {
				imageName = sessionParts[0]
				username = sessionParts[1]
			}
			sessions = append(sessions, map[string]string{
				"name":      sessionName,
				"image":     item.Image,
				"imageName": imageName,
				"username":  username,
				"state":     string(item.State),
				"status":    item.Status,
				"autoStart": strconv.FormatBool(isAutoStartSession(autoStartSessions, imageName, username)),
			})
		}
		responseData["sessions"] = sessions
		responseData["autostart"] = autoStartSessions

		// Host resource usage - system uptime, number of CPUs, memory and disk usage.
		responseData["uptime"] = runShellCommand("uptime")
		responseData["cpuCount"] = strings.TrimSpace(runShellCommand("nproc"))
		memTotal, memAvailable, swapTotal, swapFree, memErr := readMemoryInfo()
		if memErr != nil {
			http.Error(httpResponse, "Error reading memory info: " + memErr.Error(), http.StatusInternalServerError)
			return
		}
		responseData["memTotalKb"] = memTotal
		responseData["memAvailableKb"] = memAvailable
		responseData["swapTotalKb"] = swapTotal
		responseData["swapAvailableKb"] = swapFree
		diskTotal, diskAvailable, diskErr := readDiskInfo()
		if diskErr != nil {
			http.Error(httpResponse, "Error reading disk info: " + diskErr.Error(), http.StatusInternalServerError)
			return
		}
		responseData["diskTotalBytes"] = diskTotal
		responseData["diskAvailableBytes"] = diskAvailable

		// Encode the response data as a JSON string and return it to the caller.
		jsonData, jsonErr := json.Marshal(responseData)
		if jsonErr != nil {
			http.Error(httpResponse, "Error encoding JSON: " + jsonErr.Error(), http.StatusInternalServerError)
			return
		}
		httpResponse.Header().Set("Content-Type", "application/json")
		httpResponse.Write(jsonData)
	})

	// Endpoint /admin/autostart - reads or updates the session auto-start list, the sessions that
	// should be started automatically when the server (re)boots.
	// Usage: GET /admin/autostart - returns { "sessions": [ { "username": "...", "image": "..." }, ... ] }
	//        PUT /admin/autostart - accepts { "sessions": [ ... ] } and replaces the stored list.
	http.HandleFunc("/admin/autostart", func(httpResponse http.ResponseWriter, r *http.Request) {
		// Check the caller is presenting the correct admin key.
		if !isValidAdminKey(r, config.AdminKey) {
			http.Error(httpResponse, "Unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// Load the current auto-start list and return it to the caller.
			autoStartSessions, autoStartErr := loadAutoStart()
			if autoStartErr != nil {
				http.Error(httpResponse, "Error loading auto-start list: " + autoStartErr.Error(), http.StatusInternalServerError)
				return
			}
			jsonData, jsonErr := json.Marshal(AutoStartConfig{Sessions: autoStartSessions})
			if jsonErr != nil {
				http.Error(httpResponse, "Error encoding JSON: " + jsonErr.Error(), http.StatusInternalServerError)
				return
			}
			httpResponse.Header().Set("Content-Type", "application/json")
			httpResponse.Write(jsonData)
		case http.MethodPut:
			// Parse the new list from the request body.
			var newConfig AutoStartConfig
			decoderErr := json.NewDecoder(r.Body).Decode(&newConfig)
			if decoderErr != nil {
				http.Error(httpResponse, "Error parsing request: " + decoderErr.Error(), http.StatusBadRequest)
				return
			}
			// Check each entry has a username and an image, ignoring any that don't, and removing duplicates.
			var validSessions []AutoStartEntry
			seenSessions := make(map[string]bool)
			for _, entry := range newConfig.Sessions {
				username := strings.TrimSpace(entry.Username)
				imageName := strings.TrimSpace(entry.Image)
				if username == "" || imageName == "" {
					continue
				}
				sessionKey := imageName + "\x00" + username
				if seenSessions[sessionKey] {
					continue
				}
				seenSessions[sessionKey] = true
				validSessions = append(validSessions, AutoStartEntry{Username: username, Image: imageName})
			}
			// Save the new list to the config file.
			if saveErr := saveAutoStart(validSessions); saveErr != nil {
				http.Error(httpResponse, "Error saving auto-start list: " + saveErr.Error(), http.StatusInternalServerError)
				return
			}
			// Return the saved list to the caller.
			jsonData, jsonErr := json.Marshal(AutoStartConfig{Sessions: validSessions})
			if jsonErr != nil {
				http.Error(httpResponse, "Error encoding JSON: " + jsonErr.Error(), http.StatusInternalServerError)
				return
			}
			httpResponse.Header().Set("Content-Type", "application/json")
			httpResponse.Write(jsonData)
		default:
			http.Error(httpResponse, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Start any sessions marked for auto-start in the config file, so users don't have to log in to a
	// "/desktop" or "/ssh" endpoint first. Done in the background so it doesn't hold up the server
	// while each container boots up.
	go func() {
		autoStartSessions, autoStartErr := loadAutoStart()
		if autoStartErr != nil {
			log.Println("Error loading auto-start list: " + autoStartErr.Error())
			return
		}
		for _, entry := range autoStartSessions {
			if entry.Username == "" || entry.Image == "" {
				log.Println("Skipping invalid auto-start entry: " + entry.Image + " / " + entry.Username)
				continue
			}
			if startErr := startSession(cli, config, randomSeed, entry.Username, entry.Image); startErr != "" {
				log.Println("Error auto-starting session for user " + entry.Username + " (" + entry.Image + "): " + startErr)
			} else {
				fmt.Println("Auto-started session for user " + entry.Username + " (" + entry.Image + ")")
			}
		}
	}()

	fmt.Println("Server starting on :8091...")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
