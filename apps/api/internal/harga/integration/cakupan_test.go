package integration

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Row scope (02-B): Koordinator dan Agregator hanya entri di kabupaten tugasnya (ditambah entri yang ia catat sendiri),
// Mahasiswa hanya entrinya sendiri, peran tingkat provinsi seluruh Aceh. Entri di luar cakupan dijawab seperti tidak
// ada: 404 pada detail dan turunannya, tidak termuat di daftar dan ringkasan, dan tidak_ditemukan pada aksi massal.

const (
	emailAgregator  = "agregator.data@gardainflasi.id"
	emailMahasiswa2 = "mahasiswa.2@gardainflasi.id"

	// Demo: Koordinator dan Agregator bertugas di Kota Banda Aceh (1171); pasar kabupaten lain dipakai sebagai "luar".
	pasarDalam = "PSR-1171-01"
	pasarLuar  = "PSR-1108-01"
	kabDalam   = "1171"
	kabLuar    = "1108"
)

func (s *suite) penggunaID(t *testing.T, email string) string {
	t.Helper()
	var id string
	require.NoError(t, s.db.Raw("SELECT id::text FROM pengguna WHERE email = ?", email).Scan(&id).Error)
	require.NotEmpty(t, id, email)
	return id
}

// buatDi mencatat entri baru di pasar tertentu dan mengembalikan id-nya. pertahankan menghindari 409 bentrok dengan
// entri seed terdekat: yang diuji di sini adalah jangkauan, bukan deteksi bentrok.
func (s *suite) buatDi(t *testing.T, email, pasar string) string {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/harga", email, map[string]any{
		"komoditas_kode": "TLR", "pasar_id": pasar, "tipe": "KONSUMEN", "harga": 31000, "pertahankan": true})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return data[struct {
		ID string `json:"id"`
	}](t, w).ID
}

// jawabanSama memastikan dua respons tidak bisa dibedakan penelepon: status dan isi persis sama.
func jawabanSama(t *testing.T, a, b string, msg string) {
	t.Helper()
	assert.Equal(t, a, b, msg)
}

