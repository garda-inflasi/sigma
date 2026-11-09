// Package seed mengelola eksekusi seeder SQL deklaratif dari db/seeds/
// dan fungsi bantuan bootstrapping data benih untuk aplikasi dan tes.
package seed

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/seedkit"
	"github.com/garda-inflasi/api/pkg/crypto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	usage       = "pemakaian: seed reference|admin|demo|all [lapis...]"
	seedTimeout = 5 * time.Minute

	// DemoPassword adalah kata sandi default seluruh akun demo.
	DemoPassword = "Garda2024!"
	// DemoMFASecret adalah rahasia TOTP untuk akun demo user_mfa.
	DemoMFASecret = "JBSWY3DPEHPK3PXP"
	// DemoMFASlug adalah slug akun demo user_mfa.
	DemoMFASlug = "user_mfa"

	generatedPasswordBytes = 12
	adminPeran             = "ADMIN_SISTEM"
	adminSlug              = "admin-awal"
	adminLembaga           = "Garda Inflasi"
)

var (
	// DemoMFAAkunID adalah UUID akun demo user_mfa.
	DemoMFAAkunID = seedkit.IDPengguna(DemoMFASlug)
	// DemoKodeCadangan adalah kode cadangan akun demo user_mfa.
	DemoKodeCadangan = []string{"BACKUP01", "BACKUP02", "BACKUP03"}
)

// DemoAkun merepresentasikan struktur akun pengguna demo.
type DemoAkun struct {
	ID            string `json:"id"`
	Nama          string `json:"nama"`
	Email         string `json:"email"`
	Peran         string `json:"peran"`
	Lapis         string `json:"lapis"`
	Lembaga       string `json:"lembaga"`
	KabupatenKode string `json:"kabupaten_kode"`
	Status        string `json:"status"`
	MfaAktif      bool   `json:"mfa_aktif"`
	Telepon       string `json:"telepon"`
}

// AdminConfig adalah masukan konfigurasi admin awal dari lingkungan.
type AdminConfig struct {
	Email    string
	Password string
	Output   io.Writer
}

// AdminSummary melaporkan hasil SeedAdminConfig.
type AdminSummary struct {
	Dibuat int
}

