# Build script for the Per-User-Web-Server Session Manager server.

echo Building Session Manager...

# Stop any existing running service.
systemctl stop PUWSSessionManager

# Get any required mondules.
go get github.com/moby/moby/api/types/container
go get github.com/moby/moby/client

# Clear out any previously-compile binary.
rm sessionManager

go build sessionManager.go
#cp sessionManager /usr/local/bin

# Set up systemd to run PUWSSessionManager, if it isn't already.
#[ ! -f /etc/systemd/system/webconsole.service ] && cp webconsole.service /etc/systemd/system/webconsole.service && chmod 644 /etc/systemd/system/webconsole.service

# Restart the PUWSSessionManager service.
#systemctl start PUWSSessionManager
#systemctl enable PUWSSessionManager
