package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	lapangan "github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Aktivitas per petugas (08-B): nama petugas aktif, kabupaten, entri hari ini terhadap target harian, antrean
// validasi, penolakan sepekan, entri terakhir, dan tugas lapangan.

const (
	petugasSlug1 = "mahasiswa_1" // Aulia Rahman, Kota Banda Aceh (1171)
	petugasSlug2 = "mahasiswa_2" // Cut Nanda, Aceh Besar (1108)
	emailMhs1    = "mahasiswa.1@gardainflasi.id"
)

// Urutan yang diharapkan dihitung terpisah dari basis data: kabupaten, nama, lalu ID.
func urutanPetugasDiharapkan(t *testing.T, s *suite, kabupaten string) []string {
	t.Helper()
	var rows []struct {
		ID            string
		Nama          string
		KabupatenNama string
	}
	q := s.db.Table("pengguna p").Select("p.id::text AS id, p.nama, k.nama AS kabupaten_nama").
		Joins("JOIN kabupaten k ON k.kode = p.kabupaten_kode").
		Where("p.peran = 'OPS_MAHASISWA' AND p.status = 'AKTIF'")
	if kabupaten != "" {
		q = q.Where("p.kabupaten_kode = ?", kabupaten)
	}
	require.NoError(t, q.Scan(&rows).Error)
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if la, lb := strings.ToLower(a.KabupatenNama), strings.ToLower(b.KabupatenNama); la != lb {
			return la < lb
		}
		if la, lb := strings.ToLower(a.Nama), strings.ToLower(b.Nama); la != lb {
			return la < lb
		}
		return a.ID < b.ID
	})
	id := make([]string, len(rows))
	for i, r := range rows {
		id[i] = r.ID
	}
	return id
}

func idPetugas(r petugasResp) []string {
	id := make([]string, len(r.Petugas))
	for i, p := range r.Petugas {
		id[i] = p.ID
	}
	return id
}

func cariPetugas(t *testing.T, r petugasResp, id string) petugasItem {
	t.Helper()
	for _, p := range r.Petugas {
		if p.ID == id {
			return p
		}
	}
	require.Failf(t, "petugas tidak ada", "%s tidak ada di daftar", id)
	return petugasItem{}
}

// Data demo: sepuluh mahasiswa aktif yang bertugas di sepuluh kabupaten, masing-masing dengan lima tugas dan entri
// harga hari ini.
func TestAktivitasPetugas_DataDemoTidakKosong(t *testing.T) {
	s := sharedSuite(t)

	r := s.petugasDari(t, emailBerhak, "")

	assert.Equal(t, tanggal(hariIni()), r.Tanggal)
	assert.Equal(t, 7, r.JendelaHari)
	require.Len(t, r.Petugas, 10, "mahasiswa aktif berkabupaten; User Nonaktif tidak ikut")
	assert.Equal(t, urutanPetugasDiharapkan(t, s, ""), idPetugas(r), "urut kabupaten, nama, lalu ID")

	var jumlahEntri int
	for _, p := range r.Petugas {
		assert.NotEmpty(t, p.ID)
		assert.NotEmpty(t, p.Nama)
		assert.NotEmpty(t, p.KabupatenKode)
		assert.NotEmpty(t, p.KabupatenNama)
		assert.Equal(t, 14, p.TargetHarian, "tiap komoditas aktif sekali sehari: %s", p.Nama)
		assert.NotNil(t, p.EntriTerakhirPada, "%s punya riwayat entri", p.Nama)
		assert.Equal(t, 3, p.TugasAktif, "%s: BERJALAN + dua BELUM", p.Nama)
		assert.Equal(t, 1, p.TugasTerlambat, p.Nama)
		assert.Equal(t, 1, p.TugasSelesai, p.Nama)
		jumlahEntri += p.EntriHariIni
	}
	assert.Positive(t, jumlahEntri, "ada entri hari ini")
}

