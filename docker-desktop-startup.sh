# Set up VNC password.
mkdir -p /home/desktopuser/.vnc && echo "$1" | vncpasswd -f > /home/desktopuser/.vnc/passwd && chmod 600 /home/desktopuser/.vnc/passwd

echo "Starting VNC server, password $1 on display number $2."

# Start TigerVNC.
vncserver -fg -localhost no -geometry 1280x720 :$2
