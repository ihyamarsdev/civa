# civa

`civa` is an interactive Ansible-based CLI for bootstrapping one or more servers with a consistent baseline: system updates, deploy user setup, SSH hardening, firewalling, Docker, and Traefik preparation.

## Table of Contents

- [Overview](#overview)
- [Current Support](#current-support)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Documentation](#documentation)
- [Release and Installation Options](#release-and-installation-options)

## Overview

`civa` interviews the operator for server targets and selected components, generates Ansible inventory and variables, writes a Markdown execution plan, and then optionally runs `ansible-playbook`.

## Current Support

- Target families: Debian/Ubuntu and RHEL-compatible distributions such as RHEL, Rocky, AlmaLinux, CentOS, and Oracle Linux
- Commands: `apply`, `plan`, `preview`, `doctor`, `version`, `help`
- Runtime artifacts: `.civa/runs/<timestamp>/inventory.yml`, `vars.yml`, and `plan.md`

## Quick Start

Build locally:

```bash
git clone https://github.com/ihyamarsdev/civa.git
cd civa
go build -o bin/civa .
./bin/civa help
```

Run an interactive plan:

```bash
./bin/civa plan
```

Check local prerequisites:

```bash
./bin/civa doctor --ssh-private-key ~/.ssh/id_rsa --ssh-public-key ~/.ssh/id_rsa.pub
```

## Commands

- `civa apply` — generate artifacts and execute the playbook against the selected servers
- `civa plan` — generate inventory, vars, and a Markdown plan only
- `civa preview` — run the playbook in `--check --diff` mode
- `civa doctor` — validate the local machine and required files
- `civa version` — print the current version
- `civa help` — show usage information

Running `civa` without arguments shows help.

## Documentation

- `docs/installation.md` — installation methods and prerequisites
- `docs/usage.md` — command reference, interactive flow, and examples
- `docs/components.md` — component list and what each Ansible tag does
- `docs/architecture.md` — runtime artifacts, repository structure, and workflow design
- `docs/references.md` — implementation notes, limitations, and design rationale

## Release and Installation Options

- Build from source
- Download a release archive
- Use the one-line installer:

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/install.sh | bash
```

For the full install guide, see `docs/installation.md`.
