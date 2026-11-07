package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Kunci primer tugas_lapangan: BIGINT identity yang diberi basis data, ID lama (TGS-...) di kode. Antrean dan tampilan
// PWA yang dibuat sebelum migrasi 000111 memegang kode lama; ID baru berupa string angka.

// tugasSaya membaca daftar tugas milik email.
func (s *suite) tugasSaya(t *testing.T, email string) []tugas {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", email, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]tugas](t, w)
}

func TestPK_DaftarMemuatIDBigintStringDanKodeLama(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", emailMahasiswa1, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Regexp(t, `"id":"\d+"`, w.Body.String(), "ID BIGINT ditulis sebagai string JSON")
	assert.NotRegexp(t, `"id":\d`, w.Body.String(), "bukan angka JSON")
	for _, item := range data[[]tugas](t, w) {
		n, err := strconv.ParseInt(item.ID, 10, 64)
		require.NoError(t, err, "id string angka: %q", item.ID)
		assert.Positive(t, n)
		require.NotNil(t, item.Kode, "tugas seed berkode TGS-<slug>-<n>")
		assert.True(t, strings.HasPrefix(*item.Kode, "TGS-mahasiswa_1-"), *item.Kode)
	}
}

func TestPK_TautanDanAntreanLamaBerkodeTetapBerlaku(t *testing.T) {
	s := sharedSuite(t)
	daftar := s.tugasSaya(t, emailMahasiswa1)
	require.NotEmpty(t, daftar)
	var kodeLama, idBaru string
	for _, item := range daftar {
		if item.Kode != nil && *item.Kode == "TGS-mahasiswa_1-3" {
			kodeLama, idBaru = *item.Kode, item.ID
		}
	}
	require.NotEmpty(t, kodeLama)

	t.Run("detail lewat kode lama dan ID baru sama", func(t *testing.T) {
		lama := s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+kodeLama, emailMahasiswa1, nil)
		baru := s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+idBaru, emailMahasiswa1, nil)
		require.Equal(t, http.StatusOK, lama.Code, lama.Body.String())
		require.Equal(t, http.StatusOK, baru.Code, baru.Body.String())
		assert.JSONEq(t, baru.Body.String(), lama.Body.String())
	})
	t.Run("aksi massal menerima kode lama, ID baru, dan campuran", func(t *testing.T) {
		unduh := func(ids ...string) hasilBulk {
			w := s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailMahasiswa1, map[string]any{"action": "unduh_data_acuan", "ids": ids})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			return data[hasilBulk](t, w)
		}
		assert.Equal(t, hasilBulk{Berhasil: 1}, unduh(kodeLama))
		assert.Equal(t, hasilBulk{Berhasil: 1}, unduh(idBaru))
		assert.Equal(t, hasilBulk{Berhasil: 2, Gagal: 1}, unduh(kodeLama, "TGS-mahasiswa_1-1", "TGS-hilang-1"), "kode yang tidak ada dihitung gagal")
		assert.Equal(t, hasilBulk{Berhasil: 1, Gagal: 2}, unduh(idBaru, "999999999", "TGS-mahasiswa_2-1"), "ID tak ada dan tugas petugas lain gagal")
	})
	t.Run("menutup tugas lewat kode lama", func(t *testing.T) {
		require.NoError(t, s.db.Exec("UPDATE tugas_lapangan SET jumlah_selesai = target WHERE kode = 'TGS-mahasiswa_2-4'").Error)
		w := s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailMahasiswa2, map[string]any{"action": "tandai_selesai", "ids": []string{"TGS-mahasiswa_2-4"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, w))

		var status string
		require.NoError(t, s.db.Raw("SELECT status::text FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_2-4'").Scan(&status).Error)
		assert.Equal(t, "SELESAI", status)
		// Audit mencatat ID BIGINT (bukan kode) di metadata.
		var n int64
		require.NoError(t, s.db.Raw(`
			SELECT count(*) FROM audit_log WHERE aksi = 'TUGAS_TANDAI_SELESAI' AND metadata -> 'ids' @> to_jsonb((SELECT id::text FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_2-4'))`).Scan(&n).Error)
		assert.EqualValues(t, 1, n)
	})
	t.Run("menutup tugas lewat ID baru", func(t *testing.T) {
		var id string
		require.NoError(t, s.db.Raw("SELECT id::text FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_2-2'").Scan(&id).Error)
		require.NoError(t, s.db.Exec("UPDATE tugas_lapangan SET jumlah_selesai = target WHERE id = ?", id).Error)
		w := s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailMahasiswa2, map[string]any{"action": "tandai_selesai", "ids": []string{id}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, w))
	})
}

func TestPK_TugasBaruTanpaKodeDiaksesLewatID(t *testing.T) {
	s := sharedSuite(t)
	var id string
	require.NoError(t, s.db.Raw(`
		INSERT INTO tugas_lapangan (judul, pasar_id, petugas_id, komoditas, tipe, tenggat, target)
		VALUES ('Tugas baru', 'PSR-1171-01', garda_uuid_stabil('pengguna', 'mahasiswa_1'), ARRAY['BWM'], 'KONSUMEN', now() + interval '1 day', 1)
		RETURNING id::text`).Scan(&id).Error)

	w := s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+id, emailMahasiswa1, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := data[tugasDetail](t, w)
	assert.Equal(t, id, got.ID)
	assert.Nil(t, got.Kode, "tugas baru tanpa kode tampilan")
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+id, emailMahasiswa2, nil).Code)
}

func TestPK_IDSalahBentukAtauTidakAdaDijawab404(t *testing.T) {
	s := sharedSuite(t)

	salah := map[string]string{
		"nol":                    "0",
		"negatif":                "-1",
		"berawalan nol":          "007",
		"pecahan":                "1.5",
		"di luar rentang int8":   "9223372036854775808",
		"spasi":                  "%20",
		"terlalu panjang":        strings.Repeat("T", 65),
		"kode tidak ada":         "TGS-tidak-ada-1",
		"ID bentuk sah, tak ada": "999999999",
	}
	for nama, id := range salah {
		w := s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+id, emailKoordinator, nil)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s: %s", nama, w.Body.String())
	}
}

func TestPK_BulkBatasJumlahDanPanjangRujukan(t *testing.T) {
	s := sharedSuite(t)
	post := func(ids []string) int {
		return s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailMahasiswa1, map[string]any{"action": "unduh_data_acuan", "ids": ids}).Code
	}

	assert.Equal(t, http.StatusBadRequest, post([]string{" ", ""}), "daftar efektif kosong")
	assert.Equal(t, http.StatusBadRequest, post([]string{strings.Repeat("T", 65)}))
	assert.Equal(t, http.StatusOK, post([]string{strings.Repeat("T", 64)}), "64 karakter dibaca sebagai kode yang tidak ada")
}
