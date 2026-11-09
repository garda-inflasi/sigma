package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pembaca audit (daftar, detail, ekspor) menyamarkan nomor telepon pada baris entitas pengguna yang dibuat sebelum
// snapshot menyamarkannya. Data di repositori (dasar hash rantai) tidak berubah.

const (
	nomorLama     = "+6281234567890"
	nomorLamaSamr = "+62********890"
)

func barisLama(id idx.ID) domain.AuditLog {
	return domain.AuditLog{
		ID: id, Aksi: "UPDATE_PROFIL", Entitas: "pengguna", EntitasID: "u1", Hasil: domain.HasilBerhasil,
		Sebelum: ptr(`{"nama":"Ani","telepon":null}`),
		Sesudah: ptr(`{"nama":"Ani","telepon":"` + nomorLama + `"}`),
	}
}

func TestList_MenyamarkanTeleponBarisLama(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{
		barisLama(3),
		{ID: 2, Aksi: "UBAH", Entitas: "pedagang", Sesudah: ptr(`{"telepon":"` + nomorLama + `"}`)},
		{ID: 1, Aksi: "LOGIN", Entitas: "pengguna"},
	}}

	page, err := newUseCase(repo).List(context.Background(), Query{})

	require.NoError(t, err)
	require.Len(t, page.Items, 3)
	assert.JSONEq(t, `{"nama":"Ani","telepon":"`+nomorLamaSamr+`"}`, *page.Items[0].Sesudah)
	assert.JSONEq(t, `{"nama":"Ani","telepon":null}`, *page.Items[0].Sebelum)
	assert.Contains(t, *page.Items[1].Sesudah, nomorLama, "entitas lain tidak disentuh")
	assert.Nil(t, page.Items[2].Sebelum)
	assert.Contains(t, *repo.rows[0].Sesudah, nomorLama, "baris di repositori (data mentah) tidak berubah")
}

func TestList_KursorDiHitungDariBarisYangSama_BukanDariBarisTersamar(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{barisLama(3), barisLama(2), barisLama(1)}}

	page, err := newUseCase(repo).List(context.Background(), Query{Limit: 2})

	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.NotEmpty(t, page.NextCursor)
	cursor, err := decodeCursor(page.NextCursor)
	require.NoError(t, err)
	assert.Equal(t, idx.ID(2), cursor.ID)
	for _, item := range page.Items {
		assert.NotContains(t, *item.Sesudah, nomorLama)
	}
}

func TestGet_MenyamarkanBarisDanPerubahan(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{barisLama(7)}}

	detail, err := newUseCase(repo).Get(context.Background(), idx.RujukanID{ID: 7})

	require.NoError(t, err)
	assert.JSONEq(t, `{"nama":"Ani","telepon":"`+nomorLamaSamr+`"}`, *detail.Row.Sesudah)
	require.Len(t, detail.Perubahan, 1)
	assert.Equal(t, "telepon", detail.Perubahan[0].Path)
	assert.Equal(t, domain.PerubahanDiubah, detail.Perubahan[0].Jenis)
	assert.Equal(t, `null`, string(detail.Perubahan[0].Sebelum))
	assert.Equal(t, `"`+nomorLamaSamr+`"`, string(detail.Perubahan[0].Sesudah))
	assert.Contains(t, *repo.rows[0].Sesudah, nomorLama, "data mentah tidak berubah")
}

// Selisih dihitung dari data mentah: baris lama yang hanya mengubah digit tengah tetap tampil sebagai perubahan.
func TestGet_PerubahanDigitTengahBarisLamaTetapTerlihat(t *testing.T) {
	row := barisLama(8)
	row.Sebelum = ptr(`{"telepon":"+6281234567890"}`)
	row.Sesudah = ptr(`{"telepon":"+6281299997890"}`)

	detail, err := newUseCase(&fakeRepo{rows: []domain.AuditLog{row}}).Get(context.Background(), idx.RujukanID{ID: 8})

	require.NoError(t, err)
	require.Len(t, detail.Perubahan, 1)
	assert.Equal(t, domain.PerubahanDiubah, detail.Perubahan[0].Jenis)
	assert.Equal(t, detail.Perubahan[0].Sebelum, detail.Perubahan[0].Sesudah, "kedua sisi tersamar sama")
	assert.Equal(t, `"`+nomorLamaSamr+`"`, string(detail.Perubahan[0].Sesudah))
}

func TestExport_MenyamarkanTeleponBarisLama(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{barisLama(2), barisLama(1)}}

	var disajikan []domain.AuditLog
	n, truncated, err := newUseCase(repo).Export(context.Background(), domain.Filter{}, func(row domain.AuditLog) error {
		disajikan = append(disajikan, row)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.False(t, truncated)
	require.Len(t, disajikan, 2)
	for _, row := range disajikan {
		assert.JSONEq(t, `{"nama":"Ani","telepon":"`+nomorLamaSamr+`"}`, *row.Sesudah)
	}
	assert.Contains(t, *repo.rows[0].Sesudah, nomorLama, "data mentah tidak berubah")
}
