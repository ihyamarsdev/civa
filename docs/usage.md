# Usage

## Commands

- `civa apply`
- `civa plan`
- `civa preview`
- `civa doctor`
- `civa version`
- `civa help`

## Interactive Workflow

When you run `civa apply`, `civa plan`, or `civa preview` without all required flags, `civa` asks for:

- selected components
- number of servers
- server IPs or hostnames
- optional target hostnames
- SSH user, port, and authentication method
- local SSH password or private key, plus the public key to install for the deployer user
- deployer username
- timezone
- Traefik ACME email and challenge settings when Traefik is selected

Before `apply` runs, `civa` shows a summary of the selected values. Interactive `apply` asks for a final confirmation; `--non-interactive` skips that confirmation.

Component selection in interactive mode uses a Charmbracelet Huh multi-select prompt: use the Up and Down arrow keys to move, press Space to select or clear the highlighted component, then press Enter to confirm.

SSH access can use either `--ssh-auth-method key` with `--ssh-private-key`, or `--ssh-auth-method password` with `--ssh-password`. Password-based `preview` and `apply` require `sshpass` on the local machine, and password mode writes a dedicated `auth.yml` file with mode `0600` inside the run directory.

## Common Examples

Show help:

```bash
./bin/civa
```

Run an interactive apply:

```bash
./bin/civa apply
```

Generate a plan for two servers:

```bash
./bin/civa plan \
  --non-interactive \
  --server 203.0.113.10,web-01 \
  --server 203.0.113.11,api-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-private-key ~/.ssh/id_rsa \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --deployer-user deployer \
  --timezone Asia/Jakarta \
  --components all
```

Generate a plan with password-based SSH access:

```bash
./bin/civa plan \
  --non-interactive \
  --server 203.0.113.12,legacy-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-auth-method password \
  --ssh-password 'super-secret-password' \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --deployer-user deployer \
  --timezone Asia/Jakarta \
  --components all
```

Preview only a subset of components:

```bash
./bin/civa preview \
  --non-interactive \
  --server 203.0.113.10,web-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-auth-method key \
  --ssh-private-key ~/.ssh/id_rsa \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --components 2,3,4,6
```

Apply to a Rocky or AlmaLinux target:

```bash
./bin/civa apply \
  --non-interactive \
  --server 198.51.100.21,alma-edge-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-auth-method key \
  --ssh-private-key ~/.ssh/id_rsa \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --deployer-user deployer \
  --timezone Asia/Jakarta \
  --components all \
  --traefik-email admin@example.com \
  --traefik-challenge http
```
