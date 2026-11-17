package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/token"
)

func ctxImpersonasi() context.Context {
	return token.ContextWithImpersonation(context.Background(), token.ImpersonationContext{
		Impersonation:   token.Impersonation{ID: uid("imp-1"), By: token.ImpersonatedBy{UserID: uid("admin"), Role: "ADMIN_SISTEM"}},
		EffectiveUserID: uid("sasaran"),
		EffectiveRole:   "OPS_MAHASISWA",
	})
}

func repoImpersonasi() *fakeRepo {
	return &fakeRepo{actors: map[string]*domain.Actor{
		uid("admin"):   {ID: uid("admin"), Nama: "Super Admin", Email: "admin@gardainflasi.id", Peran: "ADMIN_SISTEM"},
		uid("sasaran"): {ID: uid("sasaran"), Nama: "Agen Lapangan", Email: "agen@gardainflasi.id", Peran: "OPS_MAHASISWA"},
	}}
}

func TestWrite_SelamaImpersonasiAktorAdalahPelakuAsliDanMetadataMenautkanSesi(t *testing.T) {
	repo := repoImpersonasi()

	err := newUseCase(repo).Write(ctxImpersonasi(), domain.Entry{
		AktorID: uid("sasaran"), Aksi: "CREATE", Entitas: "harga", EntitasID: "1", Hasil: domain.HasilBerhasil,
		Metadata: map[string]any{"catatan": "x"},
	})

	require.NoError(t, err)
	row := repo.created[0]
	require.NotNil(t, row.AktorID)
	assert.Equal(t, uid("admin"), *row.AktorID, "aktor_id = akun ASLI, bukan sasaran")
	assert.Equal(t, "Super Admin", *row.AktorNama)
	assert.Equal(t, "ADMIN_SISTEM", *row.AktorPeran, "snapshot peran milik pelaku asli")

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(*row.Metadata), &meta))
	assert.Equal(t, "x", meta["catatan"], "metadata bawaan entri dipertahankan")
	assert.Equal(t, map[string]any{
		"sesi_id": uid("imp-1"), "sebagai_pengguna_id": uid("sasaran"), "peran_efektif": "OPS_MAHASISWA",
	}, meta["impersonasi"])
}

func TestWrite_SelamaImpersonasiEntriTanpaMetadataTetapMemuatPenautan(t *testing.T) {
	repo := repoImpersonasi()

	require.NoError(t, newUseCase(repo).Write(ctxImpersonasi(), domain.Entry{AktorID: uid("sasaran"), Aksi: "UPDATE", Entitas: "x", EntitasID: "1", Hasil: domain.HasilBerhasil}))

	require.NotNil(t, repo.created[0].Metadata)
	assert.Contains(t, *repo.created[0].Metadata, `"peran_efektif":"OPS_MAHASISWA"`)
}

func TestWrite_SelamaImpersonasiAktorLainTidakDiganti(t *testing.T) {
	repo := repoImpersonasi()

	// Entri yang aktornya bukan akun sasaran (mis. entri otomatis oleh pelaku sendiri) tidak diubah atribusinya.
	require.NoError(t, newUseCase(repo).Write(ctxImpersonasi(), domain.Entry{AktorID: uid("admin"), Aksi: "IMPERSONASI_BERHENTI", Entitas: "pengguna", EntitasID: "1", Hasil: domain.HasilBerhasil}))
	assert.Equal(t, uid("admin"), *repo.created[0].AktorID)

	// Tanpa aktor tetap tanpa aktor.
	require.NoError(t, newUseCase(repo).Write(ctxImpersonasi(), domain.Entry{Aksi: "LOGIN", Entitas: "pengguna", EntitasID: "e", Hasil: domain.HasilGagal}))
	assert.Nil(t, repo.created[1].AktorID)
}

func TestWrite_MetadataImpersonasiBawaanPemanggilTidakDitimpa(t *testing.T) {
	repo := repoImpersonasi()

	require.NoError(t, newUseCase(repo).Write(ctxImpersonasi(), domain.Entry{
		AktorID: uid("sasaran"), Aksi: "X", Entitas: "x", EntitasID: "1", Hasil: domain.HasilBerhasil,
		Metadata: map[string]any{"impersonasi": "dari-pemanggil"},
	}))

	assert.Contains(t, *repo.created[0].Metadata, `"impersonasi":"dari-pemanggil"`)
}

func TestWrite_TanpaImpersonasiTidakBerubah(t *testing.T) {
	repo := repoImpersonasi()

	require.NoError(t, newUseCase(repo).Write(context.Background(), domain.Entry{AktorID: uid("sasaran"), Aksi: "CREATE", Entitas: "harga", EntitasID: "1", Hasil: domain.HasilBerhasil}))

	assert.Equal(t, uid("sasaran"), *repo.created[0].AktorID)
	assert.Nil(t, repo.created[0].Metadata, "tanpa impersonasi tidak ada metadata yang ditambahkan")
}

func TestWrite_MetadataCallerTidakDimutasi(t *testing.T) {
	repo := repoImpersonasi()
	meta := map[string]any{"catatan": "x"}

	require.NoError(t, newUseCase(repo).Write(ctxImpersonasi(), domain.Entry{AktorID: uid("sasaran"), Aksi: "X", Entitas: "x", EntitasID: "1", Hasil: domain.HasilBerhasil, Metadata: meta}))

	assert.Len(t, meta, 1, "peta milik pemanggil tidak diubah")
}
