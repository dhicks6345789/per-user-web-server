# This script runs as root when the user's desktop image starts up. Parameters passed in from the Docker creation process:
# $1=username
# $2=User UID
# $3=User GID
# $4=password
# $5=vncdisplay

# We haven't created the user yet, but their home folder already exists as we've mounted their "Documents" and "www" folders there at container creation time.
# Set ownership of their home folder by numeric IDs, we'll crate the actual user in the next step.
chown $2:$3 /home/$1

# First, create the user's group...
groupadd -g $3 $1
# ...and then the actual user, with a home directory and bash shell.
useradd -m --uid "$2" --gid "$3" -s /bin/bash "$1"
# Set the user's password to the passed-in password.
echo "$1:$4" | chpasswd
# Add the user to the sudoers list, letting them use "sudo" without a password.
echo "$1 ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/users
echo "Created user $1 with IDs $2:$3."



# Set up VNC home folder...
mkdir -p /home/$1/.vnc
chown $1:$1 /home/$1/.vnc/
rm /home/$1/.vnc/*

# ...with the passed-in VNC password (same as their standard user password set above)...
echo "$4" | vncpasswd -f > /home/$1/.vnc/passwd
chown $1:$1 /home/$1/.vnc/passwd
chmod 600 /home/$1/.vnc/passwd

# ...and copy in the XStartup script that starts up the user's desktop environment when they connect via VNC.
cp /root/docker-desktop-xstartup /home/$1/.vnc/xstartup
chown $1:$1 /home/$1/.vnc/xstartup
chmod u+x /home/$1/.vnc/xstartup



# Set up VNC home folder...
mkdir -p /home/$1/.config/tigervnc
chown $1:$1 /home/$1/.config
chown $1:$1 /home/$1/.config/tigervnc
rm /home/$1/.config/tigervnc/*

# ...with the passed-in VNC password (same as their standard user password set above)...
echo "$4" | tigervncpasswd -f > /home/$1/.config/tigervnc/passwd
chown $1:$1 /home/$1/.config/tigervnc/passwd
chmod 600 /home/$1/.config/tigervnc/passwd

# ...and copy in the XStartup script that starts up the user's desktop environment when they connect via VNC.
cp /root/docker-desktop-xstartup /home/$1/.config/tigervnc/xstartup
chown $1:$1 /home/$1/.config/tigervnc/xstartup
chmod u+x /home/$1/.config/tigervnc/xstartup



# Set up and run the user startup script, as the user.
cp /root/docker-desktop-user-startup.sh /home/$1/startup.sh
chown $1:$1 /home/$1/startup.sh
chmod u+x /home/$1/startup.sh
su - $1 -c "bash /home/$1/startup.sh $1 $2 $3 $4 $5"
