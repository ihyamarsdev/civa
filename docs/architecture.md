# Architecture

## What `civa` Does

`civa` is a local CLI that orchestrates Ansible.

It does not harden servers by itself. Instead, it:

1. collects operator input in Go
2. stages the embedded Ansible entrypoint and service collections for the run
3. generates inventory and vars files
4. writes a Markdown execution plan
5. optionally runs `ansible-playbook`

## Repository Structure

- `main.go` — CLI entrypoint
- `internal/cli/` — command parsing, prompts, validation, artifact generation, doctor checks, and ansible execution
- `ansible/assets.go` — embedded Ansible asset loader for the Go runtime
- `ansible/main.yml` — main playbook entrypoint
- `ansible/roles/` — provision roles grouped in a simple flat layout (bootstrap, base hardening, and web server roles such as Traefik, Nginx, and Caddy)
- `scripts/install.sh` — one-line installer target
- `scripts/uninstall.sh` — uninstall wrapper that delegates to `civa uninstall --yes` when available
- `.github/workflows/release.yml` — automated release workflow
- `.goreleaser.yaml` — release packaging configuration

## Runtime Artifacts

Each `civa plan start` run creates a timestamped directory under `~/.civa/runs/`.

Artifacts include:

- `inventory.yml`
- `vars.yml`
- `plan.json` with structured execution metadata
- `plan.md`
- `ansible/main.yml`
- `ansible/roles/**`

These files make it easier to:

- review planned changes before apply
- re-run a deployment
- audit what inputs were used for a run

## Supported Families

- Debian-family: Debian, Ubuntu
- RHEL-compatible: RHEL, Rocky, AlmaLinux, CentOS, Oracle Linux

## Execution Modes

- `plan start` — generate reusable artifacts only
- `plan list` — enumerate generated plan names from `~/.civa/runs/`
- `plan remove <nama-plan>` — remove a generated plan directory and its artifacts
- `preview <nama-plan>` — display an existing Markdown plan
- `apply <nama-plan>` — execute the artifacts recorded by an existing plan

## Safety Model

- local `doctor` checks validate prerequisites before remote execution
- embedded Ansible assets keep release binaries self-contained at runtime
- playbook support is explicitly gated by supported platform families
- generated plans provide an operator-readable record before or after execution
- structured plan metadata keeps `apply` replay independent from Markdown formatting
- `civa setup` handles password-based key installation before planning, so generated plans now assume SSH key access