// Angka per petugas dibandingkan dengan aktivitas-lapangan per kabupaten: pada data demo setiap kabupaten dikerjakan
// satu mahasiswa di pasarnya sendiri, jadi entri petugas sama dengan entri kabupaten dan target keduanya sama.
func TestAktivitasPetugas_SamaDenganAktivitasLapanganPadaDataDemo(t *testing.T) {
	s := sharedSuite(t)

	petugas := s.petugasDari(t, emailBerhak, "")
	perKabupaten := map[string]struct{ entri, target, petugas int }{}
	for _, p := range petugas.Petugas {
		k := perKabupaten[p.KabupatenKode]
		k.entri += p.EntriHariIni
		k.target += p.TargetHarian
		k.petugas++
		perKabupaten[p.KabupatenKode] = k
	}

	for _, a := range s.aktivitasDari(t, emailBerhak) {
		got := perKabupaten[a.KabupatenKode]
		assert.Equal(t, a.Entri, got.entri, "entri %s", a.KabupatenKode)
		assert.Equal(t, a.Target, got.target, "target %s", a.KabupatenKode)
		assert.Equal(t, a.JumlahPetugas, got.petugas, "jumlah petugas %s", a.KabupatenKode)
	}
	assert.Len(t, perKabupaten, 10)
}

// Hitungan tangan pada data terkendali. Hari ini = H (waktu Aceh).
//
// Aulia (mahasiswa_1, 1171):
//
//	H 09.00-09.05: KONSUMEN 3 DIAJUKAN, 2 DIPUBLIKASI, 1 DITOLAK; 09.06 KONSUMEN DRAF (diabaikan);
//	               09.07-09.08: PRODUSEN 2 DIVALIDASI (bukan KONSUMEN, tidak menghitung target; tetap entri terakhir)
//	H-1 10.00: 4 KONSUMEN DITOLAK, 2 KONSUMEN DIAJUKAN; H-6 10.00: 1 DITOLAK; H-7 10.00: 5 DITOLAK (di luar jendela)
//	H-40: 1 KONSUMEN DIAJUKAN (antrean seusia apa pun)
//	entri_hari_ini  = 3 + 2 + 1 = 6      (DRAF dan PRODUSEN tidak)
//	menunggu        = 3 + 2 + 1 = 6      (DIAJUKAN seusia apa pun)
//	ditolak_jendela = 1 + 4 + 1 = 6      (H-7 tidak)
//	entri_terakhir  = H 09.08 WIB        (DRAF 09.06 diabaikan, PRODUSEN dihitung)
func sisipDataPetugasTerkendali(t *testing.T, s *suite) {
	t.Helper()
	h := hariIni()
	const pasarBandaAceh = "PSR-1171-01"
	e := func(slug, pasar, kom, tipe, status string, waktu time.Time) {
		s.sisipEntriPasti(t, slug, pasar, kom, tipe, status, 10000, waktu)
	}
	for i, st := range []struct{ kom, status string }{
		{"TMT", "DIAJUKAN"}, {"CBM-KRT", "DIAJUKAN"}, {"BWM", "DIAJUKAN"},
		{"DAG-AYM", "DIPUBLIKASI"}, {"TLR", "DIPUBLIKASI"}, {"GLA", "DITOLAK"},
	} {
		e(petugasSlug1, pasarBandaAceh, st.kom, "KONSUMEN", st.status, pukul(h, 9, i))
	}
	e(petugasSlug1, pasarBandaAceh, "KTG", "KONSUMEN", "DRAF", pukul(h, 9, 6))
	e(petugasSlug1, pasarBandaAceh, "TMT", "PRODUSEN", "DIVALIDASI", pukul(h, 9, 7))
	e(petugasSlug1, pasarBandaAceh, "BWM", "PRODUSEN", "DIVALIDASI", pukul(h, 9, 8))
	for i, kom := range []string{"TMT", "CBM-KRT", "BWM", "KTG"} {
		e(petugasSlug1, pasarBandaAceh, kom, "KONSUMEN", "DITOLAK", pukul(h.AddDate(0, 0, -1), 10, i))
	}
	for i, kom := range []string{"TMT", "BWM"} {
		e(petugasSlug1, pasarBandaAceh, kom, "KONSUMEN", "DIAJUKAN", pukul(h.AddDate(0, 0, -1), 11, i))
	}
	e(petugasSlug1, pasarBandaAceh, "TMT", "KONSUMEN", "DITOLAK", pukul(h.AddDate(0, 0, -6), 10, 0))
	for i := range 5 {
		e(petugasSlug1, pasarBandaAceh, "TMT", "KONSUMEN", "DITOLAK", pukul(h.AddDate(0, 0, -7), 10, i))
	}
	e(petugasSlug1, pasarBandaAceh, "TMT", "KONSUMEN", "DIAJUKAN", pukul(h.AddDate(0, 0, -40), 10, 0))

	// Cut Nanda (1108): dua entri di pasar kabupatennya dan satu di pasar Banda Aceh. Entri dihitung milik petugas yang
	// mencatatnya, tanpa memandang kabupaten pasarnya.
	e(petugasSlug2, "PSR-1108-01", "TMT", "KONSUMEN", "DIPUBLIKASI", pukul(h, 8, 0))
	e(petugasSlug2, "PSR-1108-01", "BWM", "KONSUMEN", "DIPUBLIKASI", pukul(h, 8, 1))
	e(petugasSlug2, pasarBandaAceh, "BWM", "KONSUMEN", "DIPUBLIKASI", pukul(h, 8, 2))

	// Koordinator Banda Aceh juga mencatat dua entri hari ini, tetapi bukan petugas dengan target harian.
	e("koordinator_lapangan", pasarBandaAceh, "TMT", "KONSUMEN", "DIPUBLIKASI", pukul(h, 7, 0))
	e("koordinator_lapangan", pasarBandaAceh, "BWM", "KONSUMEN", "DIPUBLIKASI", pukul(h, 7, 1))
}

