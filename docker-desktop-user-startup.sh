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

echo "Starting VNC server, password $4 on display number $5."
tigervncserver -fg -localhost no -geometry 1280x720 :$5
