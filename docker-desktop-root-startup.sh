# This script runs as root when the user's desktop image starts up.

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

# Now we have a user, map the mount points passed in from the container host to the locations we want them for the user. The Documents folder, linked to the user's Google Drive home folder...
mkdir -p /home/$1/Documents
chown -R $1:$1 /home/$1/Documents
mount --bind /mnt/Documents /home/$1/Documents
# ...and the www folder, linked to the host's www folder and, therefore, served by the Apache web server container.
mkdir -p /home/$1/www
chown -R $1:$1 /home/$1/www
mount --bind /mnt/www /home/$1/www

# Set up the user startup script, which is where the VNC startup happens.
cp /root/docker-desktop-user-startup.sh /home/$1/startup.sh
chown $1:$1 /home/$1/startup.sh
chmod u+x /home/$1/startup.sh

mkdir -p /home/$1/.vnc
chown -R $1:$1 /home/$1/.vnc
cp /root/docker-desktop-xstartup /home/$1/.vnc/xstartup
chown $1:$1 /home/$1/.vnc/xstartup
chmod u+x /home/$1/.vnc/xstartup

# Run the user startup script as the user.
su - $1 -c "bash /home/$1/startup.sh $1 $2 $3 $4 $5"