// Hitungan tangan tugas Aulia. now = saat tes; jendela tugas dimulai H-6 00.00 WIB.
//
//	 1 BELUM, tenggat +5 jam                      -> aktif
//	 2 BERJALAN, tenggat +26 jam, progres 2       -> aktif
//	 3 BELUM tersimpan, tenggat +3 jam, progres 1 -> BERJALAN efektif -> aktif
//	 4 TERLAMBAT, tenggat -3 jam                  -> terlambat
//	 5 BELUM tersimpan, tenggat -1 jam            -> TERLAMBAT efektif (status tersimpan belum diperbarui)
//	 6 BERJALAN, tenggat -2 jam, progres 3        -> TERLAMBAT efektif (lewat tenggat, belum selesai)
//	 7 SELESAI, tenggat -22 jam                   -> selesai
//	 8 SELESAI, tenggat H-5                       -> selesai (dalam jendela)
//	 9 SELESAI, tenggat H-7                       -> di luar jendela, tidak dihitung
//	10 TERLAMBAT, tenggat H-8                     -> di luar jendela, tidak dihitung
//	11 TERLAMBAT tersimpan, tenggat +2 jam, progres 0 -> TERLAMBAT efektif (aturan StatusEfektif: status tersimpan dipakai)
//	12 TERLAMBAT tersimpan, tenggat +2 jam, progres 2 -> BERJALAN efektif -> aktif
//	aktif = {1, 2, 3, 12} = 4; terlambat = {4, 5, 6, 11} = 4; selesai = {7, 8} = 2
func sisipTugasAulia(t *testing.T, s *suite, now time.Time) {
	t.Helper()
	h := hariIni()
	jam := func(n float64) time.Time { return now.Add(time.Duration(n * float64(time.Hour))) }
	selesai := func(w time.Time) *time.Time { return &w }
	s.sisipTugasPasti(t, petugasSlug1, "T-01", "BELUM", jam(5), 0, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-02", "BERJALAN", jam(26), 2, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-03", "BELUM", jam(3), 1, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-04", "TERLAMBAT", jam(-3), 0, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-05", "BELUM", jam(-1), 0, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-06", "BERJALAN", jam(-2), 3, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-07", "SELESAI", jam(-22), 4, selesai(jam(-23)))
	s.sisipTugasPasti(t, petugasSlug1, "T-08", "SELESAI", pukul(h.AddDate(0, 0, -5), 12, 0), 4, selesai(pukul(h.AddDate(0, 0, -5), 11, 0)))
	s.sisipTugasPasti(t, petugasSlug1, "T-09", "SELESAI", pukul(h.AddDate(0, 0, -7), 12, 0), 4, selesai(pukul(h.AddDate(0, 0, -7), 11, 0)))
	s.sisipTugasPasti(t, petugasSlug1, "T-10", "TERLAMBAT", pukul(h.AddDate(0, 0, -8), 12, 0), 0, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-11", "TERLAMBAT", jam(2), 0, nil)
	s.sisipTugasPasti(t, petugasSlug1, "T-12", "TERLAMBAT", jam(2), 2, nil)
	// Tugas petugas lain tidak boleh tercampur.
	s.sisipTugasPasti(t, petugasSlug2, "T-20", "BELUM", jam(4), 0, nil)
}

func TestAktivitasPetugas_AngkaHasilHitungTangan(t *testing.T) {
	s := terpisah(t, "dashboard_petugas_terkendali", false)
	now := time.Now()
	sisipDataPetugasTerkendali(t, s)
	sisipTugasAulia(t, s, now)
	// Akun yang tidak boleh masuk daftar: tanpa kabupaten tugas, menunggu aktivasi, dan nonaktif walau berkabupaten.
	s.sisipPengguna(t, "Petugas Tanpa Kabupaten", "tanpa.kab@gardainflasi.id", "OPS_MAHASISWA", "AKTIF", nil)
	s.sisipPengguna(t, "Petugas Menunggu", "menunggu@gardainflasi.id", "OPS_MAHASISWA", "MENUNGGU", "1171")
	s.sisipPengguna(t, "Petugas Nonaktif Berkabupaten", "nonaktif.kab@gardainflasi.id", "OPS_MAHASISWA", "NONAKTIF", "1171")
	idAulia := s.uuidAkun(t, petugasSlug1)
	idCut := s.uuidAkun(t, petugasSlug2)

	r := s.petugasDari(t, emailBerhak, "")

	require.Len(t, r.Petugas, 10, "hanya mahasiswa aktif berkabupaten")
	aulia := cariPetugas(t, r, idAulia)
	assert.Equal(t, "Aulia Rahman", aulia.Nama)
	assert.Equal(t, "1171", aulia.KabupatenKode)
	assert.Equal(t, 6, aulia.EntriHariIni, "DRAF dan PRODUSEN tidak dihitung; DITOLAK hari ini dihitung sebagai entri")
	assert.Equal(t, 14, aulia.TargetHarian)
	assert.Equal(t, 6, aulia.MenungguValidasi, "DIAJUKAN seusia apa pun")
	assert.Equal(t, 6, aulia.DitolakJendela, "H-7 di luar jendela tujuh hari")
	require.NotNil(t, aulia.EntriTerakhirPada)
	assert.True(t, pukul(hariIni(), 9, 8).Equal(*aulia.EntriTerakhirPada), "entri terakhir 09.08 WIB (DRAF 09.06 diabaikan, PRODUSEN dihitung): %s", aulia.EntriTerakhirPada)
	assert.Equal(t, 4, aulia.TugasAktif)
	assert.Equal(t, 4, aulia.TugasTerlambat)
	assert.Equal(t, 2, aulia.TugasSelesai)

	cut := cariPetugas(t, r, idCut)
	assert.Equal(t, 3, cut.EntriHariIni, "entri dihitung milik pencatatnya walau pasarnya di kabupaten lain")
	assert.Zero(t, cut.MenungguValidasi)
	assert.Zero(t, cut.DitolakJendela)
	assert.Equal(t, 1, cut.TugasAktif, "tugas petugas lain tidak tercampur")
	assert.Zero(t, cut.TugasTerlambat+cut.TugasSelesai)

	// Petugas tanpa aktivitas tetap tampil, bernilai nol, dan entri terakhirnya null.
	for _, p := range r.Petugas {
		if p.ID == idAulia || p.ID == idCut {
			continue
		}
		assert.Zero(t, p.EntriHariIni+p.MenungguValidasi+p.DitolakJendela+p.TugasAktif+p.TugasSelesai+p.TugasTerlambat, p.Nama)
		assert.Nil(t, p.EntriTerakhirPada, p.Nama)
		assert.Equal(t, 14, p.TargetHarian, p.Nama)
	}

	// Koordinator yang mencatat entri bukan petugas dengan target harian dan tidak ada di daftar.
	for _, p := range r.Petugas {
		assert.NotEqual(t, "Zulkifli, S.P.", p.Nama)
		assert.NotContains(t, []string{"Petugas Tanpa Kabupaten", "Petugas Menunggu", "Petugas Nonaktif Berkabupaten", "User Nonaktif"}, p.Nama)
	}
}

// Status efektif tugas di SQL harus sama dengan lapangan/domain.TugasView.StatusEfektif pada semua kombinasi status
// tersimpan, progres, dan tenggat: dua definisi "terlambat" yang berbeda tidak boleh hidup berdampingan.
func TestAktivitasPetugas_StatusTugasSamaDenganStatusEfektifModulLapangan(t *testing.T) {
	s := terpisah(t, "dashboard_petugas_paritas_tugas", false)
	now := time.Now()
	h := hariIni()

	kombinasi := 0
	for _, status := range []string{"BELUM", "BERJALAN", "TERLAMBAT", "SELESAI"} {
		for _, progres := range []int{0, 2} {
			for _, geser := range []time.Duration{-30 * time.Hour, -time.Minute * 90, 90 * time.Minute, 30 * time.Hour} {
				kombinasi++
				var selesaiPada *time.Time
				if status == "SELESAI" {
					w := now.Add(geser - time.Hour)
					selesaiPada = &w
				}
				s.sisipTugasPasti(t, petugasSlug1, fmt.Sprintf("PR-%02d", kombinasi), status, now.Add(geser), progres, selesaiPada)
			}
		}
	}
	// Di luar jendela: tidak dihitung walau statusnya apa pun.
	s.sisipTugasPasti(t, petugasSlug1, "LUAR-1", "TERLAMBAT", pukul(h.AddDate(0, 0, -7), 23, 0), 0, nil)
	s.sisipTugasPasti(t, petugasSlug1, "LUAR-2", "BELUM", pukul(h.AddDate(0, 0, -30), 23, 0), 0, nil)

	// Harapan dari fungsi modul lapangan atas baris yang sama.
	var rows []struct {
		Status        string
		Tenggat       time.Time
		JumlahSelesai int
	}
	require.NoError(t, s.db.Raw(`SELECT status::text AS status, tenggat, jumlah_selesai FROM tugas_lapangan
		WHERE petugas_id = garda_uuid_stabil('pengguna', ?) AND tenggat >= ?`, petugasSlug1, pukul(h.AddDate(0, 0, -6), 0, 0)).Scan(&rows).Error)
	require.Len(t, rows, kombinasi)
	var aktif, selesai, terlambat int
	patokan := time.Now() // API menilai pada saat permintaan; kombinasi uji berjarak >= 90 menit dari tenggat
	for _, row := range rows {
		switch (lapangan.TugasView{Status: row.Status, Tenggat: row.Tenggat, JumlahSelesai: row.JumlahSelesai}).StatusEfektif(patokan) {
		case lapangan.StatusBelum, lapangan.StatusBerjalan:
			aktif++
		case lapangan.StatusSelesai:
			selesai++
		case lapangan.StatusTerlambat:
			terlambat++
		}
	}
	require.Equal(t, kombinasi, aktif+selesai+terlambat)

	p := cariPetugas(t, s.petugasDari(t, emailBerhak, ""), s.uuidAkun(t, petugasSlug1))

	assert.Equal(t, aktif, p.TugasAktif, "aktif")
	assert.Equal(t, selesai, p.TugasSelesai, "selesai")
	assert.Equal(t, terlambat, p.TugasTerlambat, "terlambat")
	assert.Positive(t, aktif)
	assert.Positive(t, selesai)
	assert.Positive(t, terlambat)
}

// Row scope (02-B): Koordinator dan Agregator hanya melihat petugas yang bertugas di kabupatennya.
func TestAktivitasPetugas_CakupanKoordinatorDanAgregator(t *testing.T) {
	s := sharedSuite(t)
	provinsi := s.petugasDari(t, emailBerhak, "")

	for _, email := range []string{emailKoordinator, emailAgregator} {
		t.Run(email, func(t *testing.T) {
			r := s.petugasDari(t, email, "")

			require.Len(t, r.Petugas, 1, "hanya Aulia Rahman bertugas di Kota Banda Aceh")
			assert.Equal(t, "Aulia Rahman", r.Petugas[0].Nama)
			assert.Equal(t, kabBanda, r.Petugas[0].KabupatenKode)
			assert.Equal(t, cariPetugas(t, provinsi, r.Petugas[0].ID), r.Petugas[0], "angkanya sama dengan tampilan seluruh Aceh")

			// Filter ke kabupaten lain: kosong, bukan 403 dan bukan data seluruh Aceh.
			lain := s.petugasDari(t, email, "?kabupaten_kode="+kabSelatan)
			assert.NotNil(t, lain.Petugas)
			assert.Empty(t, lain.Petugas)
			assert.Len(t, s.petugasDari(t, email, "?kabupaten_kode="+kabBanda).Petugas, 1)
		})
	}
}

func TestAktivitasPetugas_FilterKabupatenPeranProvinsi(t *testing.T) {
	s := sharedSuite(t)

	r := s.petugasDari(t, emailBerhak, "?kabupaten_kode="+kabSelatan)

	require.Len(t, r.Petugas, 1)
	assert.Equal(t, "Cut Nanda", r.Petugas[0].Nama)
	assert.Equal(t, kabSelatan, r.Petugas[0].KabupatenKode)

	for _, kode := range []string{"1101", "9999"} {
		kosong := s.petugasDari(t, emailBerhak, "?kabupaten_kode="+kode)
		assert.NotNil(t, kosong.Petugas, "kabupaten tanpa petugas: [] bukan null")
		assert.Empty(t, kosong.Petugas, kode)
	}
	// komoditas_kode tidak dibaca endpoint ini, bahkan yang salah bentuk.
	assert.Len(t, s.petugasDari(t, emailBerhak, "?komoditas_kode=bukan%20kode").Petugas, 10)
}

// Kabupaten tugas dibaca dari basis data pada tiap permintaan: sesudah dipindah, daftar mengikuti kabupaten baru
// walau token lama masih membawa klaim kabupaten lama dan hasil kabupaten lama sempat tersimpan di cache.
func TestAktivitasPetugas_KabupatenDibacaDariBasisData(t *testing.T) {
	s := ingestionSuite(t)
	s.login(t, emailKoordinator)
	require.Equal(t, "Aulia Rahman", s.petugasDari(t, emailKoordinator, "").Petugas[0].Nama)

	t.Run("dipindah ke kabupaten lain", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, kabSelatan)

		r := s.petugasDari(t, emailKoordinator, "")
		require.Len(t, r.Petugas, 1)
		assert.Equal(t, "Cut Nanda", r.Petugas[0].Nama, "daftar kabupaten lama tidak bocor lewat cache")
	})
	t.Run("tanpa kabupaten tugas: tertutup, bukan seluruh Aceh", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, "")

		r := s.petugasDari(t, emailKoordinator, "")
		assert.NotNil(t, r.Petugas)
		assert.Empty(t, r.Petugas)
	})
}

