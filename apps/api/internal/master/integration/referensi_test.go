package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/master"
	masterRoutes "github.com/garda-inflasi/api/internal/master/routes"
	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/seed"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Data referensi master (04-B/04-C): kecamatan, satuan, pedagang, detail, riwayat versi, dan hapus. Memakai basis
// data sendiri (bukan suite bersama) karena aksi massal pada tes lain menambah versi komoditas yang sama.

const refDBName = "master_referensi"

var (
	refOnce   sync.Once
	refShared *suite
)

func refSuite(t *testing.T) *suite {
	t.Helper()
	refOnce.Do(func() {
		db := pg.CreateDatabase(t, refDBName)
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedDemo(t.Context(), db))
		refShared = bangunSuite(db)
	})
	require.NotNil(t, refShared, "suite referensi gagal dibuat")
	return refShared
}

func bangunSuite(db *gorm.DB) *suite {
	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	masterRoutes.Register(v1, &masterRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

// objekData membaca data respons berupa satu objek.
func objekData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

// kodeEnvelope membaca field code dari envelope galat.
func kodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var res struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Code
}

func pesanEnvelope(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var res struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Message
}

func (s *suite) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	require.NoError(t, s.db.Exec(sql, args...).Error, sql)
}

func (s *suite) hitung(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw(sql, args...).Scan(&n).Error, sql)
	return n
}

func (s *suite) buatKomoditas(t *testing.T, kode string) {
	t.Helper()
	s.exec(t, `INSERT INTO komoditas (kode, nama, kategori, satuan, harga_acuan, shelf_life_hari)
		VALUES (?, ?, 'Uji', 'kg', 10000, 30)`, kode, "Komoditas "+kode)
}

func (s *suite) buatPasar(t *testing.T, id string) {
	t.Helper()
	s.exec(t, `INSERT INTO pasar (kode, nama, kabupaten_kode, lat, lng) VALUES (?, ?, '1171', 5.55, 95.32)`, id, "Pasar "+id)
}

func (s *suite) entriUntuk(t *testing.T, komoditas, pasar, pedagang string) {
	t.Helper()
	var kodePedagang *string
	if pedagang != "" {
		kodePedagang = &pedagang
	}
	s.exec(t, `INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada, kode_pedagang)
		VALUES (?, ?, 'KONSUMEN', 10000, 'kg', now(), ?)`, komoditas, pasar, kodePedagang)
}

// usulanMenunggu membuat usulan MENUNGGU untuk entitas dan mengembalikan ID-nya.
func (s *suite) usulanMenunggu(t *testing.T, entitas, kode string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.db.Raw(`
		INSERT INTO perubahan_master (entitas, entitas_id, versi, perubahan, status, diajukan_oleh_id, jenis)
		VALUES (?, ?, 2, '{"nama": "Nama baru"}'::jsonb, 'MENUNGGU', ?, 'UBAH') RETURNING id`,
		entitas, kode, uid("manajer_data")).Scan(&id).Error)
	return id
}

func (s *suite) hitungAudit(t *testing.T, entitas, kode string) int64 {
	t.Helper()
	return s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'HAPUS_MASTER' AND entitas = ? AND entitas_id = ?`, entitas, kode)
}

func TestReferensi_Otorisasi(t *testing.T) {
	s := refSuite(t)
	for _, path := range []string{
		"/v1/master/kecamatan", "/v1/master/kecamatan/1171010", "/v1/master/satuan", "/v1/master/pedagang",
		"/v1/master/komoditas/BRS-MED", "/v1/master/komoditas/BRS-MED/versi", "/v1/master/wilayah/1171", "/v1/master/pasar/PSR-1171-01",
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzn, nil).Code)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
		})
	}
}

type kecamatanRow struct {
	Kode          string  `json:"kode"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	KodeDagri     *string `json:"kode_dagri"`
}

