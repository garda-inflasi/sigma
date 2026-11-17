package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// MaxFotoSize membatasi ukuran maksimal berkas foto bukti pencacahan (5 MB).
const MaxFotoSize = 5 * 1024 * 1024

var allowedFotoMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// UploadFoto memvalidasi tipe dan ukuran foto, memeriksa kepemilikan entri, menyimpan berkas ke disk,
// dan mencatat metadata lampiran ke basis data.
func (uc *UseCase) UploadFoto(ctx context.Context, entriID idx.ID, p Pemanggil, data []byte) (*domain.LampiranEntri, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("%w: berkas foto tidak boleh kosong", ErrValidasi)
	}
	if len(data) > MaxFotoSize {
		return nil, "", fmt.Errorf("%w: ukuran foto melebihi batas maksimal 5 MB", ErrValidasi)
	}

	headLen := len(data)
	if headLen > 512 {
		headLen = 512
	}
	mimeType := http.DetectContentType(data[:headLen])
	ext, ok := allowedFotoMIME[mimeType]
	if !ok {
		return nil, "", fmt.Errorf("%w: tipe berkas %s tidak didukung, gunakan JPEG, PNG, atau WebP", ErrValidasi, mimeType)
	}

	petugasID, kabupatenKode, ditemukan, err := uc.repo.EntriMilikPetugas(ctx, entriID)
	if err != nil {
		uc.log.Error("lapangan: cek pemilik entri gagal", map[string]interface{}{"entri_id": entriID.String(), "error": err.Error()})
		return nil, "", fmt.Errorf("cek pemilik entri: %w", err)
	}
	if !ditemukan {
		return nil, "", ErrNotFound
	}
	cakupan, err := uc.Cakupan(ctx, p)
	if err != nil {
		return nil, "", fmt.Errorf("cek cakupan: %w", err)
	}
	if !cakupan.Mengizinkan(kabupatenKode, &petugasID) {
		return nil, "", ErrNotFound
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	fileName := fmt.Sprintf("entri_%s_%s%s", entriID.String(), checksum[:16], ext)

	if err := os.MkdirAll(uc.storageDir, 0o750); err != nil {
		uc.log.Error("lapangan: gagal membuat direktori penyimpanan foto", map[string]interface{}{"dir": uc.storageDir, "error": err.Error()})
		return nil, "", fmt.Errorf("buat direktori foto: %w", err)
	}

	fullPath := filepath.Join(uc.storageDir, filepath.Base(fileName))
	if err := os.WriteFile(fullPath, data, 0o640); err != nil {
		uc.log.Error("lapangan: gagal menulis berkas foto", map[string]interface{}{"path": fullPath, "error": err.Error()})
		return nil, "", fmt.Errorf("simpan berkas foto: %w", err)
	}

	fotoURL := fmt.Sprintf("/v1/lapangan/harga/%s/foto", entriID.String())
	lampiran := &domain.LampiranEntri{
		EntriID:     entriID,
		LokasiObjek: fileName,
		MIME:        mimeType,
		Ukuran:      len(data),
		Checksum:    checksum,
		DibuatPada:  uc.now(),
	}
	if err := uc.repo.SimpanLampiranEntri(ctx, lampiran, fotoURL); err != nil {
		_ = os.Remove(fullPath)
		uc.log.Error("lapangan: gagal mencatat lampiran entri", map[string]interface{}{"entri_id": entriID.String(), "error": err.Error()})
		return nil, "", fmt.Errorf("simpan lampiran: %w", err)
	}

	if err := uc.audit.Write(ctx, audit.Entry{
		AktorID:   p.ID,
		Aksi:      "UPLOAD_FOTO_ENTRI",
		Entitas:   "entri_harga",
		EntitasID: entriID.String(),
		Hasil:     audit.HasilBerhasil,
		Metadata: map[string]any{
			"lampiran_id": lampiran.ID.String(),
			"mime":        mimeType,
			"ukuran":      len(data),
			"checksum":    checksum,
		},
	}); err != nil {
		uc.log.Warn("lapangan: gagal mencatat audit upload foto", map[string]interface{}{"entri_id": entriID.String(), "error": err.Error()})
	}

	return lampiran, fotoURL, nil
}

// GetFoto mengembalikan path disk dan tipe MIME dari foto lampiran terbaru suatu entri.
func (uc *UseCase) GetFoto(ctx context.Context, entriID idx.ID, p Pemanggil) (string, string, error) {
	petugasID, kabupatenKode, ditemukan, err := uc.repo.EntriMilikPetugas(ctx, entriID)
	if err != nil {
		return "", "", fmt.Errorf("cek pemilik entri: %w", err)
	}
	if !ditemukan {
		return "", "", ErrNotFound
	}
	cakupan, err := uc.Cakupan(ctx, p)
	if err != nil {
		return "", "", fmt.Errorf("cek cakupan: %w", err)
	}
	if !cakupan.Mengizinkan(kabupatenKode, &petugasID) {
		return "", "", ErrNotFound
	}

	lampiran, err := uc.repo.LampiranTerbaru(ctx, entriID)
	if err != nil {
		return "", "", err
	}
	fullPath := filepath.Join(uc.storageDir, filepath.Base(lampiran.LokasiObjek))
	return fullPath, lampiran.MIME, nil
}

// GetKodeEtik mengembalikan kode etik aktif beserta status persetujuan pengguna.
func (uc *UseCase) GetKodeEtik(ctx context.Context, userID string) (*domain.KodeEtikStatus, error) {
	status, err := uc.repo.KodeEtikAktif(ctx, userID)
	if err != nil {
		return nil, err
	}
	return status, nil
}

// SetujuiKodeEtik mencatat persetujuan kode etik oleh petugas dan menulis audit trail.
func (uc *UseCase) SetujuiKodeEtik(ctx context.Context, userID, versi string) (*domain.PersetujuanKodeEtik, error) {
	versi = strings.TrimSpace(versi)
	if versi == "" || len(versi) > 32 {
		return nil, fmt.Errorf("%w: versi kode etik wajib diisi (maks 32 karakter)", ErrValidasi)
	}
	persetujuan, err := uc.repo.SetujuiKodeEtik(ctx, userID, versi)
	if err != nil {
		return nil, err
	}

	if err := uc.audit.Write(ctx, audit.Entry{
		AktorID:   userID,
		Aksi:      "SETUJUI_KODE_ETIK",
		Entitas:   "persetujuan_kode_etik",
		EntitasID: persetujuan.ID.String(),
		Hasil:     audit.HasilBerhasil,
		Metadata: map[string]any{
			"versi": versi,
		},
	}); err != nil {
		uc.log.Warn("lapangan: gagal mencatat audit kode etik", map[string]interface{}{"versi": versi, "error": err.Error()})
	}

	return persetujuan, nil
}

// ListKinerja mengembalikan ringkasan kinerja pencacahan petugas sesuai cakupan baris pemanggil.
func (uc *UseCase) ListKinerja(ctx context.Context, p Pemanggil) ([]domain.KinerjaPetugas, error) {
	cakupan, err := uc.Cakupan(ctx, p)
	if err != nil {
		uc.log.Error("lapangan: baca cakupan kinerja gagal", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list kinerja: %w", err)
	}
	items, err := uc.repo.ListKinerja(ctx, cakupan)
	if err != nil {
		uc.log.Error("lapangan: gagal membaca kinerja petugas", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list kinerja: %w", err)
	}
	return items, nil
}
