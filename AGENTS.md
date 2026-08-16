This repository contains scripts, applications and configuration files that create a full coding and development platform designed for use by school pupils, small businesses and corporate departments.

Do a git pull to refresh the project's Git repository.

As this project is a collection / extension of others, it uses quite a wide range of languages, technologies and concepts, most of which were decided upon by the other projects rather than by this project's choice. The basic structure is provided by the Pangolin server by Fossorial, which uses Docker containers to split up the operation into a number of micro-services. Therefore, this project tends to follows this pattern, with microservices for:
  - The web server (found in the "www" folder), written in Go.
  - The custom reverse proxy server (found in the "rcloneGUI" folder), written in Go.
  - Remote Desktop, which uses the Apache Guacamole project to provide in-browser remote desktop (VNC, RDP and SSH) functionality with authentication provided by Pangolin and a custom (Java) Guacamole plug-in.

Note: As of the v1.19 (June 11, 2026) release of Pangolin, it now natively supports much the same remote desktop functionality, however only in the Pangolin Cloud and Enterprise Editions. As we might make Pangolin an optional part of the project (leaving administrators to handle authentication and tunnelling with Cloudflare Tunnels or a similar service) we wish to keep using Guacamole. We have a custom Guacamole authentication plug-in, written in Java (and using the Maven build tool), in the "guacAutoConnect" folder.

After any coding loop, do a git commit / push to save the changes back to the Git repository.
