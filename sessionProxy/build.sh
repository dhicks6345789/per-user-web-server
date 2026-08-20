# Build script for the Per-User-Web-Server session proxy component - a small Go application that routes traffic at the "/rclone" endpoint to the appropriate user's instance of the rclone GUI.

echo Building session proxy...

# Get any required Go mondules.
#go get ...

# Clear out any previously-compile binary.
rm sessionProxy

# Build the executable. We disable dynamic linking (CGO_ENABLED=0) so the executable generated can be run anywhere, not requiring the dynamically glibc
# library, and should, therefore, be suitible to run under things like the very minimal Alpine Linux Docker image.
# Note: we build the whole package (rather than the single source file) so that the "//go:embed" directive in sessionProxy.go can embed the
# "appIndex.html" file into the binary.
CGO_ENABLED=0 GOOS=linux go build .

# Exit if we didn't manage to build the executable.
[ ! -f sessionProxy ] && { echo "Error: sessionProxy not compiled."; exit 1; }
