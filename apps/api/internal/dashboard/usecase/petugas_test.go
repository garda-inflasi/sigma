package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func petugasUji(id, nama, kabKode, kabNama string) domain.AktivitasPetugas {
	return domain.AktivitasPetugas{ID: id, Nama: nama, KabupatenKode: kabKode, KabupatenNama: kabNama, TargetHarian: 14}
}

func TestGetAktivitasPetugas_BatasWaktuAcehDanFilterDiteruskanKeRepository(t *testing.T) {
	repo := &fakeRepo{}

	hasil, err := newUC(repo).GetAktivitasPetugas(t.Context(), banda, "1171")

	require.NoError(t, err)
	assert.Equal(t, hariUji, hasil.Tanggal, "17 Oktober 03.00 WIB: hari ini 17 Oktober")
	q := repo.qPetugas
	assert.Equal(t, banda, q.Cakupan)
	assert.Equal(t, "1171", q.KabupatenKode)
	// Hari ini = [17 Okt 00.00 WIB, 18 Okt 00.00 WIB); jendela 7 hari dimulai 11 Okt 00.00 WIB.
	assert.True(t, time.Date(2026, 10, 17, 0, 0, 0, 0, wib).Equal(q.AwalHari), "awal hari WIB: %s", q.AwalHari)
	assert.True(t, time.Date(2026, 10, 18, 0, 0, 0, 0, wib).Equal(q.AkhirHari), "akhir hari WIB: %s", q.AkhirHari)
	assert.True(t, time.Date(2026, 10, 11, 0, 0, 0, 0, wib).Equal(q.AwalJendela), "awal jendela WIB: %s", q.AwalJendela)
	assert.True(t, sekarangUji.Equal(q.Sekarang), "status efektif tugas dinilai pada saat permintaan")
}

// Jam dibaca sekali: batas hari dan saat status tugas berasal dari satu pembacaan, juga tepat di sekitar tengah malam.
func TestGetAktivitasPetugas_JamDibacaSekaliDiSekitarTengahMalam(t *testing.T) {
	for _, c := range []struct {
		nama     string
		saat     time.Time
		wantHari time.Time
	}{
		{"23.59 WIB 16 Okt", time.Date(2026, 10, 16, 16, 59, 0, 0, time.UTC), tgl("2026-10-16")},
		{"00.01 WIB 17 Okt", time.Date(2026, 10, 16, 17, 1, 0, 0, time.UTC), tgl("2026-10-17")},
	} {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{}
			uc := newUC(repo)
			uc.now = func() time.Time { return c.saat }

			hasil, err := uc.GetAktivitasPetugas(t.Context(), semuaAceh, "")

			require.NoError(t, err)
			assert.Equal(t, c.wantHari, hasil.Tanggal)
			assert.True(t, awalHariWIB(c.wantHari).Equal(repo.qPetugas.AwalHari))
			assert.True(t, c.saat.Equal(repo.qPetugas.Sekarang))
		})
	}
}

func TestGetAktivitasPetugas_UrutKabupatenNamaLaluID(t *testing.T) {
	repo := &fakeRepo{petugas: func(domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
		return []domain.AktivitasPetugas{
			petugasUji("id-5", "Zainal", "1171", "KOTA BANDA ACEH"),
			petugasUji("id-3", "budi", "1108", "KABUPATEN ACEH BESAR"),
			petugasUji("id-2", "Budi", "1108", "KABUPATEN ACEH BESAR"),
			petugasUji("id-9", "Aulia", "1108", "KABUPATEN ACEH BESAR"),
			petugasUji("id-4", "Aulia", "1171", "KOTA BANDA ACEH"),
			petugasUji("id-1", "Aulia", "1171", "KOTA BANDA ACEH"),
		}, nil
	}}

	hasil, err := newUC(repo).GetAktivitasPetugas(t.Context(), semuaAceh, "")

	require.NoError(t, err)
	id := make([]string, len(hasil.Petugas))
	for i, p := range hasil.Petugas {
		id[i] = p.ID
	}
	// Kabupaten (tanpa membedakan huruf besar-kecil), lalu nama (Aulia, Budi/budi: huruf besar-kecil sama dulu, seri
	// diputus teks asli: "Budi" < "budi"), lalu ID.
	assert.Equal(t, []string{"id-9", "id-2", "id-3", "id-1", "id-4", "id-5"}, id)
}

