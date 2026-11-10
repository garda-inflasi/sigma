package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/internal/siklus/engine"
	"github.com/garda-inflasi/api/internal/siklus/repository/postgres"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- basis data kosong -----------------------------------------------------------------------------------------------

// Basis data yang baru dimigrasi (tanpa master dan tanpa aturan), atau sudah punya master tetapi belum punya aturan,
// tidak membuat mesin galat dan tidak menulis apa pun. Baru dengan aturan lengkap mesin membuka NORMAL awal.
func TestEvaluasi_BasisDataKosongTidakGalat(t *testing.T) {
	t.Run("hanya migrasi: wilayah belum ada", func(t *testing.T) {
		db := bareDB(t, "siklus_it_kosong_migrasi")
		eng := engine.New(postgres.New(db), audit.NewWriter(db), logger.NewDefault())

		hasil, err := eng.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)

		require.NoError(t, err)
		assert.True(t, hasil.Dilewati)
		assert.Equal(t, domain.SebabWilayahTakDikenal, hasil.SebabDilewati)
		assert.Empty(t, semuaSiklus(t, db))
	})

	t.Run("master ada, aturan belum di-seed", func(t *testing.T) {
		db := bareDB(t, "siklus_it_kosong_master")
		require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/01_master.sql"))
		eng := engine.New(postgres.New(db), audit.NewWriter(db), logger.NewDefault())

		hasil, err := eng.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)

		require.NoError(t, err)
		assert.True(t, hasil.Dilewati)
		assert.Equal(t, domain.SebabTanpaAturan, hasil.SebabDilewati)
		assert.Empty(t, semuaSiklus(t, db), "tanpa aturan mesin tidak membuat siklus, bahkan NORMAL")
	})

	t.Run("reference lengkap tanpa alert: NORMAL awal dibuka sekali", func(t *testing.T) {
		s := newSuite(t, "siklus_it_kosong_lengkap")

		pertama := s.evaluasi(t)
		kedua := s.evaluasi(t)

		assert.True(t, pertama.Berubah)
		assert.False(t, kedua.Berubah)
		rows := s.semuaSiklus(t)
		require.Len(t, rows, 1)
		assert.Equal(t, "NORMAL", rows[0].Siklus)
		assert.True(t, rows[0].Otomatis)
		assert.Nil(t, rows[0].AkhirPada)
		assert.Nil(t, rows[0].Kode, "baris buatan mesin tidak punya kode seed")
		require.NotNil(t, rows[0].Alasan)
		assert.Len(t, s.audit(t, "GANTI_SIKLUS"), 1)
	})
}

// --- masuk TEKANAN dan SURPLUS dari data nyata ----------------------------------------------------------------------

func TestEvaluasi_AlertSiagaLonjakanHargaMenjadiTekanan(t *testing.T) {
	s := newSuite(t, "siklus_it_tekanan")
	s.evaluasi(t) // NORMAL awal
	awal := s.aktif(t)

	// Di bawah tingkat minimum, jenis tanpa aturan, dan alert yang sudah SELESAI tidak memicu.
	s.alert(t, "EWS-A1", "LONJAKAN_HARGA", "WASPADA", "BARU", "1171")
	s.alert(t, "EWS-A2", "ANOMALI_DATA", "KRITIS", "BARU", "1171")
	s.alert(t, "EWS-A3", "LONJAKAN_HARGA", "KRITIS", "SELESAI", "1171")
	hasil := s.evaluasi(t)
	assert.False(t, hasil.Berubah, "WASPADA, ANOMALI_DATA, dan alert SELESAI bukan pemicu")
	assert.Equal(t, awal.ID, s.aktif(t).ID)

	// Satu alert SIAGA lonjakan harga yang belum selesai cukup (alert DIAKUI/DIESKALASI tetap dihitung).
	s.alert(t, "EWS-A4", "LONJAKAN_HARGA", "SIAGA", "DIAKUI", "1108")
	hasil = s.evaluasi(t)

	require.True(t, hasil.Berubah)
	assert.Equal(t, "NORMAL", hasil.Sebelumnya)
	rows := s.semuaSiklus(t)
	require.Len(t, rows, 2)
	lama, baru := rows[0], rows[1]
	assert.Equal(t, "NORMAL", lama.Siklus)
	require.NotNil(t, lama.AkhirPada)
	assert.True(t, lama.AkhirPada.Equal(baru.MulaiPada), "tutup dan buka pada instan yang sama: tanpa celah dan tanpa tumpang tindih")
	assert.True(t, lama.AkhirPada.After(lama.MulaiPada))
	assert.Equal(t, "TEKANAN", baru.Siklus)
	assert.Nil(t, baru.AkhirPada)
	assert.True(t, baru.Otomatis)
	assert.Nil(t, baru.PulihSejak)
	require.NotNil(t, baru.Alasan)
	assert.Contains(t, *baru.Alasan, "SKL-TKN-LONJAKAN-HARGA")
	assert.Contains(t, *baru.Alasan, "EWS-A4")
	assert.NotContains(t, *baru.Alasan, "EWS-A1", "alert di bawah tingkat minimum bukan alasan")

	// Audit pergantian: sebelum NORMAL, sesudah TEKANAN beserta alasannya; dilakukan sistem (tanpa aktor).
	ganti := s.audit(t, "GANTI_SIKLUS")
	require.Len(t, ganti, 2)
	terakhir := ganti[1]
	require.NotNil(t, terakhir.Sebelum)
	require.NotNil(t, terakhir.Sesudah)
	assert.Equal(t, "TEKANAN", jsonField(t, *terakhir.Sesudah, "siklus"))
	assert.Equal(t, "NORMAL", jsonField(t, *terakhir.Sebelum, "siklus"))
	assert.Nil(t, terakhir.AktorID)
	assert.Contains(t, *terakhir.Sesudah, "SKL-TKN-LONJAKAN-HARGA")
	assert.Contains(t, *terakhir.Metadata, `"pemicu": "jadwal"`)
}

