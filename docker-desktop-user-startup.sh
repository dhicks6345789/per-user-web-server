# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

# Set up VNC password.
mkdir -p /home/$1/.vnc && echo "$4" | vncpasswd -f > /home/$1/.vnc/passwd && chmod 600 /home/$1/.vnc/passwd
  
echo "Starting VNC server, password $4 on display number $5."

# Start TigerVNC.
vncserver -fg -localhost no -geometry 1280x720 :$5