func TestCakupan_DaftarDanRingkasanKoordinator(t *testing.T) {
	s := sharedSuite(t)
	koordinator := s.penggunaID(t, emailKoordinator)
	s.buatDi(t, emailKoordinator, pasarLuar) // entri miliknya sendiri di luar kabupaten tugasnya

	t.Run("daftar hanya kabupaten tugas dan entri sendiri", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/harga?limit=1000", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var want int64
		require.NoError(t, s.db.Raw(`SELECT count(*) FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
			WHERE p.kabupaten_kode = ? OR e.petugas_id = ?`, kabDalam, koordinator).Scan(&want).Error)
		assert.EqualValues(t, want, total(t, w), "X-Total-Count menghitung entri dalam cakupan saja")

		for _, e := range data[[]entri](t, w) {
			dalam := e.KabupatenKode == kabDalam || (e.PetugasID != nil && *e.PetugasID == koordinator)
			assert.True(t, dalam, "entri %s di %s oleh %v di luar cakupan", e.ID, e.KabupatenKode, e.PetugasID)
		}
	})

	t.Run("peran seluruh Aceh melihat lebih banyak dari Koordinator", func(t *testing.T) {
		var semua int64
		require.NoError(t, s.db.Raw(`SELECT count(*) FROM entri_harga`).Scan(&semua).Error)
		koord := total(t, s.call(t, http.MethodGet, "/v1/harga?limit=1", emailKoordinator, nil))
		prov := total(t, s.call(t, http.MethodGet, "/v1/harga?limit=1", emailLihatSaja, nil))
		assert.EqualValues(t, semua, prov, "strategis melihat seluruh Aceh")
		assert.Less(t, koord, prov, "Koordinator hanya kabupatennya")
		assert.Positive(t, koord)
	})

	t.Run("filter wilayah digabung AND dengan cakupan", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/harga?wilayah="+kabLuar+"&limit=1000", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code)
		for _, e := range data[[]entri](t, w) {
			require.NotNil(t, e.PetugasID)
			assert.Equal(t, koordinator, *e.PetugasID, "di kabupaten lain hanya entri miliknya yang terlihat: %s", e.ID)
		}
		var milik int64
		require.NoError(t, s.db.Raw(`SELECT count(*) FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
			WHERE p.kabupaten_kode = ? AND e.petugas_id = ?`, kabLuar, koordinator).Scan(&milik).Error)
		assert.Positive(t, milik)
		assert.EqualValues(t, milik, total(t, w), "hanya entri milik Koordinator sendiri di %s", kabLuar)
	})

	t.Run("ringkasan menghitung entri dalam cakupan", func(t *testing.T) {
		hitung := func(email string) map[string]int {
			return data[map[string]int](t, s.call(t, http.MethodGet, "/v1/harga/ringkasan?hari=365", email, nil))
		}
		type baris struct{ Diajukan, Divalidasi, Dipublikasi, Ditolak, Draf int }
		var wantKoord, wantSemua baris
		sql := `SELECT count(*) FILTER (WHERE e.status='DIAJUKAN') AS diajukan, count(*) FILTER (WHERE e.status='DIVALIDASI') AS divalidasi,
			count(*) FILTER (WHERE e.status='DIPUBLIKASI') AS dipublikasi, count(*) FILTER (WHERE e.status='DITOLAK') AS ditolak,
			count(*) FILTER (WHERE e.status='DRAF') AS draf
			FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id WHERE e.dicatat_pada >= now() - interval '365 days'`
		require.NoError(t, s.db.Raw(sql+` AND (p.kabupaten_kode = ? OR e.petugas_id = ?)`, kabDalam, koordinator).Scan(&wantKoord).Error)
		require.NoError(t, s.db.Raw(sql).Scan(&wantSemua).Error)

		got := hitung(emailKoordinator)
		assert.Equal(t, wantKoord, baris{got["diajukan"], got["divalidasi"], got["dipublikasi"], got["ditolak"], got["draf"]})
		got = hitung(emailLihatSaja)
		assert.Equal(t, wantSemua, baris{got["diajukan"], got["divalidasi"], got["dipublikasi"], got["ditolak"], got["draf"]})
		assert.Greater(t, wantSemua.Dipublikasi, wantKoord.Dipublikasi)
	})
}

func TestCakupan_EntriLuarKabupatenDijawab404SamaDenganTidakAda(t *testing.T) {
	s := sharedSuite(t)
	luar := s.buatDi(t, emailMahasiswa, pasarLuar)
	dalam := s.buatDi(t, emailMahasiswa, pasarDalam)

	// Entri seed berkode lama di kabupaten lain, milik petugas lain.
	var kodeLuar, idKodeLuar string
	require.NoError(t, s.db.Raw(`SELECT e.kode FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
		WHERE p.kabupaten_kode = ? AND e.kode IS NOT NULL AND e.petugas_id <> ? LIMIT 1`, kabLuar, s.penggunaID(t, emailKoordinator)).Scan(&kodeLuar).Error)
	require.NotEmpty(t, kodeLuar)
	require.NoError(t, s.db.Raw("SELECT id::text FROM entri_harga WHERE kode = ?", kodeLuar).Scan(&idKodeLuar).Error)

	jalur := []string{"", "/riwayat", "/versi", "/lineage"}
	for _, email := range []string{emailKoordinator, emailAgregator} {
		for _, j := range jalur {
			tidakAda := s.call(t, http.MethodGet, "/v1/harga/999999999"+j, email, nil)
			require.Equal(t, http.StatusNotFound, tidakAda.Code)
			for _, rujukan := range []string{luar, idKodeLuar, url.PathEscape(kodeLuar)} {
				w := s.call(t, http.MethodGet, "/v1/harga/"+rujukan+j, email, nil)
				assert.Equal(t, http.StatusNotFound, w.Code, "%s %s%s", email, rujukan, j)
				jawabanSama(t, tidakAda.Body.String(), w.Body.String(), "entri luar cakupan tidak boleh bisa dibedakan dari entri yang tidak ada")
			}
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+dalam+j, email, nil).Code, "%s dalam kabupaten tugas: %s", email, j)
		}
	}

	t.Run("peran seluruh Aceh membukanya, termasuk lewat kode lama", func(t *testing.T) {
		for _, j := range jalur {
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+luar+j, emailLihatSaja, nil).Code, j)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+url.PathEscape(kodeLuar)+j, emailLihatSaja, nil).Code, j)
		}
	})
}