func TestReferensi_Kecamatan(t *testing.T) {
	s := refSuite(t)
	seed, err := master.Kecamatan()
	require.NoError(t, err)
	const path = "/v1/master/kecamatan"

	t.Run("daftar penuh terurut menurut kode dengan total di header", func(t *testing.T) {
		w := s.call(t, http.MethodGet, path, emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, strconv.Itoa(len(seed)), w.Header().Get("X-Total-Count"))
		got := rows[kecamatanRow](t, w)
		require.Len(t, got, len(seed))
		assert.True(t, sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Kode < got[j].Kode }))
		for i, k := range got {
			assert.Equal(t, seed[i].Kode, k.Kode)
			assert.NotEmpty(t, k.KabupatenNama, k.Kode)
		}
	})

	t.Run("filter kabupaten", func(t *testing.T) {
		w := s.call(t, http.MethodGet, path+"?kabupaten_kode=1171", emailLihat, nil)
		got := rows[kecamatanRow](t, w)
		assert.Len(t, got, 9)
		assert.Equal(t, "9", w.Header().Get("X-Total-Count"))
		for _, k := range got {
			assert.Equal(t, "1171", k.KabupatenKode)
			assert.Equal(t, "KOTA BANDA ACEH", k.KabupatenNama)
		}

		w = s.call(t, http.MethodGet, path+"?kabupaten_kode=9999", emailLihat, nil)
		assert.Empty(t, rows[kecamatanRow](t, w))
		assert.Equal(t, "0", w.Header().Get("X-Total-Count"))
	})

	t.Run("pencarian menurut kode atau nama tanpa peka huruf besar-kecil", func(t *testing.T) {
		for _, q := range []string{"meuraxa", "MEURAXA", "1171010", "euraxa"} {
			w := s.call(t, http.MethodGet, path+"?q="+q, emailLihat, nil)
			got := rows[kecamatanRow](t, w)
			require.Len(t, got, 1, q)
			assert.Equal(t, "1171010", got[0].Kode)
			assert.Equal(t, "Meuraxa", got[0].Nama)
			assert.Equal(t, "1", w.Header().Get("X-Total-Count"))
		}
		// Gabungan dengan filter kabupaten: kata kunci di kabupaten lain tidak muncul.
		got := rows[kecamatanRow](t, s.call(t, http.MethodGet, path+"?kabupaten_kode=1101&q=meuraxa", emailLihat, nil))
		assert.Empty(t, got)
	})

	t.Run("wildcard LIKE dianggap literal", func(t *testing.T) {
		for _, q := range []string{"%25", "_", "%25%25"} {
			w := s.call(t, http.MethodGet, path+"?q="+q, emailLihat, nil)
			assert.Empty(t, rows[kecamatanRow](t, w), q)
			assert.Equal(t, "0", w.Header().Get("X-Total-Count"), q)
		}
	})

	t.Run("paginasi menelusuri semua baris tanpa dobel", func(t *testing.T) {
		const limit = 50
		seen := map[string]bool{}
		var urutan []string
		for page := 1; ; page++ {
			w := s.call(t, http.MethodGet, path+"?limit="+strconv.Itoa(limit)+"&page="+strconv.Itoa(page), emailLihat, nil)
			require.Equal(t, strconv.Itoa(len(seed)), w.Header().Get("X-Total-Count"))
			chunk := rows[kecamatanRow](t, w)
			if len(chunk) == 0 {
				assert.Greater(t, page, 1)
				break
			}
			assert.LessOrEqual(t, len(chunk), limit)
			for _, k := range chunk {
				assert.False(t, seen[k.Kode], "dobel antar halaman: %s", k.Kode)
				seen[k.Kode] = true
				urutan = append(urutan, k.Kode)
			}
			require.Less(t, page, 20, "paginasi tidak berhenti")
		}
		assert.Len(t, seen, len(seed))
		assert.True(t, sort.StringsAreSorted(urutan), "urutan antar halaman tetap menurut kode")
	})

	t.Run("parameter tidak valid ditolak", func(t *testing.T) {
		for _, query := range []string{"?limit=501", "?limit=abc", "?page=-1&limit=5", "?limit=-1"} {
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, path+query, emailLihat, nil).Code, query)
		}
	})

	t.Run("detail", func(t *testing.T) {
		w := s.call(t, http.MethodGet, path+"/1171010", emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		k := objekData[kecamatanRow](t, w)
		assert.Equal(t, "1171010", k.Kode)
		assert.Equal(t, "Meuraxa", k.Nama)
		assert.Equal(t, "1171", k.KabupatenKode)
		assert.Equal(t, "KOTA BANDA ACEH", k.KabupatenNama)
		require.NotNil(t, k.KodeDagri)
		assert.Equal(t, "117103", *k.KodeDagri)
	})

	t.Run("kode tak dikenal atau salah bentuk dijawab 404 tanpa membocorkan sebabnya", func(t *testing.T) {
		for _, kode := range []string{"1171999", "0000000", "117101", "11710100", "abc", "1171O10", "1171010%27", "%00", "1171010%20"} {
			w := s.call(t, http.MethodGet, path+"/"+kode, emailLihat, nil)
			assert.Equal(t, http.StatusNotFound, w.Code, kode)
			assert.Equal(t, "404", kodeEnvelope(t, w), kode)
			assert.Equal(t, "Kecamatan tidak ditemukan", pesanEnvelope(t, w), kode)
		}
	})
}

