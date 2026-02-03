# Build script for the Per-User-Web-Server Web Server.

echo Building Web Serer...

# Stop any existing running service.
systemctl stop wwwServer

# Get any required Go mondules.
#go get github.com/moby/moby/client
#go get github.com/moby/moby/api/types/container
#go get golang.org/x/crypto/argon2

# Clear out any previously-compile binary.
rm wwwServer

# Build the executable.
go build wwwServer.go

# Exit if we didn't manage to build the executable.
[ ! -f sessionManager ] && { echo "Error: wwwServer not compiled."; exit 1; }
