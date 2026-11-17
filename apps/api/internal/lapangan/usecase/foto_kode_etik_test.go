package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// sampleJPEG berisi header JPEG valid minimal agar http.DetectContentType mengenalinya sebagai image/jpeg.
var sampleJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xD9}

// samplePNG berisi signature PNG valid minimal agar http.DetectContentType mengenalinya sebagai image/png.
var samplePNG = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n', 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R'}

func TestUploadFoto(t *testing.T) {
	ctx := context.Background()
	ownerID := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"

	t.Run("berhasil mengunggah JPEG oleh pemilik", func(t *testing.T) {
		repo := &mockRepository{}
		uc, rec := newTestUseCase(repo)
		dir := t.TempDir()
		uc.WithStorageDir(dir)

		repo.On("EntriMilikPetugas", ctx, idx.ID(42)).Return(ownerID, "1171", true, nil)
		repo.On("SimpanLampiranEntri", ctx, mock.AnythingOfType("*domain.LampiranEntri"), "/v1/lapangan/harga/42/foto").
			Run(func(args mock.Arguments) {
				l := args.Get(1).(*domain.LampiranEntri)
				l.ID = 101
			}).
			Return(nil)

		lampiran, fotoURL, err := uc.UploadFoto(ctx, 42, Pemanggil{ID: ownerID}, sampleJPEG)
		require.NoError(t, err)
		assert.Equal(t, idx.ID(101), lampiran.ID)
		assert.Equal(t, "image/jpeg", lampiran.MIME)
		assert.Equal(t, len(sampleJPEG), lampiran.Ukuran)
		assert.Equal(t, "/v1/lapangan/harga/42/foto", fotoURL)

		sum := sha256.Sum256(sampleJPEG)
		assert.Equal(t, hex.EncodeToString(sum[:]), lampiran.Checksum)

		diskPath := filepath.Join(dir, lampiran.LokasiObjek)
		savedBytes, err := os.ReadFile(diskPath)
		require.NoError(t, err)
		assert.Equal(t, sampleJPEG, savedBytes)

		entries := rec.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, "UPLOAD_FOTO_ENTRI", entries[0].Aksi)
		assert.Equal(t, "42", entries[0].EntitasID)
		repo.AssertExpectations(t)
	})

	t.Run("berhasil mengunggah PNG oleh admin untuk entri petugas lain", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		uc.WithStorageDir(t.TempDir())

		repo.On("EntriMilikPetugas", ctx, idx.ID(55)).Return(ownerID, "1171", true, nil)
		repo.On("PenggunaCakupan", ctx, otherID).Return("ADMIN_SISTEM", "", true, nil)
		repo.On("SimpanLampiranEntri", ctx, mock.AnythingOfType("*domain.LampiranEntri"), "/v1/lapangan/harga/55/foto").Return(nil)

		lampiran, _, err := uc.UploadFoto(ctx, 55, Pemanggil{ID: otherID, Semua: true}, samplePNG)
		require.NoError(t, err)
		assert.Equal(t, "image/png", lampiran.MIME)
		repo.AssertExpectations(t)
	})

	t.Run("menolak berkas kosong, melebihi 5MB, dan tipe bukan gambar", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		uc.WithStorageDir(t.TempDir())

		_, _, err := uc.UploadFoto(ctx, 1, Pemanggil{ID: ownerID}, nil)
		assert.ErrorIs(t, err, ErrValidasi)

		oversized := make([]byte, MaxFotoSize+1)
		copy(oversized, sampleJPEG)
		_, _, err = uc.UploadFoto(ctx, 1, Pemanggil{ID: ownerID}, oversized)
		assert.ErrorIs(t, err, ErrValidasi)

		_, _, err = uc.UploadFoto(ctx, 1, Pemanggil{ID: ownerID}, []byte("<html>not an image</html>"))
		assert.ErrorIs(t, err, ErrValidasi)
	})

	t.Run("404 bila entri tidak ada atau bukan milik pemanggil", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		uc.WithStorageDir(t.TempDir())

		repo.On("EntriMilikPetugas", ctx, idx.ID(999)).Return("", "", false, nil)
		_, _, err := uc.UploadFoto(ctx, 999, Pemanggil{ID: ownerID}, sampleJPEG)
		assert.ErrorIs(t, err, ErrNotFound)

		repo.On("EntriMilikPetugas", ctx, idx.ID(42)).Return(ownerID, "1171", true, nil)
		_, _, err = uc.UploadFoto(ctx, 42, Pemanggil{ID: otherID, Semua: false}, sampleJPEG)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("menghapus file dari disk bila simpan DB gagal", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		dir := t.TempDir()
		uc.WithStorageDir(dir)

		repo.On("EntriMilikPetugas", ctx, idx.ID(42)).Return(ownerID, "1171", true, nil)
		repo.On("SimpanLampiranEntri", ctx, mock.AnythingOfType("*domain.LampiranEntri"), "/v1/lapangan/harga/42/foto").
			Return(errors.New("db down"))

		_, _, err := uc.UploadFoto(ctx, 42, Pemanggil{ID: ownerID}, sampleJPEG)
		require.Error(t, err)

		entries, readErr := os.ReadDir(dir)
		require.NoError(t, readErr)
		assert.Empty(t, entries, "file harus dibersihkan bila transaksi DB gagal")
	})
}

