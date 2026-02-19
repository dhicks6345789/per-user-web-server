# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

echo "Starting VNC server, password $4 on display number $5."
#vncserver :$5 -geometry 1280x800 -depth 24
vncserver :$5 -depth 24 -PasswordFile /home/$1/.vnc/passwd
tail -f /home/$1/.vnc/*.log
