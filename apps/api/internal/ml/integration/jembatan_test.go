package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	ewspostgres "github.com/garda-inflasi/api/internal/ews/repository/postgres"
	"github.com/garda-inflasi/api/internal/ml/jembatan"
	mlrepo "github.com/garda-inflasi/api/internal/ml/repository/mongo"
	mlpostgres "github.com/garda-inflasi/api/internal/ml/repository/postgres"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"
)

const (
	c11 = "C20261124T11Z"
	c12 = "C20261124T12Z"
	c13 = "C20261124T13Z"
)

func init() {
	for id, jam := range map[string]int{c11: 11, c12: 12, c13: 13} {
		tsSiklus[id] = time.Date(2026, 11, 24, jam, 0, 0, 0, time.UTC)
	}
}

// dbJembatan adalah basis data bersih: tier reference ditambah akun demo saja (tanpa alert dan kartu demo), supaya
// jumlah alert dan kartu yang dihitung tes hanya berasal dari jembatan.
func dbJembatan(t *testing.T, nama string) *gorm.DB {
	t.Helper()
	butuhDocker(t)
	db := pg.CreateDatabase(t, nama)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(context.Background(), db))
	require.NoError(t, seed.SeedFiles(context.Background(), db, "demo/01_auth_users.sql"))
	return db
}

// tambahSiklus menulis siklus final baru berisi sinyal saja.
func tambahSiklus(t *testing.T, m mongoUji, id string, sinyal ...bson.M) {
	t.Helper()
	m.tulis(t, m.safe, mlrepo.KoleksiSiklus, dokSiklus(id, "BERHASIL"))
	m.tulis(t, m.safe, mlrepo.KoleksiEws, sinyal...)
}

type (
	ewsAlert = ewsdomain.Alert
	ewsHasil = ewsdomain.HasilTulis
)

type pengamat struct{ hasil []string }

func (p *pengamat) AlertDitulis(_ context.Context, a ewsAlert, h ewsHasil) {
	p.hasil = append(p.hasil, string(h)+":"+a.Severity)
}

// jalankan menjalankan satu putaran jembatan dengan Service baru (tanpa cache siklus terbaru dari putaran lalu).
func jalankan(t *testing.T, db *gorm.DB, m mongoUji, obs *pengamat, now time.Time) jembatan.Ringkasan {
	t.Helper()
	log := logger.New(logger.ERROR, io.Discard)
	ews := ewsjobs.Shared(db, log)
	j := jembatan.New(m.service(t), mlpostgres.New(db), ewspostgres.New(db), audit.NewWriter(db), log, ews.Publisher, ews.Notifikasi, obs)
	j.SetJam(func() time.Time { return now })
	r, err := j.Jalankan(context.Background())
	require.NoError(t, err)
	return r
}

type alertML struct {
	ID            string
	Kode          string
	Severity      string
	Jenis         string
	Status        string
	KomoditasKode *string
	KabupatenKode *string
	Sumber        string
	AturanID      *string
	Pesan         string
}

func alertUntuk(t *testing.T, db *gorm.DB, kunci string) []alertML {
	t.Helper()
	var rows []alertML
	require.NoError(t, db.Raw(`
		SELECT id::text AS id, kode, severity::text AS severity, jenis::text AS jenis, status::text AS status, komoditas_kode,
		       kabupaten_kode, sumber, aturan_id::text AS aturan_id, pesan
		  FROM ews_alert WHERE dedup_key = ? ORDER BY id`, jembatan.DedupKey(kunci)).Scan(&rows).Error)
	return rows
}

