# A script to set up a per-user web server. See:
# https://github.com/dhicks6345789/per-user-web-server

copyOrDownload () {
    echo Copying $1 to $2, mode $3...
    if [ -f $1 ]; then
        cp $1 $2
    elif [ -f moodle-server/$1 ]; then
        cp moodle-server/$1 $2
    else
        wget https://github.com/dhicks6345789/moodle-server/raw/master/$1 -O $2
    fi
    chmod $3 $2
}

# Set default command-line flag values.
servertitle="Web Server"
sslhandler="pangolin"

# Read user-defined command-line flags.
while test $# -gt 0; do
    case "$1" in
        -servername)
            shift
            servername=$1
            shift
            ;;
        -servertitle)
            shift
            servertitle=$1
            shift
            ;;
        -sslhandler)
            shift
            sslhandler=$1
            shift
            ;;
        *)
            echo "$1 is not a recognized flag."
            exit 1;
            ;;
    esac
done

# Check all required flags are set, print a usage message if not.
if [ -z "$servername" ] || [ -z "$dbpassword" ]; then
    echo "Usage: install.sh [-servername SERVERNAME] [-servertitle SERVERTITLE] [-sslhandler pangolin | tunnel | none]"
    echo "SERVERNAME: The full domain name of this server (e.g. webserver.example.com)."
    echo "Optional: SERVERTITLE: A title for the web server (e.g. \"My Company Web Server\". Defaults to \"Web Server\"." 
    echo "Optional: \"pangolin\" or \"tunnel\" as SSL Handler options. If \"tunnel\", the server will be configured assuming an SSL tunneling"
    echo "          service (Cloudflare, NGrok, etc) will be used to provide SSL ingress. If \"pangolin\", a Pangolin server will be installed"
    echo "          and set up to auto-configure SSL. If \"none\" (the default), neither option will be configured for."
    exit 1;
fi

echo Installing web server \""$servertitle"\"...

# Make sure Git (distributed source code control system) is installed.
if [ ! -d "/etc/git" ]; then
    apt-get install -y git
fi

# Install Pangolin (server that handles SSL tunneling and user authentication).
if [ $sslhandler = "pangolin" ]; then
    if [ ! -d "/etc/pangolin" ]; then
        wget -O installer "https://github.com/fosrl/pangolin/releases/download/1.7.3/installer_linux_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && chmod +x ./installer
    fi
fi

# Install Web Console (runs server-side scripts with a simple user interface, also acts as a basic web server) via Git.
if [ ! -d "web-console" ]; then
    git clone https://github.com/dhicks6345789/web-console.git
fi
cd web-console
git pull
cd ..












exit 0


# Make sure the Apache web server is installed.
if [ ! -d "/etc/apache2" ]; then
    apt install -y apache2
    rm /var/www/html/index.html
fi

# Get Moodle via Git.
if [ ! -d "moodle" ]; then
    git clone -b $moodlebranch git://git.moodle.org/moodle.git
fi

# Create / set up the Moodle database.
mysql --user=root --password=$dbpassword -e "CREATE DATABASE moodle DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql --user=root --password=$dbpassword -e "GRANT SELECT,INSERT,UPDATE,DELETE,CREATE,CREATE TEMPORARY TABLES,DROP,INDEX,ALTER ON moodle.* TO 'moodleuser'@'localhost' IDENTIFIED BY '$dbpassword';"

# Set up the Moodle data folder.
if [ ! -d "/var/lib/moodle" ]; then
    mkdir /var/lib/moodle
    chown www-data:www-data /var/lib/moodle
fi

# Copy the Moodle code to the web server.
cp -r moodle/* /var/www/html
rm /var/www/html/config-dist.php
copyOrDownload config.php /var/www/html/config.php 0644
sed -i "s/{{DBPASSWORD}}/$dbpassword/g" /var/www/html/config.php
sed -i "s/{{SERVERNAME}}/$servername/g" /var/www/html/config.php
if [ $sslhandler = "tunnel" ] || [ $sslhandler = "caddy" ]; then
    sed -i "s/{{SSLPROXY}}/true/g" /var/www/html/config.php
else
    sed -i "s/{{SSLPROXY}}/false/g" /var/www/html/config.php
fi

# Make sure DOS2Unix is installed.
if [ ! -f "/usr/bin/dos2unix" ]; then
    apt install -y dos2unix
fi

# Set up Crontab if it doesn't already exist.
if [ ! -f "/var/spool/cron/crontabs/root" ]; then
    copyOrDownload crontab crontab 0644
    dos2unix crontab
    crontab crontab
    rm crontab
fi

# Restart Apache so any changes take effect.
service apache2 restart

# Optionally, install Caddy web server.
if [ $sslhandler = "caddy" ]; then
    if [ ! -d "/etc/caddy" ]; then
        apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
        apt update
        apt install caddy
    fi

    # To do: add Caddy config here, configure to act as HTTPS proxy for Apache.
fi
