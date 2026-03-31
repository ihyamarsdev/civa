# References and Design Notes

## Why Ansible

`civa` moved from direct shell-based hardening to Ansible because the workflow is safer and more scalable for multi-server operations.

Benefits:

- easier multi-host orchestration
- better idempotence for repeated runs
- clearer separation between local orchestration and remote execution
- reusable task selection with tags

## Current Design Notes

- the local CLI generates inventory, vars, and a plan before execution
- SSH hardening uses validated file edits and service restarts
- the deployer public key is read from the local machine and written to the target host
- Docker is installed from the official Docker repositories with family-specific handling
- Traefik files are generated but not automatically started

## Current Limitations

- only Debian-family and RHEL-compatible targets are supported
- firewall configuration uses simple command-based idempotence instead of extra Ansible collections
- preview behavior still depends on check-mode support from the underlying tasks
- DNS challenge secrets for Traefik still need to be added to the generated `.env` file on the target host

## Operational Notes

- run `civa preview` before `civa apply` when working on new server groups
- use `civa doctor` to validate local prerequisites
- review the generated `plan.md` when auditing changes or preparing a maintenance window
