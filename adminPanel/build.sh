# Build script for the Per-User-Web-Server admin panel - a small Go application that provides a web-based
# control panel for system administrators, showing the status of the server.

echo Building admin panel...

# Get any required Go mondules.
#go get ...

# Clear out any previously-compile binary.
rm adminPanel

# Build the executable. We disable dynamic linking (CGO_ENABLED=0) so the executable generated can be run anywhere, not requiring the dynamically glibc
# library, and should, therefore, be suitible to run under things like the very minimal Alpine Linux Docker image.
CGO_ENABLED=0 GOOS=linux go build adminPanel.go

# Exit if we didn't manage to build the executable.
[ ! -f adminPanel ] && { echo "Error: adminPanel not compiled."; exit 1; }
