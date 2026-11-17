package usecase

import (
	"testing"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func barisAudit(id idx.ID, aksi, aktorID, peran string) domain.AuditLog {
	ip, ua, nama, surel := "10.0.0.1", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0 Safari/537.36", "Rina", "rina@gardainflasi.id"
	sebelum := `{"telepon":"+6281200000001"}`
	return domain.AuditLog{ID: id, Waktu: fixedNow, AktorID: &aktorID, AktorNama: &nama, AktorEmail: &surel, AktorPeran: &peran,
		Aksi: aksi, Entitas: "pengguna", EntitasID: "x", Hasil: "BERHASIL", IP: &ip, UserAgent: &ua, Sebelum: &sebelum}
}

func TestListSaya_MengunciCakupanKePemanggilDanMengabaikanFilterAkunLain(t *testing.T) {
	repo := &fakeRepo{}
	uc := newUseCase(repo)

	_, err := uc.ListSaya(t.Context(), uid("saya"), QuerySaya{
		Filter: domain.Filter{
			Aksi: "LOGIN", Entitas: "pengguna", Hasil: "BERHASIL",
			// Upaya melonggarkan atau membelokkan cakupan ke akun lain: semuanya dibuang.
			Aktor: uid("admin"), Peran: "ADMIN_SISTEM", CorrelationID: "c", Q: "admin",
			PenggunaID: uid("admin"), Sebagai: domain.SebagaiSemua,
		},
		Sebagai: domain.SebagaiPelaku,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.Filter{
		Aksi: "LOGIN", Entitas: "pengguna", Hasil: "BERHASIL",
		PenggunaID: uid("saya"), Sebagai: domain.SebagaiPelaku,
	}, repo.gotFilter)
}

func TestListSaya_SebagaiKosongDibacaSemuaDanNilaiAsingDitolak(t *testing.T) {
	repo := &fakeRepo{}
	uc := newUseCase(repo)

	_, err := uc.ListSaya(t.Context(), uid("saya"), QuerySaya{})
	require.NoError(t, err)
	assert.Equal(t, domain.SebagaiSemua, repo.gotFilter.Sebagai)

	_, err = uc.ListSaya(t.Context(), uid("saya"), QuerySaya{Sebagai: "orang-lain"})
	assert.ErrorIs(t, err, ErrSebagaiTidakValid)
}

func TestListSaya_BatasHalamanDanKursor(t *testing.T) {
	repo := &fakeRepo{}
	uc := newUseCase(repo)

	_, err := uc.ListSaya(t.Context(), uid("saya"), QuerySaya{})
	require.NoError(t, err)
	assert.Equal(t, DefaultLimitSaya+1, repo.gotLimit, "satu baris ekstra untuk mendeteksi halaman berikutnya")

	_, err = uc.ListSaya(t.Context(), uid("saya"), QuerySaya{Limit: 10_000})
	require.NoError(t, err)
	assert.Equal(t, MaxLimitSaya+1, repo.gotLimit)

	_, err = uc.ListSaya(t.Context(), uid("saya"), QuerySaya{Cursor: "bukan-cursor"})
	assert.ErrorIs(t, err, ErrInvalidCursor)
}

func TestListSaya_MenyempitkanBarisKeBentukAman(t *testing.T) {
	saya, admin := uid("saya"), uid("admin")
	repo := &fakeRepo{rows: []domain.AuditLog{
		barisAudit(3, "RESET_MFA", admin, "ADMIN_SISTEM"),
		barisAudit(2, "LOGIN", saya, "OPS_MAHASISWA"),
		barisAudit(1, "LOGIN", saya, "OPS_MAHASISWA"),
	}}
	uc := newUseCase(repo)

	halaman, err := uc.ListSaya(t.Context(), saya, QuerySaya{Limit: 2})

	require.NoError(t, err)
	require.Len(t, halaman.Items, 2)
	assert.NotEmpty(t, halaman.NextCursor, "masih ada baris sesudah halaman ini")

	sasaran := halaman.Items[0]
	assert.Equal(t, domain.SebagaiSasaran, sasaran.Sebagai)
	assert.Nil(t, sasaran.IP, "IP pelaku lain tidak ditampilkan")
	assert.Nil(t, sasaran.UserAgent)
	require.NotNil(t, sasaran.OlehPeran)
	assert.Equal(t, "ADMIN_SISTEM", *sasaran.OlehPeran)

	pelaku := halaman.Items[1]
	assert.Equal(t, domain.SebagaiPelaku, pelaku.Sebagai)
	require.NotNil(t, pelaku.IP)
	assert.Equal(t, "10.0.0.1", *pelaku.IP)
	assert.Nil(t, pelaku.OlehPeran)
}
