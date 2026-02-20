# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

echo "Starting VNC server, password $4 on display number $5."
tigervncserver -fg -localhost no -geometry 1280x720 :$5

#/usr/bin/Xtigervnc :1 -rfbport 5901 -PasswordFile /home/$1/.vnc/passwd -SecurityTypes VncAuth -auth /home/d.hicks/.Xauthority -geometry 1920x1200 -depth 24
#tigervncserver :1 -rfbport 5901 -SecurityTypes VncAuth -auth /home/d.hicks/.Xauthority -geometry 1920x1200 -depth 24

## Start the settings daemon (This MUST run for the panel to know its config).
#xfsettingsd &
## Give xfconfd a second to wake up
#sleep 2
## 5. Start the panel and the rest of the desktop
#xfce4-panel &
#xfdesktop &
#xfwm4 &

tail -f /home/$1/.vnc/*.log
