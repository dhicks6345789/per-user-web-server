# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

## Set the home directory.
#export HOME=/home/$1
#export USER=$1
##export DISPLAY=:$5
#export DISPLAY=:1
#export PORT=5901

mkdir -p /home/$1/.vnc
echo "$4" | vncpasswd -f > /home/$1/.vnc/passwd
chmod 600 /home/$1/.vnc/passwd

#cp /root/docker-desktop-xstartup /home/$1/.vnc/xstartup
#chown $1:$1 /home/$1/.vnc/xstartup
#chmod u+x /home/$1/.vnc/xstartup

#mkdir -p /home/$1/.config/tigervnc
#chown -R $1:$1 /home/$1/.config/tigervnc
#cp /root/docker-desktop-xstartup /home/$1/.config/tigervnc/xstartup
#chown $1:$1 /home/$1/.config/tigervnc/xstartup
#chmod u+x /home/$1/.config/tigervnc/xstartup

echo "Starting VNC server, password $4 on display number $5."
vncserver :$5 -geometry 1280x800 -depth 24
tail -f ~/.vnc/*.log"
