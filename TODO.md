# Project Roadmap & TODO

Construct a development environment suitible for beginner, non-profesional developers, accesible via a web browser and using existing corporate / school authentication.

Schools / education establishments are an intended target market, therefore the environment should be suitible for use by school-aged pupils and interact with / utilise systems typically used by such establishments, with environment isolation between users, good support for network / internet filtering, etc.

---

## To-Do
- [ ] User instance culling / suspension to free up resources - maybe see example Go project (URL?...)
- [ ] Persistant SSH session? VNC is currently persistant (I think?), SSH opens a new session even if Guacamole disconnects for a few seconds.
- [ ] Shared VNC / SSH sessions?
- [ ] Does audio work on remote desktop? Does it need Audiomass installed?

## Potential Additional Endpoints
- [ ] /wiki - a local, multi-user, editable wiki for internal school / company use. Wiki.js?

---

## Done
### Version 0.1.0
- [x] /ssh endpoint for web-based (Guacamole) command-line-only (SSH) access to individual user environments.
- [x] /desktop endpoint for web-based (Guacamole) remote desktop (VNC) access to individual user environments.
- [x] Go-based control plane to handle on-demand startup of individual, per-user containerised Linux environments.
