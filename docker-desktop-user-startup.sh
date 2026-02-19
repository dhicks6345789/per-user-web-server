# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

echo "Starting VNC server, password $4 on display number $5."
#vncserver :$5 -geometry 1280x800 -depth 24
#/usr/bin/Xtigervnc :1 -rfbport 5901 -PasswordFile /home/$1/.vnc/passwd -SecurityTypes VncAuth -auth /home/d.hicks/.Xauthority -geometry 1920x1200 -depth 24
#tigervncserver :1 -rfbport 5901 -SecurityTypes VncAuth -auth /home/d.hicks/.Xauthority -geometry 1920x1200 -depth 24
tigervncsession $1 :1
tail -f /home/$1/.vnc/*.log
