# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

# Set the home directory.
export HOME=/home/$1
export USER=$1

# 3. Start the VNC Server
# -forever keeps it alive after disconnect, -shared allows multiple connections\n\
# -nopw is for testing (add -passwd yourpass for security)\n\
x11vnc -display :$5 -passwd $4 -listen 0.0.0.0 -xkb -forever -shared &
# 4. Keep the container alive by tailing the log or running an app.
xterm

# 2. Define the XDG paths explicitly
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_CACHE_HOME="$HOME/.cache"
export XDG_RUNTIME_DIR="/tmp/runtime-$USER"

# 3. Physically create the directories
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME" "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

echo Setting up VNC password...

#mkdir -p /home/$1/.vnc/
#echo "$4" | vncpasswd -f > /home/$1/.vnc/passwd
#chmod 600 /home/$1/.vnc/passwd
mkdir -p /home/$1/.config/tigervnc
echo "$4" | vncpasswd -f > /home/$1/.config/tigervnc/passwd
chmod 600 /home/$1/.config/tigervnc/passwd

echo "Starting VNC server, password $4 on display number $5."
vncserver -fg -localhost no -geometry 1280x720 :$5