// Jumlah alert dihitung penuh (bukan hanya contoh yang ditampilkan): tujuh alert lonjakan memberi nilai 7, sedangkan
// rujukan dibatasi lima dengan yang paling parah dan terbaru lebih dulu. Alert tanpa kode tampilan dirujuk lewat ID.
func TestEvaluasi_JumlahAlertDihitungPenuhWalauRujukanDibatasiLima(t *testing.T) {
	s := newSuite(t, "siklus_it_jumlah_alert")
	s.alert(t, "EWS-J1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.alert(t, "EWS-J2", "LONJAKAN_HARGA", "SIAGA", "BARU", "1108")
	s.alert(t, "EWS-J3", "LONJAKAN_HARGA", "KRITIS", "BARU", "1106")
	s.alert(t, "EWS-J4", "LONJAKAN_HARGA", "SIAGA", "DIESKALASI", "1107")
	s.alert(t, "EWS-J5", "LONJAKAN_HARGA", "KRITIS", "DIAKUI", "1109")
	s.alert(t, "EWS-J6", "LONJAKAN_HARGA", "SIAGA", "BARU", "1110")
	require.NoError(t, s.db.Exec(`INSERT INTO ews_alert (severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, status, sumber)
		VALUES ('SIAGA', 'LONJAKAN_HARGA', 'Tanpa kode', 'Alert tanpa kode tampilan', '1111', 10, '%', 70, 'BARU', 'Uji')`).Error)
	var idTanpaKode string
	require.NoError(t, s.db.Raw(`SELECT id::text FROM ews_alert WHERE kode IS NULL`).Scan(&idTanpaKode).Error)

	hasil := s.evaluasi(t)

	lonjakan := ambilHasil(t, hasil, "SKL-TKN-LONJAKAN-HARGA")
	assert.EqualValues(t, 7, lonjakan.Pengukuran.Nilai, "semua alert yang cocok dihitung")
	require.Len(t, lonjakan.Pengukuran.Rujukan, 5, "contoh dibatasi lima")
	assert.Subset(t, []string{"EWS-J3", "EWS-J5"}, lonjakan.Pengukuran.Rujukan[:2], "yang paling parah (KRITIS) lebih dulu")
	assert.NotEqual(t, idTanpaKode, "")
	alasan := bacaAlasan(t, *s.aktif(t).Alasan)
	require.Len(t, alasan.Pemicu, 1)
	assert.EqualValues(t, 7, alasan.Pemicu[0].Nilai)
	assert.Len(t, alasan.Pemicu[0].Rujukan, 5)

	// Alert tanpa kode dirujuk lewat ID-nya (desimal) bila termasuk contoh.
	require.NoError(t, s.db.Exec(`UPDATE ews_alert SET status = 'SELESAI', selesai_pada = now() WHERE kode IS NOT NULL`).Error)
	hasil = s.evaluasi(t)
	lonjakan = ambilHasil(t, hasil, "SKL-TKN-LONJAKAN-HARGA")
	assert.EqualValues(t, 1, lonjakan.Pengukuran.Nilai)
	assert.Equal(t, []string{idTanpaKode}, lonjakan.Pengukuran.Rujukan)
}

// Alert di provinsi lain tidak memengaruhi siklus Aceh.
func TestEvaluasi_AlertProvinsiLainTidakDihitung(t *testing.T) {
	s := newSuite(t, "siklus_it_provinsi_lain")
	require.NoError(t, s.db.Exec(`INSERT INTO provinsi (kode, nama) VALUES ('12', 'Sumatera Utara')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO kabupaten (kode, nama, provinsi_kode, jenis, ibukota, lat, lng)
		VALUES ('1271', 'KOTA MEDAN', '12', 'KOTA', 'Medan', 3.59, 98.67)`).Error)
	s.alert(t, "EWS-P1", "LONJAKAN_HARGA", "KRITIS", "BARU", "1271")

	hasil := s.evaluasi(t)

	assert.Equal(t, "NORMAL", s.aktif(t).Siklus)
	assert.True(t, hasil.Berubah, "NORMAL awal")

	// Mesin yang sama menetapkan siklus provinsi itu dari alert miliknya.
	medan, err := s.engine.Evaluasi(t.Context(), "12", opsiJadwal)
	require.NoError(t, err)
	assert.True(t, medan.Berubah)
	assert.Equal(t, "TEKANAN", medan.Aktif.Siklus)
	assert.Equal(t, "NORMAL", s.aktif(t).Siklus, "siklus Aceh tidak berubah")
}

func TestEvaluasi_OverSupplyMenjadiSurplus(t *testing.T) {
	s := newSuite(t, "siklus_it_surplus_alert")
	s.alert(t, "EWS-S1", "OVER_SUPPLY", "INFO", "BARU", "1106") // seperti alert demo: INFO tidak memicu
	s.evaluasi(t)
	assert.Equal(t, "NORMAL", s.aktif(t).Siklus)

	s.alert(t, "EWS-S2", "OVER_SUPPLY", "SIAGA", "BARU", "1117")
	hasil := s.evaluasi(t)

	require.True(t, hasil.Berubah)
	assert.Equal(t, []string{"NORMAL", "SURPLUS"}, s.urutanSiklus(t))
	assert.Contains(t, *s.aktif(t).Alasan, "SKL-SRP-OVER-SUPPLY")
}

// Utilisasi cold storage OPERASIONAL (kapasitas seed reference: CS-01 400 + CS-02 300 + CS-03 250 = 950 ton):
// 855 ton tepat 90% dan memicu, di bawahnya tidak. Lot kedaluwarsa, belum masuk, HABIS, dan fasilitas yang bukan
// OPERASIONAL tidak dihitung.
func TestEvaluasi_UtilisasiStokMenjadiSurplusTepatDiAmbang(t *testing.T) {
	s := newSuite(t, "siklus_it_surplus_stok")
	s.evaluasi(t)

	s.lot(t, "L-1", 854)
	hasil := s.evaluasi(t)
	assert.False(t, hasil.Berubah, "854 dari 950 ton = 89,89%")
	utilisasi := ambilHasil(t, hasil, "SKL-SRP-UTILISASI-STOK")
	assert.True(t, utilisasi.Pengukuran.Terukur)
	assert.InDelta(t, 100*854.0/950, utilisasi.Pengukuran.Nilai, 1e-9)
	assert.Equal(t, "854 dari 950 ton", utilisasi.Pengukuran.Rincian)
	assert.False(t, utilisasi.Memicu)

	// Lot yang tidak layak tidak menambah isi.
	require.NoError(t, s.db.Exec(`INSERT INTO stok_cold_storage (kode, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada, status) VALUES
		('L-KDL', (SELECT id FROM cold_storage WHERE kode = 'CS-01'), 'BWM', 100, now() - interval '10 days', 'K', 0.1, now() - interval '1 day', 'AKTIF'),
		('L-NXT', (SELECT id FROM cold_storage WHERE kode = 'CS-01'), 'BWM', 100, now() + interval '1 day', 'K', 0.1, now() + interval '30 days', 'AKTIF'),
		('L-HBS', (SELECT id FROM cold_storage WHERE kode = 'CS-01'), 'BWM', 100, now() - interval '1 day', 'K', 0.1, now() + interval '30 days', 'HABIS'),
		('L-PRW', (SELECT id FROM cold_storage WHERE kode = 'CS-04'), 'BWM', 100, now() - interval '1 day', 'K', 0.1, now() + interval '30 days', 'AKTIF')`).Error)
	assert.False(t, s.evaluasi(t).Berubah, "lot kedaluwarsa, belum masuk, HABIS, dan fasilitas PERAWATAN tidak dihitung")

	s.lot(t, "L-2", 1)
	hasil = s.evaluasi(t)
	require.True(t, hasil.Berubah, "855 dari 950 ton = tepat 90%")
	assert.Equal(t, 90.0, ambilHasil(t, hasil, "SKL-SRP-UTILISASI-STOK").Pengukuran.Nilai, "nilai persis 90 (tanpa galat pecahan)")
	assert.Equal(t, []string{"NORMAL", "SURPLUS"}, s.urutanSiklus(t))
	assert.Contains(t, *s.aktif(t).Alasan, "SKL-SRP-UTILISASI-STOK")
	assert.Contains(t, *s.aktif(t).Alasan, "855 dari 950 ton")
}

// Tanpa fasilitas operasional utilisasi tak terukur (bukan nol persen) dan tidak memicu.
func TestEvaluasi_TanpaFasilitasOperasionalStokTakTerukur(t *testing.T) {
	s := newSuite(t, "siklus_it_tanpa_fasilitas")
	require.NoError(t, s.db.Exec(`UPDATE cold_storage SET status = 'PERAWATAN'`).Error)

	hasil := s.evaluasi(t)

	utilisasi := ambilHasil(t, hasil, "SKL-SRP-UTILISASI-STOK")
	assert.False(t, utilisasi.Pengukuran.Terukur)
	assert.False(t, utilisasi.Memicu)
	assert.Equal(t, "NORMAL", s.aktif(t).Siklus)
}

// --- prioritas ---------------------------------------------------------------------------------------------------------

func TestEvaluasi_TekananMengalahkanSurplusDanSurplusMenungguMasaPulih(t *testing.T) {
	s := newSuite(t, "siklus_it_prioritas")
	s.alert(t, "EWS-X1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.alert(t, "EWS-X2", "OVER_SUPPLY", "KRITIS", "BARU", "1106")

	s.evaluasi(t)

	aktif := s.aktif(t)
	assert.Equal(t, "TEKANAN", aktif.Siklus, "pemicu bersamaan: TEKANAN diutamakan")
	require.NotNil(t, aktif.Alasan)
	alasan := bacaAlasan(t, *aktif.Alasan)
	require.Len(t, alasan.Pemicu, 1)
	assert.Equal(t, "SKL-TKN-LONJAKAN-HARGA", alasan.Pemicu[0].Aturan)
	require.Len(t, alasan.Ditekan, 1, "SURPLUS tercatat sebagai pemicu yang kalah")
	assert.Equal(t, "SKL-SRP-OVER-SUPPLY", alasan.Ditekan[0].Aturan)
	assert.Contains(t, alasan.Ringkasan, "tidak diutamakan karena prioritasnya lebih rendah")
	assert.Equal(t, []string{"TEKANAN"}, s.urutanSiklus(t), "langsung TEKANAN, tidak lewat NORMAL")

	// Tekanan mereda, surplus tetap: SURPLUS menunggu masa pulih TEKANAN lalu mengambil alih tanpa lewat NORMAL.
	s.selesaikanAlert(t, "EWS-X1")
	assert.False(t, s.evaluasi(t).Berubah, "masa pulih TEKANAN baru dimulai")
	assert.NotNil(t, s.aktif(t).PulihSejak)
	s.mundurkanPulih(t, 47)
	assert.False(t, s.evaluasi(t).Berubah)
	s.mundurkanPulih(t, 49)
	hasil := s.evaluasi(t)
	require.True(t, hasil.Berubah)
	assert.Equal(t, []string{"TEKANAN", "SURPLUS"}, s.urutanSiklus(t))
	assert.Contains(t, hasil.Ringkasan, "Berpindah dari TEKANAN ke SURPLUS")

	// TEKANAN muncul lagi: merebut kembali saat itu juga (prioritas lebih tinggi), tanpa menunggu masa pulih SURPLUS.
	s.alert(t, "EWS-X3", "KELANGKAAN_STOK", "SIAGA", "BARU", "1108")
	require.True(t, s.evaluasi(t).Berubah)
	assert.Equal(t, []string{"TEKANAN", "SURPLUS", "TEKANAN"}, s.urutanSiklus(t))
}

// Prioritas adalah data: operator bisa membalik urutan, dan pemicu berprioritas sama dimenangkan TEKANAN.
func TestEvaluasi_PrioritasDiubahOperatorDipatuhi(t *testing.T) {
	s := newSuite(t, "siklus_it_prioritas_operator")
	s.alert(t, "EWS-Y1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.alert(t, "EWS-Y2", "OVER_SUPPLY", "SIAGA", "BARU", "1106")

	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET prioritas = 5 WHERE kode = 'SKL-SRP-OVER-SUPPLY'`).Error)
	s.evaluasi(t)
	assert.Equal(t, "SURPLUS", s.aktif(t).Siklus, "SURPLUS diberi prioritas lebih tinggi oleh operator")

	// Sama-sama 10: TEKANAN menang (pemutus seri menurut siklus), sehingga SURPLUS yang aktif direbut.
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET prioritas = 10 WHERE kode = 'SKL-SRP-OVER-SUPPLY'`).Error)
	hasil := s.evaluasi(t)
	require.True(t, hasil.Berubah)
	assert.Equal(t, "TEKANAN", s.aktif(t).Siklus)
}

// --- aturan sebagai data ------------------------------------------------------------------------------------------------

func TestEvaluasi_AturanNonaktifJumlahMinimumDanSeverityMengikutiData(t *testing.T) {
	s := newSuite(t, "siklus_it_aturan_data")
	s.evaluasi(t)
	s.alert(t, "EWS-D1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")

	// Aturan dinonaktifkan: alert SIAGA tidak lagi memicu.
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET aktif = false WHERE kode = 'SKL-TKN-LONJAKAN-HARGA'`).Error)
	assert.False(t, s.evaluasi(t).Berubah)
	assert.Equal(t, "NORMAL", s.aktif(t).Siklus)

	// Diaktifkan lagi dengan jumlah minimum 2: satu alert tidak cukup, dua alert (tepat di batas) cukup.
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET aktif = true, jumlah_min = 2 WHERE kode = 'SKL-TKN-LONJAKAN-HARGA'`).Error)
	assert.False(t, s.evaluasi(t).Berubah)
	s.alert(t, "EWS-D2", "LONJAKAN_HARGA", "KRITIS", "DIESKALASI", "1108")
	hasil := s.evaluasi(t)
	require.True(t, hasil.Berubah)
	assert.Equal(t, "TEKANAN", s.aktif(t).Siklus)
	assert.Contains(t, hasil.Ringkasan, "2 alert LONJAKAN_HARGA ≥ SIAGA (minimal 2)")

	// Severity minimum dinaikkan ke KRITIS: hanya satu alert yang masih memenuhi, di bawah jumlah minimum 2, jadi pemicu
	// hilang dan masa pulih dimulai (TEKANAN tidak langsung dilepas).
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET severity_min = 'KRITIS' WHERE kode = 'SKL-TKN-LONJAKAN-HARGA'`).Error)
	assert.False(t, s.evaluasi(t).Berubah)
	assert.NotNil(t, s.aktif(t).PulihSejak)
}

// Tidak ada aturan aktif: mesin tidak berhak menyimpulkan apa pun, termasuk melepas siklus yang berjalan.
func TestEvaluasi_TanpaAturanAktifTidakMelepasSiklusBerjalan(t *testing.T) {
	s := newSuite(t, "siklus_it_tanpa_aturan_aktif")
	s.alert(t, "EWS-Z1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.evaluasi(t)
	s.selesaikanAlert(t, "EWS-Z1")
	s.mundurkanPulih(t, 100)
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET aktif = false`).Error)

	hasil := s.evaluasi(t)

	assert.True(t, hasil.Dilewati)
	assert.Equal(t, domain.SebabTanpaAturan, hasil.SebabDilewati)
	assert.Equal(t, "TEKANAN", s.aktif(t).Siklus)
	assert.Equal(t, []string{"TEKANAN"}, s.urutanSiklus(t))
}