// Hasil cache satu cakupan tidak boleh tersaji ke cakupan lain.
func TestAktivitasPetugas_HasilCacheTidakTersajiKeCakupanLain(t *testing.T) {
	s := sharedSuite(t)

	for range 3 {
		for _, email := range []string{emailBerhak, emailKoordinator, emailManajerData, emailAgregator} {
			provinsi := email == emailBerhak || email == emailManajerData
			assert.Len(t, s.petugasDari(t, email, "").Petugas, map[bool]int{true: 10, false: 1}[provinsi], email)
		}
	}
}

func TestAktivitasPetugas_Otorisasi(t *testing.T) {
	s := sharedSuite(t)
	jalur := awalan + "/aktivitas-petugas"

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, jalur, "", nil).Code, "tanpa token")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, jalur, emailTakBerhak, nil).Code, "mahasiswa tanpa dashboard.operasional")
	for _, email := range []string{emailBerhak, emailKoordinator, emailAgregator, emailManajerData, "superadmin@gardainflasi.id"} {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, jalur, email, nil).Code, email)
	}
}

// Nama dan ID petugas hanya boleh dilihat yang boleh melihat entri harga (harga.lihat), yang memuat petugas_id dan
// petugas_nama yang sama. Peran kustom yang memegang dashboard.operasional tanpa harga.lihat ditolak di sini, tetapi
// tetap boleh membuka ringkasan lain di grup yang sama.
func TestAktivitasPetugas_ButuhHargaLihatSelainDashboardOperasional(t *testing.T) {
	s := terpisah(t, "dashboard_petugas_izin", true)
	require.NoError(t, s.db.Exec(`DELETE FROM role_permissions WHERE role_kunci = 'STRATEGIS_BI' AND permission_nama = 'harga.lihat'`).Error)

	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, awalan+"/aktivitas-petugas", emailBerhak, nil).Code,
		"dashboard.operasional tanpa harga.lihat")
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, awalan+"/stok", emailBerhak, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, awalan+"/kpi", emailBerhak, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, awalan+"/aktivitas-lapangan", emailBerhak, nil).Code,
		"hitungan per kabupaten (tanpa nama) tetap terbuka")
}

