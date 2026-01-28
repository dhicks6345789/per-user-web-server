# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

# Create the user with a home directory and bash shell.
useradd -m --uid "$2" --gid "$3" -s /bin/bash "$1"
# Set the user's password.
echo "$1:$2" | chpasswd
echo "Created user $1 with IDs $2:$3."

sudo -u $1 bash <<EOF
  # Set up VNC password.
  mkdir -p /home/$1/.vnc && echo "$4" | vncpasswd -f > /home/$1/.vnc/passwd && chmod 600 /home/$1/.vnc/passwd
  
  echo "Starting VNC server, password $4 on display number $5."

  # Start TigerVNC.
  vncserver -fg -localhost no -geometry 1280x720 :$5  
EOF
