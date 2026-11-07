// Package seeder mengisi akun: admin awal dari lingkungan (aman di produksi) dan
// 25 akun demo (hanya pengembangan).
package seeder

import (
	"context"
	"fmt"
	"os"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/seedkit"
	"github.com/garda-inflasi/api/pkg/crypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DemoPassword adalah kata sandi seluruh akun demo. Hanya boleh muncul di seeder demo.
const DemoPassword = "Garda2024!"

// demoPasswordHash adalah bcrypt (cost 12) dari DemoPassword. Hash tetap membuat
// seed deterministik dan cepat; tes memastikan keduanya cocok.
const demoPasswordHash = "$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2"

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Akun int
}

type akunDemo struct {
	// ID adalah slug lama akun demo; UUID barisnya seedkit.IDPengguna(ID), sama dengan hasil migrasi 000110.
	ID, Nama, Email, Peran, Lapis, Lembaga string
	KabupatenKode                          string // kode BPS; kosong = tanpa wilayah
	Status                                 string
	MFA                                    bool
}

// Kode kabupaten mengikuti master.reference (BPS): 1171 Banda Aceh, 1108 Aceh Besar,
// 1107 Aceh Barat, 1109 Pidie, 1106 Aceh Tengah, 1105 Aceh Timur, 1104 Aceh Tenggara,
// 1103 Aceh Selatan, 1174 Lhokseumawe, 1173 Langsa.
var akunDemoSeeds = []akunDemo{
	{ID: "superadmin", Nama: "Super Admin", Email: "superadmin@gardainflasi.id", Peran: "ADMIN_SISTEM", Lapis: "SISTEM", Lembaga: "BPPSDMP"},

	{ID: "strategis_fmipa", Nama: "Prof. Dr.rer.nat. Ilham Maulana, S.Si.", Email: "strategis.fmipa@gardainflasi.id", Peran: "STRATEGIS_FMIPA", Lapis: "STRATEGIS", Lembaga: "FMIPA USK"},
	{ID: "strategis_bi", Nama: "Dr. Muhammad Rizal", Email: "strategis.bi@gardainflasi.id", Peran: "STRATEGIS_BI", Lapis: "STRATEGIS", Lembaga: "Bank Indonesia"},
	{ID: "strategis_bps", Nama: "Ir. Cut Putri, M.Ec.Dev", Email: "strategis.bps@gardainflasi.id", Peran: "STRATEGIS_BPS", Lapis: "STRATEGIS", Lembaga: "BPS Aceh"},
	{ID: "strategis_djpb", Nama: "Drs. Syahrul, M.M.", Email: "strategis.djpb@gardainflasi.id", Peran: "STRATEGIS_DJPB", Lapis: "STRATEGIS", Lembaga: "DJPb Aceh"},

	{ID: "manajer_program", Nama: "Rina Safitri, S.T., M.T.", Email: "manajer.program@gardainflasi.id", Peran: "MANAJER_PROGRAM", Lapis: "MANAJERIAL", Lembaga: "BPPSDMP"},
	{ID: "manajer_data", Nama: "Bambang Wijaya, S.Kom., M.Cs.", Email: "manajer.data@gardainflasi.id", Peran: "MANAJER_DATA", Lapis: "MANAJERIAL", Lembaga: "BPPSDMP"},
	{ID: "manajer_kebijakan", Nama: "Dr. Nurul Aini, S.E., M.E.", Email: "manajer.kebijakan@gardainflasi.id", Peran: "MANAJER_KEBIJAKAN", Lapis: "MANAJERIAL", Lembaga: "BPPSDMP"},
	{ID: "manajer_pembiayaan", Nama: "Fauzan, S.E., M.Si.", Email: "manajer.pembiayaan@gardainflasi.id", Peran: "MANAJER_PEMBIAYAAN", Lapis: "MANAJERIAL", Lembaga: "BPPSDMP"},
	{ID: "manajer_risiko", Nama: "Dian Permata, S.Si., M.M.", Email: "manajer.risiko@gardainflasi.id", Peran: "MANAJER_RISIKO", Lapis: "MANAJERIAL", Lembaga: "BPPSDMP"},
	{ID: "manajer_mutu", Nama: "Hendra Gunawan, S.T., M.T.", Email: "manajer.mutu@gardainflasi.id", Peran: "MANAJER_MUTU", Lapis: "MANAJERIAL", Lembaga: "BPPSDMP"},

	{ID: "koordinator_lapangan", Nama: "Zulkifli, S.P.", Email: "koordinator.lapangan@gardainflasi.id", Peran: "OPS_KOORDINATOR", Lapis: "OPERASIONAL", Lembaga: "BPPSDMP", KabupatenKode: "1171"},
	{ID: "agregator_data", Nama: "Maya Sari, S.Stat.", Email: "agregator.data@gardainflasi.id", Peran: "OPS_AGREGATOR", Lapis: "OPERASIONAL", Lembaga: "BPPSDMP", KabupatenKode: "1171"},
	{ID: "mahasiswa_1", Nama: "Aulia Rahman", Email: "mahasiswa.1@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1171"},
	{ID: "mahasiswa_2", Nama: "Cut Nanda", Email: "mahasiswa.2@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1108"},
	{ID: "mahasiswa_3", Nama: "Teuku Irfan", Email: "mahasiswa.3@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1107"},
	{ID: "mahasiswa_4", Nama: "Siti Rahmah", Email: "mahasiswa.4@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1109"},
	{ID: "mahasiswa_5", Nama: "Muhammad Rizki", Email: "mahasiswa.5@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1106"},
	{ID: "mahasiswa_6", Nama: "Nurul Fajri", Email: "mahasiswa.6@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1105"},
	{ID: "mahasiswa_7", Nama: "Rahmat Hidayat", Email: "mahasiswa.7@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1104"},
	{ID: "mahasiswa_8", Nama: "Dewi Sartika", Email: "mahasiswa.8@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1103"},
	{ID: "mahasiswa_9", Nama: "Rizky Pratama", Email: "mahasiswa.9@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1174"},
	{ID: "mahasiswa_10", Nama: "Fitriani", Email: "mahasiswa.10@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", KabupatenKode: "1173"},

	{ID: "user_nonaktif", Nama: "User Nonaktif", Email: "nonaktif@gardainflasi.id", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK", Status: "NONAKTIF"},
	{ID: "user_mfa", Nama: "User MFA", Email: "mfa@gardainflasi.id", Peran: "OPS_KOORDINATOR", Lapis: "OPERASIONAL", Lembaga: "BPPSDMP", MFA: true},
}

// Rahasia MFA akun demo user_mfa; hanya untuk pengembangan dan sengaja dikenal publik (tersimpan
// bersegel dan ter-hash seperti data sungguhan, tetapi nilainya tertulis di sini).
const (
	DemoMFASecret = "JBSWY3DPEHPK3PXP"
	demoMFASlug   = "user_mfa"
)

// DemoMFAAkunID adalah UUID akun demo user_mfa.
var DemoMFAAkunID = seedkit.IDPengguna(demoMFASlug)

// DemoKodeCadangan adalah kode cadangan akun demo user_mfa.
var DemoKodeCadangan = []string{"BACKUP01", "BACKUP02", "BACKUP03"}

// SeedDemo mengisi 25 akun demo (semua 14 peran, satu NONAKTIF, satu MFA aktif).
// Idempoten: baris di-upsert menurut email (kunci alami; id-nya UUID stabil dari slug) dan dikembalikan ke keadaan awal. Hanya
// untuk pengembangan; kata sandi seluruh akun adalah DemoPassword. Rahasia MFA akun demo
// disegel dengan AES_KEY (lingkungan, atau kunci pengembangan bila kosong) dan kode cadangannya di-hash.
func SeedDemo(ctx context.Context, db *gorm.DB) (DemoSummary, error) {
	// Akun demo tidak berguna tanpa matriks izin (Opsi B: izin dibaca dari basis data). SeedRBAC idempoten dan
	// tidak menimpa perubahan admin, jadi aman dipanggil lagi di sini; tes modul lain yang hanya memanggil SeedDemo
	// tetap mendapat matriks bawaan.
	if _, err := SeedRBAC(ctx, db); err != nil {
		return DemoSummary{}, err
	}
	ring, err := crypto.KeyringFromEnv(os.Getenv)
	if err != nil {
		return DemoSummary{}, fmt.Errorf("kunci enkripsi MFA: %w", err)
	}
	sealed, err := ring.Seal(DemoMFASecret)
	if err != nil {
		return DemoSummary{}, err
	}

	akun := BuildDemoAkun()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "email"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"nama", "email", "password_hash", "peran", "lapis", "lembaga", "kabupaten_kode", "status",
				"mfa_aktif", "mfa_secret", "mfa_langkah_terakhir", "wajib_ganti_password",
			}),
		}).Create(&akun).Error
		if err != nil {
			return fmt.Errorf("seed akun demo: %w", err)
		}

		if err := tx.Exec("UPDATE pengguna SET mfa_secret = ? WHERE id = ?", sealed, DemoMFAAkunID).Error; err != nil {
			return fmt.Errorf("seed rahasia MFA demo: %w", err)
		}
		if err := tx.Exec("DELETE FROM mfa_kode_cadangan WHERE user_id = ?", DemoMFAAkunID).Error; err != nil {
			return fmt.Errorf("bersihkan kode cadangan demo: %w", err)
		}
		kode := make([]domain.KodeCadanganMFA, len(DemoKodeCadangan))
		for i, k := range DemoKodeCadangan {
			kode[i] = domain.KodeCadanganMFA{UserID: DemoMFAAkunID, KodeHash: usecase.HashKodeCadangan(DemoMFAAkunID, k)}
		}
		if err := tx.Create(&kode).Error; err != nil {
			return fmt.Errorf("seed kode cadangan demo: %w", err)
		}
		return nil
	})
	if err != nil {
		return DemoSummary{}, err
	}
	return DemoSummary{Akun: len(akun)}, nil
}

// BuildDemoAkun menyusun baris pengguna demo.
func BuildDemoAkun() []domain.Pengguna {
	hash := demoPasswordHash
	out := make([]domain.Pengguna, len(akunDemoSeeds))
	for i, a := range akunDemoSeeds {
		p := domain.Pengguna{
			ID: seedkit.IDPengguna(a.ID), Nama: a.Nama, Email: a.Email, PasswordHash: &hash,
			Peran: a.Peran, Lapis: a.Lapis, Lembaga: a.Lembaga, Status: "AKTIF",
		}
		if a.Status != "" {
			p.Status = a.Status
		}
		if a.KabupatenKode != "" {
			kode := a.KabupatenKode
			p.KabupatenKode = &kode
		}
		p.MfaAktif = a.MFA
		out[i] = p
	}
	return out
}
