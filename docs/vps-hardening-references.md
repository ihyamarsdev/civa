# VPS Hardening References

Dokumen ini merangkum referensi dan keputusan desain untuk `civa`, yang sekarang dibuild sebagai binary Go dengan source shell tetap dipertahankan di `scripts/civa`.

## Prinsip Yang Dipakai

- Script default-nya mendeteksi distro otomatis dari `/etc/os-release`.
- Script default-nya berjalan dalam mode apply; `--plan-only` tersedia untuk mode plan-only dan `--dry-run` untuk simulasi aman.
- Reload SSH tidak dilakukan otomatis secara default; itu harus diaktifkan dengan `--reload-ssh` agar risiko lockout lebih terkendali.
- Saat dijalankan di terminal interaktif, script akan bertanya bertahap untuk nilai yang belum diberikan; `--non-interactive` dipakai bila ingin full automation lewat flag.
- Opsi `--distro` tetap bisa dipakai sebagai manual override bila kamu ingin memaksa family tertentu.
- Script tetap menerima nilai `--distro` apa pun.
- Distro yang cocok dengan family umum akan mendapat plan native: Debian/Ubuntu, RPM family, SUSE family, dan Arch family.
- Distro yang tidak cocok ke family native tetap didukung lewat `generic fallback`, jadi civa tetap menghasilkan plan, tapi command package manager, firewall, dan update policy harus ditinjau manual.
- Bila `ID` di `/etc/os-release` tidak punya mapping native, civa akan mencoba `ID_LIKE` dulu sebelum jatuh ke generic fallback.
- Fokus tetap pada hardening awal VPS yang aman: akses admin, SSH, firewall, update keamanan, brute-force protection, sinkronisasi waktu, logging, dan backup readiness.
- Hindari otomasi yang berisiko lockout, terutama perubahan SSH sebelum user admin berbasis key diverifikasi.
- Hindari memaksa override crypto OpenSSH tanpa kebutuhan compliance, karena default modern biasanya lebih aman dan lebih tahan terhadap snippet hardening yang cepat usang.

## Support Matrix

### Native Family Support

- `debian`: `debian`, `ubuntu`, `kali`, `linuxmint`, `mint`, `pop`, `pop-os`, `popos`, `neon`, `raspbian`, `elementary`, `zorin`
- `rpm`: `rhel`, `redhat`, `rocky`, `almalinux`, `alma`, `centos`, `fedora`, `amazonlinux`, `amazon-linux`, `amzn`, `oraclelinux`, `oracle-linux`, `ol`
- `suse`: `opensuse`, `opensuse-leap`, `opensuse-tumbleweed`, `sles`, `sle`, `sle-micro`, `suse`
- `arch`: `arch`, `manjaro`, `endeavouros`

### Generic Fallback

- Semua nilai lain tetap diterima dan akan menghasilkan plan dengan placeholder yang harus disesuaikan ke distro asli.
- Ini sengaja lebih aman daripada membuat civa gagal atau mengarang package manager yang belum diverifikasi.

## Kenapa Family-Native Lebih Aman

- Tool firewall tidak sama antar family: Debian/Ubuntu sering nyaman dengan `ufw`, sedangkan RPM, SUSE, dan Arch family lebih cocok dengan `firewalld` pada referensi resmi yang dipakai.
- Mekanisme auto-update juga berbeda: `unattended-upgrades` untuk Debian family, `dnf-automatic` untuk RPM family, dan policy update yang lebih bervariasi pada SUSE dan rolling release seperti Arch.
- Group admin berbeda: Debian family lazimnya memakai `sudo`, banyak distro lain memakai `wheel`.
- Validasi time sync dan MAC framework juga berbeda: ada image yang mengandalkan `chrony`, ada yang `systemd-timesyncd`, ada yang AppArmor, ada yang SELinux.

## Referensi Utama Per Family

### 1. OpenSSH

- URL: https://man.openbsd.org/sshd_config
- Kenapa dipakai:
  - Menjadi acuan umum lintas distro untuk opsi seperti `PermitRootLogin`, `PasswordAuthentication`, `KbdInteractiveAuthentication`, `AllowUsers`, dan validasi `sshd -t`.
  - Menegaskan bahwa default OpenSSH modern biasanya sudah cukup kuat sehingga civa tidak memaksakan cipher/KEX/MAC override yang cepat usang.

### 2. Debian / Ubuntu Family

- Firewall: https://documentation.ubuntu.com/server/how-to/security/firewalls/
- Automatic updates: https://documentation.ubuntu.com/server/how-to/software/automatic-updates/
- Debian periodic updates: https://wiki.debian.org/PeriodicUpdates
- Time sync: https://documentation.ubuntu.com/server/how-to/networking/timedatectl-and-timesyncd/
- Fail2Ban jail reference: https://manpages.ubuntu.com/manpages/jammy/man5/jail.conf.5.html
- Kenapa dipakai:
  - Menjadi dasar penggunaan `ufw`, `unattended-upgrades`, timer APT, dan validasi service time sync yang umum pada Debian family.

