# Project Roadmap & TODO

Construct a development environment suitible for beginner, non-profesional developers, accesible via a web browser and using existing corporate / school authentication mechanisms.

Schools / education establishments are an intended target market (along with small businesses and corporate departments / teams), therefore the environment should be suitible for use by school-aged pupils and interact with / utilise systems typically used by such establishments, with environment isolation between users, good support for network / internet filtering, general guardrails for the development environment, etc.

---

## To-Do
- [ ] /rclone endpoint:
  - [ ] Works to redirect web interface, but needs to be able to redirect separate port (consistant port number?) to receive OAuth data. Might need custom-compiled version of rclone (which we already have, just needs adding in). Might need rcloneGUI proxy re-working into more general-pupose proxy tool (for webconsole and user app access)
  - [ ] Might want to add auto-mount of added rclone remotes. Would mean watching central rclone.config directory for user changes, then running mount operation on trigger.
- [ ] Obsfucate usernames / email addresses when passed through via headers, to aid with data protection / safeguarding in school environments.
- [ ] rclone user area mount (at least with tested Google Drive connection) has a refresh issue. Might need a (cache size / timing?) setting tweaked. "rclone copy ..." doesn't seem to have this problem.
- [ ] /webconsole endpoint should route to individual user's environment with a running instance of WebConsole.
  - [ ] WebConsole should run in each user context, needs some example new item scripts to add new Flask project, new Go project, etc.
- [ ] Start menu - served at users.example.com, needs to be populated with icons on first row pointing at per-user endpoints. Other sections can be general items to act as a handy general start menu for users.
- [ ] Loading spinner for desktop / ssh connection - initial connection can take 30(?) seconds, needs some progress indication.
- [ ] Possibly add a separate start menu at public.example.com, constructed from the Caddy config file(?).
- [ ] Customise the Start toolbar on XFCE4 desktop to add browser, IDEs, etc.
- [ ] User instance culling / suspension to free up resources - maybe see example Go project (URL?...)
- [ ] Persistant SSH sessions? VNC is currently persistant (I think?), SSH opens a new session even if Guacamole disconnects for a few seconds.
- [ ] Shared VNC / SSH sessions?
- [ ] Make Pangolin optional - some users might want to use Cloudflare, etc, as their identity provider.
- [ ] Does audio work on remote desktop? Does it need Audiomass installed?

## Potential Additional Endpoints
- [ ] /wiki - a local, multi-user, editable wiki for internal school / company use. Wiki.js?
- [ ] /app/username/portnum - route through to a user's environment where they can be running a Go / Flask / whatever application
- [ ] /docs, maybe integrate the [Euro Office](https://github.com/Euro-Office) editors?

## Potential Sub-Projects
- [ ] An example Golang app repository, set up to produce a single executable (for multiple platforms) containing backend server, frontend HTML / Javascript user interface and OpenAPI documentation. Add suitible structure for AI assistance so someone can start staright away modifying single back-end .go file and one front-end file to create app with AI help.
- [ ] As Go project above, but for Python's Flask web framework.

---

## Done
### Version 0.1.0
- [x] Web server at /username that serves the contents of user's ~/www folder. Small custom Go application that serves static files and CGI scripts from a separate (shared) container with same base image as the individual user image, just without the GUI desktop or VNC. Usernames are matched with the base system, CGI scripts run (using sudo) as the user whos home folder they are stored in.
- [x] /ssh endpoint for web-based (Guacamole) command-line-only (SSH) access to individual user environments.
- [x] /desktop endpoint for web-based (Guacamole) remote desktop (VNC) access to individual user environments.
- [x] Go-based control plane to handle on-demand startup of individual, per-user containerised Linux environments.
- [x] Install process that starts from Pangolin's install script, installing Docker and Pangolin components, then adding more containers and services.