// --- pemulihan (histeresis) ---------------------------------------------------------------------------------------------

func TestEvaluasi_KembaliNormalHanyaSetelahMasaPulihTanpaPemicu(t *testing.T) {
	s := newSuite(t, "siklus_it_pulih")
	s.alert(t, "EWS-R1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.evaluasi(t)
	tekanan := s.aktif(t)
	require.Equal(t, "TEKANAN", tekanan.Siklus)

	// Alert diselesaikan: TEKANAN tidak langsung dilepas; penanda masa pulih dimulai dan tetap sama antar evaluasi.
	s.selesaikanAlert(t, "EWS-R1")
	assert.False(t, s.evaluasi(t).Berubah)
	mulaiPulih := s.aktif(t).PulihSejak
	require.NotNil(t, mulaiPulih)
	assert.False(t, s.evaluasi(t).Berubah)
	require.NotNil(t, s.aktif(t).PulihSejak)
	assert.True(t, mulaiPulih.Equal(*s.aktif(t).PulihSejak), "penanda tidak bergeser tiap evaluasi")
	assert.Contains(t, s.evaluasi(t).Ringkasan, "masa pulih 48 jam")

	// 47 jam tanpa pemicu: masih TEKANAN. Pemicu kembali di tengah masa pulih: penanda dihapus.
	s.mundurkanPulih(t, 47)
	assert.False(t, s.evaluasi(t).Berubah)
	s.alert(t, "EWS-R2", "ANOMALI_CUACA", "SIAGA", "BARU", "1106")
	assert.False(t, s.evaluasi(t).Berubah)
	assert.Nil(t, s.aktif(t).PulihSejak, "pemicu kembali membatalkan pemulihan")
	assert.Equal(t, tekanan.ID, s.aktif(t).ID)

	// Pemicu hilang lagi: masa pulih dihitung dari awal, bukan dari yang lama.
	s.selesaikanAlert(t, "EWS-R2")
	assert.False(t, s.evaluasi(t).Berubah)
	s.mundurkanPulih(t, 47.5)
	assert.False(t, s.evaluasi(t).Berubah, "47,5 jam belum cukup")

	// 49 jam: dilepas ke NORMAL; baris lama ditutup (penanda pulih dibuang, CHECK) dan NORMAL dibuka.
	s.mundurkanPulih(t, 49)
	hasil := s.evaluasi(t)
	require.True(t, hasil.Berubah)
	assert.Contains(t, hasil.Ringkasan, "Kembali ke NORMAL: tidak ada aturan TEKANAN yang memicu selama 48 jam.")
	rows := s.semuaSiklus(t)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"TEKANAN", "NORMAL"}, s.urutanSiklus(t))
	require.NotNil(t, rows[0].AkhirPada)
	assert.Nil(t, rows[0].PulihSejak, "baris yang ditutup tidak membawa penanda pulih")
	assert.True(t, rows[0].AkhirPada.Equal(rows[1].MulaiPada))
	assert.Nil(t, rows[1].AkhirPada)
	assert.Len(t, s.audit(t, "GANTI_SIKLUS"), 2)

	// NORMAL stabil: evaluasi berikutnya tidak mengubah apa pun.
	assert.False(t, s.evaluasi(t).Berubah)
	assert.Len(t, s.semuaSiklus(t), 2)
}