func TestReferensi_Satuan(t *testing.T) {
	s := refSuite(t)
	// Harapan dihitung dari basis data (tes lain menambah komoditas uji), bukan dari seed.
	var perSatuanRows []struct {
		Satuan string `gorm:"column:satuan"`
		N      int64  `gorm:"column:n"`
	}
	require.NoError(t, s.db.Raw(`SELECT satuan, COUNT(*) AS n FROM komoditas GROUP BY satuan`).Scan(&perSatuanRows).Error)
	perSatuan := map[string]int64{}
	for _, r := range perSatuanRows {
		perSatuan[r.Satuan] = r.N
	}
	require.Positive(t, perSatuan["kg"])
	require.Positive(t, perSatuan["liter"])

	w := s.call(t, http.MethodGet, "/v1/master/satuan", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := rows[struct {
		Kode            string `json:"kode"`
		Nama            string `json:"nama"`
		JumlahKomoditas int64  `json:"jumlah_komoditas"`
	}](t, w)
	assert.Equal(t, strconv.Itoa(len(master.Satuan())), w.Header().Get("X-Total-Count"))
	require.Len(t, got, len(master.Satuan()))
	byKode := map[string]int64{}
	for _, row := range got {
		byKode[row.Kode] = row.JumlahKomoditas
		assert.NotEmpty(t, row.Nama)
	}
	assert.Equal(t, perSatuan["kg"], byKode["kg"])
	assert.Equal(t, perSatuan["liter"], byKode["liter"])
	assert.Equal(t, "kg", got[0].Kode, "terurut menurut kode")

	got = rows[struct {
		Kode            string `json:"kode"`
		Nama            string `json:"nama"`
		JumlahKomoditas int64  `json:"jumlah_komoditas"`
	}](t, s.call(t, http.MethodGet, "/v1/master/satuan?q=LIT", emailLihat, nil))
	require.Len(t, got, 1)
	assert.Equal(t, "liter", got[0].Kode)
}

type pedagangRow struct {
	ID          string  `json:"id"`
	PasarID     string  `json:"pasar_id"`
	PasarNama   string  `json:"pasar_nama"`
	Kode        string  `json:"kode"`
	Nama        *string `json:"nama"`
	Aktif       bool    `json:"aktif"`
	JumlahEntri int64   `json:"jumlah_entri"`
}

func TestReferensi_Pedagang(t *testing.T) {
	s := refSuite(t)
	const path = "/v1/master/pedagang"

	// Entri harga: PDG-A dua kali, PDG-B sekali (di PSR-1171-01), PDG-C sekali (PSR-1108-01); PDG-Z belum punya entri.
	s.entriUntuk(t, "BWM", "PSR-1171-01", "PDG-A")
	s.entriUntuk(t, "CBM-KRT", "PSR-1171-01", "PDG-A")
	s.entriUntuk(t, "BWM", "PSR-1171-01", "PDG-B")
	s.entriUntuk(t, "BWM", "PSR-1108-01", "PDG-C")
	// Kode yang sama di pasar lain tidak dihitung bersama.
	s.entriUntuk(t, "BWM", "PSR-1171-02", "PDG-A")
	s.exec(t, `INSERT INTO pedagang (pasar_id, kode, nama, aktif) VALUES
		('PSR-1171-01', 'PDG-A', 'Cut Aminah', true),
		('PSR-1171-01', 'PDG-B', NULL, false),
		('PSR-1108-01', 'PDG-C', 'Teuku Daud', true),
		('PSR-1171-02', 'PDG-Z', 'Tanpa Entri', true)`)

	t.Run("daftar terurut menurut pasar_id lalu kode, dengan jumlah entri", func(t *testing.T) {
		w := s.call(t, http.MethodGet, path, emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "4", w.Header().Get("X-Total-Count"))
		got := rows[pedagangRow](t, w)
		require.Len(t, got, 4)

		urutan := make([]string, len(got))
		for i, p := range got {
			urutan[i] = p.PasarID + "/" + p.Kode
			_, err := strconv.ParseInt(p.ID, 10, 64)
			assert.NoError(t, err, "id berupa string desimal: %q", p.ID)
		}
		assert.Equal(t, []string{"PSR-1108-01/PDG-C", "PSR-1171-01/PDG-A", "PSR-1171-01/PDG-B", "PSR-1171-02/PDG-Z"}, urutan)

		assert.Equal(t, "Pasar Induk Lambaro", got[0].PasarNama)
		assert.EqualValues(t, 1, got[0].JumlahEntri)
		assert.EqualValues(t, 2, got[1].JumlahEntri, "dua entri PDG-A di PSR-1171-01 (entri PDG-A di pasar lain tidak ikut)")
		assert.EqualValues(t, 1, got[2].JumlahEntri)
		assert.EqualValues(t, 0, got[3].JumlahEntri)
		require.NotNil(t, got[1].Nama)
		assert.Equal(t, "Cut Aminah", *got[1].Nama)
		assert.True(t, got[1].Aktif)
		assert.False(t, got[2].Aktif)
	})

	t.Run("nama yang kosong tidak ditulis", func(t *testing.T) {
		var res struct {
			Data []map[string]any `json:"data"`
		}
		w := s.call(t, http.MethodGet, path+"?pasar_id=PSR-1171-01&q=pdg-b", emailLihat, nil)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		require.Len(t, res.Data, 1)
		_, ada := res.Data[0]["nama"]
		assert.False(t, ada, "nama dihilangkan saat belum diisi")
	})

	t.Run("filter pasar dan aktif", func(t *testing.T) {
		got := rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?pasar_id=PSR-1171-01", emailLihat, nil))
		assert.Len(t, got, 2)

		got = rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?aktif=true", emailLihat, nil))
		assert.Len(t, got, 3)
		for _, p := range got {
			assert.True(t, p.Aktif)
		}
		got = rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?aktif=false", emailLihat, nil))
		require.Len(t, got, 1)
		assert.Equal(t, "PDG-B", got[0].Kode)

		got = rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?pasar_id=PSR-1171-01&aktif=true", emailLihat, nil))
		require.Len(t, got, 1)
		assert.Equal(t, "PDG-A", got[0].Kode)

		assert.Empty(t, rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?pasar_id=NGAWUR", emailLihat, nil)))
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, path+"?aktif=mungkin", emailLihat, nil).Code)
	})

	t.Run("pencarian menurut kode atau nama", func(t *testing.T) {
		got := rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?q=AMINAH", emailLihat, nil))
		require.Len(t, got, 1)
		assert.Equal(t, "PDG-A", got[0].Kode)

		got = rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?q=pdg-", emailLihat, nil))
		assert.Len(t, got, 4)

		assert.Empty(t, rows[pedagangRow](t, s.call(t, http.MethodGet, path+"?q=%25", emailLihat, nil)))
	})

	t.Run("paginasi", func(t *testing.T) {
		w := s.call(t, http.MethodGet, path+"?limit=3&page=2", emailLihat, nil)
		assert.Equal(t, "4", w.Header().Get("X-Total-Count"))
		got := rows[pedagangRow](t, w)
		require.Len(t, got, 1)
		assert.Equal(t, "PDG-Z", got[0].Kode)
	})
}

