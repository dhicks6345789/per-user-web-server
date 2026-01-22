# Build script for the Per-User-Web-Server Session Manager server.

echo Building Session Manager...

# Stop any existing running service.
systemctl stop PUWSSessionManager

# go get github.com/nfnt/resize
rm sessionManager
go build sessionManager.go
#cp sessionManager /usr/local/bin

# Set up systemd to run PUWSSessionManager, if it isn't already.
#[ ! -f /etc/systemd/system/webconsole.service ] && cp webconsole.service /etc/systemd/system/webconsole.service && chmod 644 /etc/systemd/system/webconsole.service

# Restart the PUWSSessionManager service.
#systemctl start PUWSSessionManager
#systemctl enable PUWSSessionManager
