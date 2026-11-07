Sudah digabung ke main dan di-push ke GitHub tanpa konflik (0123302..004b501). main lokal dan origin/main kini sama di 004b501.

Cabang feat/auth-backlog saya hapus karena isinya sudah ada di main, jadi tinggal cabang main. Seperti sebelumnya, go.work.sum dan accounts.md tetap di lokal dan tidak ikut di-commit.

Untuk deploy produksi, tiga migrasi baru (104–106) akan jalan otomatis lewat service migrate. Variabel env yang baru ada di env.example dan docs/DEPLOYMENT.md:

AUTH_RATE_LIMIT_PER_MENIT: bawaan 10.
JOB_AUTH_PEMBERSIHAN_*: jadwal dan retensi job pembersihan.
SSO_GOOGLE_*: opsional; kosong berarti tombol Google tidak tampil.