type versiRow struct {
	ID            string     `json:"id"`
	Versi         int        `json:"versi"`
	Nama          string     `json:"nama"`
	Kategori      string     `json:"kategori"`
	Satuan        string     `json:"satuan"`
	HargaAcuan    int        `json:"harga_acuan"`
	ShelfLifeHari int        `json:"shelf_life_hari"`
	Pilot         bool       `json:"pilot"`
	BerlakuDari   time.Time  `json:"berlaku_dari"`
	BerlakuSampai *time.Time `json:"berlaku_sampai"`
	PerubahanID   *string    `json:"perubahan_id"`
}

type komoditasDetail struct {
	komoditasRow
	Kategori     string     `json:"kategori"`
	Satuan       string     `json:"satuan"`
	RiwayatVersi []versiRow `json:"riwayat_versi"`
}

func TestReferensi_KomoditasDetailDanRiwayatVersi(t *testing.T) {
	s := refSuite(t)

	t.Run("komoditas seed punya satu versi yang masih berlaku", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/komoditas/BRS-MED", emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		d := objekData[komoditasDetail](t, w)
		assert.Equal(t, "BRS-MED", d.Kode)
		assert.Equal(t, "Beras medium", d.Nama)
		assert.Equal(t, "kg", d.Satuan)
		assert.Equal(t, "Pangan pokok", d.Kategori)
		assert.Equal(t, 13900, d.HargaAcuan)
		assert.True(t, d.Pilot)
		assert.Equal(t, 1, d.Versi)
		assert.Equal(t, "AKTIF", d.Status)
		require.Len(t, d.RiwayatVersi, 1)
		v := d.RiwayatVersi[0]
		assert.Equal(t, 1, v.Versi)
		assert.Nil(t, v.BerlakuSampai, "versi yang berlaku tidak punya akhir")
		assert.Nil(t, v.PerubahanID)
		assert.False(t, v.BerlakuDari.IsZero())
		_, err := strconv.ParseInt(v.ID, 10, 64)
		assert.NoError(t, err)
	})

	t.Run("bentuk baris sama dengan daftar", func(t *testing.T) {
		var detail struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(s.call(t, http.MethodGet, "/v1/master/komoditas/BRS-MED", emailLihat, nil).Body.Bytes(), &detail))
		var list struct {
			Data []map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(s.call(t, http.MethodGet, "/v1/master/komoditas?q=BRS-MED", emailLihat, nil).Body.Bytes(), &list))
		require.Len(t, list.Data, 1)
		for kunci, nilai := range list.Data[0] {
			assert.Equal(t, nilai, detail.Data[kunci], kunci)
		}
		assert.Contains(t, detail.Data, "riwayat_versi")
	})

	t.Run("UBAH harga_acuan dan versi menambah versi, versi lama berakhir saat yang baru mulai", func(t *testing.T) {
		usulan := s.usulanMenunggu(t, "komoditas", "TLR")
		require.NoError(t, s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`SELECT set_config('garda.perubahan_master_id', ?, true)`, strconv.FormatInt(usulan, 10)).Error; err != nil {
				return err
			}
			return tx.Exec(`UPDATE komoditas SET harga_acuan = 31000, versi = 2 WHERE kode = 'TLR'`).Error
		}))
		// Perubahan status saja bukan versi baru.
		s.exec(t, `UPDATE komoditas SET status = 'MENUNGGU_PERSETUJUAN' WHERE kode = 'TLR'`)
		s.exec(t, `UPDATE komoditas SET status = 'AKTIF' WHERE kode = 'TLR'`)

		d := objekData[komoditasDetail](t, s.call(t, http.MethodGet, "/v1/master/komoditas/TLR", emailLihat, nil))
		assert.Equal(t, 31000, d.HargaAcuan)
		assert.Equal(t, 2, d.Versi)
		require.Len(t, d.RiwayatVersi, 2, "terbaru dulu; ubah status tidak menambah versi")
		baru, lama := d.RiwayatVersi[0], d.RiwayatVersi[1]
		assert.Equal(t, 2, baru.Versi)
		assert.Equal(t, 31000, baru.HargaAcuan)
		assert.Nil(t, baru.BerlakuSampai)
		require.NotNil(t, baru.PerubahanID)
		assert.Equal(t, strconv.FormatInt(usulan, 10), *baru.PerubahanID, "versi menunjuk usulan yang menghasilkannya")
		assert.Equal(t, 1, lama.Versi)
		assert.Equal(t, 29500, lama.HargaAcuan)
		require.NotNil(t, lama.BerlakuSampai)
		assert.True(t, lama.BerlakuSampai.Equal(baru.BerlakuDari), "berlaku_sampai versi lama = berlaku_dari versi baru")
		assert.True(t, baru.BerlakuDari.After(lama.BerlakuDari))
		assert.Nil(t, lama.PerubahanID)
	})

	t.Run("kode tak dikenal atau salah bentuk dijawab 404", func(t *testing.T) {
		for _, kode := range []string{"NGAWUR", "brs-med", "%00", "a%0Ab", "BRS-MED%27%20OR%201=1"} {
			w := s.call(t, http.MethodGet, "/v1/master/komoditas/"+kode, emailLihat, nil)
			assert.Equal(t, http.StatusNotFound, w.Code, kode)
			assert.Equal(t, "Komoditas tidak ditemukan", pesanEnvelope(t, w), kode)
		}
	})
}