var rawDemoSeeds = []struct {
	slug, nama, email, peran, lapis, lembaga, kabupatenKode, status string
	mfa                                                             bool
	telepon                                                         string
}{
	{slug: "superadmin", nama: "Super Admin", email: "superadmin@gardainflasi.id", peran: "ADMIN_SISTEM", lapis: "SISTEM", lembaga: "BPPSDMP", telepon: "+6281200000001"},
	{slug: "strategis_fmipa", nama: "Prof. Dr.rer.nat. Ilham Maulana, S.Si.", email: "strategis.fmipa@gardainflasi.id", peran: "STRATEGIS_FMIPA", lapis: "STRATEGIS", lembaga: "FMIPA USK", telepon: "+6281200000002"},
	{slug: "strategis_bi", nama: "Dr. Muhammad Rizal", email: "strategis.bi@gardainflasi.id", peran: "STRATEGIS_BI", lapis: "STRATEGIS", lembaga: "Bank Indonesia", telepon: "+6281200000003"},
	{slug: "strategis_bps", nama: "Ir. Cut Putri, M.Ec.Dev", email: "strategis.bps@gardainflasi.id", peran: "STRATEGIS_BPS", lapis: "STRATEGIS", lembaga: "BPS Aceh", telepon: "+6281200000004"},
	{slug: "strategis_djpb", nama: "Drs. Syahrul, M.M.", email: "strategis.djpb@gardainflasi.id", peran: "STRATEGIS_DJPB", lapis: "STRATEGIS", lembaga: "DJPb Aceh", telepon: "+6281200000005"},
	{slug: "manajer_program", nama: "Rina Safitri, S.T., M.T.", email: "manajer.program@gardainflasi.id", peran: "MANAJER_PROGRAM", lapis: "MANAJERIAL", lembaga: "BPPSDMP", telepon: "+6281200000006"},
	{slug: "manajer_data", nama: "Bambang Wijaya, S.Kom., M.Cs.", email: "manajer.data@gardainflasi.id", peran: "MANAJER_DATA", lapis: "MANAJERIAL", lembaga: "BPPSDMP", telepon: "+6281200000007"},
	{slug: "manajer_kebijakan", nama: "Dr. Nurul Aini, S.E., M.E.", email: "manajer.kebijakan@gardainflasi.id", peran: "MANAJER_KEBIJAKAN", lapis: "MANAJERIAL", lembaga: "BPPSDMP", telepon: "+6281200000008"},
	{slug: "manajer_pembiayaan", nama: "Fauzan, S.E., M.Si.", email: "manajer.pembiayaan@gardainflasi.id", peran: "MANAJER_PEMBIAYAAN", lapis: "MANAJERIAL", lembaga: "BPPSDMP", telepon: "+6281200000009"},
	{slug: "manajer_risiko", nama: "Dian Permata, S.Si., M.M.", email: "manajer.risiko@gardainflasi.id", peran: "MANAJER_RISIKO", lapis: "MANAJERIAL", lembaga: "BPPSDMP", telepon: "+6281200000010"},
	{slug: "manajer_mutu", nama: "Hendra Gunawan, S.T., M.T.", email: "manajer.mutu@gardainflasi.id", peran: "MANAJER_MUTU", lapis: "MANAJERIAL", lembaga: "BPPSDMP", telepon: "+6281200000011"},
	{slug: "koordinator_lapangan", nama: "Zulkifli, S.P.", email: "koordinator.lapangan@gardainflasi.id", peran: "OPS_KOORDINATOR", lapis: "OPERASIONAL", lembaga: "BPPSDMP", kabupatenKode: "1171", telepon: "+6281200000012"},
	{slug: "agregator_data", nama: "Maya Sari, S.Stat.", email: "agregator.data@gardainflasi.id", peran: "OPS_AGREGATOR", lapis: "OPERASIONAL", lembaga: "BPPSDMP", kabupatenKode: "1171", telepon: "+6281200000013"},
	{slug: "mahasiswa_1", nama: "Aulia Rahman", email: "mahasiswa.1@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1171", telepon: "+6281200000014"},
	{slug: "mahasiswa_2", nama: "Cut Nanda", email: "mahasiswa.2@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1108", telepon: "+6281200000015"},
	{slug: "mahasiswa_3", nama: "Teuku Irfan", email: "mahasiswa.3@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1107", telepon: "+6281200000016"},
	{slug: "mahasiswa_4", nama: "Siti Rahmah", email: "mahasiswa.4@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1109", telepon: "+6281200000017"},
	{slug: "mahasiswa_5", nama: "Muhammad Rizki", email: "mahasiswa.5@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1106", telepon: "+6281200000018"},
	{slug: "mahasiswa_6", nama: "Nurul Fajri", email: "mahasiswa.6@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1105", telepon: "+6281200000019"},
	{slug: "mahasiswa_7", nama: "Rahmat Hidayat", email: "mahasiswa.7@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1104", telepon: "+6281200000020"},
	{slug: "mahasiswa_8", nama: "Dewi Sartika", email: "mahasiswa.8@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1103", telepon: "+6281200000021"},
	{slug: "mahasiswa_9", nama: "Rizky Pratama", email: "mahasiswa.9@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1174", telepon: "+6281200000022"},
	{slug: "mahasiswa_10", nama: "Fitriani", email: "mahasiswa.10@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", kabupatenKode: "1173", telepon: "+6281200000023"},
	{slug: "user_nonaktif", nama: "User Nonaktif", email: "nonaktif@gardainflasi.id", peran: "OPS_MAHASISWA", lapis: "OPERASIONAL", lembaga: "FMIPA USK", status: "NONAKTIF", telepon: "+6281200000024"},
	{slug: "user_mfa", nama: "User MFA", email: "mfa@gardainflasi.id", peran: "OPS_KOORDINATOR", lapis: "OPERASIONAL", lembaga: "BPPSDMP", mfa: true, telepon: "+6281200000025"},
}

// BuildDemoAkun menyusun 25 daftar baris akun pengguna demo.
func BuildDemoAkun() []DemoAkun {
	out := make([]DemoAkun, len(rawDemoSeeds))
	for i, r := range rawDemoSeeds {
		status := r.status
		if status == "" {
			status = "AKTIF"
		}
		out[i] = DemoAkun{
			ID:            seedkit.IDPengguna(r.slug),
			Nama:          r.nama,
			Email:         r.email,
			Peran:         r.peran,
			Lapis:         r.lapis,
			Lembaga:       r.lembaga,
			KabupatenKode: r.kabupatenKode,
			Status:        status,
			MfaAktif:      r.mfa,
			Telepon:       r.telepon,
		}
	}
	return out
}

