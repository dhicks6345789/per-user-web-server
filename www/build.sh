# Build script for the Per-User-Web-Server Web Server.

echo Building Web Serer...

# Get any required Go mondules.
#go get ...

# Clear out any previously-compile binary.
rm wwwServer

# Build the executable.
go build wwwServer.go

# Exit if we didn't manage to build the executable.
[ ! -f sessionManager ] && { echo "Error: wwwServer not compiled."; exit 1; }
