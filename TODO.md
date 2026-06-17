# Project Roadmap & TODO

Construct a development environment suitible for beginner, non-profesional developers, accesible via a web browser and using existing corporate / school authentication.

Schools / education establishments are an intended target market, therefore the environment should be suitible for use by school-aged pupils and interact with / utilise systems typically used by such establishments, with environment isolation between users, good support for network / internet filtering, etc.

---

## To-Do
- [ ] User instance culling / suspension to free up resources.

---

## Done
### Version 1.0.0
- [x] /ssh endpoint for web-based (Guacamole) command-line-only (SSH) access to individual user environments.
- [x] /desktop endpoint for web-based (Guacamole) remote desktop (VNC) access to individual user environments.
- [x] Go-based control plane to handle on-demand startup of individual, per-user containerised Linux environments.