// LocateSeedsDir mencari lokasi folder db/seeds secara deterministik.
func LocateSeedsDir() (string, error) {
	if envDir := os.Getenv("SEEDS_DIR"); envDir != "" {
		if stat, err := os.Stat(envDir); err == nil && stat.IsDir() {
			return envDir, nil
		}
	}

	// 1. Periksa path umum langsung
	candidates := []string{
		"db/seeds",
		"../../db/seeds",
		"../../../db/seeds",
		"../../../../db/seeds",
		"/seeds",
	}
	for _, c := range candidates {
		if stat, err := os.Stat(c); err == nil && stat.IsDir() {
			if abs, err := filepath.Abs(c); err == nil {
				return abs, nil
			}
			return c, nil
		}
	}

	// 2. Telusuri folder induk dari working directory
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for range 8 {
			target := filepath.Join(dir, "db", "seeds")
			if stat, err := os.Stat(target); err == nil && stat.IsDir() {
				return target, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 3. Fallback path relatif dari file sumber runner.go
	_, file, _, ok := runtime.Caller(0)
	if ok {
		target := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "seeds"))
		if stat, err := os.Stat(target); err == nil && stat.IsDir() {
			return target, nil
		}
	}

	return "", errors.New("direktori db/seeds tidak ditemukan")
}

// ExecuteSQLFile mengeksekusi satu file SQL pada basis data.
func ExecuteSQLFile(ctx context.Context, db *gorm.DB, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("baca berkas %s: %w", filePath, err)
	}
	query := strings.TrimSpace(string(content))
	if query == "" {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return fmt.Errorf("eksekusi %s: %w", filepath.Base(filePath), err)
		}
		return nil
	}
	if _, err := sqlDB.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("eksekusi %s: %w", filepath.Base(filePath), err)
	}
	return nil
}

// ExecuteTier mengeksekusi seluruh berkas SQL dalam lapis/tier secara terurut alfabet.
func ExecuteTier(ctx context.Context, db *gorm.DB, tier string, seedsDir string, out io.Writer) error {
	tierDir := filepath.Join(seedsDir, tier)
	entries, err := os.ReadDir(tierDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("direktori lapis %s tidak ditemukan di %s", tier, seedsDir)
		}
		return fmt.Errorf("baca direktori lapis %s: %w", tier, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, fileName := range files {
		start := time.Now()
		fullPath := filepath.Join(tierDir, fileName)
		if err := ExecuteSQLFile(ctx, db, fullPath); err != nil {
			return err
		}
		dur := time.Since(start).Round(time.Millisecond)
		if out != nil {
			fmt.Fprintf(out, "%-32s %-9s [ok] %s\n", fileName, tier, dur)
		}
	}
	return nil
}

// SeedReference menjalankan seeder tier reference.
func SeedReference(ctx context.Context, db *gorm.DB) error {
	seedsDir, err := LocateSeedsDir()
	if err != nil {
		return err
	}
	return ExecuteTier(ctx, db, "reference", seedsDir, nil)
}

// SeedAdmin menjalankan seeder tier admin.
func SeedAdmin(ctx context.Context, db *gorm.DB) error {
	seedsDir, err := LocateSeedsDir()
	if err != nil {
		return err
	}
	return ExecuteTier(ctx, db, "admin", seedsDir, nil)
}

// SeedDemo menjalankan seeder tier demo.
func SeedDemo(ctx context.Context, db *gorm.DB) error {
	seedsDir, err := LocateSeedsDir()
	if err != nil {
		return err
	}
	return ExecuteTier(ctx, db, "demo", seedsDir, nil)
}

// SeedFiles mengeksekusi berkas seed tertentu, berurutan sesuai argumen. Setiap nama relatif terhadap folder seeds
// dan memuat tiernya, mis. SeedFiles(ctx, db, "demo/01_auth_users.sql", "demo/14_eksekutif_data.sql").
//
// Dipakai tes integrasi yang hanya membutuhkan data satu atau beberapa modul: SeedDemo menjalankan seluruh tier
// sehingga tes melihat data demo semua modul. Nama yang keluar dari folder seeds, bukan .sql, atau tidak ada adalah
// galat; tidak ada berkas yang dijalankan sebelum semua nama tervalidasi.
func SeedFiles(ctx context.Context, db *gorm.DB, files ...string) error {
	seedsDir, err := LocateSeedsDir()
	if err != nil {
		return err
	}
	paths, err := resolveSeedFiles(seedsDir, files)
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := ExecuteSQLFile(ctx, db, p); err != nil {
			return err
		}
	}
	return nil
}

