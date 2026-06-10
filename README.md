# Per User Web Server
Configures a Debian installation as a development and hosting environment for a set of users. Integrates authentication handled via [Pangolin](https://github.com/fosrl/pangolin) with web-based remote desktop access via Guacamole to give each user an individual desktop environment.

Intended for situations where you want to let each of a set of users develop and host their own web site and/or applications, but still private within your organisation. Ideal for schools and similar institutions.

## What Does This Project Do?
This project is basically an installation script that you run on a Debian Linux machine (physical or virtual) that installs a number of open source projects and then adds some configuration and additional code to integrate those projects together.

When installation is complete, you should have a server that your set of users can log in to and access a (Linux-based) remote desktop environment. That remote desktop can be linked to a user's cloud storage (e.g. Google Drive), so their cloud storage appears seemlessly as part of the normal filesystem. There will also be a "www" folder accesible to the user, any files they place in there will be accesible on a web server. That web server is itself behind the authentication gateway, so you can restrict the people who can view those user websites.

## Features
- Gives users in your organisation remote desktop access to a software development environment, including AI coding tools (if wanted).
  - Users can be organised and given permissions by group, so a smaller group of users could be assigned access to the development environment, while a larger group could be app-usage-only users, able to use the resources provided by the first group - handy for corporate internal applications.
- Remote access is web-based, available from pretty much any device able to run a web browser, no client-side software installation needed. Users can access a Linux-based GUI desktop environment or an SSH terminal.
- User logins can utilise standard OAuth (login-with-Google / Microsoft / etc) providers, so no separate accounts / password to setup or maintain.
  - User logins can be obfuscated before being passed to user-created applications, useful for schools if allowing access by parents to pupil-created applications.
- A selection of programming languages, libraries, IDEs, tools and utilities installed in a ready-to-use setup, including:
  - Python, with common libraries (Pandas, NumPy, Jinja2, OpenCV, Pillow).
  - Go, with common libraries
  - PHP
  - The Hugo static site generation tool.
- A built-in web server, able to handle basic static files and CGI scripts, for internal sites and tools.
- Configuration and setup of components installed is modifiable, so admins can select which items get installed.
- Self-hostable - an open source project, this project can be installed in your organisation with no ongoing fees.

## Installation
For details on installation, see the [installation documentation](documentation/installation.md).
