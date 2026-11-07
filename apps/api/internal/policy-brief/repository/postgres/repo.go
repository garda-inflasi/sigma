package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// view memilih kolom BriefView dari policy_brief digabung nama penyusun. ID uuid dibaca sebagai teks kanonik.
func (r *Repository) view(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("policy_brief AS pb").
		Select(`pb.id::text AS id, pb.kode, pb.periode, pb.judul, pb.status, pb.ringkasan, pb.halaman,
			u.nama AS disusun_oleh, jsonb_array_length(pb.rekomendasi) AS jumlah_rekomendasi,
			pb.sumber_narasi, pb.dibuat_pada, pb.diperbarui`).
		Joins("JOIN pengguna u ON u.id = pb.disusun_oleh_id")
}

// List mengurutkan menurut kode: PB-YYYY-MM terurut leksikografis sama dengan kronologis, sedangkan UUID tidak
// punya urutan yang bermakna. Indeks uq_policy_brief_kode melayani urutan ini.
func (r *Repository) List(ctx context.Context) ([]domain.BriefView, error) {
	var items []domain.BriefView
	if err := r.view(ctx).Order("pb.kode DESC").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list policy brief: %w", err)
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, id string) (*domain.BriefView, error) {
	var item domain.BriefView
	res := r.view(ctx).Where("pb.id = ?", id).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("get policy brief: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) GetDetail(ctx context.Context, ref idx.RujukanUUID) (*domain.DetailView, error) {
	var item domain.DetailView
	res := r.view(ctx).Select(`pb.id::text AS id, pb.kode, pb.periode, pb.judul, pb.status, pb.ringkasan, pb.halaman,
			u.nama AS disusun_oleh, jsonb_array_length(pb.rekomendasi) AS jumlah_rekomendasi,
			pb.sumber_narasi, pb.dibuat_pada, pb.diperbarui,
			pb.konten::text AS konten, pb.model_llm, pb.prompt_versi, pb.prompt_hash, pb.galat`).
		Where(ref.Kondisi("pb.id", "pb.kode")).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("get detail policy brief: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) ListVersi(ctx context.Context, briefID string) ([]domain.VersiView, error) {
	var items []domain.VersiView
	err := r.db.WithContext(ctx).Table("policy_brief_versi AS v").
		Select("v.id, v.versi, u.nama AS oleh, v.waktu").
		Joins("JOIN pengguna u ON u.id = v.oleh").
		Where("v.brief_id = ?", briefID).Order("v.versi DESC").Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list versi policy brief: %w", err)
	}
	return items, nil
}

// Create menyimpan brief dengan kode sebagai kunci idempotensi: ON CONFLICT (kode) DO NOTHING, jadi satu periode
// tidak pernah punya dua brief walau dua permintaan datang bersamaan. ID UUID diberikan basis data (RETURNING).
func (r *Repository) Create(ctx context.Context, pb *domain.PolicyBrief) (bool, error) {
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "kode"}}, DoNothing: true}).Create(pb)
	if res.Error != nil {
		return false, fmt.Errorf("create policy brief: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		return true, nil
	}
	// Kodenya sudah dipakai: ambil UUID brief yang ada supaya pemanggil dapat memutuskan (mulai ulang atau 409).
	var id string
	err := r.db.WithContext(ctx).Table("policy_brief").Select("id::text").Where("kode = ?", pb.Kode).Limit(1).Scan(&id).Error
	if err != nil {
		return false, fmt.Errorf("cari policy brief berkode %s: %w", pb.Kode, err)
	}
	if id == "" {
		return false, fmt.Errorf("create policy brief: kode %s bentrok tetapi barisnya tidak ditemukan", pb.Kode)
	}
	pb.ID = id
	return false, nil
}

func (r *Repository) MulaiUlang(ctx context.Context, id string, basiSebelum time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&domain.PolicyBrief{}).
		Where("id = ? AND (status = ? OR (status = ? AND diperbarui < ?))", id, domain.StatusGagal, domain.StatusDiproses, basiSebelum).
		Updates(map[string]any{"status": domain.StatusDiproses, "galat": nil, "diperbarui": gorm.Expr("now()")})
	if res.Error != nil {
		return false, fmt.Errorf("mulai ulang policy brief: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *Repository) Transisi(ctx context.Context, ref idx.RujukanUUID, dari, ke string) (string, bool, error) {
	kondisi, arg := ref.Kondisi("id", "kode")
	var ids []string
	err := r.db.WithContext(ctx).
		Raw(`UPDATE policy_brief SET status = ?, diperbarui = now() WHERE `+kondisi+` AND status = ? RETURNING id::text`, ke, arg, dari).
		Scan(&ids).Error
	if err != nil {
		return "", false, fmt.Errorf("ubah status policy brief: %w", err)
	}
	if len(ids) == 0 {
		return "", false, nil
	}
	return ids[0], true, nil
}

func (r *Repository) MulaiJob(ctx context.Context, job *domain.BriefJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("mulai job brief: %w", err)
	}
	return nil
}

func (r *Repository) SelesaikanDraf(ctx context.Context, d domain.HasilDraf) (bool, error) {
	tersimpan := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.PolicyBrief{}).Where("id = ? AND status = ?", d.BriefID, domain.StatusDiproses).Updates(map[string]any{
			"status": domain.StatusDraf, "judul": d.Judul, "ringkasan": d.Ringkasan, "rekomendasi": d.Rekomendasi,
			"halaman": d.Halaman, "konten": d.Konten, "snapshot_data": d.Snapshot, "sumber_narasi": d.Penyusun,
			"model_llm": d.ModelLLM, "prompt_versi": d.PromptVersi, "prompt_hash": d.PromptHash, "galat": d.Catatan,
			"diperbarui": gorm.Expr("now()"),
		})
		if res.Error != nil {
			return fmt.Errorf("simpan draf: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}

		var versi int
		err := tx.Model(&domain.BriefVersi{}).Where("brief_id = ?", d.BriefID).Select("COALESCE(MAX(versi), 0) + 1").Scan(&versi).Error
		if err != nil {
			return fmt.Errorf("hitung versi brief: %w", err)
		}
		// ID BIGINT versi (versi_id) diberikan basis data; (brief_id, versi) tetap unik.
		if err := tx.Create(&domain.BriefVersi{BriefID: d.BriefID, Versi: versi, Konten: d.Konten, Oleh: d.OlehID, Waktu: d.Waktu}).Error; err != nil {
			return fmt.Errorf("simpan versi brief: %w", err)
		}
		err = tx.Model(&domain.BriefJob{}).Where("id = ?", d.JobID).
			Updates(map[string]any{"status": domain.JobSelesai, "selesai": d.Waktu, "galat": d.Catatan}).Error
		if err != nil {
			return fmt.Errorf("tutup job brief: %w", err)
		}
		tersimpan = true
		return nil
	})
	return tersimpan, err
}

func (r *Repository) GagalkanDraf(ctx context.Context, briefID string, jobID idx.ID, galat string, waktu time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&domain.PolicyBrief{}).Where("id = ? AND status = ?", briefID, domain.StatusDiproses).
			Updates(map[string]any{"status": domain.StatusGagal, "galat": galat, "diperbarui": gorm.Expr("now()")}).Error
		if err != nil {
			return fmt.Errorf("tandai brief gagal: %w", err)
		}
		err = tx.Model(&domain.BriefJob{}).Where("id = ?", jobID).
			Updates(map[string]any{"status": domain.JobGagal, "selesai": waktu, "galat": galat}).Error
		if err != nil {
			return fmt.Errorf("tandai job gagal: %w", err)
		}
		return nil
	})
}