func TestEvaluasi_SurplusMemakaiMasaPulihTujuhPuluhDuaJam(t *testing.T) {
	s := newSuite(t, "siklus_it_pulih_surplus")
	s.alert(t, "EWS-U1", "OVER_SUPPLY", "SIAGA", "BARU", "1106")
	s.evaluasi(t)
	require.Equal(t, "SURPLUS", s.aktif(t).Siklus)

	s.selesaikanAlert(t, "EWS-U1")
	s.evaluasi(t)
	s.mundurkanPulih(t, 71)
	assert.False(t, s.evaluasi(t).Berubah, "TEKANAN akan dilepas pada 48 jam, SURPLUS baru pada 72 jam")
	s.mundurkanPulih(t, 73)
	require.True(t, s.evaluasi(t).Berubah)
	assert.Equal(t, []string{"SURPLUS", "NORMAL"}, s.urutanSiklus(t))
}

// --- idempoten dan serentak ---------------------------------------------------------------------------------------------

func TestEvaluasi_BerulangTidakMenambahBarisAtauAudit(t *testing.T) {
	s := newSuite(t, "siklus_it_idempoten")
	s.alert(t, "EWS-I1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.evaluasi(t)
	awal := s.aktif(t)
	jumlahAudit := len(s.audit(t, "GANTI_SIKLUS"))

	for range 5 {
		assert.False(t, s.evaluasi(t).Berubah)
	}

	akhir := s.aktif(t)
	assert.Equal(t, awal.ID, akhir.ID)
	assert.True(t, awal.MulaiPada.Equal(akhir.MulaiPada), "tanggal mulai tidak bergeser")
	assert.Nil(t, akhir.PulihSejak)
	assert.Len(t, s.semuaSiklus(t), 1)
	assert.Len(t, s.audit(t, "GANTI_SIKLUS"), jumlahAudit)
}

// Beberapa instance (di sini: beberapa mesin dengan repositori terpisah pada kolam koneksi yang sama) mengevaluasi
// bersamaan. Kunci advisory per wilayah menggilirkan mereka: tepat satu baris aktif, tepat satu pergantian.
func TestEvaluasi_SerentakTidakMenghasilkanDuaBarisAktif(t *testing.T) {
	jalankan := func(t *testing.T, s *suite, instance, putaran int) {
		t.Helper()
		var wg sync.WaitGroup
		galat := make(chan error, instance*putaran)
		for range instance {
			eng := engine.New(postgres.New(s.db), audit.NewWriter(s.db), logger.NewDefault())
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range putaran {
					if _, err := eng.Evaluasi(context.Background(), domain.WilayahBawaan, opsiJadwal); err != nil {
						galat <- err
					}
				}
			}()
		}
		wg.Wait()
		close(galat)
		for err := range galat {
			require.NoError(t, err)
		}
	}
	aktifTunggal := func(t *testing.T, s *suite) {
		t.Helper()
		var jumlahAktif int64
		require.NoError(t, s.db.Raw(`SELECT count(*) FROM siklus_ekonomi WHERE wilayah_kode = '11' AND akhir_pada IS NULL`).Scan(&jumlahAktif).Error)
		assert.EqualValues(t, 1, jumlahAktif, "tepat satu siklus aktif")
		for _, r := range s.semuaSiklus(t) {
			if r.AkhirPada != nil {
				assert.True(t, r.AkhirPada.After(r.MulaiPada), "CHECK akhir_pada > mulai_pada")
			}
		}
	}

	t.Run("dari basis data kosong dengan pemicu", func(t *testing.T) {
		s := newSuite(t, "siklus_it_serentak_kosong")
		s.alert(t, "EWS-C1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")

		jalankan(t, s, 8, 3)

		aktifTunggal(t, s)
		assert.Equal(t, []string{"TEKANAN"}, s.urutanSiklus(t), "hanya satu baris dibuat")
		assert.Len(t, s.audit(t, "GANTI_SIKLUS"), 1)
	})

	t.Run("saat pergantian jatuh tempo", func(t *testing.T) {
		s := newSuite(t, "siklus_it_serentak_ganti")
		s.evaluasi(t) // NORMAL
		s.alert(t, "EWS-C2", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")

		jalankan(t, s, 8, 3)

		aktifTunggal(t, s)
		assert.Equal(t, []string{"NORMAL", "TEKANAN"}, s.urutanSiklus(t), "satu kali NORMAL -> TEKANAN, bukan delapan")
		assert.Len(t, s.audit(t, "GANTI_SIKLUS"), 2)
	})

	t.Run("saat pelepasan jatuh tempo", func(t *testing.T) {
		s := newSuite(t, "siklus_it_serentak_lepas")
		s.alert(t, "EWS-C3", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
		s.evaluasi(t)
		s.selesaikanAlert(t, "EWS-C3")
		s.evaluasi(t)
		s.mundurkanPulih(t, 49)

		jalankan(t, s, 8, 3)

		aktifTunggal(t, s)
		assert.Equal(t, []string{"TEKANAN", "NORMAL"}, s.urutanSiklus(t))
	})
}

// Evaluasi lain yang menahan kunci wilayah lebih lama dari batas tunggu ditolak dengan ErrEvaluasiSibuk (409 di API),
// bukan menggantung; konteks yang habis lebih dulu juga menghentikan penantian. Tidak ada yang ditulis selama itu.
func TestEvaluasi_KunciYangDitahanMenghasilkanSibukBukanGantung(t *testing.T) {
	s := newSuite(t, "siklus_it_kunci")
	tahan := s.db.Begin()
	require.NoError(t, tahan.Error)
	t.Cleanup(func() { tahan.Rollback() })
	require.NoError(t, tahan.Exec(`SELECT pg_advisory_xact_lock(hashtextextended('garda.siklus.evaluasi.11', 0))`).Error)

	t.Run("lewat batas tunggu: ErrEvaluasiSibuk", func(t *testing.T) {
		pendek := engine.New(postgres.New(s.db).DenganBatasKunci(200*time.Millisecond), audit.NewWriter(s.db), logger.NewDefault())
		mulai := time.Now()

		_, err := pendek.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)

		require.ErrorIs(t, err, domain.ErrEvaluasiSibuk)
		assert.Less(t, time.Since(mulai), 10*time.Second, "menyerah di batas tunggu, tidak menggantung")
	})

	t.Run("konteks habis lebih dulu: berhenti", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
		defer cancel()
		selesai := make(chan error, 1)
		go func() {
			_, err := s.engine.Evaluasi(ctx, domain.WilayahBawaan, opsiJadwal)
			selesai <- err
		}()
		select {
		case err := <-selesai:
			require.Error(t, err)
			assert.NotErrorIs(t, err, domain.ErrEvaluasiSibuk)
		case <-time.After(10 * time.Second):
			t.Fatal("evaluasi menggantung walau konteks sudah habis")
		}
	})

	assert.Empty(t, s.semuaSiklus(t), "tidak ada yang ditulis selagi kunci dipegang pihak lain")

	require.NoError(t, tahan.Rollback().Error)
	assert.True(t, s.evaluasi(t).Berubah, "kunci dilepas: evaluasi berjalan normal")
}