func TestKodeEtikDanKinerja(t *testing.T) {
	ctx := context.Background()
	userID := "11111111-1111-1111-1111-111111111111"

	t.Run("kode etik aktif dan persetujuan", func(t *testing.T) {
		repo := &mockRepository{}
		uc, rec := newTestUseCase(repo)

		expected := &domain.KodeEtikStatus{Versi: "v1.0", Judul: "Kode Etik", Isi: "Isi", SudahSetuju: false}
		repo.On("KodeEtikAktif", ctx, userID).Return(expected, nil)

		got, err := uc.GetKodeEtik(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "v1.0", got.Versi)
		assert.False(t, got.SudahSetuju)

		_, err = uc.SetujuiKodeEtik(ctx, userID, "   ")
		assert.ErrorIs(t, err, ErrValidasi)

		persetujuan := &domain.PersetujuanKodeEtik{ID: 7, UserID: userID, Versi: "v1.0", DisetujuiPada: time.Now()}
		repo.On("SetujuiKodeEtik", ctx, userID, "v1.0").Return(persetujuan, nil)

		saved, err := uc.SetujuiKodeEtik(ctx, userID, "v1.0")
		require.NoError(t, err)
		assert.Equal(t, "v1.0", saved.Versi)
		entries := rec.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, "SETUJUI_KODE_ETIK", entries[0].Aksi)
	})

	t.Run("list kinerja meneruskan cakupan", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)

		rows := []domain.KinerjaPetugas{{PetugasID: userID, NamaPetugas: "Budi", TotalEntri: 10}}
		repo.On("ListKinerja", ctx, hargadomain.CakupanMilik(userID)).Return(rows, nil)

		got, err := uc.ListKinerja(ctx, Pemanggil{ID: userID, Semua: false})
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, 10, got[0].TotalEntri)
	})
}

func TestRowScope_Lapangan(t *testing.T) {
	ctx := context.Background()

	t.Run("koordinator dibatasi kabupaten tugasnya dari DB", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		koorID := "koor-1171"
		repo.On("PenggunaCakupan", ctx, koorID).Return(hargadomain.PeranKoordinator, "1171", true, nil)
		repo.On("ListSaya", ctx, hargadomain.Cakupan{KabupatenKode: "1171", PemilikID: koorID}).Return([]domain.TugasView{}, nil)

		_, err := uc.ListSaya(ctx, Pemanggil{ID: koorID, Semua: true})
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("koordinator tanpa kabupaten gagal-tutup hanya miliknya", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		koorID := "koor-kosong"
		repo.On("PenggunaCakupan", ctx, koorID).Return(hargadomain.PeranKoordinator, "", true, nil)

		cakupan, err := uc.Cakupan(ctx, Pemanggil{ID: koorID, Semua: true})
		require.NoError(t, err)
		assert.False(t, cakupan.Semua)
		assert.Equal(t, "", cakupan.KabupatenKode)
		assert.Equal(t, koorID, cakupan.PemilikID)
	})

	t.Run("provinsi membuka seluruh Aceh", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		provID := "prov-1"
		repo.On("PenggunaCakupan", ctx, provID).Return("STRATEGIS_BI", "", true, nil)

		cakupan, err := uc.Cakupan(ctx, Pemanggil{ID: provID, Semua: true})
		require.NoError(t, err)
		assert.True(t, cakupan.Semua)
	})

	t.Run("pengguna tidak ditemukan gagal-tutup total", func(t *testing.T) {
		repo := &mockRepository{}
		uc, _ := newTestUseCase(repo)
		repo.On("PenggunaCakupan", ctx, "ghost").Return("", "", false, nil)

		cakupan, err := uc.Cakupan(ctx, Pemanggil{ID: "ghost", Semua: true})
		require.NoError(t, err)
		assert.Equal(t, hargadomain.Cakupan{}, cakupan)
	})
}
