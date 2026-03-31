# civa Ansible Notes

Dokumen ini merangkum alasan desain untuk workflow Ansible di `civa`.

## Fokus Implementasi

- `civa` sekarang berperan sebagai orchestrator lokal
- inventory, vars, dan plan digenerate oleh `scripts/civa`
- eksekusi remote dijalankan oleh `ansible/playbook.yml`
- scope target saat ini adalah Ubuntu dan Debian

## Kenapa Diganti ke Ansible

- lebih aman untuk banyak server dibanding shell satu-host
- idempotent untuk banyak task penting seperti package install, file management, dan service state
- lebih mudah memilih subset task lewat tags
- inventory dan vars yang tergenerate membuat audit trail lebih jelas

## Mapping Komponen ke Tags

- `system_update` -> update cache dan dist-upgrade
- `user_management` -> create deployer, sudoers, authorized_keys
- `ssh_hardening` -> hardening `sshd_config`
- `security_firewall` -> UFW dan Fail2Ban
- `system_config` -> timezone, hostname opsional, swap 2GB
- `dependencies` -> utilitas dasar
- `containerization` -> Docker Engine dan Compose Plugin
- `traefik` -> direktori `/opt/traefik`, network `proxy`, `.env`, `docker-compose.yml`

## Implementasi Penting

- SSH hardening memakai `lineinfile` dengan validasi `sshd -t`
- deployer key dipasang dari public key lokal operator melalui `lookup('file', civa_public_key_path)`
- Traefik compose digenerate dari template dan belum otomatis di-`up`
- Docker dipasang dari repo resmi Docker via keyring + `apt_repository`
- swap file dibuat hanya jika `/swapfile` belum ada

## Keterbatasan Saat Ini

- target distro non-Debian belum disupport oleh playbook ini
- UFW diimplementasikan dengan command line idempotence sederhana, bukan collection tambahan
- preview `--check --diff` tetap bergantung pada dukungan check mode task yang dipakai Ansible
- Traefik DNS challenge masih memerlukan secret provider untuk dimasukkan ke `.env` target

## Artifact Run

Setiap run `civa` membuat artifact lokal di `.civa/runs/<timestamp>/`:

- `inventory.yml`
- `vars.yml`
- `plan.md`

Artifact ini memudahkan re-run, audit, dan review sebelum apply berikutnya.
