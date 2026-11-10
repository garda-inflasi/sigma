package integration

import (
	"testing"

	"github.com/garda-inflasi/api/internal/intervensi/workflow"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const berkasAturanSiklus = "reference/14_siklus_aturan.sql"

type barisAturan struct {
	ID         int64
	Kode       string
	Siklus     string
	Indikator  string
	JenisEws   *string
	Severity   *string `gorm:"column:severity_min"`
	JumlahMin  *int
	Ambang     *float64
	Prioritas  int
	PulihJam   int
	Aktif      bool
	Keterangan string
}

func bacaAturan(t *testing.T, s *suite) map[string]barisAturan {
	t.Helper()
	var rows []barisAturan
	require.NoError(t, s.db.Raw(`SELECT id, kode, siklus::text AS siklus, indikator, jenis_ews::text AS jenis_ews, severity_min::text AS severity_min,
		jumlah_min, ambang, prioritas, pulih_jam, aktif, keterangan FROM aturan_siklus ORDER BY id`).Scan(&rows).Error)
	out := map[string]barisAturan{}
	for _, r := range rows {
		out[r.Kode] = r
	}
	return out
}

// Seed reference idempoten menurut kode dan TIDAK PERNAH menimpa aturan yang sudah ada: ambang, prioritas, masa pulih,
// status aktif, dan keterangan hasil kalibrasi operator bertahan di antara deploy; ID baris tidak berganti.
func TestSeedReference_AturanSiklusIdempotenDanTidakMenimpaPerubahanOperator(t *testing.T) {
	s := newSuite(t, "siklus_it_seed_aturan")
	awal := bacaAturan(t, s)
	require.Len(t, awal, 5)

	require.NoError(t, seed.SeedReference(t.Context(), s.db), "seed reference kedua")
	assert.Equal(t, awal, bacaAturan(t, s), "dijalankan ulang tanpa perubahan operator: tidak ada yang berubah")

	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET ambang = 95, aktif = false, prioritas = 7, pulih_jam = 12, keterangan = 'Dikalibrasi ulang'
		WHERE kode = 'SKL-SRP-UTILISASI-STOK'`).Error)
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET jumlah_min = 3, severity_min = 'KRITIS' WHERE kode = 'SKL-TKN-LONJAKAN-HARGA'`).Error)
	diubah := bacaAturan(t, s)

	require.NoError(t, seed.SeedFiles(t.Context(), s.db, berkasAturanSiklus))
	require.NoError(t, seed.SeedReference(t.Context(), s.db), "seed reference ketiga")
	setelah := bacaAturan(t, s)

	assert.Equal(t, diubah, setelah, "hasil kalibrasi operator dipertahankan, termasuk ID")
	utilisasi := setelah["SKL-SRP-UTILISASI-STOK"]
	require.NotNil(t, utilisasi.Ambang)
	assert.EqualValues(t, 95, *utilisasi.Ambang)
	assert.False(t, utilisasi.Aktif)
	assert.Equal(t, 7, utilisasi.Prioritas)
	assert.Equal(t, 12, utilisasi.PulihJam)
	assert.Equal(t, "Dikalibrasi ulang", utilisasi.Keterangan)
	lonjakan := setelah["SKL-TKN-LONJAKAN-HARGA"]
	require.NotNil(t, lonjakan.JumlahMin)
	assert.Equal(t, 3, *lonjakan.JumlahMin)
	require.NotNil(t, lonjakan.Severity)
	assert.Equal(t, "KRITIS", *lonjakan.Severity)
	assert.Len(t, setelah, 5, "tidak ada baris ganda")
}

