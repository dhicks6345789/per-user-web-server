echo "First Argument: $1"
echo "Second Argument: $2"

# Set up VNC password.
mkdir -p /home/desktopuser/.vnc && echo "vncpassword" | vncpasswd -f > /home/desktopuser/.vnc/passwd && chmod 600 /home/desktopuser/.vnc/passwd

# Start TigerVNC.
vncserver -fg -localhost no -geometry 1280x720 :1

echo "Desktop started."