func hitung(t *testing.T, db *gorm.DB, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestJembatan_IdempotenINFOdanAlurEWS(t *testing.T) {
	db := dbJembatan(t, "ml_jembatan_idempoten")
	m := mongoBaru(t)
	isiStandar(t, m)
	obs := &pengamat{}
	now := tsSiklus[c09].Add(10 * time.Minute)

	r := jalankan(t, db, m, obs, now)
	assert.Equal(t, c09, r.CycleID, "siklus BERJALAN yang lebih baru (C10) tidak pernah dijembatani")
	assert.Equal(t, 4, r.Dibuat)
	assert.Equal(t, 1, r.DiabaikanInfo, "SUMBER_DATA INFO (sengaja dilewati) tidak menjadi alert")
	assert.Zero(t, r.Galat)
	assert.Len(t, obs.hasil, 4)
	assert.Empty(t, alertUntuk(t, db, kDilewati))
	assert.Zero(t, hitung(t, db, `SELECT count(*) FROM ews_alert WHERE komoditas_kode = 'GLA'`), "sinyal KRITIS C10 setengah tertulis tidak terbaca")

	a := alertUntuk(t, db, kAnomali)
	require.Len(t, a, 1)
	assert.Equal(t, "LONJAKAN_HARGA", a[0].Jenis)
	assert.Equal(t, "SIAGA", a[0].Severity)
	assert.Equal(t, "BARU", a[0].Status)
	assert.Equal(t, "CBM-KRT", *a[0].KomoditasKode)
	assert.Equal(t, "1171", *a[0].KabupatenKode)
	assert.Equal(t, "Pipeline ML", a[0].Sumber)
	assert.Nil(t, a[0].AturanID)
	assert.Contains(t, a[0].Pesan, "pipeline ML siklus "+c09)

	g := alertUntuk(t, db, kGagal)
	require.Len(t, g, 1, "kegagalan crawler nyata menjadi alert")
	assert.Equal(t, "ANOMALI_DATA", g[0].Jenis)
	assert.Nil(t, g[0].KabupatenKode, "sinyal tingkat sistem tanpa kabupaten")
	assert.Nil(t, g[0].KomoditasKode)

	tk := alertUntuk(t, db, kTekanan)
	require.Len(t, tk, 1)
	assert.Equal(t, "1107", *tk[0].KabupatenKode)
	assert.Nil(t, tk[0].KomoditasKode)

	// Ikut alur EWS: riwayat, peristiwa SSE, notifikasi (SIAGA ke atas sesuai bawaan), audit; tanpa kartu intervensi.
	assert.Equal(t, 4, hitung(t, db, `SELECT count(*) FROM ews_riwayat r JOIN ews_alert a ON a.id = r.alert_id
		WHERE a.sumber = 'Pipeline ML' AND r.catatan LIKE 'Dibuat oleh jembatan pipeline ML%'`))
	assert.Equal(t, 4, hitung(t, db, `SELECT count(*) FROM ews_event e JOIN ews_alert a ON a.id = e.alert_id WHERE a.sumber = 'Pipeline ML'`))
	assert.Positive(t, hitung(t, db, `SELECT count(*) FROM ews_notifikasi WHERE alert_id = ?`, g[0].ID), "alert sistem tanpa aturan tetap dinotifikasi")
	assert.Positive(t, hitung(t, db, `SELECT count(*) FROM ews_notifikasi WHERE alert_id = ?`, a[0].ID))
	assert.Zero(t, hitung(t, db, `SELECT count(*) FROM ews_notifikasi WHERE alert_id = ?`, tk[0].ID), "WASPADA di bawah tingkat minimum bawaan")
	assert.Equal(t, 4, hitung(t, db, `SELECT count(*) FROM audit_log WHERE entitas = 'ews_alert' AND aksi = 'BUAT_OTOMATIS' AND metadata->>'sumber' = 'Pipeline ML'`))
	assert.Zero(t, hitung(t, db, `SELECT count(*) FROM intervensi`), "alert pipeline tidak otomatis menjadi kartu intervensi")

	// Putaran ulang dalam siklus yang sama: tidak menulis apa pun.
	obs.hasil = nil
	r = jalankan(t, db, m, obs, now.Add(5*time.Minute))
	assert.Equal(t, 4, r.SudahDiterapkan)
	assert.Zero(t, r.Dibuat+r.Dinaikkan+r.Dilewati)
	assert.Empty(t, obs.hasil)

	// Siklus berikutnya dengan kondisi sama: tidak ada duplikat.
	tambahSiklus(t, m, c11, sinyalSiklus(c11)...)
	r = jalankan(t, db, m, obs, tsSiklus[c11].Add(time.Minute))
	assert.Equal(t, c11, r.CycleID)
	assert.Zero(t, r.Dibuat)
	assert.Equal(t, 4, r.Dilewati)
	assert.Equal(t, 4, hitung(t, db, `SELECT count(*) FROM ews_alert WHERE sumber = 'Pipeline ML'`))
	assert.Equal(t, 4, hitung(t, db, `SELECT count(*) FROM ews_sinyal_ml WHERE cycle_id = ? AND alert_id IS NOT NULL`, c11))
}

func TestJembatan_KritisMembukaKembaliTanpaCooldownTanpaTutupOtomatis(t *testing.T) {
	db := dbJembatan(t, "ml_jembatan_kritis")
	m := mongoBaru(t)
	obs := &pengamat{}

	tambahSiklus(t, m, c09,
		sinyal(c09, kAnomali, "SIAGA", "ANOMALI_HARGA", s("CBM-KRT"), s("1171"), f(12), true),
		sinyal(c09, kProyeksi, "WASPADA", "PROYEKSI_LONJAKAN", s("BWM"), s("1174"), f(6), true),
		sinyal(c09, kProvinsi, "WASPADA", "TEKANAN_INFLASI", nil, s("11"), f(62), true),
		sinyal(c09, "ANOMALI_HARGA:XYZ:1171:NAIK", "SIAGA", "ANOMALI_HARGA", s("XYZ"), s("1171"), f(15), true),
	)
	r := jalankan(t, db, m, obs, tsSiklus[c09].Add(time.Minute))
	assert.Equal(t, 3, r.Dibuat)
	assert.Equal(t, 1, r.Galat, "komoditas di luar master dilaporkan, sinyal lain tetap diterapkan")
	prov := alertUntuk(t, db, kProvinsi)
	require.Len(t, prov, 1)
	assert.Nil(t, prov[0].KabupatenKode, "provinsi 11 tidak punya kabupaten")
	assert.Contains(t, prov[0].Pesan, "Tingkat provinsi 11")

	// Pengguna mengakui alert SIAGA dan menutup alert WASPADA.
	a := alertUntuk(t, db, kAnomali)[0]
	require.NoError(t, db.Exec(`UPDATE ews_alert SET status = 'DIAKUI', diakui_pada = now() WHERE id = ?`, a.ID).Error)
	p := alertUntuk(t, db, kProyeksi)[0]
	require.NoError(t, db.Exec(`UPDATE ews_alert SET status = 'SELESAI', selesai_pada = ? WHERE id = ?`, tsSiklus[c09].Add(2*time.Minute), p.ID).Error)

	// Siklus berikutnya: anomali naik ke KRITIS (dibuka kembali ke BARU); proyeksi WASPADA masih ada (diredam cooldown).
	tambahSiklus(t, m, c11,
		sinyal(c11, kAnomali, "KRITIS", "ANOMALI_HARGA", s("CBM-KRT"), s("1171"), f(31), true),
		sinyal(c11, kProyeksi, "WASPADA", "PROYEKSI_LONJAKAN", s("BWM"), s("1174"), f(6), true),
	)
	obs.hasil = nil
	r = jalankan(t, db, m, obs, tsSiklus[c11].Add(time.Minute))
	assert.Equal(t, 1, r.Dinaikkan)
	assert.Equal(t, 1, r.Diredam, "WASPADA yang baru ditutup diredam 24 jam")
	assert.Equal(t, []string{"dinaikkan:KRITIS"}, obs.hasil)
	a2 := alertUntuk(t, db, kAnomali)
	require.Len(t, a2, 1)
	assert.Equal(t, "KRITIS", a2[0].Severity)
	assert.Equal(t, "BARU", a2[0].Status, "naik ke KRITIS membuka kembali ke BARU")
	assert.Equal(t, 1, hitung(t, db, `SELECT count(*) FROM ews_riwayat WHERE alert_id = ? AND catatan LIKE '%dibuka kembali%'`, a.ID))

	// Pengguna menutup alert KRITIS; kondisi masih KRITIS pada siklus berikut: alert baru tanpa cooldown.
	require.NoError(t, db.Exec(`UPDATE ews_alert SET status = 'SELESAI', selesai_pada = ? WHERE id = ?`, tsSiklus[c11].Add(5*time.Minute), a.ID).Error)
	tambahSiklus(t, m, c12, sinyal(c12, kAnomali, "KRITIS", "ANOMALI_HARGA", s("CBM-KRT"), s("1171"), f(33), true))
	r = jalankan(t, db, m, obs, tsSiklus[c12].Add(time.Minute))
	assert.Equal(t, 1, r.Dibuat, "KRITIS tidak kena cooldown")
	a3 := alertUntuk(t, db, kAnomali)
	require.Len(t, a3, 2)
	assert.Equal(t, "SELESAI", a3[0].Status)
	assert.Equal(t, "BARU", a3[1].Status)

	// Kondisi pulih (sinyal aktif=false): dicatat di riwayat, alert KRITIS tidak ditutup otomatis.
	tambahSiklus(t, m, c13, sinyal(c13, kAnomali, "INFO", "ANOMALI_HARGA", s("CBM-KRT"), s("1171"), nil, false))
	r = jalankan(t, db, m, obs, tsSiklus[c13].Add(time.Minute))
	assert.Equal(t, 1, r.Mereda)
	akhir := alertUntuk(t, db, kAnomali)
	assert.Equal(t, "BARU", akhir[1].Status, "tidak ditutup otomatis")
	assert.Equal(t, "KRITIS", akhir[1].Severity)
	assert.Equal(t, 1, hitung(t, db, `SELECT count(*) FROM ews_riwayat WHERE alert_id = ? AND catatan LIKE '%tidak lagi terdeteksi%'`, akhir[1].ID))

	// Siklus ekonomi 08-B tidak menghitung alert pipeline.
	assert.Zero(t, hitung(t, db, `SELECT count(*) FROM intervensi`))
}

// Alert jembatan terlihat di /v1/ews sesuai cakupan baris, dan lampiran ml menautkan sinyal ke alertnya.
func TestJembatan_TerlihatDiEndpointEWS(t *testing.T) {
	db := dbJembatan(t, "ml_jembatan_api")
	m := mongoBaru(t)
	isiStandar(t, m)
	jalankan(t, db, m, &pengamat{}, tsSiklus[c09].Add(time.Minute))
	api := routerUji(t, db, m.service(t))

	type alertRow struct {
		ID            string  `json:"id"`
		KabupatenKode string  `json:"kabupaten_kode"`
		Wilayah       string  `json:"wilayah"`
		Sumber        string  `json:"sumber"`
		KomoditasKode *string `json:"komoditas_kode"`
	}
	w := api.get(t, "/v1/ews?limit=100", emailAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var rows []alertRow
	require.NoError(t, json.Unmarshal(baca(t, w).Data, &rows))
	sistem := ""
	for _, r := range rows {
		if r.Sumber == "Pipeline ML" && r.KabupatenKode == "" {
			sistem = r.ID
			assert.Equal(t, "PROVINSI ACEH", r.Wilayah)
		}
	}
	require.NotEmpty(t, sistem, "alert sumber data tingkat sistem tampil untuk peran provinsi")

	w = api.get(t, "/v1/ews?limit=100", emailKoordinator)
	require.NoError(t, json.Unmarshal(baca(t, w).Data, &rows))
	for _, r := range rows {
		assert.Equal(t, "1171", r.KabupatenKode, "peran berkabupaten hanya melihat kabupatennya")
	}
	assert.NotEmpty(t, rows)
	assert.Equal(t, http.StatusNotFound, api.get(t, "/v1/ews/"+sistem, emailKoordinator).Code)

	// Detail alert: lampiran ml memuat sinyal asal beserta tautan alertnya.
	l := ml(t, api.get(t, "/v1/ews/"+sistem, emailAdmin))
	require.NotNil(t, l.Ews)
	require.Len(t, l.Ews.Sinyal, 1)
	assert.Equal(t, kGagal, l.Ews.Sinyal[0].SinyalKunci)
	require.NotNil(t, l.Ews.Sinyal[0].Alert)
	assert.Equal(t, sistem, l.Ews.Sinyal[0].Alert.ID)
	assert.Equal(t, "BARU", l.Ews.Sinyal[0].Alert.Status)
}
