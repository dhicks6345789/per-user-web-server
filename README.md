# per-user-web-server
Configures a Debian installation as a development and hosting environment for a set of users. Integrates authentication handled via [Pangolin](https://github.com/fosrl/pangolin) with web-based remote desktop access via Guacamole to give each user an individual desktop environment.

Intended for situations where you want to let each of a set of users develop and host their own web site and/or applications, but still private within your organisation. Ideal for schools and similar institutions.

## What Does This Project Do?
This project is basically an installation script that you run on a Debian Linux machine (physical or virtual) that installs a number of open source projects and then adds some configuration and additional code to integrate those projects together.

When installation is complete, you should have a server that your set of users can log in to and access a (Linux-based) remote desktop environment. That remote desktop can be linked to a user's cloud storage (e.g. Google Drive), so their storage appears seemlessly as part of the normal filesystem. There will also be a "www" folder accesible to the user, any files they place in there will be accesible on a web server. That web server is itself behind the authentication gateway, so you can restrict the people who can view those user websites.

For more details on installation, see the [installation documentation](documentation/installation.md).
