package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePada(t *testing.T) {
	t.Run("RFC3339 dipakai apa adanya", func(t *testing.T) {
		got, err := ParsePada("2026-03-01T10:00:00+07:00")
		require.NoError(t, err)
		assert.True(t, got.Equal(time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC)))

		got, err = ParsePada("2026-03-01T10:00:00Z")
		require.NoError(t, err)
		assert.True(t, got.Equal(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)))

		got, err = ParsePada("2026-03-01T10:00:00.250Z")
		require.NoError(t, err)
		assert.Equal(t, 250_000_000, got.Nanosecond())
	})

	t.Run("tanggal saja berarti akhir hari WIB", func(t *testing.T) {
		got, err := ParsePada("2026-03-01")
		require.NoError(t, err)
		// 23:59:59.999999999 WIB = 16:59:59.999999999 UTC pada hari yang sama.
		assert.True(t, got.Equal(time.Date(2026, 3, 1, 16, 59, 59, 999_999_999, time.UTC)), got.String())

		pagi, err := ParsePada("2026-03-02")
		require.NoError(t, err)
		assert.True(t, pagi.After(got))
	})

	t.Run("spasi di tepi dibuang", func(t *testing.T) {
		_, err := ParsePada(" 2026-03-01 ")
		assert.NoError(t, err)
	})

	t.Run("bentuk lain ditolak", func(t *testing.T) {
		for _, s := range []string{"", "kemarin", "2026-13-01", "2026-02-30", "01-03-2026", "2026-03-01T10:00:00", "2026-03-01 10:00:00", "1772000000"} {
			_, err := ParsePada(s)
			assert.ErrorIs(t, err, ErrPadaTidakValid, "%q", s)
		}
	})
}

func TestBentukKodeValid(t *testing.T) {
	for _, ok := range []string{"BRS-MED", "PSR-1171-01", "a", "Kode dengan spasi", strings.Repeat("x", 255)} {
		assert.True(t, BentukKodeValid(ok), "%q", ok)
	}
	for _, buruk := range []string{"", "ada\x00nul", "baris\nbaru", "tab\t", "\xff\xfe", strings.Repeat("x", 256)} {
		assert.False(t, BentukKodeValid(buruk), "%q", buruk)
	}
}

// repoReferensiPalsu menghitung panggilan agar tes dapat membuktikan kode salah bentuk tidak menyentuh repositori.
type repoReferensiPalsu struct {
	domain.RepositoryReferensi
	dipanggil int
	hapusErr  error
	komoditas domain.Komoditas
}

func (r *repoReferensiPalsu) GetKecamatan(context.Context, string) (domain.KecamatanRef, error) {
	r.dipanggil++
	return domain.KecamatanRef{}, domain.ErrTidakDitemukan
}

func (r *repoReferensiPalsu) HapusKomoditas(context.Context, string) (domain.Komoditas, error) {
	r.dipanggil++
	return r.komoditas, r.hapusErr
}

type auditPalsu struct{ entries []audit.Entry }

func (a *auditPalsu) Write(_ context.Context, e audit.Entry) error {
	a.entries = append(a.entries, e)
	return nil
}

func TestUseCaseReferensi_KodeSalahBentukTanpaRepositori(t *testing.T) {
	repo := &repoReferensiPalsu{}
	uc := NewReferensi(repo, &auditPalsu{}, logger.NewDefault())

	for _, kode := range []string{"", "abc", "117101", "11710100", "1171010 ", "1171O10"} {
		_, err := uc.GetKecamatan(t.Context(), kode)
		assert.ErrorIs(t, err, domain.ErrTidakDitemukan, "%q", kode)
	}
	assert.Zero(t, repo.dipanggil)

	_, err := uc.GetWilayah(t.Context(), "11")
	assert.ErrorIs(t, err, domain.ErrTidakDitemukan)
	assert.ErrorIs(t, uc.HapusKomoditas(t.Context(), "ada\x00nul", "u"), domain.ErrTidakDitemukan)
	assert.ErrorIs(t, uc.HapusPasar(t.Context(), "", "u"), domain.ErrTidakDitemukan)
	assert.Zero(t, repo.dipanggil)

	_, err = uc.GetKecamatan(t.Context(), "1171010")
	assert.ErrorIs(t, err, domain.ErrTidakDitemukan)
	assert.Equal(t, 1, repo.dipanggil, "kode yang benar bentuknya sampai ke repositori")
}

func TestUseCaseReferensi_HapusMencatatAuditHanyaBilaBerhasil(t *testing.T) {
	wr := &auditPalsu{}
	repo := &repoReferensiPalsu{komoditas: domain.Komoditas{Kode: "GLA", Nama: "Gula pasir", Satuan: "kg", HargaAcuan: 18000, Versi: 2, Status: "AKTIF"}}
	uc := NewReferensi(repo, wr, logger.NewDefault())

	for _, galat := range []error{domain.ErrAdaUsulan, domain.ErrDirujuk, domain.ErrTidakDitemukan, errors.New("basis data mati")} {
		repo.hapusErr = galat
		assert.ErrorIs(t, uc.HapusKomoditas(t.Context(), "GLA", "u-1"), galat)
	}
	assert.Empty(t, wr.entries, "penghapusan yang gagal tidak dicatat sebagai HAPUS_MASTER")

	repo.hapusErr = nil
	require.NoError(t, uc.HapusKomoditas(t.Context(), "GLA", "u-1"))
	require.Len(t, wr.entries, 1)
	e := wr.entries[0]
	assert.Equal(t, "HAPUS_MASTER", e.Aksi)
	assert.Equal(t, "komoditas", e.Entitas)
	assert.Equal(t, "GLA", e.EntitasID)
	assert.Equal(t, "u-1", e.AktorID)
	assert.Equal(t, audit.HasilBerhasil, e.Hasil)
	sebelum, ok := e.Sebelum.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Gula pasir", sebelum["nama"])
	assert.Equal(t, 2, sebelum["versi"])
}
