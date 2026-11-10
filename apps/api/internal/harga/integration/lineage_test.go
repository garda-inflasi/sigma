package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Lineage dan versi entri (03-C, migrasi 000118): pemicu basis data menulis satu versi per perubahan isi,
// GET /v1/harga/:id/versi dan /lineage menampilkannya dengan aturan akses yang sama dengan /riwayat.

type versiEntri struct {
	Versi     int             `json:"versi"`
	Aksi      string          `json:"aksi"`
	Status    string          `json:"status"`
	Harga     int             `json:"harga"`
	Oleh      *string         `json:"oleh"`
	OlehNama  *string         `json:"oleh_nama"`
	Waktu     string          `json:"waktu"`
	Perubahan []string        `json:"perubahan"`
	Data      json.RawMessage `json:"data"`
}

type tahapLineage struct {
	Oleh   *string `json:"oleh"`
	Nama   *string `json:"nama"`
	Pada   *string `json:"pada"`
	Alasan *string `json:"alasan"`
}

type lineage struct {
	Entri struct {
		ID            string  `json:"id"`
		Kode          *string `json:"kode"`
		KomoditasKode string  `json:"komoditas_kode"`
		PasarNama     string  `json:"pasar_nama"`
		KabupatenNama string  `json:"kabupaten_nama"`
		Harga         int     `json:"harga"`
		Status        string  `json:"status"`
	} `json:"entri"`
	Asal struct {
		Sumber        string   `json:"sumber"`
		PetugasID     *string  `json:"petugas_id"`
		PetugasNama   *string  `json:"petugas_nama"`
		DicatatPada   string   `json:"dicatat_pada"`
		DiterimaPada  string   `json:"diterima_pada"`
		DalamGeofence bool     `json:"dalam_geofence"`
		Lat           *float64 `json:"lat"`
		Tugas         []struct {
			ID    string `json:"id"`
			Judul string `json:"judul"`
		} `json:"tugas"`
	} `json:"asal"`
	ValidasiOtomatis struct {
		FlagValidasi map[string]any `json:"flag_validasi"`
		SkorAnomali  float64        `json:"skor_anomali"`
		DeviasiPct   float64        `json:"deviasi_pct"`
	} `json:"validasi_otomatis"`
	Validasi     *tahapLineage `json:"validasi"`
	Publikasi    *tahapLineage `json:"publikasi"`
	Penolakan    *tahapLineage `json:"penolakan"`
	VersiTerkini int           `json:"versi_terkini"`
	Versi        []versiEntri  `json:"versi"`
	Riwayat      []riwayat     `json:"riwayat"`
	Audit        []struct {
		ID         string  `json:"id"`
		Aksi       string  `json:"aksi"`
		AktorNama  *string `json:"aktor_nama"`
		AktorPeran *string `json:"aktor_peran"`
		Hasil      string  `json:"hasil"`
	} `json:"audit"`
}

func (s *suite) versi(t *testing.T, id, email string) []versiEntri {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/harga/"+id+"/versi", email, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]versiEntri](t, w)
}

func ringkasVersi(versi []versiEntri) []string {
	out := make([]string, len(versi))
	for i, v := range versi {
		out[i] = v.Aksi + "/" + v.Status
	}
	return out
}

