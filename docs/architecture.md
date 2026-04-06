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

Each `civa plan init` run creates a timestamped directory under `~/.civa/runs/`.

Artifacts include:

- `inventory.yml`
- `vars.yml`
- `plan.json` with structured execution metadata
- `plan.md`
- `ansible/main.yml`
- `ansible/roles/**`

Additional runtime state directories:

- `~/.civa/secrets/` (`store.json`, `key.bin`) for encrypted local secret management
- `~/.civa/drift/` for per-plan drift snapshots
- `~/.civa/rollback/state.json` for last successful/failed rollback metadata

These files make it easier to:

- review planned changes before apply
- re-run a deployment
- audit what inputs were used for a run

## Supported Families

- Debian-family: Debian, Ubuntu
- RHEL-compatible: RHEL, Rocky, AlmaLinux, CentOS, Oracle Linux

## Execution Modes

- `start` — open beginner wizard and route to guided `setup` or `plan init`
- `plan init` — generate reusable artifacts only
- `plan review <nama-plan>` — render an existing Markdown plan
- `plan edit <nama-plan>` — edit an existing Markdown plan in your editor
- `plan list` — enumerate generated plan names from `~/.civa/runs/`
- `plan remove <nama-plan>` — remove a generated plan directory and its artifacts
- `apply <nama-plan>` — execute the artifacts recorded by an existing plan
- `apply review <nama-plan>` — verify with `--check --diff` without changing server state
- `apply drift <nama-plan>` — detect drift from check-mode recap plus local artifact snapshot comparison
- `apply rollback [nama-plan]` — run rollback preflight and apply from last successful plan (or explicit target)

## Beginner Mode

`civa start` is an explicit beginner entrypoint (wizard mode). It keeps the execution model unchanged by mapping selections back into the same request/runner paths used by direct commands:

- setup path -> `civa setup`
- planning path -> `civa plan init`

So wizard mode improves discoverability and defaults without creating a separate automation engine. For full operator-facing walkthrough, see `beginner-mode.md`.

## Safety Model

- local `doctor` checks validate prerequisites before remote execution
- embedded Ansible assets keep release binaries self-contained at runtime
- playbook support is explicitly gated by supported platform families
- generated plans provide an operator-readable record before or after execution
- structured plan metadata keeps `apply` replay independent from Markdown formatting
- `civa setup` handles password-based key installation before planning, so generated plans now assume SSH key access
