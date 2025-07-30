# per-user-web-server
A script to configure a Debian installation as a web server, with a sub-directory for each of a set of users.

Intended by use for situations where you want to let each of a set of users host their own web site, but still private within your organisation. Ideal for schools and similar institutions.

## What Does This Project Do?
This project provides a setup script that is intended for people who want a (hopefully) simple, one-step mechanism to set up a web server for hosting user websites, complete with user authentication with default access to only your users.

### Prerequisites

If you're using this project it's assumed you are probably a system administrator of some sort (maybe working for a school or other learning establishment, or maybe a small-scale hosting provider) wanting to set up a web server for your users. This project is not something you'll want to run on your desktop machine, you'll be wanting at least a basic, publicly-accessible server, either hosted on your own hardware or by a cloud hosting provider of some sort. As of writing (July 2025), a suitible hosted virtual machine from a public provider is available for under $5 a month.

You will also need the ability to set up a [sub-domain](https://en.wikipedia.org/wiki/Subdomain) (e.g. webserver.example.com) and point it at your server. Generally, this means having access to the DNS configuration for your domain name.

### Installed Components

This project is simply an installation script, it pulls in and sets up resources from a number of other projects. Starting with a basic Debian install, you should (hopefully) end upo with:
- A [WebConsole](https://github.com/dhicks6345789/web-console) server. This will act both as the underlying web server that hosts the users' content and as the tool that builds each user's site.
- A [Pangolin](https://github.com/fosrl/pangolin) server. This provides authentication services for your users. By default, only your users will have access to 