func TestVersi_PemicuMencatatTiapPerubahanDenganPelakunya(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{id}}).Berhasil)
	require.Equal(t, 1, s.bulk(t, emailManajerData, map[string]any{"action": "publikasi", "ids": []string{id}}).Berhasil)

	versi := s.versi(t, id, emailKoordinator)

	require.Equal(t, []string{"INPUT/DIAJUKAN", "VALIDASI/DIVALIDASI", "PUBLIKASI/DIPUBLIKASI"}, ringkasVersi(versi))
	for i, v := range versi {
		assert.Equal(t, i+1, v.Versi)
		assert.Equal(t, 31000, v.Harga)
		assert.NotNil(t, v.Perubahan, "perubahan selalu larik")
		// data adalah salinan baris apa adanya dari basis data: ID berupa angka JSON, bukan string seperti di respons API.
		var salinan struct {
			ID     json.Number `json:"id"`
			Status string      `json:"status"`
			Harga  int         `json:"harga"`
		}
		require.NoError(t, json.Unmarshal(v.Data, &salinan))
		assert.Equal(t, v.Status, salinan.Status, "data adalah salinan baris pada versi itu")
		assert.Equal(t, id, salinan.ID.String())
		require.NotNil(t, v.OlehNama)
		assert.NotEmpty(t, *v.OlehNama)
	}
	assert.Empty(t, versi[0].Perubahan)
	assert.Equal(t, []string{"divalidasi_oleh", "status"}, versi[1].Perubahan, "kolom waktu tidak dihitung")
	assert.Equal(t, []string{"dipublikasi_oleh", "status"}, versi[2].Perubahan)
	assert.Equal(t, uid("mahasiswa_1"), *versi[0].Oleh)
	assert.Equal(t, uid("koordinator_lapangan"), *versi[1].Oleh)
	assert.Equal(t, uid("manajer_data"), *versi[2].Oleh)
}

func TestVersi_KembalikanMencatatPelakuDariTransaksi(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{id}}).Berhasil)
	// KEMBALIKAN mengosongkan divalidasi_oleh dan tidak punya kolom pelaku sendiri: pelaku datang dari GUC transaksi.
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "kembalikan", "ids": []string{id}}).Berhasil)
	// Pemilik mengajukan ulang (DRAF -> DIAJUKAN).
	require.Equal(t, 1, s.bulk(t, emailMahasiswa, map[string]any{"action": "ajukan", "ids": []string{id}}).Berhasil)
	// Penolakan memuat alasan.
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "tolak", "ids": []string{id}, "alasan": "satuan salah"}).Berhasil)

	versi := s.versi(t, id, emailKoordinator)

	require.Equal(t, []string{"INPUT/DIAJUKAN", "VALIDASI/DIVALIDASI", "KEMBALIKAN/DRAF", "AJUKAN/DIAJUKAN", "TOLAK/DITOLAK"}, ringkasVersi(versi))
	require.NotNil(t, versi[2].Oleh)
	assert.Equal(t, uid("koordinator_lapangan"), *versi[2].Oleh, "KEMBALIKAN dicatat atas nama pelaku transaksi")
	assert.Equal(t, uid("mahasiswa_1"), *versi[3].Oleh)
	assert.Equal(t, uid("koordinator_lapangan"), *versi[4].Oleh)
	assert.Contains(t, versi[2].Perubahan, "divalidasi_oleh")
	assert.Contains(t, versi[4].Perubahan, "alasan_penolakan")
}

func TestVersi_NilaiGUCRusakTidakMembatalkanTransaksiBisnis(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	for _, guc := range []string{"bukan-uuid", "33333333-3333-3333-3333-333333333333", ""} {
		require.NoError(t, s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT set_config('garda.aktor_id', ?, true)", guc).Error; err != nil {
				return err
			}
			// catatan, bukan harga: harga entri ini ikut dibandingkan tes lain pada pasar dan komoditas yang sama.
			return tx.Exec("UPDATE entri_harga SET catatan = ? WHERE id = ?", "koreksi "+guc, id).Error
		}), "GUC %q", guc)
	}

	versi := s.versi(t, id, emailKoordinator)
	require.Len(t, versi, 4)
	for _, v := range versi[1:] {
		assert.Equal(t, "UBAH", v.Aksi)
		assert.Nil(t, v.Oleh, "pelaku tidak sah tidak dipakai dan tidak bisa diturunkan dari baris")
		assert.Equal(t, []string{"catatan"}, v.Perubahan)
	}
}

