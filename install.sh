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
SERVERTITLE="Web Server"
SSLHANDLER="pangolin"
SERVERNAME=`dnsdomainname`
INSTALL_PANGOLIN=false
WEBCONSOLE_DOCKER_IMAGE="sansay.co.uk-webconsole:0.1-beta.3"
WWWSERVER_DOCKER_IMAGE="sansay.co.uk-wwwserver:0.1-beta.3"
DOCKERDESKTOP_DOCKER_IMAGE="sansay.co.uk-dockerdesktop:0.1-beta.3"

# Read user-defined command-line flags.
while test $# -gt 0; do
    case "$1" in
        -servername)
            shift
            SERVERNAME=$1
            shift
            ;;
        -servertitle)
            shift
            SERVERTITLE=$1
            shift
            ;;
        -sslhandler)
            shift
            SSLHANDLER=$1
            shift
            ;;
        -cloudflared_token)
            shift
            CLOUDFLARED_TOKEN=$1
            shift
            ;;
        -pangolin)
            shift
            INSTALL_PANGOLIN=true
            ;;
        -cloudflared_api_token)
            shift
            CLOUDFLARE_API_TOKEN=$1
            shift
            ;;
        -cloudflared_account_id)
            shift
            CLOUDFLARE_ACCOUNT_ID=$1
            shift
            ;;
        -cloudflared_tunnel_id)
            shift
            CLOUDFLARE_TUNNEL_ID=$1
            shift
            ;;
        -cloudflare_zone_id)
            shift
            CLOUDFLARE_ZONE_ID=$1
            shift
            ;;
        *)
            echo "$1 is not a recognized flag."
            exit 1;
            ;;
    esac
done


# Figure out the version (by release codename) of Debian we are using.
debianversion=`cat /etc/os-release | grep CODENAME | sed 's/=/\n/g' | grep -v CODENAME`

# Check all required flags are set, print a usage message if not.
#if [ -z "$SERVERNAME" ] || [ -z "$CLOUDFLARED_TOKEN" ] || [ -z "$CLOUDFLARE_API_TOKEN" ] || [ -z "$CLOUDFLARE_ACCOUNT_ID" ] || [ -z "$CLOUDFLARE_TUNNEL_ID" ] || [ -z "$CLOUDFLARE_ZONE_ID" ]; then
if [ -z "$SERVERNAME" ]; then
    echo "Usage: install.sh [-servername SERVERNAME] [-servertitle SERVERTITLE] [-sslhandler pangolin | tunnel | none]"
    echo "Optional: SERVERNAME: The full domain name of this server (e.g. webserver.example.com). Deafaults to the value provided by dnsdomainname."
    echo "Optional: SERVERTITLE: A title for the web server (e.g. \"My Company Web Server\". Defaults to \"Web Server\"." 
    echo "Optional: \"pangolin\" or \"tunnel\" as SSL Handler options. If \"tunnel\", the server will be configured assuming an SSL tunneling"
    echo "          service (Cloudflare, NGrok, etc) will be used to provide SSL ingress. If \"pangolin\", a Pangolin server will be installed"
    echo "          and set up to auto-configure SSL. If \"none\" (the default), neither option will be configured for. Defaults to Pangolin."
    exit 1;
fi

echo Installing web server \""$SERVERTITLE"\"...

# Make sure sudo (run commands as root) is installed.
if [ ! -f "/usr/bin/sudo" ]; then
    apt-get install -y sudo
fi

# Make sure Git (distributed source code control system) is installed.
if [ ! -f "/usr/bin/git" ]; then
    apt-get install -y git
fi

# Make sure Curl (used for downloading web-based resources) is installed.
if [ ! -f "/usr/bin/curl" ]; then
    apt-get install -y curl
fi

# Make sure Maven (a Java build tool, used to build a custom Guacamole plugin) is installed.
if [ ! -f "/usr/bin/maven" ]; then
    apt-get install -y maven
fi

# Make sure FUSE (for mounting file systems) is installed.
if [ ! -f "/usr/bin/fusermount" ]; then
    apt-get install -y fuse
fi

