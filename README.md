# per-user-web-server
A script to configure a Debian installation as a web server, with a sub-directory for each of a set of users.

Intended by use for situations where you want to let each of a set of users host their own web site, or basic "intranet", but still private within your organisation. Ideal for schools and similar institutions.

## What Does This Project Do?
This project provides a setup script that is intended for people who want a (hopefully) simple, one-step mechanism to set up a web server for hosting user websites, complete with user authentication with default access to only your users.

### Prerequisites

If you're using this project it's assumed you are probably a system administrator of some sort (maybe working for a school or other learning establishment, or maybe a small-scale hosting provider) wanting to set up a web server for your users. This project is not something you'll want to run on your desktop machine, you'll be wanting at least a basic, publicly-accessible server, either hosted on your own hardware or by a cloud hosting provider of some sort. As of writing (July 2025), a suitible hosted virtual machine from a public provider is available for under $5 a month, possibly even for free.

#### Linux Distribution

This project has been tested on a Debian 13 "Trixie" server (August 2025) running on (virtual) AMD64 hardware. Other versions of Debian (the previous version 12, "Bookworm", in particular) will probably work okay, as would similar versions of Ubuntu. Other Linux distributions shouldn't be that difficult to adjust for if needed, as should the ARM version of Debian (for the Raspberry Pi and similar hardware - both WebConsole and Pangolin have binaries available for ARM hardware). Adjusting this project directly for a Windows (or MacOS) install might not be possible as Pangolin seems to be a Linux-only project, but using a different tunneling / authentication provider such as [Cloudflare Zero-Trust Tunnels](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) along with the WebConsole server should work.

#### VM Resources

You should probably run this script on a virtual machine (VM) dedicated to running this service, something that is backed up / able to be restored to a known-working checkpoint. Running this script on an existing machine with other services installed might give unpredictable results, although if you know what you're doing it is just a (hopefully well-commented) Bash script and you can check through it to see exactly what is being done and how that might intefere with your current setup.

The VM used for this server can probably be quite small, at least initially. The suggested reasonable minimum for Debian Trixie is 2 GB of RAM and 40 GB of storage, if you're using a hosting provider like AWS then their smallest available VM should be okay - for something like AWS, you might be able to manage with the resources available in the free pricing tier. Exactly how much RAM / CPU / storage you are going to need will depend on how many users you have and how much they'll be using the server, but you can probably set up a minimal test server initially and add more RAM / CPU / storage to it later.

#### Domain / DNS

You will need the ability to set up a [sub-domain](https://en.wikipedia.org/wiki/Subdomain) (e.g. webserver.example.com) and point it at your server. Generally, this means having access to the DNS configuration for your domain name.

In theory, you could set this server up as your domain's default web server, i.e. at "www.example.com". However, this would give you a public website showing the main WebConsole menu page, which probably isn't want you want for your public-facing website. You might do better to install a standard web server alongside this setup and have that serve any public-facing web pages.

Possible todo: add an option to install an instance of [Caddy](https://caddyserver.com/) to handle this.

### Users

You will need some way of getting a list of users in CSV format onto the server. That can be a one-off operation, manually edited to add / remove users, but some way of getting a list of users from your system updated at least daily would probably be best. If you are in a school, this list will probably be from your school's Management Information System (MIS) or from Google Workspace / Microsoft 365. Some example scripts are included to help with that process.

If your school / workplace that you are setting up for uses Google Workspace / Microsoft 365 and you want to have users be able to directly add files to a "website" sub-folder in their home folder and have it published, then you will need super-admin access for your Workspace / 365 instance. Another option is that users can individually create a public-to-all-domain-users folder and choose to have that published as a site.

### Components Installed

This project is mostly just an installation script, along with some template config files and some helper scripts, it pulls in and sets up resources from a number of other projects. Starting with a basic Debian install, you should (hopefully) end up with:
- A [WebConsole](https://github.com/dhicks6345789/web-console) server. This will act both as the underlying web server that hosts the users' content and as the tool that builds each user's site.
- A [Pangolin](https://github.com/fosrl/pangolin) server. This handles secure HTTPS connections to the server and provides authentication services for your users. By default, your users (and only your users) will have read access to any of the sites hosted by this server - each user can only change their own site, they can view everyone else's, but the general public cannot see any of the sites. This is intended to be ideal for schools and similar places to give pupils a mechanism to have their own website but keep control of how the content they produce is accessed. You can, of course, modify the default configuration after installation to make certain sites public, if you wish, or further limit access.
- The [Hugo](https://gohugo.io/) static site generation tool. Users can, if they want, use the Hugo directory layout for website templating and so on.
- [Docs To Markdown](https://github.com/dhicks6345789/docs-to-markdown/tree/WebconsoleUpdate). A wrapper built around larger tools like [Pandoc](https://pandoc.org/) and [ffmpeg](https://ffmpeg.org/) to convert files in various formats (Word / Google Docs, Excel / Google Sheets, etc, video and audio files) to formats (such as Markdown) and structures usable as input for static site generations tools (like Hugo), letting your users build and edit websites by simply editing content in Google Workspace / Microsoft 365 cloud-based file systems.
- The [Rclone](https://rclone.org/) file sync tool. Each user site can be built from files hosted on your choice of cloud storage system, handy for schools with users on Google Workspace / Micropsoft 365 - each user simply has a "website" folder in their home storage area where they can put / edit HTML / CSSD / Javascript files, or Markdown files and Hugo templates, and that folder is published to the web server.
