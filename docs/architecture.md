# Architecture

## What `civa` Does

`civa` is a local CLI that orchestrates Ansible.

It does not harden servers by itself. Instead, it:

1. collects operator input
2. generates inventory and vars files
3. writes a Markdown execution plan
4. optionally runs `ansible-playbook`

## Repository Structure

- `main.go` — compiled Go entrypoint that embeds and launches `scripts/civa`
- `scripts/civa` — shell-based CLI orchestration logic
- `ansible/playbook.yml` — main playbook
- `ansible/templates/` — Traefik and Fail2Ban templates
- `install.sh` — one-line installer target
- `.github/workflows/release.yml` — automated release workflow
- `.goreleaser.yaml` — release packaging configuration

## Runtime Artifacts

Each run creates a timestamped directory under `.civa/runs/`.

Artifacts include:

- `inventory.yml`
- `vars.yml`
- `plan.md`

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
- playbook support is explicitly gated by supported platform families
- generated plans provide an operator-readable record before or after execution
