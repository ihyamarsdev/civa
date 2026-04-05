# CIVA

`civa` stands for `CLI Interactive VPS Automation`. It is an interactive VPS automation CLI built on top of Ansible and helps operators bootstrap one or more servers with a consistent baseline: system updates, deploy user setup, SSH hardening, firewalling, Docker, and web-server preparation.

## Table of Contents

- [Overview](#overview)
- [Current Support](#current-support)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Documentation](#documentation)
- [Release and Installation Options](#release-and-installation-options)

## Overview

`civa` is a Go-native implementation of `CLI Interactive VPS Automation`: it interviews the operator for server targets and selected components, stages the embedded Ansible assets for each run, generates inventory and variables, writes a reusable Markdown execution plan, and then executes that recorded plan when requested.

## Current Support

- Target families: Debian/Ubuntu and RHEL-compatible distributions such as RHEL, Rocky, AlmaLinux, CentOS, and Oracle Linux
- Commands: `setup`, `auth cloudflare set|get|list|remove`, `tools cloudflare zones`, `secret set|list|remove`, `config`, `plan init|review|edit|list|remove`, `apply <nama-plan>`, `apply review <nama-plan>`, `apply drift <nama-plan>`, `apply rollback [nama-plan]`, `completion <shell>`, `doctor`, `uninstall`, `version`, `help`
- Runtime artifacts: `~/.civa/runs/<timestamp>/inventory.yml`, `vars.yml`, `plan.md`, and staged embedded Ansible assets

## Quick Start

Build locally:

```bash
git clone https://github.com/ihyamarsdev/civa.git
cd civa
go build -o bin/civa .
./bin/civa help
```

Install your public key on a fresh server:

```bash
./bin/civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub
```

Run an interactive plan:

```bash
./bin/civa plan init
```

Configure persisted web server settings interactively per provider:

```bash
./bin/civa config nginx init
```

Bootstrap a fresh server for key-based access:

```bash
./bin/civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_rsa.pub
```

Then generate a key-based plan:

```bash
./bin/civa plan init --ssh-private-key ~/.ssh/id_rsa
```

Check local prerequisites:

```bash
./bin/civa doctor
./bin/civa doctor fix
```

## Commands

- `civa setup` — install your local public key onto a fresh server with ssh-copy-id, optionally supplying the password via `sshpass`
- `civa tools` — run interactive external provider tools
- `civa auth cloudflare set|get|list|remove` — manage stored Cloudflare auth profiles
- `civa tools cloudflare zones` — manage Cloudflare zones using token from auth profile (`--profile`, default: `default`)
- `civa config <nginx|caddy> init [plan-name]` — initialize or update persistent web server profile (nginx/caddy), choose target hostname(s), and run separate config playbook using inventory from generated plan (latest by default)
- `civa config <nginx|caddy|all> list` — list persisted web server config profiles by provider (or all)
- `civa config <nginx|caddy> remove <plan-name>` — remove persisted web server config profile for one provider in a specific plan context
- `civa plan init` — generate inventory, vars, and a Markdown plan only after key-based access is ready
- `civa plan list` — show generated plan names under `~/.civa/runs/`
- `civa plan remove <nama-plan>` — delete a generated plan and its artifacts
- `civa plan review <nama-plan>` — display an existing generated `plan.md`
- `civa plan edit <nama-plan>` — edit an existing generated `plan.md` with your editor
- `civa apply <nama-plan>` — execute the artifacts referenced by an existing generated plan
- `civa apply review <nama-plan>` — verify an applied plan with Ansible check mode (`--check --diff`)
- `civa apply drift <nama-plan>` — run drift detection with check-mode output and local artifact snapshot comparison
- `civa apply rollback [nama-plan]` — rollback to the last successful applied plan (or a specific plan)
- `civa secret set <name> --value-file <path>` — encrypt and store runtime secrets locally under `~/.civa/secrets/` (preferred over inline `--value`)
- `civa secret list` / `civa secret remove <name>` — inspect or remove stored secret names
- `civa doctor` — validate local Go, Ansible, and Python requirements
- `civa doctor fix` — install or update missing local doctor dependencies
- `civa uninstall` — remove the installed binary
- `civa version` — print the current version
- `civa help` — show usage information

Shell completion scripts are available via `civa completion bash|zsh|fish`.

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
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/scripts/install.sh | bash
```

For the full install guide, see `docs/installation.md`.