// --- keamanan data ------------------------------------------------------------------------------------------------------

// Aturan yang gagal diukur membatalkan seluruh putaran: pengukuran yang gagal tidak boleh dibaca "tidak ada pemicu"
// dan melepas TEKANAN yang sebenarnya masih berlangsung.
func TestEvaluasi_GagalMengukurTidakMengubahSiklus(t *testing.T) {
	s := newSuite(t, "siklus_it_gagal_ukur")
	s.alert(t, "EWS-G1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	s.evaluasi(t)
	s.selesaikanAlert(t, "EWS-G1")
	s.evaluasi(t)
	s.mundurkanPulih(t, 100) // tanpa galat, ini melepas TEKANAN

	require.NoError(t, s.db.Exec(`ALTER TABLE cold_storage RENAME TO cold_storage_rusak`).Error)
	_, err := s.engine.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)
	require.Error(t, err)
	assert.ErrorContains(t, err, "SKL-SRP-UTILISASI-STOK")
	assert.Equal(t, "TEKANAN", s.aktif(t).Siklus, "TEKANAN tidak dilepas karena pengukuran gagal")
	assert.Equal(t, []string{"TEKANAN"}, s.urutanSiklus(t))

	require.NoError(t, s.db.Exec(`ALTER TABLE cold_storage_rusak RENAME TO cold_storage`).Error)
	assert.True(t, s.evaluasi(t).Berubah, "setelah pulih dari gangguan, pelepasan berjalan normal")
	assert.Equal(t, "NORMAL", s.aktif(t).Siklus)
}

