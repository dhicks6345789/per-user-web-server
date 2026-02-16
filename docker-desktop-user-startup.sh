# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

# Ensure HOME is set.
export HOME=/home/$1
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_CACHE_HOME="$HOME/.cache"

# Create the xfce4 directory manually just in case.
mkdir -p $XDG_CONFIG_HOME/xfce4

echo Setting up VNC password...

#mkdir -p /home/$1/.vnc/
#echo "$4" | vncpasswd -f > /home/$1/.vnc/passwd
#chmod 600 /home/$1/.vnc/passwd
mkdir -p /home/$1/.config/tigervnc
echo "$4" | vncpasswd -f > /home/$1/.config/tigervnc/passwd
chmod 600 /home/$1/.config/tigervnc/passwd

echo "Starting VNC server, password $4 on display number $5."
vncserver -fg -localhost no -geometry 1280x720 :$5
