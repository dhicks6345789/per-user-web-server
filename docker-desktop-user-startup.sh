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
#
# Since rclone v1.74 the old single-port "rclone rcd --rc-web-gui" has been replaced by the "rclone gui" command, which serves
# the web GUI and the remote control (RC) API on two SEPARATE ports. Our session proxy therefore needs both ports:
#   - port 8090 hosts the embedded web GUI (the interface the user sees in their browser)
#   - port 5572 hosts the RC API, which the web GUI talks to. The proxy routes "/rclone/rc" to this port.
# The ports are fixed so the session proxy can find them. We bind to "0.0.0.0" so the (separate) session proxy container,
# on the same Docker network, can reach them - if we used "localhost" or "127.0.0.1" only local connections would be accepted.
echo "Starting rclone GUI server (GUI on port 8090, RC API on port 5572), username $1."
rclone gui --addr 0.0.0.0:8090 --api-addr 0.0.0.0:5572 --user $1 --pass $4 --no-open-browser &

if [ -f "/home/$1/startup.sh" ]; then
  HOME=/home/$1 bash /home/$1/startup.sh $1 $2 $3 $4 $5
fi

echo "Starting VNC server, password $4 on display number $5."
tigervncserver -fg -localhost no -geometry 1280x720 :$5