func TestCakupan_BulkMelewatkanEntriLuarCakupanSebagaiTidakDitemukan(t *testing.T) {
	s := sharedSuite(t)
	dalam := s.buatDi(t, emailMahasiswa, pasarDalam)
	luar := s.buatDi(t, emailMahasiswa, pasarLuar)

	hasil := s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{dalam, luar, "999999999"}})

	assert.Equal(t, 1, hasil.Berhasil, "hanya entri di kabupaten tugas yang divalidasi")
	assert.Equal(t, 2, hasil.Gagal)
	assert.Equal(t, []rincianBulk{{ID: luar, Sebab: "tidak_ditemukan"}, {ID: "999999999", Sebab: "tidak_ditemukan"}}, hasil.Rincian,
		"entri luar cakupan dilaporkan persis seperti id yang tidak ada")
	assert.Equal(t, "DIVALIDASI", s.status(t, dalam))

	var status string
	require.NoError(t, s.db.Raw("SELECT status::text FROM entri_harga WHERE id = ?", luar).Scan(&status).Error)
	assert.Equal(t, "DIAJUKAN", status, "entri luar cakupan tidak berubah")
	var riwayat int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM riwayat_status_entri WHERE entri_id = ?", luar).Scan(&riwayat).Error)
	assert.EqualValues(t, 1, riwayat, "tidak ada riwayat transisi baru (hanya kelahiran entri)")

	t.Run("semua aksi terkena cakupan", func(t *testing.T) {
		for _, aksi := range []string{"tolak", "kembalikan", "validasi"} {
			body := map[string]any{"action": aksi, "ids": []string{luar}, "alasan": "tidak berlaku"}
			h := s.bulk(t, emailKoordinator, body)
			assert.Equal(t, hasilBulkRincian{Gagal: 1, Rincian: []rincianBulk{{ID: luar, Sebab: "tidak_ditemukan"}}}, *h, aksi)
		}
		require.NoError(t, s.db.Raw("SELECT status::text FROM entri_harga WHERE id = ?", luar).Scan(&status).Error)
		assert.Equal(t, "DIAJUKAN", status)
	})

	t.Run("peran yang berwenang atas kabupaten itu menjangkaunya", func(t *testing.T) {
		// Entri DIAJUKAN di luar kabupaten Koordinator divalidasi Koordinator kabupaten itu; di demo tidak ada, jadi
		// kabupaten tugas Koordinator dipindah sementara. Cakupan dibaca dari basis data pada tiap permintaan.
		pindahKabupaten(t, s, emailKoordinator, kabLuar)
		h := s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{luar, dalam}})
		assert.Equal(t, 1, h.Berhasil, "kini %s dalam cakupan, %s di luar (dan sudah divalidasi)", luar, dalam)
		require.NoError(t, s.db.Raw("SELECT status::text FROM entri_harga WHERE id = ?", luar).Scan(&status).Error)
		assert.Equal(t, "DIVALIDASI", status)
	})
}

