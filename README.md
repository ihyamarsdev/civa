# civa

`civa` adalah CLI interaktif untuk hardening awal VPS yang sekarang dibuild sebagai binary compiled. Tool ini memandu interview user, menyiapkan plan, menjalankan apply distro-aware, menampilkan progress per fase, dan menutup proses dengan summary yang jelas.

Singkatnya, `civa` dirancang untuk membuat bootstrap keamanan VPS terasa lebih aman, lebih terstruktur, dan lebih mudah dioperasikan.

## Status

- Status: aktif dan siap dipakai untuk bootstrap hardening VPS
- Mode utama: apply langsung ke host
- Distro detection: otomatis dari `/etc/os-release`, dengan override manual bila perlu
- Security focus: SSH hardening, firewall, Fail2Ban, update policy, time sync, logging, dan backup readiness
- Safety default: perubahan SSH ditulis langsung, tetapi reload SSH tetap manual kecuali `--reload-ssh`

Script ini bisa:

- baseline package update
- user admin berbasis SSH key
- hardening SSH
- firewall host
- Fail2Ban
- update policy
- sinkronisasi waktu
- logging, audit, dan backup readiness
- menulis file Markdown sebagai catatan plan hasil konfigurasi

## Kenapa Planner Ini Aman

- Ada `--dry-run` untuk preview aman sebelum apply
- Ada `--plan-only` jika kamu hanya ingin generate plan
- Tidak otomatis me-reload SSH sebelum kamu sengaja memilih `--reload-ssh`
- Mendukung fallback generik untuk distro yang tidak punya mapping native
- Menjaga langkah hardening rawan lockout tetap eksplisit dan bisa direview dulu

## Fitur Utama

- Auto-detect distro dari `/etc/os-release`
- Manual override dengan `--distro` bila diperlukan
- Apply langsung ke host sebagai default behavior
- Prompt interaktif bertahap untuk nilai penting saat dijalankan di terminal
- `--plan-only` untuk kembali ke mode plan-only
- `--dry-run` untuk mensimulasikan apply tanpa perubahan host
- `--reload-ssh` untuk mengaktifkan perubahan SSH langsung pada apply mode
- `--non-interactive` untuk automation tanpa prompt
- Native profile untuk family berikut:
  - Debian/Ubuntu
  - RPM family
  - SUSE family
  - Arch family
- Generic fallback untuk distro lain
- Fail2Ban tetap masuk ke plan dengan `banaction` yang disesuaikan per family
- Metadata hasil detect ikut ditulis ke output plan

## Struktur Repo

- `main.go` - wrapper Go untuk binary compiled `civa`
- `scripts/civa` - source shell yang di-embed ke binary
- `docs/vps-hardening-references.md` - referensi dan keputusan desain
- `bin/` - output binary hasil build
- `plans/` - direktori output hasil generate

## Instalasi dan Build

Install dari GitHub repository resmi:

```bash
git clone https://github.com/ihyamarsdev/civa.git
cd civa
go build -o bin/civa .
```

Kalau ingin binary `civa` bisa dipanggil dari mana saja:

```bash
sudo install -m 755 bin/civa /usr/local/bin/civa
```

Setelah itu kamu bisa menjalankan:

```bash
civa
civa help
civa doctor
```

Prasyarat minimal untuk build:

- `git`
- `go`
- `bash` tersedia di host target saat binary dijalankan

Build binary lokal:

```bash
go build -o bin/civa .
```

Jalankan binary hasil build:

```bash
./bin/civa
```

## Cara Pakai

Jalankan tanpa argumen untuk melihat help:

```bash
./bin/civa
```

Command yang tersedia:

- `civa apply` - apply langsung ke host
- `civa plan` - generate plan saja
- `civa preview` - preview apply tanpa perubahan host
- `civa doctor` - cek kesiapan host saat ini untuk apply
- `civa version` - tampilkan versi `civa`
- `civa help` - tampilkan bantuan

Contoh command tambahan:

```bash
./bin/civa version
./bin/civa doctor
```

Jalankan interaktif dan biarkan script bertanya step-by-step dengan section yang lebih jelas, lalu tampilkan ringkasan jawaban sebelum apply dimulai atau dibatalkan:

```bash
./bin/civa apply
```

Apply langsung dengan auto-detect distro:

