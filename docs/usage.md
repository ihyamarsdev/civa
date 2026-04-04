# Usage

## Commands

- `civa plan start`
- `civa config [plan-name]`
- `civa config edit [plan-name]`
- `civa config list`
- `civa config remove [nginx|caddy|all]`
- `civa plan list`
- `civa plan remove <nama-plan>`
- `civa preview <nama-plan>`
- `civa apply <nama-plan>`
- `civa apply review <nama-plan>`
- `civa setup`
- `civa completion <shell>`
- `civa doctor`
- `civa doctor fix`
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

`civa preview <nama-plan>` shows an existing `plan.md` rendered with Glow-style terminal formatting when stdout is a TTY. When redirected or piped, it falls back to plain non-TTY formatting. `civa apply <nama-plan>` only executes an existing plan and asks for a final confirmation unless you pass `--yes`. `civa apply review <nama-plan>` runs the same plan artifacts in Ansible check mode (`--check --diff`) to verify post-installation convergence.

Generated plan names now use your primary hostname as the base (for example `web-01`). If you generate again with the same hostname, civa automatically creates versioned names such as `web-01-v2`, `web-01-v3`, and so on. Use `civa plan list web-01` (or `civa plan web-01 list`) to see all versions.

Component selection in interactive mode uses a Charmbracelet Huh multi-select prompt: use the Up and Down arrow keys to move, press Space to select or clear the highlighted component, then press Enter to confirm.

`civa plan start` assumes SSH key access and only needs the local SSH private key path. The matching public key path is derived automatically unless you override it explicitly.

Use `civa setup` to install your local public key onto a fresh VPS with its built-in user account. Before running SSH key installation, `civa setup` checks required local dependencies and auto-installs missing packages using your OS package manager (`apt-get`, `dnf`, or `yum`) with `sudo`. If you pass `--ssh-password`, setup uses `sshpass -e ssh-copy-id`; otherwise it runs `ssh-copy-id` directly and lets that tool prompt for the password in your terminal. Before it connects, `civa setup` rewrites only the matching host entry in `~/.ssh/known_hosts` so stale host keys for the target host do not block the first login while other hosts stay untouched. For first contact it uses `StrictHostKeyChecking=accept-new`, which is convenient but still a trust-on-first-use trade-off.

Use `--web-server none|traefik|nginx|caddy` to choose which web server to prepare. The default web server remains `traefik` when the `web_server` component is selected and no explicit choice is provided.

Use `civa config [plan-name]` (or `civa config edit [plan-name]`) to store web server runtime configuration interactively and immediately run a separate config playbook using inventory from an existing generated plan (`civa plan start`). If `plan-name` is omitted, civa uses the latest generated plan.

Use `civa config list` to inspect persisted web server profiles, and `civa config remove [nginx|caddy|all]` to remove one or all persisted profiles.

For now this covers Nginx/Caddy reverse-proxy site definitions, target hostname(s) where the web server should be installed, and for Nginx you can enable HTTPS per-domain with Certbot.

## Common Examples

Show help:

```bash
./bin/civa
```

Run an interactive plan:

```bash
./bin/civa plan start
```

Configure persisted web server profiles (interactive):

```bash
./bin/civa config
```

List persisted config profiles:

```bash
./bin/civa config list
```

Remove persisted nginx profile:

```bash
./bin/civa config remove nginx
```

Apply web server config using a specific generated plan inventory:

```bash
./bin/civa config web-01-v2
```

Generate a plan for two servers:

```bash
./bin/civa plan start \
  --non-interactive \
  --server 203.0.113.10,web-01,2201 \
  --server 203.0.113.11,api-01,2202 \
  --ssh-user root \
  --ssh-port 22 \
  --web-server nginx \
  --ssh-private-key ~/.ssh/id_rsa \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --deployer-user deployer \
  --timezone Asia/Jakarta \
  --components all
```

`--server` now supports `addr[,hostname][,port]`. If `port` is omitted, `civa plan start` falls back to `--ssh-port` and defaults to `22`.

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

List all versions for one hostname-based plan:

```bash
./bin/civa plan list web-01
# or
./bin/civa plan web-01 list
```

Preview an existing plan (plan names now follow your primary hostname):

```bash
./bin/civa preview web-01
```

Apply an existing plan to a Rocky or AlmaLinux target:

```bash
./bin/civa apply web-01 --yes
```

Remove a generated plan:

```bash
./bin/civa plan remove web-01 --yes
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