func TestGetAktivitasPetugas_TidakMengubahIrisanDariRepository(t *testing.T) {
	asli := []domain.AktivitasPetugas{
		petugasUji("id-2", "Budi", "1171", "KOTA BANDA ACEH"),
		petugasUji("id-1", "Aulia", "1171", "KOTA BANDA ACEH"),
	}
	repo := &fakeRepo{petugas: func(domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) { return asli, nil }}

	hasil, err := newUC(repo).GetAktivitasPetugas(t.Context(), semuaAceh, "")

	require.NoError(t, err)
	assert.Equal(t, "id-1", hasil.Petugas[0].ID)
	assert.Equal(t, "id-2", asli[0].ID, "hasil dibagikan lewat cache; irisan milik repository tidak boleh diubah urutannya")
}

func TestGetAktivitasPetugas_MeneruskanAngkaApaAdanya(t *testing.T) {
	terakhir := time.Date(2026, 10, 17, 2, 30, 0, 0, time.UTC)
	repo := &fakeRepo{petugas: func(domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
		return []domain.AktivitasPetugas{{
			ID: "id-1", Nama: "Aulia", KabupatenKode: "1171", KabupatenNama: "KOTA BANDA ACEH",
			EntriHariIni: 10, TargetHarian: 14, MenungguValidasi: 19, DitolakJendela: 7, EntriTerakhirPada: &terakhir,
			TugasAktif: 3, TugasSelesai: 1, TugasTerlambat: 1,
		}}, nil
	}}

	hasil, err := newUC(repo).GetAktivitasPetugas(t.Context(), semuaAceh, "")

	require.NoError(t, err)
	require.Len(t, hasil.Petugas, 1)
	p := hasil.Petugas[0]
	assert.Equal(t, 10, p.EntriHariIni)
	assert.Equal(t, 14, p.TargetHarian)
	assert.Equal(t, 19, p.MenungguValidasi)
	assert.Equal(t, 7, p.DitolakJendela)
	assert.Equal(t, 3, p.TugasAktif)
	assert.Equal(t, 1, p.TugasSelesai)
	assert.Equal(t, 1, p.TugasTerlambat)
	require.NotNil(t, p.EntriTerakhirPada)
	assert.True(t, terakhir.Equal(*p.EntriTerakhirPada))
}

func TestGetAktivitasPetugas_TanpaPetugasDaftarKosongBukanNil(t *testing.T) {
	hasil, err := newUC(&fakeRepo{}).GetAktivitasPetugas(t.Context(), semuaAceh, "")

	require.NoError(t, err)
	assert.NotNil(t, hasil.Petugas, "JSON [] bukan null")
	assert.Empty(t, hasil.Petugas)
}

func TestGetAktivitasPetugas_CakupanTertutupDanIrisanKosongTanpaBasisData(t *testing.T) {
	cases := []struct {
		nama    string
		cakupan domain.Cakupan
		filter  string
	}{
		{"cakupan tertutup", tertutup, ""},
		{"cakupan tertutup dengan filter kabupaten", tertutup, "1171"},
		{"kabupaten di luar cakupan", banda, "1108"},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{}
			uc := newUC(repo)

			hasil, err := uc.GetAktivitasPetugas(t.Context(), c.cakupan, c.filter)

			require.NoError(t, err)
			assert.NotNil(t, hasil.Petugas)
			assert.Empty(t, hasil.Petugas)
			assert.Equal(t, hariUji, hasil.Tanggal)
			assert.Zero(t, repo.totalPanggilanData())
			assert.Zero(t, uc.cache.Ukuran(), "hasil kosong karena cakupan tidak masuk cache")
		})
	}
}

func TestGetAktivitasPetugas_ParameterTidakValidTidakMenyentuhBasisData(t *testing.T) {
	for _, kab := range []string{"abc", "117", "11711", "1171 "} {
		repo := &fakeRepo{}
		uc := newUC(repo)

		_, err := uc.GetAktivitasPetugas(t.Context(), semuaAceh, kab)

		require.ErrorIs(t, err, ErrValidasi, kab)
		assert.Zero(t, repo.totalPanggilanData(), kab)
		assert.Zero(t, uc.cache.Ukuran(), kab)
	}
}

func TestGetAktivitasPetugas_GalatRepositoryDibungkusDanBukanValidasi(t *testing.T) {
	boom := errors.New("db mati")
	repo := &fakeRepo{petugas: func(domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) { return nil, boom }}
	uc := newUC(repo)

	_, err := uc.GetAktivitasPetugas(t.Context(), semuaAceh, "")

	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrValidasi, "galat basis data adalah 500, bukan 400")
	assert.Zero(t, uc.cache.Ukuran(), "galat tidak disimpan")
}
