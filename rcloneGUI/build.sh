# Build script for the Per-User-Web-Server rclone GUI component - a small Go application that routes traffic at the "/rclone" endpoint to the appropriate user's instance of the rclone GUI.

echo Building rclone GUI...

# Get any required Go mondules.
#go get ...

# Clear out any previously-compile binary.
rm rcloneGUI

# Build the executable.
go build rcloneGUI.go

# Exit if we didn't manage to build the executable.
[ ! -f rcloneGUI ] && { echo "Error: rcloneGUI not compiled."; exit 1; }
