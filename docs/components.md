# Components

`civa` can run all tasks or a selected subset.

## Component List

1. `system_update`
2. `user_management`
3. `ssh_hardening`
4. `security_firewall`
5. `system_config`
6. `dependencies`
7. `containerization`
8. `traefik`

## What Each Component Does

### `system_update`

- refresh package metadata
- upgrade installed packages to the latest available versions for the target family

### `user_management`

- create the deployer user
- place the user in `sudo` or `wheel`
- configure passwordless sudo
- install the provided SSH public key into `authorized_keys`

### `ssh_hardening`

- disable root login
- disable password authentication
- validate and restart the SSH service

### `security_firewall`

- install and configure Fail2Ban
- configure `ufw` on Debian-family hosts
- configure `firewalld` on RHEL-compatible hosts
- allow ports 22, 80, and 443

### `system_config`

- set the server timezone
- optionally set the hostname
- create and enable a 2 GB swap file when missing

### `dependencies`

- install `git`, `curl`, `wget`, `htop`, `vim`, `unzip`, `jq`, and `net-tools`

### `containerization`

- install Docker Engine from the official Docker repository
- install the Docker Compose plugin
- enable and start the Docker service
- add the deployer user to the `docker` group

### `traefik`

- create `/opt/traefik`
- create the external Docker network `proxy`
- generate `.env` and `docker-compose.yml`
- prepare Traefik v3 with ACME HTTP or DNS challenge settings

## Notes

- Task execution order remains safe and deterministic even when only some components are selected.
- `traefik` prepares files and Docker resources, but does not automatically run `docker compose up`.