// Penjaga asumsi di atas: setiap peran bawaan yang memegang dashboard.operasional juga memegang harga.lihat, jadi
// syarat tambahan itu tidak mengurangi akses siapa pun. Bila suatu hari peran baru diberi dashboard.operasional tanpa
// harga.lihat, tes ini memaksa keputusan diulang (beri izinnya, atau terima bahwa peran itu tidak melihat nama petugas).
func TestAktivitasPetugas_SemuaPeranDashboardOperasionalMemegangHargaLihat(t *testing.T) {
	s := sharedSuite(t)

	var tanpa []string
	require.NoError(t, s.db.Raw(`
		SELECT role_kunci FROM role_permissions WHERE permission_nama = 'dashboard.operasional'
		EXCEPT
		SELECT role_kunci FROM role_permissions WHERE permission_nama = 'harga.lihat'`).Scan(&tanpa).Error)

	assert.Empty(t, tanpa, "peran pemegang dashboard.operasional yang tidak memegang harga.lihat")
}

// Hanya nama tampilan dan ID publik: tidak ada email, telepon, atau data pribadi lain di respons.
func TestAktivitasPetugas_TanpaDataPribadiLain(t *testing.T) {
	s := sharedSuite(t)

	mentah := string(s.mentah(t, awalan+"/aktivitas-petugas", emailBerhak))

	for _, larangan := range []string{"@", "telepon", "email", "+62", "password", "mfa", "lembaga"} {
		assert.NotContains(t, strings.ToLower(mentah), larangan)
	}
	var r struct {
		Petugas []map[string]any `json:"petugas"`
	}
	require.NoError(t, json.Unmarshal([]byte(mentah), &r))
	require.NotEmpty(t, r.Petugas)
	assert.ElementsMatch(t, []string{"id", "nama", "kabupaten_kode", "kabupaten_nama", "entri_hari_ini", "target_harian", "menunggu_validasi",
		"ditolak_jendela", "entri_terakhir_pada", "tugas_aktif", "tugas_selesai", "tugas_terlambat"}, kunciPeta(r.Petugas[0]))
}

