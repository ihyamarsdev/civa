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
- Commands: `plan start|list|remove`, `preview <nama-plan>`, `apply <nama-plan>`, `completion <shell>`, `doctor`, `uninstall`, `version`, `help`
- Runtime artifacts: `.civa/runs/<timestamp>/inventory.yml`, `vars.yml`, `plan.md`, and staged embedded Ansible assets

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
./bin/civa plan start
```

Check local prerequisites:

```bash
./bin/civa doctor --ssh-private-key ~/.ssh/id_rsa --ssh-public-key ~/.ssh/id_rsa.pub
```

## Commands

- `civa plan start` — generate inventory, vars, auth artifacts, and a Markdown plan only
- `civa plan list` — show generated plan names under `.civa/runs/`
- `civa plan remove <nama-plan>` — delete a generated plan and its artifacts
- `civa preview <nama-plan>` — display an existing generated `plan.md`
- `civa apply <nama-plan>` — execute the artifacts referenced by an existing generated plan
- `civa doctor` — validate the local machine and required files
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
