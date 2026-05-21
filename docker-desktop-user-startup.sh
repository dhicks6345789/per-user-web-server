# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

if [ ! -d "/home/$1/Documents/www" ]; then
  mkdir -p "/home/$1/Documents/www"
fi

if [ ! -d "/home/$1/Documents/Hugo" ]; then
  cd "/home/$1/Documents"
  hugo new site Hugo
  cd
fi

# Run rclone in "GUI" mode as a service. This lets the user connect to a (web based) graphical user interface to use rclone.
# A separate container provides a per-user proxy for that GUI interface, so users can connect to the rclone GUI via the Pangolin gateway.
# We use "0.0.0.0" as the IP address so the rclone application binds to the local network interface and allows connections from other
# machines (in this case, our rclone proxy container) - if we used "localhost" or "127.0.0.1" only local connections will be accepted.
echo "Starting rclone GUI server, username $1, password $4 on port 8090."
rclone rcd --rc-web-gui --rc-addr 0.0.0.0:8090 --rc-web-gui-no-open-browser --rc-user $1 --rc-pass $4 --rc-no-auth-inline &

echo "Starting VNC server, password $4 on display number $5."
tigervncserver -fg -localhost no -geometry 1280x720 :$5
