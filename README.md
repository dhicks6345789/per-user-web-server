# Per-User Web Server
Host individual pupil / employee web development projects on a school (or small business / corporate department) shared server

Configures a Debian Linux installation as a development and hosting environment for your users. Give each user a development environment with tools ready for them to start learning and developing useful stuff with, but with enough security and guardrails built-in to let them experiment safely. BY default, websites and applications are limited to other users in your organisation, although items can be made public if approved by the administrator.

## Rationale
The UK's general Information & Communications Technology ("ICT") school syllabus and GCSE (14-16 years old) / A-Level (16-18 years old) Computing exams are producing school leavers with a reasonable knowledge of basic computer science priciples and the practice of computer programming. Some of those leavers, of course, may then go on to specialist university courses to become professional software developers, data scienctists and so on. However, in the modern workplace (and, indeed, home setting), non-specialists very much have a role in tasks and projects that involve some level of software development.

This project is intended to provide both a solid foundation for an environment where school-aged learners can experiment and learn, and is designed to fit in with the kind of systems and processes used in a typical school environment, and for where those same people who, when they start a job or their own business, need an environment that provides useful tools for day-to-day use.

## Features
- Gives users in your organisation [web-based remote desktop](https://guacamole.apache.org/) access (an XFCE4 GUI desktop or SSH command line) to a software development environment, including (optionally) AI coding tools.
- Integrates with [Pangolin](https://github.com/fosrl/pangolin) to handle authentication, including OAuth (login-with-Google / Microsoft / etc) providers, so no separate accounts / password to setup or maintain.
- Integrates with cloud storage (Google Drive, Microsoft OneDrive, etc), [mounting](https://rclone.org/commands/rclone_mount/) each user's cloud storage area as a local file system accesible via standard desktop and command-line tools.
- User logins can be obfuscated before being passed to user-created applications, useful for schools if allowing access by parents to pupil-created applications.
- A selection of programming languages, libraries, IDEs, tools and utilities installed in a ready-to-use setup, including:
  - [Python](https://www.python.org/), with common libraries ([Pandas](https://pandas.pydata.org/) for data handling, [NumPy](https://numpy.org/) for scientific / mathematical computing, [Jinja2](https://pypi.org/project/Jinja2/) templates, [OpenCV](https://pypi.org/project/opencv-python/) for computer vision, [Pillow](https://pillow.readthedocs.io/en/stable/) for image handling) and IDE ([Idle](https://en.wikipedia.org/wiki/IDLE)).
  - [Go](https://go.dev/), with common libraries
  - [PHP](https://www.php.net/)
  - [Node.js](https://nodejs.org/en)
  - The [Hugo](https://gohugo.io/) static site generation tool
  - The [VS Code](https://code.visualstudio.com/) and [Thonny](https://thonny.org/) IDEs.
  - Gemini AI client
- A built-in web server, able to handle basic static files and CGI scripts, for internal sites and tools. Each user has a "www" folder they can publish materials / applications to.
- Configuration and setup of components installed is modifiable, so admins can select which items get installed.
- Self-hostable - an open source project, this project can be installed in your organisation with no setup or ongoing fees.

## Installation
Quick start: install a fresh Debian server, either a VM or physical server, install Git (not typically installed by default on Debian):

```
apt-get install -y git
```

Clone the repository:

```
git clone https://github.com/dhicks6345789/per-user-web-server.git
```

And run the installer:

```
bash per-user-web-server/install.sh -pangolin
```

This project is basically an installation script that you run on a Debian Linux machine (physical or virtual) that installs a number of open-source projects and then adds some configuration and additional code to integrate those projects together.

For moredetailed instructions, see the [installation documentation](documentation/installation.md).

## Usage
After installation, you should basically have a freshly-installed Pangolin setup with some additional components and Docker containers added. You will need to walk through the initial Pangolin setup and configure a few settings before it is ready to use.

## Contributing
Please contact via Github if you are interested in contributing. Suggestions for additional web-based packages to put behind an endpoint are always welcome, as are test sites.

This project is mainly an almagamation of various others, and as such uses quite a wide range of languages and tools. Java (with the Mavan build tool) is used to build a custom extension for Guacamole, whereas the main control application that handles the user container lifecycle is written is Go, as is the custom proxy functionality and the internal webserver. Otherwise, the project is largly a collection of config files and Bash scripts.

Code and config files specific to this project tend, so far, to be written mostly by hand, with some AI assistance to figure out some of the deeper technical aspects. There might be more agenticly-generated code added in the future, but as a general rule any additions should be reviewed and understood as part of the project as a whole by a human, we will tend not to accept large blocks of agenticly-generated code or documentation. Of course, other projects included or used by this one might be mostly / entirely written by AI. There is a basic [agents](AGENTS.md) file included is this repository.

## License
This project is distributed under a permissive [Apache Version 2.0 license](LICENSE) - you can use this project and any modifications you make for commercial purposes, you are free to add to, change, or delete parts of the code and you can distribute the original code or your modified version.
