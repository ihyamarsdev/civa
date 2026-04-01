# Installation

## Requirements

Local machine requirements:

- `git`
- `go`
- `ansible-playbook`
- `python` or `python3`
- a valid SSH private key and matching public key

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
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/install.sh | bash
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/install.sh | CIVA_VERSION=v1.1.2 bash
```

## Verify the Installation

```bash
civa help
civa version
civa doctor --ssh-private-key ~/.ssh/id_rsa --ssh-public-key ~/.ssh/id_rsa.pub
```
