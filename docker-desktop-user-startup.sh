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
rclone gui --addr localhost:8090 --api-addr localhost:8091 --no-open-browser --no-auth
#--pass string            Password for RC authentication
#--user string            User name for RC authentication

echo "Starting VNC server, password $4 on display number $5."
tigervncserver -fg -localhost no -geometry 1280x720 :$5