func TestReferensi_VersiKomoditasPadaWaktu(t *testing.T) {
	s := refSuite(t)

	// UJI-VER: versi 3 berlaku sejak sekarang (ditulis pemicu); versi 1 dan 2 disisipkan dengan waktu tetap.
	// Versi 2 mulai 2025-06-01T18:00Z = 2025-06-02 01:00 WIB, jadi akhir hari 2025-06-01 menurut WIB (16:59:59Z)
	// masih versi 1, sedangkan akhir hari menurut UTC sudah versi 2: pembeda WIB dan UTC.
	s.exec(t, `INSERT INTO komoditas (kode, nama, kategori, satuan, harga_acuan, shelf_life_hari, versi)
		VALUES ('UJI-VER', 'Komoditas uji versi', 'Uji', 'kg', 10000, 30, 3)`)
	s.exec(t, `INSERT INTO komoditas_versi (komoditas_kode, versi, nama, kategori, satuan, harga_acuan, shelf_life_hari, pilot, berlaku_dari) VALUES
		('UJI-VER', 1, 'Nama v1', 'Uji', 'kg', 8000, 30, false, '2025-01-01T00:00:00Z'),
		('UJI-VER', 2, 'Nama v2', 'Uji', 'kg', 9000, 30, false, '2025-06-01T18:00:00Z')`)
	const path = "/v1/master/komoditas/UJI-VER/versi"

	t.Run("tanpa pada: seluruh riwayat terbaru dulu dengan berlaku_sampai berantai", func(t *testing.T) {
		w := s.call(t, http.MethodGet, path, emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := rows[versiRow](t, w)
		require.Len(t, got, 3)
		assert.Equal(t, []int{3, 2, 1}, []int{got[0].Versi, got[1].Versi, got[2].Versi})
		assert.Nil(t, got[0].BerlakuSampai)
		require.NotNil(t, got[1].BerlakuSampai)
		assert.True(t, got[1].BerlakuSampai.Equal(got[0].BerlakuDari))
		require.NotNil(t, got[2].BerlakuSampai)
		assert.True(t, got[2].BerlakuSampai.Equal(time.Date(2025, 6, 1, 18, 0, 0, 0, time.UTC)))

		// Sama dengan riwayat_versi pada detail.
		d := objekData[komoditasDetail](t, s.call(t, http.MethodGet, "/v1/master/komoditas/UJI-VER", emailLihat, nil))
		assert.Equal(t, got, d.RiwayatVersi)
	})

	cases := []struct {
		pada  string
		versi int
		harga int
	}{
		{"2000-01-01", 1, 8000}, // lebih tua dari riwayat: jatuh ke versi paling awal
		{"2024-12-31T23:59:59Z", 1, 8000},
		{"2025-01-01T00:00:00Z", 1, 8000}, // tepat saat versi 1 mulai
		{"2025-06-01T17:59:59Z", 1, 8000},
		{"2025-06-01T18:00:00Z", 2, 9000}, // tepat saat versi 2 mulai
		{"2025-06-02T00:59:59+07:00", 1, 8000},
		{"2025-06-02T01:00:00+07:00", 2, 9000},
		{"2025-06-01", 1, 8000}, // akhir hari WIB 16:59:59Z, versi 2 belum mulai
		{"2025-06-02", 2, 9000},
		{"2999-12-31", 3, 10000},
	}
	for _, tc := range cases {
		t.Run("pada="+tc.pada, func(t *testing.T) {
			w := s.call(t, http.MethodGet, path+"?pada="+url.QueryEscape(tc.pada), emailLihat, nil)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			v := objekData[versiRow](t, w)
			assert.Equal(t, tc.versi, v.Versi)
			assert.Equal(t, tc.harga, v.HargaAcuan)
		})
	}

	t.Run("versi yang dipilih memuat berlaku_sampai", func(t *testing.T) {
		v := objekData[versiRow](t, s.call(t, http.MethodGet, path+"?pada=2025-06-01", emailLihat, nil))
		require.NotNil(t, v.BerlakuSampai)
		assert.True(t, v.BerlakuSampai.Equal(time.Date(2025, 6, 1, 18, 0, 0, 0, time.UTC)))
		v = objekData[versiRow](t, s.call(t, http.MethodGet, path+"?pada=2999-12-31", emailLihat, nil))
		assert.Nil(t, v.BerlakuSampai)
	})

	t.Run("pada tidak valid dijawab 400", func(t *testing.T) {
		for _, pada := range []string{"kemarin", "2026-02-30", "2026-13-01", "01-03-2026", "2025-06-01T10:00:00", "2025-06-01+10:00", "1772000000"} {
			w := s.call(t, http.MethodGet, path+"?pada="+url.QueryEscape(pada), emailLihat, nil)
			assert.Equal(t, http.StatusBadRequest, w.Code, pada)
		}
	})

	t.Run("komoditas tak dikenal dijawab 404", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/master/komoditas/NGAWUR/versi", emailLihat, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/master/komoditas/NGAWUR/versi?pada=2025-06-01", emailLihat, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/master/komoditas/%00/versi?pada=2025-06-01", emailLihat, nil).Code)
	})
}