func TestAktivitasPetugas_ParameterTidakValidDijawab400(t *testing.T) {
	s := sharedSuite(t)
	for _, q := range []string{"?kabupaten_kode=abc", "?kabupaten_kode=117", "?kabupaten_kode=11711", "?kabupaten_kode=1171%20"} {
		w := s.call(t, http.MethodGet, awalan+"/aktivitas-petugas"+q, emailBerhak, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
		assert.Contains(t, w.Body.String(), "kabupaten_kode harus 4 digit", q)
	}
}

// Tanpa petugas sama sekali: 200 dengan larik kosong, bukan galat dan bukan null.
func TestAktivitasPetugas_TanpaPetugasTidakGalat(t *testing.T) {
	s := terpisah(t, "dashboard_petugas_kosong", false)
	require.NoError(t, s.db.Exec(`UPDATE pengguna SET status = 'NONAKTIF' WHERE peran = 'OPS_MAHASISWA'`).Error)

	mentah := string(s.mentah(t, awalan+"/aktivitas-petugas", emailBerhak))

	assert.Contains(t, mentah, `"petugas":[]`)
}

// Basis data kosong (hanya akun demo): semua petugas tampil dengan angka nol.
func TestAktivitasPetugas_BasisDataKosongNolBukanGalat(t *testing.T) {
	s := terpisah(t, "dashboard_petugas_nol", false)

	r := s.petugasDari(t, emailBerhak, "")

	require.Len(t, r.Petugas, 10)
	for _, p := range r.Petugas {
		assert.Zero(t, p.EntriHariIni+p.MenungguValidasi+p.DitolakJendela+p.TugasAktif+p.TugasSelesai+p.TugasTerlambat, p.Nama)
		assert.Nil(t, p.EntriTerakhirPada)
		assert.Equal(t, 14, p.TargetHarian)
	}
}