func TestCakupan_EntriYangDikirimSendiriTetapTerlihat(t *testing.T) {
	s := sharedSuite(t)
	// Koordinator mengirim harga dari pasar di luar kabupaten tugasnya: entri itu tidak boleh lenyap darinya.
	milik := s.buatDi(t, emailKoordinator, pasarLuar)

	for _, j := range []string{"", "/riwayat", "/versi", "/lineage"} {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+milik+j, emailKoordinator, nil).Code, j)
		// Agregator di kabupaten yang sama dengan Koordinator tetap tidak menjangkaunya.
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+milik+j, emailAgregator, nil).Code, "agregator %s", j)
	}
	saya := data[[]entri](t, s.call(t, http.MethodGet, "/v1/harga/saya?limit=1000", emailKoordinator, nil))
	var ada bool
	for _, e := range saya {
		ada = ada || e.ID == milik
	}
	assert.True(t, ada, "muncul di /v1/harga/saya")

	// Aksi massal juga menjangkaunya: dikembalikan ke DRAF lalu diajukan ulang oleh pemiliknya (hanyaPemilik).
	assert.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "kembalikan", "ids": []string{milik}}).Berhasil)
	assert.Equal(t, hasilBulkRincian{Gagal: 1, Rincian: []rincianBulk{{ID: milik, Sebab: "tidak_ditemukan"}}},
		*s.bulk(t, emailAgregator, map[string]any{"action": "ajukan", "ids": []string{milik}}), "agregator tidak menjangkaunya")
	assert.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "ajukan", "ids": []string{milik}}).Berhasil)
	var status string
	require.NoError(t, s.db.Raw("SELECT status::text FROM entri_harga WHERE id = ?", milik).Scan(&status).Error)
	assert.Equal(t, "DIAJUKAN", status)
}

func TestCakupan_MahasiswaHanyaEntrinyaSendiri(t *testing.T) {
	s := sharedSuite(t)
	milik := s.buatDi(t, emailMahasiswa, pasarDalam)
	milikLain := s.buatDi(t, emailMahasiswa2, pasarDalam) // kabupaten yang sama, pencatat berbeda

	for _, j := range []string{"/riwayat", "/versi", "/lineage"} {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+milik+j, emailMahasiswa, nil).Code, j)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+milikLain+j, emailMahasiswa, nil).Code,
			"%s: kabupaten yang sama tidak cukup bagi Mahasiswa, hanya entrinya sendiri", j)
	}
	// Detail dan daftar membutuhkan harga.lihat, yang Mahasiswa tidak punya.
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga/"+milik, emailMahasiswa, nil).Code)

	saya := data[[]entri](t, s.call(t, http.MethodGet, "/v1/harga/saya?limit=1000", emailMahasiswa, nil))
	require.NotEmpty(t, saya)
	for _, e := range saya {
		require.NotNil(t, e.PetugasID)
		assert.Equal(t, uid("mahasiswa_1"), *e.PetugasID)
	}
}

func TestCakupan_PeranTingkatProvinsiMelihatSeluruhAceh(t *testing.T) {
	s := sharedSuite(t)
	luar := s.buatDi(t, emailMahasiswa, pasarLuar)
	dalam := s.buatDi(t, emailMahasiswa, pasarDalam)

	for _, email := range []string{emailLihatSaja, emailManajerData} {
		for _, id := range []string{luar, dalam} {
			for _, j := range []string{"", "/riwayat", "/versi", "/lineage"} {
				assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+id+j, email, nil).Code, "%s %s%s", email, id, j)
			}
		}
	}

	t.Run("publikasi lintas kabupaten", func(t *testing.T) {
		// Entri di kabupaten lain yang sudah divalidasi pihak berwenang di sana.
		require.NoError(t, s.db.Exec(`UPDATE entri_harga SET status = 'DIVALIDASI', divalidasi_oleh = ?, divalidasi_pada = now() WHERE id = ?`,
			s.penggunaID(t, emailKoordinator), luar).Error)
		h := s.bulk(t, emailManajerData, map[string]any{"action": "publikasi", "ids": []string{luar}})
		assert.Equal(t, 1, h.Berhasil)
		var status string
		require.NoError(t, s.db.Raw("SELECT status::text FROM entri_harga WHERE id = ?", luar).Scan(&status).Error)
		assert.Equal(t, "DIPUBLIKASI", status)
	})
}