# Make sure rclone (for accessing / mounting cloud storage services such as Google Drive) is installed.
if [ ! -f "/usr/bin/rclone" ]; then
    apt-get install -y rclone
    # We also install the rclone plugin for Docker, so we can use rclone to mount file systems directly inside containers.
    sudo mkdir -p /var/lib/docker-plugins/rclone/config
    sudo mkdir -p /var/lib/docker-plugins/rclone/cache
fi

cp per-user-web-server/rclone.conf /var/lib/docker-plugins/rclone/config/rclone.conf
if [ ! -f "pangolin.json" ]; then
    echo "Missing pangolin.json - authentication credentials for rclone to connect to Google Drive. Stopping."
    exit 1
fi
cp pangolin.json /var/lib/docker-plugins/rclone/config/pangolin.json

# 18/12/2025: The Pangolin installer seems to make use of the "add-apt-repository" command. This isn't available in Debian 13 (Trixie) as the "software-properties-common" package has been removed from the distribution.
# What seems to work is installing "software-properties-common" from a .deb file (making sure its dependencies are installed first).
if [ $debianversion = "trixie" ]; then
    wget http://ftp.de.debian.org/debian/pool/main/s/software-properties/python3-software-properties_0.111-1_all.deb
    dpkg -i python3-software-properties_0.111-1_all.deb
    rm python3-software-properties_0.111-1_all.deb
    apt --fix-broken install -y
    
    wget http://ftp.de.debian.org/debian/pool/main/s/software-properties/software-properties-common_0.111-1_all.deb
    dpkg -i software-properties-common_0.111-1_all.deb
    rm software-properties-common_0.111-1_all.deb
    apt --fix-broken install -y
fi

# Make sure Go (programming language, used by Web Console) is installed.
if [ ! -f "/usr/bin/go" ]; then
    if [ $debianversion = "bookworm" ]; then
        cp per-user-web-server/debian-backports.sources /etc/apt/sources.list.d/debian-backports.sources
        apt-get update
        apt install -y -t bookworm-backports golang-go
    else
        apt-get install -y golang
    fi
fi

# Update the Ace (in-browser Javascript text editor, used by Web Console) source code (Git) folder.
if [ ! -d "ace-builds" ]; then
    git clone https://github.com/ajaxorg/ace-builds.git
fi
cd ace-builds
git pull
cd ..

# Build Web Console (runs server-side scripts with a simple user interface, also acts as a basic web server) via Git.
if [ ! -d "web-console" ]; then
    git clone https://github.com/dhicks6345789/web-console.git
fi
cd web-console
git pull
bash build.sh
cd ..

