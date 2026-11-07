package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
)

// Batas halaman daftar usulan.
const (
	DefaultLimitUsulan = 50
	MaxLimitUsulan     = 200
)

// UseCasePerubahan menjalankan alur usulan perubahan master data (04-B) dan mencatat setiap langkahnya di audit
// trail. Izin per langkah (master.ajukan / master.setujui) diperiksa di rute.
type UseCasePerubahan struct {
	repo  domain.RepositoryPerubahan
	audit audit.Writer
	log   *logger.Logger
}

func NewPerubahan(repo domain.RepositoryPerubahan, auditWriter audit.Writer, log *logger.Logger) *UseCasePerubahan {
	return &UseCasePerubahan{repo: repo, audit: auditWriter, log: log}
}

// AjukanInput adalah usulan dari pengguna; Perubahan berisi nilai JSON mentah per bidang.
type AjukanInput struct {
	Entitas   string
	Jenis     string
	EntitasID string
	Perubahan map[string]json.RawMessage
	Alasan    string
}

// Ajukan memvalidasi dan menyimpan usulan TAMBAH atau UBAH.
func (uc *UseCasePerubahan) Ajukan(ctx context.Context, in AjukanInput, aktorID string) (domain.PerubahanView, error) {
	isi, err := domain.ValidasiUsulan(in.Entitas, in.Jenis, in.EntitasID, in.Perubahan)
	if err != nil {
		return domain.PerubahanView{}, err
	}
	alasan, err := domain.ValidasiAlasan(in.Alasan, true)
	if err != nil {
		return domain.PerubahanView{}, err
	}
	p, err := uc.repo.Ajukan(ctx, domain.UsulanBaru{
		Entitas: in.Entitas, Jenis: in.Jenis, EntitasID: in.EntitasID, Perubahan: isi, Alasan: alasan, AktorID: aktorID,
	})
	if err != nil {
		return domain.PerubahanView{}, err
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "AJUKAN_PERUBAHAN_MASTER", Entitas: p.Entitas, EntitasID: p.EntitasID,
		Hasil: audit.HasilBerhasil, Sebelum: json.RawMessage(p.Sebelum), Sesudah: json.RawMessage(p.Perubahan),
		Metadata: map[string]any{"perubahan_id": p.ID.String(), "jenis": p.Jenis, "versi": p.Versi, "alasan": alasan},
	})
	return uc.repo.GetPerubahan(ctx, p.ID)
}

// Putuskan menyetujui, menolak, atau membatalkan satu usulan menunggu. Penolakan wajib beralasan; pembatalan
// (oleh pengaju) boleh tanpa alasan.
func (uc *UseCasePerubahan) Putuskan(ctx context.Context, id idx.ID, aksi, aktorID, alasan string) (domain.PerubahanView, error) {
	alasan, err := domain.ValidasiAlasan(alasan, aksi == domain.KeputusanTolak)
	if err != nil {
		return domain.PerubahanView{}, err
	}
	putusan, err := uc.repo.Putuskan(ctx, id, domain.Keputusan{Aksi: aksi, AktorID: aktorID, Alasan: alasan})
	if err != nil {
		return domain.PerubahanView{}, err
	}
	uc.catatPutusan(ctx, putusan, aksi, aktorID, alasan)
	return uc.repo.GetPerubahan(ctx, id)
}

// Get mengembalikan satu usulan; domain.ErrUsulanTidakAda bila tidak ada.
func (uc *UseCasePerubahan) Get(ctx context.Context, id idx.ID) (domain.PerubahanView, error) {
	return uc.repo.GetPerubahan(ctx, id)
}

// List mengembalikan satu halaman usulan beserta jumlah seluruh hasil filter.
func (uc *UseCasePerubahan) List(ctx context.Context, f domain.FilterPerubahan) ([]domain.PerubahanView, int64, error) {
	switch f.Status {
	case "", domain.UsulanMenunggu, domain.UsulanDisetujui, domain.UsulanDitolak:
	default:
		return nil, 0, fmt.Errorf("%w: status %q tidak dikenal", domain.ErrValidasi, f.Status)
	}
	switch f.Entitas {
	case "", domain.EntitasKomoditas, domain.EntitasPasar:
	default:
		return nil, 0, fmt.Errorf("%w: entitas %q tidak dikenal", domain.ErrValidasi, f.Entitas)
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimitUsulan
	}
	f.Limit = min(f.Limit, MaxLimitUsulan)
	f.Page = max(f.Page, 1)
	return uc.repo.ListPerubahan(ctx, f)
}

func (uc *UseCasePerubahan) catatPutusan(ctx context.Context, putusan domain.Putusan, aksi, aktorID, alasan string) {
	p := putusan.Usulan
	entry := audit.Entry{
		AktorID: aktorID, Aksi: aksi + "_PERUBAHAN_MASTER", Entitas: p.Entitas, EntitasID: p.EntitasID,
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"perubahan_id": p.ID.String(), "jenis": p.Jenis, "versi": p.Versi},
	}
	if aksi == domain.KeputusanSetujui {
		entry.Sebelum, entry.Sesudah = nilaiAtauNil(putusan.Sebelum), nilaiAtauNil(putusan.Sesudah)
	} else if alasan != "" {
		entry.Metadata["alasan"] = alasan
	}
	uc.catat(ctx, entry)
}

// nilaiAtauNil mencegah peta nil tercatat sebagai "null" yang tak bermakna.
func nilaiAtauNil(m map[string]any) any {
	if m == nil {
		return nil
	}
	return m
}

// catat menulis audit; kegagalan mencatat tidak membatalkan keputusan yang sudah tersimpan (pola modul master).
func (uc *UseCasePerubahan) catat(ctx context.Context, entry audit.Entry) {
	if err := uc.audit.Write(ctx, entry); err != nil {
		uc.log.Warn("master: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

// PesanItemGagal menerjemahkan galat domain alur perubahan menjadi pesan item aksi massal. ok false berarti
// galat itu bukan penolakan bisnis (galat sistem) dan harus dikembalikan.
func PesanItemGagal(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrPemisahanTugas):
		return "Anda yang mengajukan usulan ini; perlu penyetuju lain", true
	case errors.Is(err, domain.ErrKedaluwarsa):
		return "Data berubah sejak usulan diajukan; tolak usulan ini lalu ajukan ulang", true
	case errors.Is(err, domain.ErrUsulanMenunggu):
		return "Sudah ada usulan yang menunggu persetujuan", true
	case errors.Is(err, domain.ErrStatusTidakSesuai):
		return "Status tidak sesuai untuk aksi ini", true
	case errors.Is(err, domain.ErrEntitasTidakAda):
		return "Komoditas tidak ditemukan", true
	case errors.Is(err, domain.ErrBukanMenunggu):
		return "Usulan sudah diputuskan; muat ulang lalu coba lagi", true
	case errors.Is(err, domain.ErrValidasi), errors.Is(err, domain.ErrKodeSudahAda):
		return err.Error(), true
	}
	return "", false
}
