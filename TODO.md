# Project Roadmap & TODO

Construct a development environment suitible for beginner, non-profesional developers, accesible via a web browser and using existing corporate / school authentication.

Schools / education establishments are an intended target market, therefore the environment should be suitible for use by school-aged pupils and interact with / utilise systems typically used by such establishments, with environment isolation between users, good support for network / internet filtering, etc.

---


## To-Do
- [ ] Implement user authentication tokens - [ ] Fix memory leak in database connection pool (Issue #42)
- [ ] Write unit tests for the payment gateway

## Backlog

### High Priority
- [ ] Upgrade React to the latest major version
- [ ] Refactor the legacy billing service

### Medium Priority
- [ ] Improve API documentation for endpoints
- [ ] Optimize image assets for faster loading times

### Low Priority / Nice-to-Have
- [ ] Add dark mode support to the UI
- [ ] Explore migrating from REST to GraphQL

---

## Done
### Version 1.0.0
- [x] /ssh endpoint for SSH-only access to the same desktop instance as the /desktop endpoint.
- [x] /desktop endpoint for web-based (Guacamole) remote desktop (VNC) access to 
- [ ] Go-based control plane to handle on-demand startup of individual, per-user desktops