type pasarRow struct {
	ID            idx.ID  `json:"id"`
	Kode          string  `json:"kode"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KecamatanKode *string `json:"kecamatan_kode"`
	KecamatanNama *string `json:"kecamatan_nama"`
	Versi         int     `json:"versi"`
}

func TestReferensi_WilayahDanPasarDetail(t *testing.T) {
	s := refSuite(t)
	s.exec(t, `UPDATE pasar SET kecamatan_kode = '1171030' WHERE kode = 'PSR-1171-01'`) // Kuta Alam

	t.Run("wilayah menyertakan cacah kecamatan dan pasar", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/wilayah/1171", emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		d := objekData[struct {
			Kode            string  `json:"kode"`
			Nama            string  `json:"nama"`
			Jenis           string  `json:"jenis"`
			Ibukota         string  `json:"ibukota"`
			Lat             float64 `json:"lat"`
			ProvinsiKode    string  `json:"provinsi_kode"`
			JumlahKecamatan int64   `json:"jumlah_kecamatan"`
			JumlahPasar     int64   `json:"jumlah_pasar"`
		}](t, w)
		assert.Equal(t, "1171", d.Kode)
		assert.Equal(t, "KOTA BANDA ACEH", d.Nama)
		assert.Equal(t, "Kota", d.Jenis)
		assert.Equal(t, "11", d.ProvinsiKode)
		assert.EqualValues(t, 9, d.JumlahKecamatan)
		assert.Equal(t, s.hitung(t, `SELECT COUNT(*) FROM pasar WHERE kabupaten_kode = '1171'`), d.JumlahPasar)
		assert.GreaterOrEqual(t, d.JumlahPasar, int64(2))
	})

	t.Run("wilayah tak dikenal atau salah bentuk dijawab 404", func(t *testing.T) {
		for _, kode := range []string{"9999", "11", "abc", "11711", "1171%27", "%00"} {
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/master/wilayah/"+kode, emailLihat, nil).Code, kode)
		}
	})

	t.Run("pasar menyertakan nama kabupaten dan kecamatan serta cacah pedagang", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/pasar/PSR-1171-01", emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		d := objekData[struct {
			pasarRow
			RadiusMeter    int    `json:"radius_meter"`
			Aktif          bool   `json:"aktif"`
			KabupatenNama  string `json:"kabupaten_nama"`
			JumlahPedagang int64  `json:"jumlah_pedagang"`
		}](t, w)
		assert.Positive(t, d.ID.Int64())
		assert.Equal(t, "PSR-1171-01", d.Kode)
		assert.Equal(t, "Pasar Peunayong", d.Nama)
		assert.Equal(t, "KOTA BANDA ACEH", d.KabupatenNama)
		require.NotNil(t, d.KecamatanKode)
		assert.Equal(t, "1171030", *d.KecamatanKode)
		require.NotNil(t, d.KecamatanNama)
		assert.Equal(t, "Kuta Alam", *d.KecamatanNama)
		assert.Equal(t, 1, d.Versi)
		assert.Equal(t, 400, d.RadiusMeter)
		assert.True(t, d.Aktif)
		assert.Equal(t, s.hitung(t, `SELECT COUNT(*) FROM pedagang WHERE pasar_id = 'PSR-1171-01'`), d.JumlahPedagang)
	})

	t.Run("pasar tanpa kecamatan: kunci ada dengan nilai null", func(t *testing.T) {
		var res struct {
			Data map[string]any `json:"data"`
		}
		w := s.call(t, http.MethodGet, "/v1/master/pasar/PSR-1171-02", emailLihat, nil)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		for _, kunci := range []string{"kecamatan_kode", "kecamatan_nama"} {
			nilai, ada := res.Data[kunci]
			assert.True(t, ada, kunci)
			assert.Nil(t, nilai, kunci)
		}
	})

	t.Run("pasar tak dikenal atau salah bentuk dijawab 404", func(t *testing.T) {
		for _, kode := range []string{"NGAWUR", "psr-1171-01", "%00", "PSR-1171-01%27"} {
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/master/pasar/"+kode, emailLihat, nil).Code, kode)
		}
	})

	t.Run("daftar pasar memuat kecamatan_kode, kecamatan_nama, dan versi", func(t *testing.T) {
		got := rows[pasarRow](t, s.call(t, http.MethodGet, "/v1/master/pasar?kabupaten_kode=1171&limit=500", emailLihat, nil))
		byID := map[string]pasarRow{}
		for _, p := range got {
			byID[p.Kode] = p
			assert.Equal(t, 1, p.Versi, p.Kode)
		}
		peunayong := byID["PSR-1171-01"]
		require.NotNil(t, peunayong.KecamatanKode)
		assert.Equal(t, "1171030", *peunayong.KecamatanKode)
		require.NotNil(t, peunayong.KecamatanNama)
		assert.Equal(t, "Kuta Alam", *peunayong.KecamatanNama)
		lain := byID["PSR-1171-02"]
		assert.Nil(t, lain.KecamatanKode)
		assert.Nil(t, lain.KecamatanNama)

		// Pencarian dan paginasi tetap berfungsi dengan kolom tambahan.
		w := s.call(t, http.MethodGet, "/v1/master/pasar?q=peunayong", emailLihat, nil)
		assert.Equal(t, "1", w.Header().Get("X-Total-Count"))
		cari := rows[pasarRow](t, w)
		require.Len(t, cari, 1)
		require.NotNil(t, cari[0].KecamatanNama)
	})
}

func TestReferensi_Hapus(t *testing.T) {
	s := refSuite(t)

	t.Run("otorisasi", func(t *testing.T) {
		s.buatKomoditas(t, "UJI-IZIN")
		s.buatPasar(t, "PSR-1171-90")
		for _, path := range []string{"/v1/master/komoditas/UJI-IZIN", "/v1/master/pasar/PSR-1171-90"} {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodDelete, path, "", nil).Code, path)
			// Hanya master.setujui: pengaju, pelihat, dan peran tanpa izin master ditolak.
			for _, email := range []string{emailAjukan, emailManajer, emailLihat, emailTanpaIzn} {
				assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodDelete, path, email, nil).Code, path+" "+email)
			}
		}
		assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas WHERE kode = 'UJI-IZIN'`), "403 tidak menghapus")
		assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM pasar WHERE kode = 'PSR-1171-90'`), "403 tidak menghapus")
		assert.Zero(t, s.hitungAudit(t, "komoditas", "UJI-IZIN"))
	})

	t.Run("komoditas tanpa rujukan dihapus: 204, versi ikut hilang, audit tercatat", func(t *testing.T) {
		s.buatKomoditas(t, "UJI-HPS")
		require.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas_versi WHERE komoditas_kode = 'UJI-HPS'`))

		w := s.call(t, http.MethodDelete, "/v1/master/komoditas/UJI-HPS", emailSetujui, nil)
		require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM komoditas WHERE kode = 'UJI-HPS'`))
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM komoditas_versi WHERE komoditas_kode = 'UJI-HPS'`), "riwayat versi ikut terhapus (CASCADE)")
		assert.EqualValues(t, 1, s.hitungAudit(t, "komoditas", "UJI-HPS"))
		var jejak struct {
			AktorID string `gorm:"column:aktor_id"`
			Nama    string `gorm:"column:nama"`
			Harga   int    `gorm:"column:harga"`
			Hasil   string `gorm:"column:hasil"`
		}
		require.NoError(t, s.db.Raw(`SELECT aktor_id::text AS aktor_id, sebelum->>'nama' AS nama, (sebelum->>'harga_acuan')::int AS harga, hasil::text AS hasil
			FROM audit_log WHERE aksi = 'HAPUS_MASTER' AND entitas = 'komoditas' AND entitas_id = 'UJI-HPS'`).Scan(&jejak).Error)
		assert.Equal(t, uid("strategis_bps"), jejak.AktorID)
		assert.Equal(t, "Komoditas UJI-HPS", jejak.Nama, "Sebelum memuat salinan baris")
		assert.Equal(t, 10000, jejak.Harga)

		// Kedua kali: sudah tidak ada.
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodDelete, "/v1/master/komoditas/UJI-HPS", emailFMIPA, nil).Code)
		assert.EqualValues(t, 1, s.hitungAudit(t, "komoditas", "UJI-HPS"), "404 tidak menambah audit")
	})

	t.Run("komoditas yang dirujuk data lain: 409 MASTER_DIRUJUK dan tidak berubah", func(t *testing.T) {
		s.buatKomoditas(t, "UJI-ENTRI")
		s.entriUntuk(t, "UJI-ENTRI", "PSR-1171-01", "")
		s.buatKomoditas(t, "UJI-BATAS")
		s.exec(t, `INSERT INTO batas_harga (komoditas_kode, batas_min, batas_maks, berlaku_dari) VALUES ('UJI-BATAS', 1000, 5000, current_date)`)

		for _, kode := range []string{"UJI-ENTRI", "UJI-BATAS"} {
			w := s.call(t, http.MethodDelete, "/v1/master/komoditas/"+kode, emailSetujui, nil)
			require.Equal(t, http.StatusConflict, w.Code, kode+": "+w.Body.String())
			assert.Equal(t, "MASTER_DIRUJUK", kodeEnvelope(t, w))
			assert.Contains(t, pesanEnvelope(t, w), "arsipkan komoditas")
			assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas WHERE kode = ?`, kode))
			assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas_versi WHERE komoditas_kode = ?`, kode), "versi tetap ada")
			assert.Zero(t, s.hitungAudit(t, "komoditas", kode), "penghapusan yang ditolak tidak dicatat sebagai HAPUS_MASTER")
		}
	})

	t.Run("komoditas dengan usulan menunggu: 409 MASTER_ADA_USULAN, sesudah ditolak boleh dihapus", func(t *testing.T) {
		s.buatKomoditas(t, "UJI-USUL")
		usulan := s.usulanMenunggu(t, "komoditas", "UJI-USUL")

		w := s.call(t, http.MethodDelete, "/v1/master/komoditas/UJI-USUL", emailSetujui, nil)
		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		assert.Equal(t, "MASTER_ADA_USULAN", kodeEnvelope(t, w))
		assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas WHERE kode = 'UJI-USUL'`))

		// Usulan untuk entitas lain tidak menghalangi.
		s.buatKomoditas(t, "UJI-LAIN")
		assert.Equal(t, http.StatusNoContent, s.call(t, http.MethodDelete, "/v1/master/komoditas/UJI-LAIN", emailSetujui, nil).Code)

		s.exec(t, `UPDATE perubahan_master SET status = 'DITOLAK', alasan_penolakan = 'uji', ditolak_oleh_id = ?, ditolak_pada = now() WHERE id = ?`,
			uid("strategis_bps"), usulan)
		assert.Equal(t, http.StatusNoContent, s.call(t, http.MethodDelete, "/v1/master/komoditas/UJI-USUL", emailSetujui, nil).Code)
		assert.EqualValues(t, 1, s.hitungAudit(t, "komoditas", "UJI-USUL"))
	})

	t.Run("pasar tanpa rujukan dihapus: 204 dan audit tercatat", func(t *testing.T) {
		s.buatPasar(t, "PSR-1171-91")
		w := s.call(t, http.MethodDelete, "/v1/master/pasar/PSR-1171-91", emailFMIPA, nil)
		require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM pasar WHERE kode = 'PSR-1171-91'`))
		assert.EqualValues(t, 1, s.hitungAudit(t, "pasar", "PSR-1171-91"))
		var nama string
		require.NoError(t, s.db.Raw(`SELECT sebelum->>'nama' FROM audit_log WHERE aksi = 'HAPUS_MASTER' AND entitas = 'pasar' AND entitas_id = 'PSR-1171-91'`).Scan(&nama).Error)
		assert.Equal(t, "Pasar PSR-1171-91", nama)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodDelete, "/v1/master/pasar/PSR-1171-91", emailFMIPA, nil).Code)
	})

	t.Run("pasar yang dirujuk: 409 MASTER_DIRUJUK dengan saran menonaktifkan", func(t *testing.T) {
		s.buatPasar(t, "PSR-1171-92")
		s.entriUntuk(t, "BWM", "PSR-1171-92", "")
		s.buatPasar(t, "PSR-1171-93")
		s.exec(t, `INSERT INTO pedagang (pasar_id, kode) VALUES ('PSR-1171-93', 'PDG-X')`)
		// Daftar pedagang di tes lain menghitung seluruh tabel; jangan tinggalkan baris ini.
		t.Cleanup(func() { s.exec(t, `DELETE FROM pedagang WHERE pasar_id = 'PSR-1171-93'`) })

		for _, id := range []string{"PSR-1171-92", "PSR-1171-93"} {
			w := s.call(t, http.MethodDelete, "/v1/master/pasar/"+id, emailSetujui, nil)
			require.Equal(t, http.StatusConflict, w.Code, id+": "+w.Body.String())
			assert.Equal(t, "MASTER_DIRUJUK", kodeEnvelope(t, w))
			assert.Contains(t, pesanEnvelope(t, w), "nonaktifkan pasar (aktif = false)")
			assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM pasar WHERE kode = ?`, id))
			assert.Zero(t, s.hitungAudit(t, "pasar", id))
		}
	})

	t.Run("pasar dengan usulan menunggu: 409 MASTER_ADA_USULAN", func(t *testing.T) {
		s.buatPasar(t, "PSR-1171-94")
		s.usulanMenunggu(t, "pasar", "PSR-1171-94")
		w := s.call(t, http.MethodDelete, "/v1/master/pasar/PSR-1171-94", emailSetujui, nil)
		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		assert.Equal(t, "MASTER_ADA_USULAN", kodeEnvelope(t, w))
		assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM pasar WHERE kode = 'PSR-1171-94'`))
		// Usulan komoditas dengan kode yang sama tidak menghalangi pasar (entitas dibedakan).
		s.buatPasar(t, "PSR-1171-95")
		s.usulanMenunggu(t, "komoditas", "PSR-1171-95")
		assert.Equal(t, http.StatusNoContent, s.call(t, http.MethodDelete, "/v1/master/pasar/PSR-1171-95", emailSetujui, nil).Code)
	})

	t.Run("kode tak dikenal atau salah bentuk dijawab 404", func(t *testing.T) {
		before := s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'HAPUS_MASTER'`)
		for _, path := range []string{
			"/v1/master/komoditas/NGAWUR", "/v1/master/komoditas/%00", "/v1/master/komoditas/BRS-MED%27%3B--",
			"/v1/master/pasar/NGAWUR", "/v1/master/pasar/%00",
		} {
			w := s.call(t, http.MethodDelete, path, emailSetujui, nil)
			assert.Equal(t, http.StatusNotFound, w.Code, path)
			assert.Equal(t, "404", kodeEnvelope(t, w), path)
		}
		assert.Equal(t, before, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'HAPUS_MASTER'`))
		assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas WHERE kode = 'BRS-MED'`))
	})

	t.Run("seed tetap utuh sesudah semua penghapusan", func(t *testing.T) {
		for _, k := range master.Komoditas() {
			assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM komoditas WHERE kode = ?`, k.Kode), k.Kode)
		}
		for _, p := range master.Pasar() {
			assert.EqualValues(t, 1, s.hitung(t, `SELECT COUNT(*) FROM pasar WHERE kode = ?`, p.Kode), p.Kode)
		}
	})
}