echo Copying over Webconsole config and Tasks...
cp per-user-web-server/webconsole-config.csv /etc/webconsole/config.csv
cp -r per-user-web-server/tasks/* /etc/webconsole/tasks

echo Building the Go Session Manager server.
cd per-user-web-server/sessionManager
bash build.sh
cd ..
cd ..
if [ ! -f "per-user-web-server/sessionManager/sessionManager" ]; then
    echo "Problem building the Go Session Manager server - stopping."
    exit 1
fi

echo Building the Go web server.
cd per-user-web-server/www
bash build.sh
cd ..
cd ..
if [ ! -f "per-user-web-server/www/wwwServer" ]; then
    echo "Problem building the Go web server - stopping."
    exit 1
fi


echo Building the custom Java authentication plugin for Guacamole...
rm per-user-web-server/guacAutoConnect/target/guacamole-auto-connect-1.6.0.jar
cd per-user-web-server/guacAutoConnect; mvn package; cd ..; cd ..
mkdir /etc/guacamole > /dev/null 2>&1
mkdir /etc/guacamole/extensions > /dev/null 2>&1
if [ ! -f "per-user-web-server/guacAutoConnect/target/guacamole-auto-connect-1.6.0.jar" ]; then
    echo "Problem building custom Java authentication plugin for Guacamole - stopping."
    exit 1
fi
cp per-user-web-server/guacAutoConnect/target/guacamole-auto-connect-1.6.0.jar /etc/guacamole/extensions

echo Make sure the Apache log files exist.
mkdir -p /var/log/apache2
touch /var/log/apache2/access.log
touch /var/log/apache2/error.log

echo Make sure the "www" folder for user website folders exists.
if [ ! -d "/var/www/html" ]; then
    mkdir -p /var/www/html
fi

echo Copy over the index page that gives users the interface to the "www" folder.
# To do: copy MenuPage to /var/www as "index.html", ready to be served by Go wwwServer.

# If the user has supplied a token for Cloudflare, but we aren't installing Pangolin (and, therefore, Docker) on this server, install cloudflared via apt.
if [ $INSTALL_PANGOLIN = false ]; then
    if [ ! -z "$CLOUDFLARED_TOKEN" ]; then
        # Add cloudflare gpg key.
        mkdir -p --mode=0755 /usr/share/keyrings
        curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null

        # Add Cloudflare repo to the apt repositories.
        echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main' | tee /etc/apt/sources.list.d/cloudflared.list

        # Install cloudflared.
        apt-get update && apt-get install cloudflared

        cloudflared service install $CLOUDFLARED_TOKEN
    fi
fi

# Install Pangolin (reverse proxy server that handles SSL tunneling and user authentication).
if [ $INSTALL_PANGOLIN = true ]; then
    echo Handing over to Pangolin installation script.
    if [ ! -z "$CLOUDFLARED_TOKEN" ]; then
        echo "--- Note: You have chosen to use Cloudflare for tunneling. Therefore, when asked by the Pangolin install script, you should select \"no\" when asked if you want to install Gerbil, Pangolin\'s tunneling component. ---"
    fi
    
    wget -O installer "https://github.com/fosrl/pangolin/releases/download/1.7.3/installer_linux_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && chmod +x ./installer
    ./installer

    # Install the rclone Docker plugin.
    docker plugin install rclone/docker-volume-rclone:amd64 args="-v" --alias rclone --grant-all-permissions
        
    if [ ! -z "$CLOUDFLARED_TOKEN" ]; then
        echo "Installing cloudflared and Webconsole inside Docker."

        # Stop any running Docker containers.
        # docker compose down
        docker stop $(docker ps -aq) && docker rm $(docker ps -aq)
    
        # Stop the standard Webconsole service from running - we want to use the version running inside Docker.
        systemctl stop webconsole
        systemctl disable webconsole

        echo "Building the Linux desktop Docker image - this might take a few minutes..."
        cp per-user-web-server/docker-desktop-Dockerfile .
        docker build -f docker-desktop-Dockerfile --progress=plain --tag=$DOCKERDESKTOP_DOCKER_IMAGE . 2>&1

        echo "Building our custom Docker image for the web server - this might take a few minutes..."
        sed -i "s/{{DOCKERDESKTOP_DOCKER_IMAGE}}/$DOCKERDESKTOP_DOCKER_IMAGE/g" wwwServer-Dockerfile
        cp per-user-web-server/wwwServer-Dockerfile .
        docker build -f wwwServer-Dockerfile --progress=plain --tag=$WWWSERVER_DOCKER_IMAGE . 2>&1

        echo "Building Docker image for Webconsole - this might take a few minutes..."
        cp per-user-web-server/webconsole-Dockerfile .
        docker build -f webconsole-Dockerfile --progress=plain --tag=$WEBCONSOLE_DOCKER_IMAGE . 2>&1

        # Replace the Docker Compose setup provided by the Pangolin install script, use ours with values for the Webconsole Docker image and the cloudflared token.
        cp per-user-web-server/docker-compose.yml ./docker-compose.yml
        sed -i "s/{{WWWSERVER_DOCKER_IMAGE}}/$WWWSERVER_DOCKER_IMAGE/g" docker-compose.yml
        sed -i "s/{{WEBCONSOLE_DOCKER_IMAGE}}/$WEBCONSOLE_DOCKER_IMAGE/g" docker-compose.yml
        sed -i "s/{{DOCKERDESKTOP_DOCKER_IMAGE}}/$DOCKERDESKTOP_DOCKER_IMAGE/g" docker-compose.yml
        sed -i "s/{{CLOUDFLARED_TOKEN}}/$CLOUDFLARED_TOKEN/g" docker-compose.yml

        # Start up the Docker containers.
        docker compose up -d
    fi
fi