// api dan worker terhubung dengan login berhak minimum (garda_aplikasi: tanpa DDL, tanpa UPDATE/DELETE audit; 000119).
// Mesin harus berjalan penuh dengan hak itu: kunci advisory, baca alert dan stok, tulis siklus, tulis audit.
func TestEvaluasi_BerjalanDenganLoginAplikasiBerhakMinimum(t *testing.T) {
	const namaDB, nama, sandi = "siklus_it_hak_minimum", "garda_app_siklus_mesin", "sandi-uji-cukup-panjang-9"
	s := newSuite(t, namaDB)
	sqlDB, err := s.db.DB()
	require.NoError(t, err)
	require.NoError(t, migrate.SiapkanPenggunaAplikasi(t.Context(), sqlDB, nama, sandi))
	app := pg.BukaSebagai(t, namaDB, nama, sandi)
	eng := engine.New(postgres.New(app), audit.NewWriter(app), logger.NewDefault())
	s.alert(t, "EWS-H1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")

	hasil, err := eng.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)
	require.NoError(t, err)
	require.True(t, hasil.Berubah)
	assert.Equal(t, "TEKANAN", s.aktif(t).Siklus)
	assert.Len(t, s.audit(t, "GANTI_SIKLUS"), 1, "audit tertulis lewat login berhak minimum")

	s.selesaikanAlert(t, "EWS-H1")
	_, err = eng.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)
	require.NoError(t, err)
	s.mundurkanPulih(t, 49)
	hasil, err = eng.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)
	require.NoError(t, err)
	assert.True(t, hasil.Berubah)
	assert.Equal(t, []string{"TEKANAN", "NORMAL"}, s.urutanSiklus(t))
}