// Penghapusan komoditas ikut menghapus komoditas_versi lewat ON DELETE CASCADE, padahal login aplikasi (03-C) tidak
// berhak DELETE pada tabel append-only itu: cascade berjalan dengan hak pemilik tabel, jadi tetap berhasil.
func TestReferensi_HapusDiBawahHakMinimum(t *testing.T) {
	const nama, sandi = "garda_app_referensi", "sandi-uji-cukup-panjang-1"
	s := refSuite(t)
	sqlDB, err := s.db.DB()
	require.NoError(t, err)
	require.NoError(t, migrate.SiapkanPenggunaAplikasi(t.Context(), sqlDB, nama, sandi))
	app := bangunSuite(pg.BukaSebagai(t, refDBName, nama, sandi))

	// Aplikasi memang tidak boleh menyentuh riwayat versi secara langsung.
	for _, stmt := range []string{`UPDATE komoditas_versi SET nama = nama`, `DELETE FROM komoditas_versi`} {
		err := app.db.Exec(stmt).Error
		require.Error(t, err, stmt)
		assert.Contains(t, err.Error(), "42501", stmt)
	}

	s.buatKomoditas(t, "UJI-APP")
	s.buatPasar(t, "PSR-1171-89")
	w := app.call(t, http.MethodDelete, "/v1/master/komoditas/UJI-APP", emailSetujui, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM komoditas_versi WHERE komoditas_kode = 'UJI-APP'`))
	assert.EqualValues(t, 1, s.hitungAudit(t, "komoditas", "UJI-APP"))

	w = app.call(t, http.MethodDelete, "/v1/master/pasar/PSR-1171-89", emailSetujui, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.EqualValues(t, 1, s.hitungAudit(t, "pasar", "PSR-1171-89"))
}
