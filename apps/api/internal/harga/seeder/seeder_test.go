package seeder

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/harga/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// seedNow jatuh pukul 14.30 WIB sehingga hari ini sudah punya jam kerja lapangan.
var seedNow = time.Date(2026, 10, 16, 7, 30, 0, 0, time.UTC)

func sampleInput() Input {
	return Input{
		Komoditas: []Komoditas{
			{Kode: "CBM-KRT", Satuan: "kg", HargaAcuan: 52000},
			{Kode: "BWM", Satuan: "kg", HargaAcuan: 41000},
			{Kode: "TLR", Satuan: "kg", HargaAcuan: 29500},
		},
		Petugas: []Petugas{
			{ID: uid("mahasiswa_1"), KabupatenKode: "1171", PasarID: "PSR-1171-01", PasarLat: 5.5603, PasarLng: 95.3237, RadiusMeter: 400},
			{ID: uid("mahasiswa_2"), KabupatenKode: "1108", PasarID: "PSR-1108-01", PasarLat: 5.5085, PasarLng: 95.364, RadiusMeter: 600},
		},
		Validator:  uid("koordinator_lapangan"),
		Publikator: uid("manajer_data"),
	}
}

func TestBuild_IsDeterministic(t *testing.T) {
	assert.Equal(t, Build(sampleInput(), seedNow), Build(sampleInput(), seedNow))
}

func TestBuild_UniqueKodeAndRowCount(t *testing.T) {
	entri := Build(sampleInput(), seedNow)

	kode := map[string]bool{}
	for _, e := range entri {
		// ID BIGINT diberi basis data; kunci idempotensi seeder adalah kode (sama dengan ID lama).
		assert.Zero(t, e.ID)
		require.NotNil(t, e.Kode)
		assert.False(t, kode[*e.Kode], "kode ganda: %s", *e.Kode)
		kode[*e.Kode] = true
	}
	assert.Contains(t, kode, "HRG-D00-00-01-K", "bentuk kode sama dengan ID lama")
	// Per hari 2 petugas x (3 harga konsumen + 2 harga produsen cabai dan bawang merah) = 10 baris;
	// hari ini hanya sebagian yang sudah tercatat.
	assert.GreaterOrEqual(t, len(entri), 10*(HariHistori-1))
	assert.LessOrEqual(t, len(entri), 11*HariHistori, "paling banyak sepuluh baris sehari ditambah duplikat")
}

func TestBuild_RowsAreValid(t *testing.T) {
	in := sampleInput()
	dalamHistori := seedNow.AddDate(0, 0, -HariHistori-1)

	for _, e := range Build(in, seedNow) {
		require.Positive(t, e.Harga, *e.Kode)
		require.True(t, domain.StatusValid(e.Status), "%s: status %q", *e.Kode, e.Status)
		assert.False(t, e.DicatatPada.After(seedNow), "%s dicatat di masa depan", *e.Kode)
		assert.True(t, e.DicatatPada.After(dalamHistori), "%s di luar histori", *e.Kode)
		require.NotNil(t, e.PetugasID)
		assert.NotEqual(t, in.Validator, *e.PetugasID)

		if e.Status == domain.StatusDivalidasi || e.Status == domain.StatusDipublikasi {
			require.NotNil(t, e.DivalidasiOleh, *e.Kode)
			assert.NotEqual(t, *e.PetugasID, *e.DivalidasiOleh, "pemisahan tugas: %s", *e.Kode)
			assert.False(t, e.DivalidasiPada.After(seedNow), *e.Kode)
		} else {
			assert.Nil(t, e.DivalidasiOleh, *e.Kode)
		}
		if e.Status == domain.StatusDipublikasi {
			require.NotNil(t, e.DipublikasiOleh, *e.Kode)
			assert.False(t, e.DipublikasiPada.After(seedNow), *e.Kode)
		} else {
			assert.Nil(t, e.DipublikasiOleh, *e.Kode)
		}
	}
}