// resolveSeedFiles memvalidasi nama berkas SeedFiles dan mengubahnya menjadi path absolut di bawah seedsDir.
func resolveSeedFiles(seedsDir string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, errors.New("SeedFiles: tidak ada berkas")
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		rel := filepath.Clean(filepath.FromSlash(f))
		if filepath.IsAbs(rel) || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return nil, fmt.Errorf("SeedFiles: %q harus relatif terhadap folder seeds", f)
		}
		if filepath.Ext(rel) != ".sql" {
			return nil, fmt.Errorf("SeedFiles: %q bukan berkas .sql", f)
		}
		p := filepath.Join(seedsDir, rel)
		if stat, err := os.Stat(p); err != nil || stat.IsDir() {
			return nil, fmt.Errorf("SeedFiles: berkas %q tidak ditemukan di %s", f, seedsDir)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// SeedAll menjalankan seluruh seeder (reference, admin, demo).
func SeedAll(ctx context.Context, db *gorm.DB) error {
	if err := SeedReference(ctx, db); err != nil {
		return err
	}
	if err := SeedAdmin(ctx, db); err != nil {
		return err
	}
	return SeedDemo(ctx, db)
}

// SeedAdminConfig membuat satu akun admin awal secara dinamis bila belum ada akun ADMIN_SISTEM.
func SeedAdminConfig(ctx context.Context, db *gorm.DB, cfg AdminConfig) (AdminSummary, error) {
	var count int64
	if err := db.WithContext(ctx).Table("pengguna").Where("peran = ?", adminPeran).Count(&count).Error; err != nil {
		return AdminSummary{}, fmt.Errorf("periksa akun admin: %w", err)
	}
	if count > 0 {
		return AdminSummary{Dibuat: 0}, nil
	}

	email := strings.ToLower(strings.TrimSpace(cfg.Email))
	if email == "" {
		return AdminSummary{}, errors.New("SEED_ADMIN_EMAIL wajib diisi karena belum ada akun admin")
	}

	password := cfg.Password
	generated := false
	if strings.TrimSpace(password) == "" {
		raw := make([]byte, generatedPasswordBytes)
		if _, err := rand.Read(raw); err != nil {
			return AdminSummary{}, fmt.Errorf("generate random password: %w", err)
		}
		password = fmt.Sprintf("Ga-%x-Ra9", raw)
		generated = true
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return AdminSummary{}, fmt.Errorf("hash sandi admin: %w", err)
	}

	adminID := seedkit.IDPengguna(adminSlug)
	insertSQL := `
		INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_wajib, wajib_ganti_password, telepon)
		VALUES (?, 'Administrator Sistem', ?, ?, ?, 'SISTEM', ?, 'AKTIF', false, true, true, '+6281200000001')
		ON CONFLICT (email) DO NOTHING;
	`
	if err := db.WithContext(ctx).Exec(insertSQL, adminID, email, hash, adminPeran, adminLembaga).Error; err != nil {
		return AdminSummary{}, fmt.Errorf("simpan admin awal: %w", err)
	}

	if generated && cfg.Output != nil {
		fmt.Fprintf(cfg.Output, "Admin awal %s dibuat dengan kata sandi sementara (tampil sekali, wajib diganti saat masuk): %s\n", email, password)
	}

	return AdminSummary{Dibuat: 1}, nil
}

// Run menjalankan proses seeding sesuai argumen tingkat/tier yang diberikan.
func Run(args []string, getenv func(string) string, out io.Writer) error {
	tiers, err := seedkit.ParseTiers(args)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, usage)
	}

	appEnv := getenv("APP_ENV")
	if strings.EqualFold(strings.TrimSpace(appEnv), seedkit.ProductionEnv) {
		for _, t := range tiers {
			if t == seedkit.TierDemo {
				return fmt.Errorf("demo: %w", seedkit.ErrDemoInProduction)
			}
		}
	}

	dsn := getenv("DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}

	seedsDir := getenv("SEEDS_DIR")
	if seedsDir == "" {
		var err error
		seedsDir, err = LocateSeedsDir()
		if err != nil {
			return err
		}
	}

	db, err := connect(dsn)
	if err != nil {
		return err
	}
	defer closeDB(db)

	ctx, cancel := context.WithTimeout(context.Background(), seedTimeout)
	defer cancel()

	orderedTiers := []seedkit.Tier{seedkit.TierReference, seedkit.TierAdmin, seedkit.TierDemo}
	for _, tier := range orderedTiers {
		if !slices.Contains(tiers, tier) {
			continue
		}

		if tier == seedkit.TierAdmin {
			adminEmail := getenv("SEED_ADMIN_EMAIL")
			if adminEmail != "" {
				adminPwd := getenv("SEED_ADMIN_PASSWORD")
				cfg := AdminConfig{
					Email:    adminEmail,
					Password: adminPwd,
					Output:   out,
				}
				if _, err := SeedAdminConfig(ctx, db, cfg); err != nil {
					return err
				}
				continue
			}
		}

		if err := ExecuteTier(ctx, db, string(tier), seedsDir, out); err != nil {
			return err
		}
	}

	return nil
}

func connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Error)})
	if err != nil {
		return nil, fmt.Errorf("hubungkan basis data: %w", err)
	}
	return db, nil
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
