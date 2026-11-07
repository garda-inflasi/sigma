// Package seeder mengisi contoh jejak audit (hanya demo). Baris audit bersifat
// append-only, jadi seeder memakai DO NOTHING pada kode (demo-audit-NNNN, kunci
// alami baris demo): menjalankannya ulang tidak mengubah atau menambah apa pun.
package seeder

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DemoSummary melaporkan baris yang diusulkan oleh SeedDemo.
type DemoSummary struct {
	Entri int
}

const (
	jumlahEntri = 80
	// seedPRNG membuat urutan acak sama di setiap jalan.
	seedPRNG = 20260930
	// kodePrefix menandai baris demo agar mudah disaring; kode (UNIQUE) adalah kunci idempotensi seeder.
	kodePrefix = "demo-audit-"
)

type aksiContoh struct {
	Aksi, Entitas string
	// DiriSendiri berarti entitas adalah akun pelaku sendiri (mis. masuk).
	DiriSendiri bool
}

var aksiContohSeeds = []aksiContoh{
	{Aksi: "LOGIN", Entitas: "pengguna", DiriSendiri: true},
	{Aksi: "LOGIN", Entitas: "pengguna", DiriSendiri: true},
	{Aksi: "LOGOUT", Entitas: "pengguna", DiriSendiri: true},
	{Aksi: "VALIDASI_HARGA", Entitas: "entri_harga"},
	{Aksi: "PUBLIKASI_HARGA", Entitas: "entri_harga"},
	{Aksi: "AKUI_EWS", Entitas: "ews_alert"},
	{Aksi: "PINDAH_TAHAP", Entitas: "intervensi"},
	{Aksi: "SETUJUI_INTERVENSI", Entitas: "intervensi"},
	{Aksi: "UBAH_AMBANG_EWS", Entitas: "aturan_ews"},
	{Aksi: "EKSPOR_DATA", Entitas: "entri_harga"},
	{Aksi: "AJUKAN_MASTER", Entitas: "komoditas"},
	{Aksi: "BULK_rekomendasi_olahan", Entitas: "stok_cold_storage"},
	{Aksi: "ASSIGN_ROLE", Entitas: "pengguna"},
}

// SeedDemo mengisi 80 contoh jejak audit dengan aktor akun demo, tersebar mundur
// dari now. Prasyarat: akun demo sudah ada (auth.demo).
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	rows := BuildEntri(now)
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		DoNothing: true,
	}).CreateInBatches(&rows, 50).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed audit demo: %w", err)
	}
	return DemoSummary{Entri: len(rows)}, nil
}

// BuildEntri menyusun baris audit contoh secara deterministik terhadap now.
func BuildEntri(now time.Time) []domain.AuditLog {
	aktor := aktorAktif()
	rng := rand.New(rand.NewPCG(seedPRNG, seedPRNG))
	now = now.UTC()

	rows := make([]domain.AuditLog, jumlahEntri)
	for i := range rows {
		contoh := aksiContohSeeds[rng.IntN(len(aksiContohSeeds))]
		pelaku := aktor[rng.IntN(len(aktor))]
		waktu := now.Add(-time.Duration(i*17+rng.IntN(15)) * time.Minute)

		row := domain.AuditLog{
			Kode: new(fmt.Sprintf("%s%04d", kodePrefix, i+1)), Waktu: waktu,
			AktorID: &pelaku.ID, AktorNama: &pelaku.Nama, AktorEmail: &pelaku.Email, AktorPeran: &pelaku.Peran,
			Aksi: contoh.Aksi, Entitas: contoh.Entitas, EntitasID: pelaku.ID,
			Hasil: hasilContoh(rng), IP: new(fmt.Sprintf("10.0.%d.%d", rng.IntN(4), 10+rng.IntN(200))),
			Metadata: new(`{"demo":true}`),
		}
		if !contoh.DiriSendiri {
			row.EntitasID = fmt.Sprintf("%s-%04d", kodeEntitas(contoh.Entitas), 1000+rng.IntN(9000))
		}
		switch contoh.Aksi {
		case "ASSIGN_ROLE":
			row.Sebelum = new(`{"peran":"OPS_MAHASISWA"}`)
			row.Sesudah = new(`{"peran":"OPS_KOORDINATOR"}`)
		case "VALIDASI_HARGA":
			row.Sebelum = new(`{"status":"MENUNGGU_VALIDASI"}`)
			row.Sesudah = new(`{"status":"TERVALIDASI"}`)
		case "UBAH_AMBANG_EWS":
			row.Sebelum = new(`{"ambang":{"waspada":5,"siaga":10}}`)
			row.Sesudah = new(`{"ambang":{"waspada":4,"siaga":10,"kritis":15}}`)
		}
		rows[i] = row
	}
	return rows
}

// aktorAktif adalah akun demo yang bisa menjadi pelaku (bukan yang NONAKTIF).
func aktorAktif() []domain.Actor {
	var out []domain.Actor
	for _, a := range authseeder.BuildDemoAkun() {
		if a.Status == "AKTIF" {
			out = append(out, domain.Actor{ID: a.ID, Nama: a.Nama, Email: a.Email, Peran: a.Peran})
		}
	}
	return out
}

// hasilContoh: sebagian kecil aksi ditolak agar filter hasil ada isinya.
func hasilContoh(rng *rand.Rand) string {
	if rng.IntN(100) < 8 {
		return domain.HasilDitolak
	}
	return domain.HasilBerhasil
}

// kodeEntitas menyingkat nama entitas menjadi tiga huruf kapital untuk id contoh.
func kodeEntitas(entitas string) string {
	return strings.ToUpper(entitas[:3])
}
