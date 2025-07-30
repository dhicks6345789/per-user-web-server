# per-user-web-server
A script to configure a Debian installation as a web server, with a sub-directory for each of a set of users.

Intended by use for situations where you want to let each of a set of users host their own web site, but still private within your organisation. Ideal for schools and similar institutions.

## What Does This Project Do?
This project provides a setup script that is intended for people who want a (hopefully) simple, one-step mechanism to set up a web server for hosting user websites, complete with user authentication with default access to only your users.

### Prerequisites

If you're using this project it's assumed you are probably a system administrator of some sort (maybe working for a school or other learning establishment, or maybe a small-scale hosting provider) wanting to set up a web server for your users. This project is not something you'll want to run on your desktop machine, you'll be wanting at least a basic, publicly-accessible server, either hosted on your own hardware or by a cloud hosting provider of some sort. As of writing (July 2025), a suitible hosted virtual machine from a public provider is available for under $5 a month.

You will also need the ability to set up a [sub-domain](https://en.wikipedia.org/wiki/Subdomain) (e.g. webserver.example.com) and point it at your server. Generally, this means having access to the DNS configuration for your domain name.

### Installed Components

This project is simply an installation script, it pulls in and sets up resources from a number of other projects. Starting with a basic Debian install, you should (hopefully) end up with:
- A [WebConsole](https://github.com/dhicks6345789/web-console) server. This will act both as the underlying web server that hosts the users' content and as the tool that builds each user's site.
- A [Pangolin](https://github.com/fosrl/pangolin) server. This handles secure HTTPS connections to the server and provides authentication services for your users. By default, your users (and only your users) will have read access to any of the sites hosted by this server - each user can only change their own site, they can view everyone else's, but the general public cannot see any of the sites. This should be ideal for schools and similar places where you can give pupils a mechanism to have their own website but keep control of how the content they produce is accessed. You can, of course, modify the default configuration after installation to make certain sites public, if you wish, or further limit access.
- The [Hugo](https://gohugo.io/) static site generation tool. Users can, if they want, use the Hugo directory layout for website templating and so on.
- The [Rclone](https://rclone.org/) file sync tool. Each user site can be built from files hosted on your choice of cloud storage system, handy for schools with users on Google Workspace / Micropsoft 365 - each user simply has a "website" folder in their home storage area where they can put / edit HTML / CSSD / Javascript files, or Markdown files and Hugo templates, and that folder is published to the web server.
