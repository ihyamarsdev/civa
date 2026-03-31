# civa

`civa` adalah CLI interaktif berbasis Ansible untuk bootstrap banyak server sekaligus. Tool ini mewawancarai operator tentang target server, komponen yang ingin dijalankan, lalu menghasilkan inventory, vars, dan plan sebelum mengeksekusi `ansible-playbook`.

## Status

- Fokus target saat ini: Ubuntu dan Debian
- Mode utama: `apply`, `plan`, `preview`, `doctor`, `version`
- Engine eksekusi: Ansible playbook multi-server
- Artifact run: inventory, vars, dan plan Markdown di `.civa/runs/`

## Yang Bisa Dikerjakan

- update dan upgrade sistem
- buat user deployer, passwordless sudo, dan pasang SSH public key
- hardening SSH: nonaktifkan root login dan password auth
- instal dan konfigurasi UFW + Fail2Ban
- set timezone `Asia/Jakarta` dan buat swap 2GB
- instal utilitas dasar
- instal Docker Engine + Docker Compose Plugin dari repo resmi Docker
- siapkan Traefik v3 dengan ACME HTTP atau DNS challenge

## Struktur Repo

- `main.go` - wrapper Go untuk binary compiled `civa`
- `scripts/civa` - source shell yang mengorkestrasi Ansible
- `ansible/playbook.yml` - playbook utama
- `ansible/templates/` - template Traefik dan Fail2Ban
- `docs/vps-hardening-references.md` - referensi implementasi dan alasan desain
- `bin/` - output binary build
- `plans/` - contoh atau catatan plan manual jika diperlukan

## Instalasi dari GitHub

Ada 3 cara install `civa`:

1. build dari source
2. download binary release
3. one-line installer

### Opsi 1 - Build dari source

```bash
git clone https://github.com/ihyamarsdev/civa.git
cd civa
go build -o bin/civa .
```

Kalau ingin binary `civa` bisa dipanggil dari mana saja:

```bash
sudo install -m 755 bin/civa /usr/local/bin/civa
```

### Opsi 2 - Download binary release

Kalau kamu hanya ingin binary siap pakai, buka halaman release GitHub lalu ambil file sesuai OS dan arsitektur:

- `civa_linux_amd64.tar.gz`
- `civa_linux_arm64.tar.gz`
- `civa_darwin_amd64.tar.gz`
- `civa_darwin_arm64.tar.gz`

Release dibuat otomatis lewat workflow GitHub Actions saat tag `v*` dipush.

### Opsi 3 - One-line installer

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/install.sh | bash
```

Kalau ingin install versi tertentu:

```bash
curl -fsSL https://raw.githubusercontent.com/ihyamarsdev/civa/main/install.sh | CIVA_VERSION=v0.3.0 bash
```

Setelah itu:

```bash
civa
civa help
civa doctor
```

Prasyarat local machine:

- `git`
- `go`
- `bash`
- `ansible-playbook`
- SSH private key dan public key yang valid

## Build

```bash
go build -o bin/civa .
```

## Command

- `civa apply` - jalankan playbook ke server target
- `civa plan` - generate inventory, vars, dan plan saja
- `civa preview` - jalankan `ansible-playbook` dengan `--check --diff`
- `civa doctor` - cek kesiapan local machine
- `civa version` - tampilkan versi
- `civa help` - tampilkan bantuan

Menjalankan `civa` tanpa argumen akan menampilkan help.

## Interview Interaktif

Saat menjalankan `civa apply`, `civa plan`, atau `civa preview` tanpa flag lengkap, `civa` akan bertanya secara bertahap tentang:

- komponen yang ingin dijalankan
- jumlah server
- IP atau address tiap server
- hostname opsional tiap server
- SSH user, SSH port, dan private key lokal
- public key lokal yang akan diinstal ke user deployer
- nama user deployer
- timezone
- email dan challenge type untuk Traefik bila komponen Traefik dipilih

Sebelum apply sungguhan berjalan, `civa` akan menampilkan ringkasan jawaban operator.

## Pilihan Komponen

Kamu bisa memilih `all` atau subset berikut:

1. `system_update`
2. `user_management`
3. `ssh_hardening`
4. `security_firewall`
5. `system_config`
6. `dependencies`
7. `containerization`
8. `traefik`

Urutan task di playbook tetap mengikuti urutan bootstrap yang aman, walaupun kamu hanya memilih beberapa komponen.

## Contoh Pakai

Lihat help:

```bash
./bin/civa
```

Apply interaktif:

```bash
./bin/civa apply
```

Plan saja dengan dua server:

```bash
./bin/civa plan \
  --non-interactive \
  --server 203.0.113.10,web-01 \
  --server 203.0.113.11,api-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-private-key ~/.ssh/id_ed25519 \
  --ssh-public-key ~/.ssh/id_ed25519.pub \
  --deployer-user deployer \
  --timezone Asia/Jakarta \
  --components all
```

Preview hanya komponen user, SSH, firewall, dan dependencies:

```bash
./bin/civa preview \
  --non-interactive \
  --server 203.0.113.10,web-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-private-key ~/.ssh/id_ed25519 \
  --ssh-public-key ~/.ssh/id_ed25519.pub \
  --components 2,3,4,6
```

Apply dengan Traefik DNS challenge:

```bash
./bin/civa apply \
  --non-interactive \
  --server 203.0.113.10,edge-01 \
  --ssh-user root \
  --ssh-port 22 \
  --ssh-private-key ~/.ssh/id_ed25519 \
  --ssh-public-key ~/.ssh/id_ed25519.pub \
  --components 1,2,3,4,5,6,7,8 \
  --traefik-email admin@example.com \
  --traefik-challenge dns \
  --traefik-dns-provider cloudflare
```

## Apa yang Dihasilkan `civa`

Setiap run membuat direktori baru di `.civa/runs/<timestamp>/` berisi:

- `inventory.yml`
- `vars.yml`
- `plan.md`

`civa` juga menampilkan progress lokal per fase sebelum Ansible mengambil alih task-level output.

## Catatan Keamanan

- playbook sekarang ditujukan untuk Ubuntu/Debian karena task memakai `apt`, `ufw`, dan repo Docker Debian-style
- SSH hardening akan menonaktifkan root login dan password authentication
- jalankan `civa preview` lebih dulu bila ingin melihat perubahan tanpa mengeksekusi apply langsung
- untuk Traefik DNS challenge, file `.env` yang dibuat di target masih butuh secret provider yang valid

## Referensi

Lihat `docs/vps-hardening-references.md` untuk alasan desain playbook dan referensi modul/pendekatan yang dipakai.