```bash
./bin/civa apply \
  --non-interactive \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Apply langsung dan reload SSH juga:

```bash
./bin/civa apply \
  --reload-ssh \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Preview apply tanpa mengubah host:

```bash
./bin/civa preview \
  --non-interactive \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Mode plan-only:

```bash
./bin/civa plan \
  --non-interactive \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Manual override distro:

```bash
./bin/civa apply \
  --non-interactive \
  --distro ubuntu \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Contoh generic fallback:

```bash
./bin/civa plan \
  --non-interactive \
  --distro alpine \
  --hostname edge-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 22 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Output default akan ditulis ke `plans/<hostname>-hardening-plan.md`.

Catatan path key:

- Default `--public-key-file` adalah `~/.ssh/id_ed25519.pub` dan sekarang akan diekspansi ke home user yang menjalankan script.
- Untuk apply mode, file public key itu harus benar-benar ada di host target.

## Contoh Output

Potongan hasil plan auto-detect:

```md
# Initial VPS Hardening Plan

## Target Profile

- Mode: apply
- Distro request: auto
- Distro source: auto-detected-id_like
- Detected OS ID: cachyos
- Detected OS ID_LIKE: arch
- Distro family: Arch family

## Phase 5 - Brute-Force Protection With Fail2Ban

~~~bash
sudo tee /etc/fail2ban/jail.local >/dev/null <<'JAIL'
[DEFAULT]
bantime = 1h
findtime = 10m
maxretry = 5
banaction = firewallcmd-rich-rules
~~~
```

## Auto-Detect Distro

Jika `--distro` tidak diisi, civa akan:

1. membaca `ID` dari `/etc/os-release`
2. mencoba memetakan `ID` ke family native
3. jika belum cocok, mencoba `ID_LIKE`
4. jika masih belum cocok, turun ke `generic fallback`

Jika script dijalankan di terminal interaktif, dia juga akan menanyakan nilai yang belum diberikan, seperti mode, hostname, user admin, port SSH, reload SSH, public key path, dan timezone.

Di hasil plan, metadata berikut ikut ditulis:

- `Distro request`
- `Distro source`
- `Distro detection note`
- `Detected OS ID`
- `Detected OS ID_LIKE`
- `Mode`
- `Mode note`

## Support Matrix

Native family mapping saat ini:

- `debian`: `debian`, `ubuntu`, `kali`, `linuxmint`, `mint`, `pop`, `pop-os`, `popos`, `neon`, `raspbian`, `elementary`, `zorin`
- `rpm`: `rhel`, `redhat`, `rocky`, `almalinux`, `alma`, `centos`, `fedora`, `amazonlinux`, `amazon-linux`, `amzn`, `oraclelinux`, `oracle-linux`, `ol`
- `suse`: `opensuse`, `opensuse-leap`, `opensuse-tumbleweed`, `sles`, `sle`, `sle-micro`, `suse`
- `arch`: `arch`, `manjaro`, `endeavouros`

Semua input distro lain tetap diterima, tetapi output-nya akan memakai placeholder yang harus ditinjau manual.

## Fail2Ban

Script selalu menyiapkan section dan konfigurasi Fail2Ban ke output plan, dan akan menerapkannya saat mode apply aktif.

Mapping backend saat ini:

- Debian family -> `banaction = ufw`
- RPM, SUSE, Arch -> `banaction = firewallcmd-rich-rules`
- Generic fallback -> placeholder manual

Catatan penting:

- Fail2Ban tidak menggantikan hardening SSH dan firewall
- Kalau firewall backend di host berbeda dari yang diprediksi civa, ubah `banaction` sebelum menerapkan plan
- `--dry-run` adalah preview command, bukan bukti bahwa apply mode pasti berhasil di host

## Batasan

- Tidak memilih tool backup tertentu
- Tidak mengerjakan hardening lanjutan seperti CIS benchmark penuh
- Distro di generic fallback tetap butuh review manual sebelum dipakai di production
- Apply mode akan menolak generic fallback agar tidak menjalankan placeholder berbahaya
- Untuk perubahan SSH yang sensitif, pastikan akses console atau rescue tetap tersedia saat menjalankan apply mode
- Jika mau pindah port SSH, gunakan `--reload-ssh`; tanpa itu script akan menolak perubahan port agar host tidak setengah berubah

## Referensi

Lihat `docs/vps-hardening-references.md` untuk sumber resmi dan alasan desain per distro family.