// inputBesar meniru ukuran data demo: sepuluh petugas dan empat belas komoditas.
func inputBesar() Input {
	in := sampleInput()
	in.Petugas = nil
	for i := range 10 {
		in.Petugas = append(in.Petugas, Petugas{
			ID: fmt.Sprintf("mahasiswa_%d", i+1), KabupatenKode: fmt.Sprintf("11%02d", i+1),
			PasarID: fmt.Sprintf("PSR-11%02d-01", i+1), PasarLat: 5.5, PasarLng: 95.3, RadiusMeter: 400,
		})
	}
	in.Komoditas = nil
	for _, kode := range []string{"BRS-MED", "BRS-PRM", "BWM", "BWP", "CBM-KRT", "CBR-RWT", "DAG-AYM", "DAG-SPI", "GLA", "IKN-TGK", "KTG", "MYK", "TLR", "TMT"} {
		in.Komoditas = append(in.Komoditas, Komoditas{Kode: kode, Satuan: "kg", HargaAcuan: 20000})
	}
	return in
}

func TestBuild_ShapesHistoryAndStatuses(t *testing.T) {
	entri := Build(inputBesar(), seedNow)

	perStatus := map[string]int{}
	var outlier, luarGeofence, duplikat, foto int
	for _, e := range entri {
		perStatus[e.Status]++
		if math.Abs(e.DeviasiPct) > domain.AmbangAnomaliPct {
			outlier++
		}
		if !e.DalamGeofence {
			luarGeofence++
		}
		if len(*e.Kode) > 4 && (*e.Kode)[len(*e.Kode)-4:] == "-dup" {
			duplikat++
		}
		if e.FotoURL != nil {
			foto++
		}
	}

	for _, status := range []string{domain.StatusDraf, domain.StatusDiajukan, domain.StatusDivalidasi, domain.StatusDipublikasi, domain.StatusDitolak} {
		assert.Positive(t, perStatus[status], "sebaran status harus memuat %s", status)
	}
	assert.Greater(t, perStatus[domain.StatusDipublikasi], perStatus[domain.StatusDiajukan], "histori didominasi entri terbit")
	assert.Positive(t, outlier, "harus ada outlier")
	assert.Positive(t, luarGeofence, "harus ada entri di luar geofence")
	assert.Positive(t, duplikat, "harus ada duplikat")
	assert.Positive(t, foto)
	assert.Less(t, foto, len(entri), "sebagian entri tanpa foto")
}

func TestBuild_GeofenceFlagMatchesDistance(t *testing.T) {
	in := sampleInput()
	pasar := map[string]Petugas{}
	for _, p := range in.Petugas {
		pasar[p.PasarID] = p
	}

	for _, e := range Build(in, seedNow) {
		p := pasar[e.PasarID]
		const meterPerDerajat = 111_320.0
		dLat := (*e.Lat - p.PasarLat) * meterPerDerajat
		dLng := (*e.Lng - p.PasarLng) * meterPerDerajat * math.Cos(p.PasarLat*math.Pi/180)
		jarak := math.Hypot(dLat, dLng)
		if e.DalamGeofence {
			assert.LessOrEqual(t, jarak, float64(p.RadiusMeter), "%s ditandai dalam geofence", *e.Kode)
		} else {
			assert.Greater(t, jarak, float64(p.RadiusMeter), "%s ditandai luar geofence", *e.Kode)
		}
	}
}

