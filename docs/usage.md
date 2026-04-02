# Usage

## Commands

- `civa plan start`
- `civa plan list`
- `civa plan remove <nama-plan>`
- `civa preview <nama-plan>`
- `civa apply <nama-plan>`
- `civa setup`
- `civa completion <shell>`
- `civa doctor`
- `civa uninstall`
- `civa version`
- `civa help`

## Interactive Workflow

When you run `civa plan start` without all required flags, `civa` asks for:

- selected components
- number of servers
- server IPs or hostnames
- optional target hostnames
- SSH user and port
- local SSH private key path
- deployer username
- timezone
- web server choice (`none`, `traefik`, `nginx`, or `caddy`) when the web server component is enabled
- Traefik ACME email and challenge settings when Traefik is selected

`civa preview <nama-plan>` shows an existing `plan.md` rendered with Glow-style terminal formatting when stdout is a TTY. When redirected or piped, it falls back to plain non-TTY formatting. `civa apply <nama-plan>` only executes an existing plan and asks for a final confirmation unless you pass `--yes`.

Generated plan names come from the run directory under `.civa/runs/`, for example `20260401-152334-210329559`. Use `civa plan list` to see available names. `--plan-file` remains available as a manual override, but the normal flow is name-based.

Component selection in interactive mode uses a Charmbracelet Huh multi-select prompt: use the Up and Down arrow keys to move, press Space to select or clear the highlighted component, then press Enter to confirm.

`civa plan start` assumes SSH key access and only needs the local SSH private key path. The matching public key path is derived automatically unless you override it explicitly.

Use `civa setup` to install your local public key onto a fresh VPS with its built-in user and password. The setup command uses `sshpass -e ssh-copy-id`, so `ssh-copy-id` and `sshpass` must be available locally. For first contact it uses `StrictHostKeyChecking=accept-new`, which is convenient but still a trust-on-first-use trade-off.

Use `--web-server none|traefik|nginx|caddy` to choose which web server to prepare. The default web server remains `traefik` when the `web_server` component is selected and no explicit choice is provided.

## Common Examples

Show help:

```bash
./bin/civa
```

Run an interactive plan:

```bash
./bin/civa plan start
```

Generate a plan for two servers:

```bash
./bin/civa plan start \
  --non-interactive \
  --server 203.0.113.10,web-01 \
  --server 203.0.113.11,api-01 \
  --ssh-user root \
  --ssh-port 22 \
  --web-server nginx \
  --ssh-private-key ~/.ssh/id_rsa \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --deployer-user deployer \
  --timezone Asia/Jakarta \
  --components all
```

Install your public key on a fresh server before planning:

```bash
./bin/civa setup \
  --server 203.0.113.12 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-password 'super-secret-password' \
  --ssh-public-key ~/.ssh/id_rsa.pub
```

List generated plans:

```bash
./bin/civa plan list
```

Preview an existing plan:

```bash
./bin/civa preview 20260401-152334-210329559
```

Apply an existing plan to a Rocky or AlmaLinux target:

```bash
./bin/civa apply 20260401-152334-210329559 --yes
```

Remove a generated plan:

```bash
./bin/civa plan remove 20260401-152334-210329559 --yes
```

Print a completion script for Bash:

```bash
./bin/civa completion bash
```

Activate shell completion examples:

```bash
# Bash
./bin/civa completion bash > ~/.local/share/bash-completion/completions/civa

# Zsh
./bin/civa completion zsh > ~/.zfunc/_civa

# Fish
./bin/civa completion fish > ~/.config/fish/completions/civa.fish
```
