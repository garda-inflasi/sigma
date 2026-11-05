package domain

import (
	"context"
	"time"
)

// KunciLogin memetakan tabel kunci_login: keadaan lockout satu email ternormalisasi (kuncinya adalah
// hash email, bukan id pengguna, sehingga email yang tak terdaftar berperilaku sama dengan yang terdaftar).
//
// Aturannya, semuanya dalam metode di bawah supaya satu-satunya sumber kebenaran (dipakai repositori
// Postgres dan fake di tes):
//   - Setiap percobaan memeriksa rahasia (kata sandi, kode OTP) lebih dulu "dipesan" lewat CatatPercobaan,
//     baru rahasianya diperiksa. Percobaan ke-N dalam satu jendela memasang kunci sampai
//     waktunya + durasi; percobaan itu sendiri masih diperiksa (kata sandi benar pada percobaan ke-N
//     tetap berhasil dan menghapus keadaan ini).
//   - Selama terkunci, percobaan ditolak tanpa mengubah apa pun: kunci tidak diperpanjang.
//   - Jendela hitung berumur satu durasi sejak percobaan gagal pertamanya; sesudah itu, atau sesudah
//     kunci berakhir, hitungan mulai dari nol.
type KunciLogin struct {
	EmailHash string `gorm:"column:email_hash;primaryKey"`
	// Gagal adalah percobaan gagal (termasuk yang masih dalam pemeriksaan) pada jendela saat ini.
	Gagal        int       `gorm:"column:gagal"`
	JendelaMulai time.Time `gorm:"column:jendela_mulai"`
	// TerkunciSampai nil bila tidak pernah terkunci pada jendela ini.
	TerkunciSampai *time.Time `gorm:"column:terkunci_sampai"`
	DiperbaruiPada time.Time  `gorm:"column:diperbarui_pada;default:now()"`
}

func (KunciLogin) TableName() string {
	return "kunci_login"
}

// Terkunci melaporkan apakah kunci masih berlaku pada waktu now.
func (k KunciLogin) Terkunci(now time.Time) bool {
	return k.TerkunciSampai != nil && k.TerkunciSampai.After(now)
}

// CatatPercobaan menerapkan satu percobaan pada waktu now dan mengembalikan keadaan baru. diizinkan
// false bila akun sedang terkunci: keadaan dikembalikan apa adanya dan rahasia tidak boleh diperiksa.
func (k KunciLogin) CatatPercobaan(now time.Time, maks int, durasi time.Duration) (baru KunciLogin, diizinkan bool) {
	if k.Terkunci(now) {
		return k, false
	}
	// Kunci yang sudah berakhir, jendela yang sudah lewat, dan baris kosong memulai hitungan baru.
	if k.TerkunciSampai != nil || k.Gagal <= 0 || !now.Before(k.JendelaMulai.Add(durasi)) {
		k.Gagal, k.JendelaMulai, k.TerkunciSampai = 0, now, nil
	}
	k.Gagal++
	if k.Gagal >= maks {
		sampai := now.Add(durasi)
		k.TerkunciSampai = &sampai
	}
	k.DiperbaruiPada = now
	return k, true
}

// Kembalikan membatalkan satu percobaan yang ternyata bukan tebakan gagal (kata sandi benar tetapi
// MFA masih menunggu, kesalahan infrastruktur, akun nonaktif). Kunci yang dipasang percobaan itu ikut dicabut.
func (k KunciLogin) Kembalikan(maks int, now time.Time) KunciLogin {
	if k.Gagal > 0 {
		k.Gagal--
	}
	if k.Gagal < maks {
		k.TerkunciSampai = nil
	}
	k.DiperbaruiPada = now
	return k
}

// KunciRepository menyimpan keadaan lockout. Semua metode atomik terhadap pemanggil serentak untuk
// email yang sama.
type KunciRepository interface {
	// MulaiPercobaan memesan satu percobaan untuk emailHash (lihat KunciLogin.CatatPercobaan). Mengembalikan
	// keadaan sesudahnya dan apakah rahasia boleh diperiksa; bila tidak, keadaan memuat akhir kunci.
	MulaiPercobaan(ctx context.Context, emailHash string, now time.Time, maks int, durasi time.Duration) (KunciLogin, bool, error)
	// KembalikanPercobaan membatalkan satu percobaan yang dipesan MulaiPercobaan (lihat KunciLogin.Kembalikan).
	KembalikanPercobaan(ctx context.Context, emailHash string, maks int, now time.Time) error
	// HapusKunci menghapus seluruh keadaan emailHash (login berhasil lengkap atau buka-kunci admin) dan
	// mengembalikan keadaan yang dihapus, nil bila tidak ada.
	HapusKunci(ctx context.Context, emailHash string) (*KunciLogin, error)
	// KunciAktif mengembalikan akhir kunci untuk emailHash yang sedang terkunci pada waktu now; yang
	// tidak terkunci tidak ada di peta.
	KunciAktif(ctx context.Context, emailHash []string, now time.Time) (map[string]time.Time, error)
}