func TestBuild_CabaiTertekanDanTomatSurplus(t *testing.T) {
	// Skenario demo: harga cabai hari ini jauh di atas awal histori, tomat di bawahnya.
	in := sampleInput()
	in.Komoditas = []Komoditas{{Kode: "CBM-KRT", Satuan: "kg", HargaAcuan: 52000}, {Kode: "TMT", Satuan: "kg", HargaAcuan: 12000}}
	acuan := map[string]float64{"CBM-KRT": 52000, "TMT": 12000}
	entri := Build(in, seedNow)

	// rasio adalah rata-rata harga konsumen wajar terhadap harga acuan pada rentang umur (hari).
	rasio := func(kode string, dari, sampai int) float64 {
		var total, n float64
		for _, e := range entri {
			if e.KomoditasKode != kode || e.Tipe != domain.TipeKonsumen || math.Abs(e.DeviasiPct) > domain.AmbangAnomaliPct {
				continue
			}
			if umur := int(seedNow.Sub(e.DicatatPada).Hours() / 24); umur >= dari && umur <= sampai {
				total += float64(e.Harga) / acuan[kode]
				n++
			}
		}
		return total / n
	}

	assert.Greater(t, rasio("CBM-KRT", 0, 3), rasio("CBM-KRT", 60, 80)*1.15)
	assert.Less(t, rasio("TMT", 0, 3), rasio("TMT", 60, 80)*0.9)
}

func TestSeedDemo_RequiresReferenceData(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_kosong")
	pgtest.MigrateUp(t, db)

	_, err := SeedDemo(t.Context(), db, seedNow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "master.reference")
}

func TestSeedDemo_IsIdempotent(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	first, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)
	assert.Greater(t, first.Entri, 10_000)

	count := func() (n int64) {
		require.NoError(t, db.Raw("SELECT count(*) FROM entri_harga").Scan(&n).Error)
		return n
	}
	assert.EqualValues(t, first.Entri, count())
	riwayat := func() (n int64) {
		require.NoError(t, db.Raw("SELECT count(*) FROM riwayat_status_entri").Scan(&n).Error)
		return n
	}
	assert.EqualValues(t, first.Riwayat, riwayat())

	// Dijalankan ulang pada waktu berbeda: baris sama, tanggal bergeser.
	second, err := SeedDemo(t.Context(), db, seedNow.Add(26*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, first.Entri, second.Entri)
	assert.EqualValues(t, first.Entri, count())
	assert.Equal(t, first.Riwayat, second.Riwayat)
	assert.EqualValues(t, second.Riwayat, riwayat(), "riwayat ditulis ulang, bukan ditambah")

	var tanpaPetugas int64
	require.NoError(t, db.Raw("SELECT count(*) FROM entri_harga WHERE petugas_id IS NULL").Scan(&tanpaPetugas).Error)
	assert.Zero(t, tanpaPetugas)
}