func TestVersi_EntriBerkodeLamaDanBackfillSeed(t *testing.T) {
	s := sharedSuite(t)
	const kode = "HRG-D05-00-02-K"
	var id string
	require.NoError(t, s.db.Raw("SELECT id::text FROM entri_harga WHERE kode = ?", kode).Scan(&id).Error)
	require.NotEmpty(t, id)

	// Entri seed ini bisa di kabupaten mana pun: dibuka peran seluruh Aceh (Koordinator hanya kabupatennya).
	lewatKode := s.call(t, http.MethodGet, "/v1/harga/"+kode+"/versi", emailLihatSaja, nil)
	lewatID := s.call(t, http.MethodGet, "/v1/harga/"+id+"/versi", emailLihatSaja, nil)

	require.Equal(t, http.StatusOK, lewatKode.Code, lewatKode.Body.String())
	assert.JSONEq(t, lewatID.Body.String(), lewatKode.Body.String(), "alias kode lama menunjuk entri yang sama")
	versi := data[[]versiEntri](t, lewatKode)
	require.NotEmpty(t, versi, "entri seed punya versi dari pemicu")
	assert.Equal(t, "INPUT", versi[0].Aksi)
}

func TestVersi_AksesSamaDenganRiwayat(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	for _, jalur := range []string{"versi", "lineage"} {
		t.Run(jalur, func(t *testing.T) {
			url := "/v1/harga/" + id + "/" + jalur
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, url, emailKoordinator, nil).Code, "pemegang harga.lihat")
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, url, emailMahasiswa, nil).Code, "petugas pencatat")
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, url, "mahasiswa.2@gardainflasi.id", nil).Code, "petugas lain: tampil sebagai tidak ada")
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, url, "superadmin@gardainflasi.id", nil).Code, "tanpa harga.lihat dan harga.input")
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, url, "", nil).Code)
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/tidak-ada/"+jalur, emailKoordinator, nil).Code)
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/999999999/"+jalur, emailKoordinator, nil).Code)
		})
	}
}

func TestLineage_MenjelaskanAsalValidasiPublikasiDanAudit(t *testing.T) {
	s := sharedSuite(t)
	// Tugas lapangan aktif milik petugas, supaya entri baru tertaut ke tugas; ditutup setelahnya agar tes lain tak ikut tertaut.
	require.NoError(t, s.db.Exec(`
		INSERT INTO tugas_lapangan (judul, pasar_id, petugas_id, komoditas, tipe, tenggat, status, target, dibuat_pada)
		VALUES ('Survei lineage uji', 'PSR-1171-01', garda_uuid_stabil('pengguna', 'mahasiswa_1'), ARRAY['TLR'], 'KONSUMEN',
		        now() + interval '1 day', 'BERJALAN', 5, now() - interval '1 hour')`).Error)
	id := s.buat(t, emailMahasiswa)
	require.NoError(t, s.db.Exec("UPDATE tugas_lapangan SET status = 'SELESAI' WHERE judul = 'Survei lineage uji'").Error)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{id}}).Berhasil)
	require.Equal(t, 1, s.bulk(t, emailManajerData, map[string]any{"action": "publikasi", "ids": []string{id}}).Berhasil)

	w := s.call(t, http.MethodGet, "/v1/harga/"+id+"/lineage", emailKoordinator, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	l := data[lineage](t, w)
	assert.Equal(t, id, l.Entri.ID)
	assert.Equal(t, "TLR", l.Entri.KomoditasKode)
	assert.Equal(t, "DIPUBLIKASI", l.Entri.Status)
	assert.NotEmpty(t, l.Entri.PasarNama)
	assert.NotEmpty(t, l.Entri.KabupatenNama)

	assert.Equal(t, "LAPANGAN", l.Asal.Sumber)
	require.NotNil(t, l.Asal.PetugasID)
	assert.Equal(t, uid("mahasiswa_1"), *l.Asal.PetugasID)
	require.NotNil(t, l.Asal.PetugasNama)
	assert.NotEmpty(t, *l.Asal.PetugasNama)
	assert.NotEmpty(t, l.Asal.DicatatPada)
	assert.NotEmpty(t, l.Asal.DiterimaPada)
	require.Len(t, l.Asal.Tugas, 1)
	assert.Equal(t, "Survei lineage uji", l.Asal.Tugas[0].Judul)

	assert.NotNil(t, l.ValidasiOtomatis.FlagValidasi)
	assert.Contains(t, l.ValidasiOtomatis.FlagValidasi, "outlier")

	require.NotNil(t, l.Validasi)
	assert.Equal(t, uid("koordinator_lapangan"), *l.Validasi.Oleh)
	require.NotNil(t, l.Validasi.Nama)
	assert.NotEmpty(t, *l.Validasi.Nama)
	assert.NotNil(t, l.Validasi.Pada)
	require.NotNil(t, l.Publikasi)
	assert.Equal(t, uid("manajer_data"), *l.Publikasi.Oleh)
	assert.Nil(t, l.Penolakan, "tidak ditolak")

	assert.Equal(t, 3, l.VersiTerkini)
	assert.Equal(t, []string{"INPUT/DIAJUKAN", "VALIDASI/DIVALIDASI", "PUBLIKASI/DIPUBLIKASI"}, ringkasVersi(l.Versi))
	require.Len(t, l.Riwayat, 3)
	assert.Equal(t, "DIPUBLIKASI", l.Riwayat[2].Ke)

	aksi := map[string]string{}
	for _, a := range l.Audit {
		aksi[a.Aksi] = a.Hasil
	}
	assert.Contains(t, aksi, "HARGA_INPUT", "audit input (entitas_id = ID entri)")
	assert.Contains(t, aksi, "HARGA_VALIDASI", "audit aksi massal (metadata.ids memuat ID entri)")
	assert.Contains(t, aksi, "HARGA_PUBLIKASI")
	assert.Len(t, l.Audit, 3, "hanya audit yang menyebut entri ini, tidak aksi massal entri lain")
	for _, a := range l.Audit {
		assert.NotNil(t, a.AktorNama)
	}
}

