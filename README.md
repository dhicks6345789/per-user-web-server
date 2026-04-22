# per-user-web-server
Configures a Debian installation as a development and hosting environment for a set of users. Integrates authentication handled via [Pangolin](https://github.com/fosrl/pangolin) with web-based remote desktop access via Guacamole to give each user an individual desktop environment.

Intended for situations where you want to let each of a set of users develop and host their own web site and/or applications, but still private within your organisation. Ideal for schools and similar institutions.

**Note: as of 26th February 2026, still a work-in-progress. Installing should give you a working multi-user remote desktop environment complete with development tools, but some further work is still required.**

## What Does This Project Do?
This project is basically an installation script that you run on a Debian Linux machine (physical or virtual) that installs a number of open source projects and then adds some configuration and additional code to integrate those projects together.

When installation is complete, you should have a server that your set of users can log in to and access a (Linux-based) remote desktop environment. That remote desktop can be linked to a user's cloud storage (e.g. Google Drive), so their storage appears seemlessly as part of the normal filesystem. There will also be a "www" folder accesible to the user, any files they place in there will be accesible on a web server. That web server is itself behind the authentication gateway, so you can restrict the people who can view those user websites.

### Prerequisites
If you're using this project it's assumed you are probably a system administrator of some sort (maybe working for a school or other learning establishment, or maybe a small-scale hosting provider) wanting to set up a web server / development environment for your users. This project is not something you'll want to run on your desktop machine, you'll be wanting at least a basic, publicly-accessible server, either hosted on your own hardware or by a cloud hosting provider of some sort. As of writing (February 2026), a suitible hosted virtual machine from a public provider is available for under $5 a month, possibly even for free.

#### Linux Distribution
This project has been tested on a Debian 13 "Trixie" server (August 2025) running on (virtual) AMD64 hardware. Other versions of Debian (the previous version 12, "Bookworm", in particular) will probably work okay, as would similar versions of Ubuntu. Other Linux distributions shouldn't be that difficult to adjust for if needed, as should the ARM version of Debian (for the Raspberry Pi and similar hardware - both WebConsole and Pangolin have binaries available for ARM hardware). Adjusting this project directly for a Windows (or MacOS) install might not be possible as Pangolin seems to be a Linux-only project, but using a different tunneling / authentication provider such as [Cloudflare Zero-Trust Tunnels](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) should work.

The custom authentication / connection plugin written as part of this project for Guacamole simply returns details of a VNC remote desktop connection, so additional remote desktop connections to other devices in your organisation should be possible to add if wanted. This could include things like RDP connections to a Windows Remote Desktop Services session, or an RDP connection to an individual Windows desktop machine. You could also add VNC connections to indiviual Raspberry Pi (or similar) devices, so for instance an educational institution could assign a Raspberry Pi per pupil and have them connect (using their standard school login) via web-based remote desktop, as a handy way to handle the logistics of getting a whole class logged in to devices, and also to allow for home access.

#### VM Resources
You should probably run this script on a virtual machine (VM) dedicated to running this service, something that is backed up / able to be restored to a known-working checkpoint. Running this script on an existing machine with other services installed might give unpredictable results, although if you know what you're doing it is just a (hopefully well-commented) Bash script and you can check through it to see exactly what is being done and how that might intefere with your current setup.

The VM used for this server can probably be quite small, at least initially. A suggested reasonable minimum might be 4 GB of RAM and 40 GB of storage, if you're using a hosting provider like AWS then their smallest available VM might be okay - for something like AWS, you might be able to manage with the resources available in the free pricing tier. Exactly how much RAM / CPU / storage you are going to need will depend on how many users you have and how much they'll be using the server, but you can probably set up a minimal test server initially and test it as a single user, then add more RAM / CPU / storage to it later.

#### Domain / DNS
You will need the ability to set up [sub-domains](https://en.wikipedia.org/wiki/Subdomain) to point it at your server. Generally, this means having access to the DNS configuration for your domain name. You will probably want two sub-domains - one for the Pangolin server (e.g. pangolin.example.com) and one for the web server (e.g. webserver.example.com). Both sub-domains can point at the same physical (or virtual) server, or they can be separate servers if you want.

You can set this web server up as your domain's default web server, i.e. at "www.example.com". This project uses its own simple Go-based webserver to serve files and CGI scripts that hasn't been tested with a high-traffic environment yet, so it might not be ideal for a large public site.

#### Tunneling
The Pangolin server handles authentication and routing of reqeusts to the correct handler. It can also handle tunneling connections, so the Pangolin server can be on a publically-accesible server whilest the web server can be behind a firewall. Alternativly, you can used your preffered tunneling solution (e.g. Cloudflare Tunnels) to make netwok services available, with Pangolin being used just for authentication and routing.

### Users
You will need some way of getting a list of users in CSV format onto the server. That can be a one-off operation, manually edited to add / remove users, but some way of getting a list of users from your system updated at least daily would probably be best. If you are in a school, this list will probably be from your school's Management Information System (MIS) or from Google Workspace / Microsoft 365. Some example scripts are included to help with that process.

### Components Installed
This project is mostly just an installation script, along with some template config files and some helper scripts, it pulls in and sets up resources from a number of other projects. Starting with a basic Debian install, you should (hopefully) end up with:
- A [Docker](https://www.docker.com) installation. Pangolin, in particular, is designed to work as a number of components inside separate Docker containers, so the rest of the services tend to follow.
- A [Pangolin](https://github.com/fosrl/pangolin) server. This handles secure HTTPS connections to the server and provides authentication services for your users. By default, your users (and only your users) will have read access to any of the sites hosted by this server - each user can only change their own site, they can view everyone else's, but the general public cannot see any of the sites. This is intended to be ideal for schools and similar places to give pupils a mechanism to have their own website but keep control of how the content they produce is accessed. You can, of course, modify the default configuration after installation to make certain sites public, if you wish, or further limit access.
- A [WebConsole](https://github.com/dhicks6345789/web-console) server. This will act as the interface to the tools that build and manage the system.
- A web server. It proved to be simpler to simply write a basic web server in Go to fit the structure of the project rather than try and use an existing web server. Go has a very capable built-in web server library, so rather than writing a web server "from scratch" this is more like adding a bit of project-specific logic to an already existing product. This web server serves basic HTML / Javascript / CSS / etc files. It can also serve CGI scripts, so if wanted you users can build applications using CGI.
- The [Hugo](https://gohugo.io/) static site generation tool. Users can, if they want, use the Hugo directory layout for website templating and so on.
- [Docs To Markdown](https://github.com/dhicks6345789/docs-to-markdown/tree/WebconsoleUpdate). A wrapper built around larger tools like [Pandoc](https://pandoc.org/) and [ffmpeg](https://ffmpeg.org/) to convert files in various formats (Word / Google Docs, Excel / Google Sheets, etc, video and audio files) to formats (such as Markdown) and structures usable as input for static site generations tools (like Hugo), letting your users build and edit websites by simply editing content in Google Workspace / Microsoft 365 cloud-based file systems.
- The [Rclone](https://rclone.org/) file sync tool. Each user site can be built from files hosted on your choice of cloud storage system, handy for schools with users on Google Workspace / Microsoft 365 - each user simply has a "website" folder in their home storage area where they can put / edit HTML / CSS / Javascript files, or Markdown files and Hugo templates, and that folder is published to the web server.

## Installation
The instllation is split into two parts, the Pangolin setup and the web server setup. This is so the two operations can be carried out on separate servers, although running both on a single server is also fine.

### Option 1 - One server running WebConsole, Pangolin and Cloudflare Tunnels Inside Docker.
This will give you a setup running on a single server. It extends the Pangolin Docker-based setup to include Webconsole and Cloudflare's tunnel client, with all the components running in one Docker project.

In Cloudflare's control panel, you will need to create a tunnel - from the main Control Panel, select "Zero Trust" from the left-hand menu, then "Networks", then "Tunnels".

You will need to assign two public hostnames to that one tunnel. One for Pangolin ("pangolin.example.com") and one for Web Console ("website.example.com"). In the "service" settings for each of the public hostnames, the service type / URL is simply "HTTPS" and "traefik". Under "Additional application settings" -> "TLS", turn on the "No TLS Verify" option. This skips checking the (self-signed) HTTPS certificate provided to the Cloudflare tunneling client by the Pangolin server (which is all internal traffic inside the VM itself, so shouldn't be a problem).

The install script will take care of installing the Cloudflare client, you just need to provide the "cloudflared_token" value to the installer. This is the token found in the "Overview" tab in the edit section for your tunnel. You just want the long (184 characters) value at the end of the "Run the following command" section.

From the command line on your server, run the following:
```
git clone https://github.com/dhicks6345789/per-user-web-server.git
bash per-user-web-server/install.sh -pangolin -cloudflared_token TOKEN_GOES_HERE
```
Note: the (very simple) Dockerfile used to compile the Webconsole container currently uses "python:3.12-slim-bookworm" as a base, which provides a Python environment. If you want Webconsole to be able to run things other that Python you will need to install them into the "Webconsole" container somehow, either by changing / adding to the existing Dockerfile, or maybe by adding components via apt when the container is running. If you do the latter, bear in mind your additions will disapear every time you shut down / restart the container.

Note: the docker-compose.yml provided in this project will replace the default one provided by the Pangolin install script. It's similar to the Pangolin version, but includes Cloudflared and Webconsole. You might also notice it doesn't expose any network ports (ports 80 or 443) as the default Pangolin setup does, so this Pangolin instance won't be available on your internal network, only at the end of the Cloudflare tunnel that also runs inside Docker.

After running the above, you should be able to access the Pangolin interface on your sub-domain (pangolin.example.com). After setting up the initial admin user credentials, Organisation and Site, go to the "Resources" section, add a Resource, give your new resource a name, select the (default) "HTTPS Resource" option, and enter your second sub-domain (website.example.com). When asked to set up a Target, set the "Method" as "HTTP", "IP / Hostname" as "webconsole" and "Port" as "8090". After hitting "Save All Settings" you should, hopefully, be able to access the Webconsole server via your subdomain.