// TestSeedDemo_IDBigintStabilDanMengikutiWaktu: kode (ID lama) menjadi kunci idempotensi, ID BIGINT diberi basis data
// menaik menurut waktu pencatatan, tidak berubah saat seeder diulang, dan riwayat menunjuk entri yang benar.
func TestSeedDemo_IDBigintStabilDanMengikutiWaktu(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_id")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	first, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)

	scan := func(sql string, args ...any) (n int64) {
		require.NoError(t, db.Raw(sql, args...).Scan(&n).Error, sql)
		return n
	}
	assert.EqualValues(t, first.Entri, scan("SELECT count(DISTINCT kode) FROM entri_harga WHERE kode IS NOT NULL"), "setiap baris seed punya kode unik")
	assert.EqualValues(t, 1, scan("SELECT min(id) FROM entri_harga"))
	assert.EqualValues(t, first.Entri, scan("SELECT max(id) FROM entri_harga"), "ID rapat dari 1")
	assert.Zero(t, scan(`
		SELECT count(*) FROM entri_harga a JOIN entri_harga b ON b.id = a.id + 1 WHERE b.dicatat_pada < a.dicatat_pada`),
		"ID menaik menurut dicatat_pada")

	// Setiap riwayat menunjuk entri yang jejaknya sesuai: riwayat 'DIPUBLIKASI' hanya milik entri DIPUBLIKASI.
	assert.Zero(t, scan(`
		SELECT count(*) FROM riwayat_status_entri h JOIN entri_harga e ON e.id = h.entri_id
		 WHERE h.ke = 'DIPUBLIKASI' AND e.status <> 'DIPUBLIKASI'`))
	assert.Zero(t, scan(`
		SELECT count(*) FROM entri_harga e
		 WHERE e.status = 'DITOLAK' AND NOT EXISTS (SELECT 1 FROM riwayat_status_entri h WHERE h.entri_id = e.id AND h.ke = 'DITOLAK')`))

	var sebelum struct{ Min, Max int64 }
	require.NoError(t, db.Raw("SELECT min(id) AS min, max(id) AS max FROM entri_harga WHERE kode = 'HRG-D00-00-01-K'").Scan(&sebelum).Error)
	idContoh := scan("SELECT id FROM entri_harga WHERE kode = 'HRG-D00-00-01-K'")
	_, err = SeedDemo(t.Context(), db, seedNow.Add(26*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, idContoh, scan("SELECT id FROM entri_harga WHERE kode = 'HRG-D00-00-01-K'"), "ID baris yang sudah ada tidak berubah")
	assert.EqualValues(t, first.Entri, scan("SELECT max(id) FROM entri_harga"), "tidak ada ID baru terpakai")
}

// TestSeedDemo_DiAtasBasisDataHasilMigrasi: baris yang ID-nya sudah BIGINT dan kodenya ID lama (hasil migrasi 000111)
// diperbarui, tidak digandakan.
func TestSeedDemo_DiAtasBasisDataHasilMigrasi(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_migrasi")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	// Baris lama (kode = ID lama) dengan ID BIGINT yang jauh dari urutan seeder, seperti hasil migrasi.
	require.NoError(t, db.Exec(`
		INSERT INTO entri_harga (id, kode, komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada)
		OVERRIDING SYSTEM VALUE
		SELECT 900001, 'HRG-D00-00-01-K', k.kode, p.id, 'KONSUMEN', 1000, 'kg', now()
		  FROM komoditas k, pasar p ORDER BY k.kode, p.id LIMIT 1`).Error)
	var awal int64
	require.NoError(t, db.Raw("SELECT id FROM entri_harga WHERE kode = 'HRG-D00-00-01-K'").Scan(&awal).Error)
	require.EqualValues(t, 900001, awal)

	hasil, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)

	var total, idSetelah, harga int64
	require.NoError(t, db.Raw("SELECT count(*) FROM entri_harga").Scan(&total).Error)
	require.NoError(t, db.Raw("SELECT id FROM entri_harga WHERE kode = 'HRG-D00-00-01-K'").Scan(&idSetelah).Error)
	require.NoError(t, db.Raw("SELECT harga FROM entri_harga WHERE kode = 'HRG-D00-00-01-K'").Scan(&harga).Error)
	assert.EqualValues(t, hasil.Entri, total, "tidak ada baris ganda")
	assert.Equal(t, awal, idSetelah, "ID baris lama dipertahankan")
	assert.NotEqual(t, int64(1000), harga, "isi baris lama disegarkan oleh seeder")
	var riwayat int64
	require.NoError(t, db.Raw("SELECT count(*) FROM riwayat_status_entri WHERE entri_id = ?", awal).Scan(&riwayat).Error)
	assert.Positive(t, riwayat, "riwayat menunjuk ID baris lama")
}