// CHECK akhir_pada > mulai_pada tidak boleh dilanggar walau jam basis data mundur atau mulai_pada berada di masa depan.
func TestEvaluasi_TutupSiklusYangMulaiDiMasaDepanTidakMelanggarCheck(t *testing.T) {
	s := newSuite(t, "siklus_it_check_waktu")
	require.NoError(t, s.db.Exec(`INSERT INTO siklus_ekonomi (kode, wilayah_kode, siklus, mulai_pada) VALUES ('SE-11-MASA-DEPAN', '11', 'NORMAL', now() + interval '2 hours')`).Error)
	s.alert(t, "EWS-W1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")

	hasil := s.evaluasi(t)

	require.True(t, hasil.Berubah)
	rows := s.semuaSiklus(t)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].AkhirPada)
	assert.True(t, rows[0].AkhirPada.After(rows[0].MulaiPada))
	assert.True(t, rows[0].AkhirPada.Equal(rows[1].MulaiPada))
	assert.Equal(t, "TEKANAN", rows[1].Siklus)
}

// Siklus buatan seed atau manual diperlakukan seperti siklus lain: dipertahankan selama dipicu, dilepas setelah pulih.
func TestEvaluasi_SiklusManualDariSeedDiambilAlihMesin(t *testing.T) {
	s := newSuite(t, "siklus_it_manual_diambil_alih")
	require.NoError(t, s.db.Exec(`INSERT INTO siklus_ekonomi (kode, wilayah_kode, siklus, mulai_pada) VALUES ('SE-11-MANUAL', '11', 'TEKANAN', now() - interval '3 days')`).Error)

	hasil := s.evaluasi(t)
	assert.False(t, hasil.Berubah, "belum ada pemicu tetapi masa pulih baru dimulai")
	aktif := s.aktif(t)
	assert.Equal(t, "TEKANAN", aktif.Siklus)
	assert.False(t, aktif.Otomatis)
	require.NotNil(t, aktif.PulihSejak)

	s.mundurkanPulih(t, 49)
	require.True(t, s.evaluasi(t).Berubah)
	rows := s.semuaSiklus(t)
	require.Len(t, rows, 2)
	assert.Equal(t, "SE-11-MANUAL", *rows[0].Kode)
	assert.NotNil(t, rows[0].AkhirPada)
	assert.Equal(t, "NORMAL", rows[1].Siklus)
	assert.True(t, rows[1].Otomatis)
}

func ambilHasil(t *testing.T, h domain.HasilEvaluasi, kode string) domain.HasilAturan {
	t.Helper()
	for _, a := range h.Aturan {
		if a.Aturan.Kode == kode {
			return a
		}
	}
	require.Failf(t, "aturan tidak ada di hasil", "%s", kode)
	return domain.HasilAturan{}
}
