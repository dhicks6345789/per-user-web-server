# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

echo "Starting VNC server, password $4 on display number $5."
tigervncserver -fg -localhost no -geometry 1280x720 :$5

tail -f /home/$1/.vnc/*.log
