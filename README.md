# Per-User Web Server
Configure a Debian Linux installation as a development and hosting environment for your users. Give each user a development environment with tools ready for them to start learning and developing useful stuff with, but with enough security and guardrails built-in to let them experiment safely. BY default, websites and applications are limited to other users, although items can be made public if wished by the administrator.

## Features
- Gives users in your organisation [web-based remote desktop](https://guacamole.apache.org/) access (an XFCE4 GUI desktop or SSH command line) to a software development environment, including (optionally) AI coding tools.
- Integrates with [Pangolin](https://github.com/fosrl/pangolin) to handle authentication, including OAuth (login-with-Google / Microsoft / etc) providers, so no separate accounts / password to setup or maintain.
- Can integrate with cloud storage (Google Drive, Microsoft OneDrive, etc), mounting each user's cloud storage area as a local file system accesible via standard desktop and command-line tools.
- User logins can be obfuscated before being passed to user-created applications, useful for schools if allowing access by parents to pupil-created applications.
- A selection of programming languages, libraries, IDEs, tools and utilities installed in a ready-to-use setup, including:
  - Python, with common libraries (Pandas, NumPy, Jinja2, OpenCV, Pillow) and IDE (Idle).
  - Go, with common libraries
  - PHP
  - Node.js
  - The Hugo static site generation tool
  - The VS Code IDE
  - Gemini AI client
- A built-in web server, able to handle basic static files and CGI scripts, for internal sites and tools. Each user has a "www" folder they can publish materials / applications to.
- Configuration and setup of components installed is modifiable, so admins can select which items get installed.
- Self-hostable - an open source project, this project can be installed in your organisation with no setup or ongoing fees.

## Installation
This project is basically an installation script that you run on a Debian Linux machine (physical or virtual) that installs a number of open-source projects and then adds some configuration and additional code to integrate those projects together.

For details on installation, see the [installation documentation](documentation/installation.md).

## Usage
After installation, you should basically have a freshly-installed Pangolin setup with some additional components and Docker containers added. You will need to walk through the initial Pangolin setup and configure a few settings before it is ready to use.

## Contributing
Please contact via Github if you are interested in contributing. Suggestions for additional web-based packages to put behind an endpoint are always welcome, as are test sites.

## License
This project is distributed under a permissive [Apache Version 2.0 license](LICENSE) - you can use this project and any modifications you make for commercial purposes, you are free to add to, change, or delete parts of the code and you can distribute the original code or your modified version.
