# Architecture

## What `civa` Does

`civa` is a local CLI that orchestrates Ansible.

It does not harden servers by itself. Instead, it:

1. collects operator input in Go
2. stages the embedded Ansible playbook and templates for the run
3. generates inventory and vars files
4. writes a Markdown execution plan
5. optionally runs `ansible-playbook`

## Repository Structure

- `main.go` — CLI entrypoint
- `internal/cli/` — command parsing, prompts, validation, artifact generation, doctor checks, and ansible execution
- `ansible/assets.go` — embedded Ansible asset loader for the Go runtime
- `ansible/playbook.yml` — main playbook
- `ansible/templates/` — Traefik and Fail2Ban templates
- `scripts/install.sh` — one-line installer target
- `scripts/uninstall.sh` — uninstall wrapper that delegates to `civa uninstall --yes` when available
- `.github/workflows/release.yml` — automated release workflow
- `.goreleaser.yaml` — release packaging configuration

## Runtime Artifacts

Each run creates a timestamped directory under `.civa/runs/`.

Artifacts include:

- `inventory.yml`
- `vars.yml`
- `plan.md`
- `ansible/playbook.yml`
- `ansible/templates/*`

These files make it easier to:

- review planned changes before apply
- re-run a deployment
- audit what inputs were used for a run

## Supported Families

- Debian-family: Debian, Ubuntu
- RHEL-compatible: RHEL, Rocky, AlmaLinux, CentOS, Oracle Linux

## Execution Modes

- `plan` — generate artifacts only
- `preview` — run Ansible in `--check --diff`
- `apply` — execute the playbook normally

## Safety Model

- local `doctor` checks validate prerequisites before remote execution
- embedded Ansible assets keep release binaries self-contained at runtime
- playbook support is explicitly gated by supported platform families
- generated plans provide an operator-readable record before or after execution
