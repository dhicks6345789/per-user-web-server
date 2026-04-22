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

# Create a folder for the user to write any log files to.
mkdir -p /home/$1/.local/state
chown $1:$1 /home/$1/.local/state

# Set up the TigerVNC home folder...
mkdir -p /home/$1/.config/tigervnc
chown $1:$1 /home/$1/.config
chown $1:$1 /home/$1/.config/tigervnc
rm /home/$1/.config/tigervnc/*

# ...with the passed-in VNC password (same as their standard user password set above)...
echo "$4" | tigervncpasswd -f > /home/$1/.config/tigervnc/passwd
chown $1:$1 /home/$1/.config/tigervnc/passwd
chmod 600 /home/$1/.config/tigervnc/passwd

# ...and copy in the XStartup script that starts up the user's desktop environment when they connect via VNC.
cp /root/docker-exams-xstartup /home/$1/.config/tigervnc/xstartup
chown $1:$1 /home/$1/.config/tigervnc/xstartup
chmod u+x /home/$1/.config/tigervnc/xstartup



cat << EOF > /home/$1/autoResize.sh
while true; do
  # This waits for the root window to change size.
  xev -root -event structure | grep -m 1 "ConfigureNotify"
  # Force XFCE to refresh the workspace.
  #xfdesktop --reload
  xfwm4 --replace &
done
EOF
chown $1:$1 /home/$1/autoResize.sh
chmod u+x /home/$1/autoResize.sh



mkdir -p /etc/xdg/xfce4/kiosk
cat << EOF > /etc/xdg/xfce4/kiosk/kioskrc
[xfce4-session]
CustomizeSettings=NONE
Shutdown=NONE

[xfce4-panel]
Customize=NONE

[xfdesktop]
UserMenu=NONE
CustomizeBackdrop=NONE
EOF



#cat << EOF >> /etc/xdg/openbox/autostart
#picom --backend glx --no-fading-openclose --shadow-exclude 'class_g = "Wine"' &
#EOF



sed -i '/<\/applications>/i\
  <application name="exampad+.exe">\
    <fullscreen>yes</fullscreen>\
    <decor>no</decor>\
    <focus>yes</focus>\
    <layer>above</layer>\
    <position force="yes">\
      <x>0</x>\
      <y>0</y>\
    </position>\
    <size force="yes">\
      <width>100%</width>\
      <height>100%</height>\
    </size>\
  </application>' /etc/xdg/openbox/rc.xml



# To do: edit:
# Remove XFCE4 from image?



# Copy over the MSI installer to the user's home folder.
cp /root/ExamPad+.msi /home/$1/ExamPad+.msi
chown $1:$1 /home/$1/ExamPad+.msi

# Set up and run the user startup script, as the user.
cp /root/docker-exams-user-startup.sh /home/$1/startup.sh
chown $1:$1 /home/$1/startup.sh
chmod u+x /home/$1/startup.sh
su - $1 -c "bash /home/$1/startup.sh $1 $2 $3 $4 $5"