func TestBuildRiwayat_FollowsEachEntryTrail(t *testing.T) {
	entri := Build(inputBesar(), seedNow)
	// ID diberi basis data saat entri disimpan; BuildRiwayat membutuhkannya.
	for i := range entri {
		entri[i].ID = idx.ID(i + 1)
	}
	riwayat := BuildRiwayat(entri)

	perEntri := map[idx.ID][]string{}
	for _, r := range riwayat {
		perEntri[r.EntriID] = append(perEntri[r.EntriID], r.Ke)
	}
	jalur := map[string][]string{
		domain.StatusDraf:        {domain.StatusDraf},
		domain.StatusDiajukan:    {domain.StatusDiajukan},
		domain.StatusDivalidasi:  {domain.StatusDiajukan, domain.StatusDivalidasi},
		domain.StatusDipublikasi: {domain.StatusDiajukan, domain.StatusDivalidasi, domain.StatusDipublikasi},
		domain.StatusDitolak:     {domain.StatusDiajukan, domain.StatusDitolak},
	}
	for _, e := range entri {
		assert.Equal(t, jalur[e.Status], perEntri[e.ID], "entri %s berstatus %s", *e.Kode, e.Status)
		if e.Status == domain.StatusDitolak {
			require.NotNil(t, e.AlasanPenolakan, *e.Kode)
			require.NotNil(t, e.DitolakOleh, *e.Kode)
		}
	}
}

func TestSeedReference_OneBoundPerActiveCommodityAndIdempotent(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_reference")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)

	first, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, 14, first.Batas, "satu batas per komoditas aktif")

	second, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	var salah int64
	require.NoError(t, db.Raw(`
		SELECT count(*) FROM batas_harga b JOIN komoditas k ON k.kode = b.komoditas_kode
		 WHERE b.batas_min >= k.harga_acuan OR b.batas_maks <= k.harga_acuan OR b.batas_min <= 0`).Scan(&salah).Error)
	assert.Zero(t, salah, "harga acuan selalu berada di dalam batas")
}

func TestSeedReference_WithoutCommoditiesWritesNothing(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_reference_kosong")
	pgtest.MigrateUp(t, db)

	summary, err := SeedReference(t.Context(), db)

	require.NoError(t, err)
	assert.Zero(t, summary.Batas)
}

// TestSeedReference_DiAtasBasisDataHasilMigrasi: baris batas_harga hasil migrasi 000111 (kode = ID lama 'BH-<komoditas>',
// ID BIGINT milik basis data) tidak digandakan atau ditimpa, dan batas buatan admin dengan kode lain untuk kunci
// alami yang sama (komoditas, kabupaten, berlaku dari) tidak membuat seeder gagal atau menambah baris.
func TestSeedReference_DiAtasBasisDataHasilMigrasi(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_reference_migrasi")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		INSERT INTO batas_harga (id, kode, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
		OVERRIDING SYSTEM VALUE VALUES
			(500, 'BH-CBM-KRT', 'CBM-KRT', NULL, 1111, 22222, '2024-01-01'),
			(501, 'BH-ADMIN-BWM', 'BWM', NULL, 3333, 44444, '2024-01-01')`).Error)

	for range 2 {
		hasil, err := SeedReference(t.Context(), db)
		require.NoError(t, err)
		assert.Equal(t, 14, hasil.Batas, "satu batas awal (kabupaten NULL) per komoditas aktif")
	}

	var total int64
	require.NoError(t, db.Raw("SELECT count(*) FROM batas_harga").Scan(&total).Error)
	assert.EqualValues(t, 14, total, "tidak ada baris ganda")
	var cbm struct{ ID, Min, Maks int64 }
	require.NoError(t, db.Raw("SELECT id, batas_min AS min, batas_maks AS maks FROM batas_harga WHERE kode = 'BH-CBM-KRT'").Scan(&cbm).Error)
	assert.Equal(t, int64(500), cbm.ID, "ID baris hasil migrasi dipertahankan")
	assert.Equal(t, int64(1111), cbm.Min, "isi yang diubah admin tidak ditimpa")
	assert.Equal(t, int64(22222), cbm.Maks)
	var bwm int64
	require.NoError(t, db.Raw("SELECT count(*) FROM batas_harga WHERE komoditas_kode = 'BWM' AND kabupaten_kode IS NULL AND berlaku_dari = '2024-01-01'").Scan(&bwm).Error)
	assert.EqualValues(t, 1, bwm, "batas admin dengan kode lain menempati kunci alami yang sama")
}