// Aturan yang hilang ditanam kembali (kunci kode), sedangkan yang masih ada tidak disentuh: operator mematikan aturan
// lewat aktif = false, bukan DELETE.
func TestSeedReference_AturanSiklusYangDihapusDitanamKembali(t *testing.T) {
	s := newSuite(t, "siklus_it_seed_aturan_hapus")
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET prioritas = 15 WHERE kode = 'SKL-TKN-ANOMALI-CUACA'`).Error)
	require.NoError(t, s.db.Exec(`DELETE FROM aturan_siklus WHERE kode = 'SKL-SRP-OVER-SUPPLY'`).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), s.db, berkasAturanSiklus))

	aturan := bacaAturan(t, s)
	assert.Len(t, aturan, 5)
	assert.Equal(t, 20, aturan["SKL-SRP-OVER-SUPPLY"].Prioritas, "ditanam kembali dengan nilai bawaan")
	assert.Equal(t, 15, aturan["SKL-TKN-ANOMALI-CUACA"].Prioritas, "yang masih ada tidak disentuh")
}

// Pemetaan jenis alert -> siklus pada aturan bawaan sama dengan pemetaan kartu intervensi otomatis (14-C): bila salah
// satunya berubah tanpa yang lain, tes ini gagal. Seluruh nilai enum jenis_ews diperiksa, jadi jenis baru yang dipetakan
// intervensi tetapi belum punya aturan siklus (atau sebaliknya) langsung ketahuan.
func TestSeedReference_AturanSiklusSelarasDenganPemetaanKartuOtomatis(t *testing.T) {
	s := newSuite(t, "siklus_it_seed_selaras")
	aturan := bacaAturan(t, s)

	var jenisSemua []string
	require.NoError(t, s.db.Raw(`SELECT unnest(enum_range(NULL::jenis_ews))::text`).Scan(&jenisSemua).Error)
	require.NotEmpty(t, jenisSemua)

	for _, jenis := range jenisSemua {
		siklus, dipetakan := workflow.SiklusUntuk(jenis)
		var cocok []barisAturan
		for _, a := range aturan {
			if a.Indikator == "ALERT_EWS" && a.JenisEws != nil && *a.JenisEws == jenis {
				cocok = append(cocok, a)
			}
		}
		if !dipetakan {
			assert.Empty(t, cocok, "jenis %s tidak memicu kartu otomatis, jadi tidak boleh memicu siklus", jenis)
			continue
		}
		assert.NotEmpty(t, cocok, "jenis %s memicu siklus %s di kartu otomatis, harus punya aturan siklus", jenis, siklus)
		for _, a := range cocok {
			assert.Equal(t, siklus, a.Siklus, "aturan %s harus memicu siklus yang sama dengan kartu otomatis", a.Kode)
			require.NotNil(t, a.Severity, a.Kode)
			assert.Equal(t, workflow.DefaultKartuOtomatisMin, *a.Severity, "ambang tingkat sama dengan kartu otomatis bawaan (%s)", a.Kode)
		}
	}
}

// --- interaksi dengan seed demo -------------------------------------------------------------------------------------

func TestSeedDemo_SiklusDemoTetapTekananKarenaAlertDemo(t *testing.T) {
	s := newSuite(t, "siklus_it_demo_tekanan")
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/06_ews_alerts.sql", "demo/16_refresh_views.sql"))
	demo := s.aktif(t)
	require.Equal(t, "SE-11-DEMO", *demo.Kode)
	require.Equal(t, "TEKANAN", demo.Siklus)
	require.False(t, demo.Otomatis)

	hasil := s.evaluasi(t)

	assert.False(t, hasil.Berubah, "alert demo (lonjakan SIAGA/KRITIS, cuaca SIAGA) membenarkan TEKANAN yang sudah ada")
	assert.Equal(t, demo.ID, s.aktif(t).ID)
	assert.Nil(t, s.aktif(t).PulihSejak)
	assert.Empty(t, s.audit(t, "GANTI_SIKLUS"))
	// Utilisasi stok tidak memicu SURPLUS: lot demo belum di-seed di sini, dan di seed demo penuh utilisasinya ~82%.
	assert.Equal(t, "TEKANAN", hasil.Aktif.Siklus)
}

// Seed demo -> mesin mengubah siklus -> seed demo lagi: tidak galat, tetap tepat satu siklus aktif, dan siklus demo yang
// sudah ditutup mesin tidak dibuka kembali (sejarah tidak ditimpa, indeks unik siklus aktif tidak dilanggar).
func TestSeedDemo_AmanDijalankanUlangSetelahMesinMenggantiSiklus(t *testing.T) {
	s := newSuite(t, "siklus_it_demo_ulang")
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/06_ews_alerts.sql", "demo/16_refresh_views.sql"))
	demo := s.aktif(t)
	require.Equal(t, "SE-11-DEMO", *demo.Kode)

	// Mesin melepas TEKANAN demo: semua alert selesai, lalu masa pulih lewat.
	require.NoError(t, s.db.Exec(`UPDATE ews_alert SET status = 'SELESAI', selesai_pada = now()`).Error)
	assert.False(t, s.evaluasi(t).Berubah, "masa pulih baru dimulai")
	s.mundurkanPulih(t, 49)
	require.True(t, s.evaluasi(t).Berubah)
	require.Equal(t, []string{"TEKANAN", "NORMAL"}, s.urutanSiklus(t))
	normal := s.aktif(t)
	assert.Nil(t, normal.Kode, "baris buatan mesin")

	// Seed demo dijalankan lagi (seluruh berkas siklus): tanpa galat; satu siklus aktif, yaitu milik mesin.
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/16_refresh_views.sql"), "seed demo kedua setelah mesin berjalan")
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/16_refresh_views.sql"), "seed demo ketiga")
	assert.Equal(t, normal.ID, s.aktif(t).ID, "siklus milik mesin tidak digantikan")
	rows := s.semuaSiklus(t)
	require.Len(t, rows, 2, "SE-11-DEMO tidak digandakan maupun dibuka kembali")
	assert.Equal(t, "SE-11-DEMO", *rows[0].Kode)
	require.NotNil(t, rows[0].AkhirPada, "siklus demo yang sudah ditutup tetap tertutup")

	// Seed alert demo dijalankan lagi (alert kembali aktif): mesin menyatukan kembali siklus dengan keadaan demo.
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/06_ews_alerts.sql", "demo/16_refresh_views.sql"))
	require.True(t, s.evaluasi(t).Berubah)
	assert.Equal(t, []string{"TEKANAN", "NORMAL", "TEKANAN"}, s.urutanSiklus(t))
	assert.Equal(t, "TEKANAN", s.aktif(t).Siklus)
}

// Mesin lebih dulu menanam siklus (mis. worker menyala sebelum seed demo): seed demo tidak menambah siklus aktif kedua
// dan tidak galat; siklus demo disusul mesin begitu alert demo terbaca.
func TestSeedDemo_MesinSudahMenanamSiklusSebelumSeed(t *testing.T) {
	s := newSuite(t, "siklus_it_demo_setelah_mesin")
	s.evaluasi(t)
	require.Equal(t, []string{"NORMAL"}, s.urutanSiklus(t))

	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/06_ews_alerts.sql", "demo/16_refresh_views.sql"))

	assert.Equal(t, []string{"NORMAL"}, s.urutanSiklus(t), "seed demo tidak menambah siklus aktif kedua")
	var demo int64
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM siklus_ekonomi WHERE kode = 'SE-11-DEMO'`).Scan(&demo).Error)
	assert.Zero(t, demo)
	require.True(t, s.evaluasi(t).Berubah)
	assert.Equal(t, []string{"NORMAL", "TEKANAN"}, s.urutanSiklus(t))
}