### 3. RPM Family

- Fedora automatic updates: https://docs.fedoraproject.org/en-US/quick-docs/autoupdates/
- DNF automatic: https://dnf.readthedocs.io/en/latest/automatic.html
- Red Hat firewalld docs: https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/configuring_firewalls_and_packet_filters/
- Red Hat chrony docs: https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/configuring_basic_system_settings/configuring-time-synchronization_configuring-basic-system-settings
- Kenapa dipakai:
  - Menjadi dasar `dnf-automatic`, `firewalld`, dan validasi `chronyd` pada distro turunan RHEL/Fedora.

### 4. SUSE Family

- openSUSE package management: https://doc.opensuse.org/documentation/leap/reference/html/book-reference/cha-sw-cl.html
- openSUSE firewall guide: https://doc.opensuse.org/documentation/leap/security/html/book-security/cha-security-firewall.html
- SUSE zypper guide: https://documentation.suse.com/sles/15-SP6/html/SLES-all/cha-sw-cl.html
- Kenapa dipakai:
  - Menjadi dasar perbedaan `zypper`, `firewalld`, dan fakta bahwa automation policy di SUSE family lebih bervariasi antar edition.

### 5. Arch Family

- Pacman: https://wiki.archlinux.org/title/Pacman
- OpenSSH: https://wiki.archlinux.org/title/OpenSSH
- Firewalld: https://wiki.archlinux.org/title/Firewalld
- Chrony: https://wiki.archlinux.org/title/Chrony
- Kenapa dipakai:
  - Menjadi dasar kenapa civa tidak menyamakan rolling release dengan Debian-style unattended updates.

## Keputusan Desain Script

- Script mengeksekusi hardening langsung dalam mode default, tetapi tetap menulis Markdown plan sebagai catatan hasil konfigurasi.
- `--plan-only` dipertahankan untuk inspeksi atau review sebelum apply.
- `--dry-run` dipakai untuk menampilkan command apply tanpa menyentuh host.
- `--reload-ssh` diperlukan jika operator ingin perubahan SSH langsung diaktifkan pada run yang sama.
- civa menampilkan apakah distro berasal dari deteksi otomatis atau override manual, supaya hasil generate tidak ambigu.
- Distro di-normalize ke family profile agar alias umum tetap dapat output yang relevan.
- Input distro yang tidak punya mapping native tidak gagal; script turun ke `generic fallback` agar user tetap mendapat kerangka hardening yang aman.
- Drop-in file SSH tetap dipilih agar file vendor tidak disentuh langsung.
- Jika `AllowUsers` dipakai, semua akun admin atau break-glass yang memang harus bisa masuk wajib dicantumkan.
- Jika pindah dari port SSH lama ke port baru, rule lama dan baru harus hidup bersamaan selama cutover, lalu rule lama dihapus setelah login ke port baru berhasil.
- civa memakai firewall backend berbeda sesuai family: `ufw` untuk Debian profile, `firewalld` untuk RPM/SUSE/Arch profile, dan placeholder manual untuk generic fallback.
- civa memakai update policy berbeda sesuai family: `unattended-upgrades`, `dnf-automatic`, workflow patch policy SUSE, atau manual reviewed updates untuk Arch.
- Validasi time sync dibuat toleran ke `chrony`, `chronyd`, atau `systemd-timesyncd`.

## Hal Yang Sengaja Tidak Diotomasi

- Verifikasi login SSH kedua secara otomatis dari luar host.
- Menonaktifkan password/root login tanpa verifikasi akses admin baru.
- Pemilihan tool backup tertentu.
- Hardening kernel lanjutan, EDR, atau compliance profile spesifik seperti CIS benchmark penuh.
- Mapping final untuk distro yang hanya masuk `generic fallback`.

## Cara Pakai

Contoh apply default dengan auto-detect:

```bash
./bin/civa \
  --non-interactive \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Contoh preview aman dengan dry-run:

```bash
./bin/civa \
  --non-interactive \
  --dry-run \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Contoh manual override Debian family:

```bash
./bin/civa \
  --non-interactive \
  --distro ubuntu \
  --hostname app-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 2222 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Contoh RPM family:

```bash
./bin/civa \
  --distro rocky \
  --hostname api-prod-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 22 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Contoh generic fallback:

```bash
./bin/civa \
  --non-interactive \
  --plan-only \
  --distro alpine \
  --hostname edge-01 \
  --admin-user deploy \
  --current-ssh-port 22 \
  --ssh-port 22 \
  --public-key-file /root/bootstrap/deploy.pub \
  --timezone Asia/Jakarta
```

Output default ditulis ke `plans/<hostname>-hardening-plan.md`.

Catatan: nilai `--public-key-file` diperlakukan sebagai path file public key yang tersedia di server target saat langkah bootstrap dijalankan. Jika key masih hanya ada di workstation kamu, salin dulu atau gunakan `ssh-copy-id` sebelum mematikan login berbasis password.