func TestLineage_EntriDitolakMemuatAlasanDanEntriBerkodeLama(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "tolak", "ids": []string{id}, "alasan": "bukti kurang"}).Berhasil)

	l := data[lineage](t, s.call(t, http.MethodGet, "/v1/harga/"+id+"/lineage", emailKoordinator, nil))

	assert.Nil(t, l.Validasi)
	assert.Nil(t, l.Publikasi)
	require.NotNil(t, l.Penolakan)
	assert.Equal(t, uid("koordinator_lapangan"), *l.Penolakan.Oleh)
	require.NotNil(t, l.Penolakan.Alasan)
	assert.Equal(t, "bukti kurang", *l.Penolakan.Alasan)
	assert.Empty(t, l.Asal.Tugas, "tanpa tugas lapangan: larik kosong, bukan null")

	const kode = "HRG-D05-00-02-K"
	var idSeed string
	require.NoError(t, s.db.Raw("SELECT id::text FROM entri_harga WHERE kode = ?", kode).Scan(&idSeed).Error)
	lewatKode := data[lineage](t, s.call(t, http.MethodGet, "/v1/harga/"+kode+"/lineage", emailLihatSaja, nil))
	assert.Equal(t, idSeed, lewatKode.Entri.ID)
	require.NotNil(t, lewatKode.Entri.Kode)
	assert.Equal(t, kode, *lewatKode.Entri.Kode)
	assert.NotEmpty(t, lewatKode.Versi)
	assert.NotNil(t, lewatKode.Asal.Lat)
}

func TestVersi_TabelAppendOnlyDanIkutTerhapusBersamaEntri(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	for nama, sql := range map[string]string{
		"UPDATE":   "UPDATE entri_harga_versi SET harga = 1 WHERE entri_id = " + id,
		"DELETE":   "DELETE FROM entri_harga_versi WHERE entri_id = " + id,
		"TRUNCATE": "TRUNCATE entri_harga_versi",
	} {
		err := s.db.Exec(sql).Error
		require.Error(t, err, nama)
		assert.Contains(t, err.Error(), "append-only", nama)
		assert.Contains(t, err.Error(), "23001", nama+": restrict_violation")
	}
	assert.Len(t, s.versi(t, id, emailKoordinator), 1)

	// Satu-satunya penghapusan sah: ON DELETE CASCADE saat entrinya dihapus.
	require.NoError(t, s.db.Exec("DELETE FROM entri_harga WHERE id = ?", id).Error)
	var sisa int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM entri_harga_versi WHERE entri_id = ?", id).Scan(&sisa).Error)
	assert.Zero(t, sisa)
}
