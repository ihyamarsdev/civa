# Installation

## Requirements

Local machine requirements:

- `git`
- `go`
- `ansible-playbook`
- `python` or `python3`
- `tar`
- `curl` or `wget`
- a valid SSH public key to install for `civa setup`
- a valid SSH private key for `civa plan start`

If you plan to use `civa setup` against a fresh server that still needs password login, install `ssh-copy-id` on the local machine. Install `sshpass` as well if you want to pass the password non-interactively with `--ssh-password`.

`civa setup` is the bootstrap step for installing your SSH public key onto the server. `civa plan start` is key-only and expects that your public key has already been installed on the server.

## Option 1: Build From Source

```bash
git clone https://github.com/ihyamarsdev/civa.git
cd civa
go build -o bin/civa .
```

Run the binary directly:

```bash
./bin/civa help
```

Install it into your `PATH`:

```bash
sudo install -m 755 bin/civa /usr/local/bin/civa
```

## Option 2: Download a Release Binary

Download the archive that matches your platform from the GitHub releases page:

- `civa_linux_amd64.tar.gz`
- `civa_linux_arm64.tar.gz`
- `civa_darwin_amd64.tar.gz`
- `civa_darwin_arm64.tar.gz`

Extract it and install the binary:

```bash
tar -xzf civa_linux_amd64.tar.gz
sudo install -m 755 civa /usr/local/bin/civa
```

## Option 3: One-Line Installer

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/scripts/install.sh | bash
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/scripts/install.sh | CIVA_VERSION=v1.1.7 bash
```

Uninstall the installed binary:

```bash
civa uninstall
```

For scripted or non-interactive removal:

```bash
civa uninstall --yes
```

Or use the uninstall script wrapper:

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/scripts/uninstall.sh | bash
```

Both scripts honor `INSTALL_DIR`, so you can install or remove `civa` from a custom path without editing the script. The uninstall script prefers `civa uninstall --yes` when the installed binary already supports the command.

## Verify the Installation

```bash
civa help
civa version
civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'super-secret-password' --ssh-public-key ~/.ssh/id_rsa.pub
civa doctor
civa doctor fix
```

When `--ssh-password` is provided, `civa setup` runs `sshpass -e ssh-copy-id`. Without `--ssh-password`, it runs `ssh-copy-id` directly and lets that tool prompt for the password in your terminal. In both cases it uses `StrictHostKeyChecking=accept-new` on the first connection for convenience, which is still a trust-on-first-use trade-off compared with pre-verifying the host key yourself.
