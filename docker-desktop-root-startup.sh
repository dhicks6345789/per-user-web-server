# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

# First, create the user's group.
groupadd -g $3 $1
# Create the user with a home directory and bash shell.
useradd -m --uid "$2" --gid "$3" -s /bin/bash "$1"
# Set the user's password.
echo "$1:$4" | chpasswd
echo "Created user $1 with IDs $2:$3."

cp /root/docker-desktop-user-startup.sh /home/$1/startup.sh
chown $1 /home/$1/startup.sh
chmod u+x /home/$1/startup.sh

mkdir -p /home/$1/.vnc
cp /root/docker-desktop-xstartup /home/$1/.vnc/xstartup
chown $1 /home/$1/.vnc/xstartup
chmod u+x /home/$1/.vnc/xstartup

su - $1 -c "bash /home/$1/startup.sh $1 $2 $3 $4 $5"