func TestCakupan_AgregatorMengikutiKabupatennya(t *testing.T) {
	s := sharedSuite(t)
	agregator := s.penggunaID(t, emailAgregator)

	w := s.call(t, http.MethodGet, "/v1/harga?limit=1000", emailAgregator, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := data[[]entri](t, w)
	require.NotEmpty(t, rows)
	for _, e := range rows {
		assert.True(t, e.KabupatenKode == kabDalam || (e.PetugasID != nil && *e.PetugasID == agregator), "entri %s di %s", e.ID, e.KabupatenKode)
	}
}

// pindahKabupaten mengubah kabupaten tugas pengguna di basis data (seperti PATCH /v1/users/:id) tanpa login ulang dan
// memulihkannya saat tes selesai. Token yang sudah terbit tetap membawa klaim `kab` lama: cakupan harus mengikuti
// basis data, bukan klaim itu.
func pindahKabupaten(t *testing.T, s *suite, email, kabupaten string) {
	t.Helper()
	var asal *string
	require.NoError(t, s.db.Raw("SELECT kabupaten_kode FROM pengguna WHERE email = ?", email).Scan(&asal).Error)
	var nilai any
	if kabupaten != "" {
		nilai = kabupaten
	}
	require.NoError(t, s.db.Exec("UPDATE pengguna SET kabupaten_kode = ? WHERE email = ?", nilai, email).Error)
	t.Cleanup(func() { s.db.Exec("UPDATE pengguna SET kabupaten_kode = ? WHERE email = ?", asal, email) })
}

func TestCakupan_KabupatenDibacaDariBasisDataBukanKlaimToken(t *testing.T) {
	s := sharedSuite(t)
	koordinator := s.penggunaID(t, emailKoordinator)
	dalam := s.buatDi(t, emailMahasiswa, pasarDalam)
	luar := s.buatDi(t, emailMahasiswa, pasarLuar)
	s.login(t, emailKoordinator) // token terbit dengan kabupaten 1171

	t.Run("dipindah ke kabupaten lain tanpa login ulang", func(t *testing.T) {
		pindahKabupaten(t, s, emailKoordinator, kabLuar)
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+luar, emailKoordinator, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+dalam, emailKoordinator, nil).Code)
		for _, e := range data[[]entri](t, s.call(t, http.MethodGet, "/v1/harga?limit=1000", emailKoordinator, nil)) {
			assert.True(t, e.KabupatenKode == kabLuar || (e.PetugasID != nil && *e.PetugasID == koordinator), e.ID)
		}
	})

	t.Run("tanpa kabupaten tugas: hanya entri sendiri, bukan seluruh Aceh", func(t *testing.T) {
		pindahKabupaten(t, s, emailKoordinator, "")
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+dalam, emailKoordinator, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+luar, emailKoordinator, nil).Code)
		w := s.call(t, http.MethodGet, "/v1/harga?limit=1000", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code)
		for _, e := range data[[]entri](t, w) {
			require.NotNil(t, e.PetugasID)
			assert.Equal(t, koordinator, *e.PetugasID, "hanya entri miliknya: %s", e.ID)
		}
		h := s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{dalam, luar}})
		assert.Equal(t, 0, h.Berhasil)
	})
}

func TestCakupan_SetiapPeranDiTabelRolesPunyaAturan(t *testing.T) {
	s := sharedSuite(t)
	var peran []string
	require.NoError(t, s.db.Raw("SELECT kunci::text FROM roles ORDER BY urutan").Scan(&peran).Error)
	require.Len(t, peran, 14)
	for _, p := range peran {
		assert.True(t, domain.PeranDikenal(p),
			"peran %s belum punya aturan jangkauan baris: tambahkan ke domain.CakupanPeran (dan perananSeluruhAceh bila tingkat provinsi)", p)
	}
}
