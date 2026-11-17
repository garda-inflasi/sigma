package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
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

const viewColumns = `t.id, t.kode, t.judul, t.pasar_id, p.nama AS pasar_nama, t.petugas_id, t.komoditas,
	t.tipe, t.tenggat, t.status, t.target, t.jumlah_selesai`

func (r *Repository) tugasView(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("tugas_lapangan AS t").
		Joins("JOIN pasar p ON p.kode = t.pasar_id").
		Select(viewColumns)
}

func kondisiCakupan(c hargadomain.Cakupan, kabExpr, pemilikExpr string) (string, []any) {
	if c.Semua {
		return "TRUE", nil
	}
	var clauses []string
	var args []any
	if c.KabupatenKode != "" && kabExpr != "" {
		clauses = append(clauses, kabExpr+" = ?")
		args = append(args, c.KabupatenKode)
	}
	if c.PemilikID != "" && pemilikExpr != "" {
		clauses = append(clauses, pemilikExpr+" = ?")
		args = append(args, c.PemilikID)
	}
	if len(clauses) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// PenggunaCakupan membaca peran dan kabupaten tugas pengguna langsung dari DB per request (02-B).
func (r *Repository) PenggunaCakupan(ctx context.Context, userID string) (string, string, bool, error) {
	var rows []struct {
		Peran         string `gorm:"column:peran"`
		KabupatenKode string `gorm:"column:kabupaten_kode"`
	}
	err := r.db.WithContext(ctx).Raw(
		`SELECT peran::text AS peran, COALESCE(kabupaten_kode, '') AS kabupaten_kode FROM pengguna WHERE id = ?`,
		userID,
	).Scan(&rows).Error
	if err != nil {
		return "", "", false, fmt.Errorf("baca peran pengguna: %w", err)
	}
	if len(rows) == 0 {
		return "", "", false, nil
	}
	return rows[0].Peran, rows[0].KabupatenKode, true, nil
}

// ListSaya mengembalikan tugas sesuai cakupan baris pemanggil; yang sudah SELESAI di urutan akhir.
func (r *Repository) ListSaya(ctx context.Context, cakupan hargadomain.Cakupan) ([]domain.TugasView, error) {
	kondisi, args := kondisiCakupan(cakupan, "p.kabupaten_kode", "t.petugas_id")
	var items []domain.TugasView
	err := r.tugasView(ctx).
		Where(kondisi, args...).
		Order("(t.status = 'SELESAI'), t.tenggat, t.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list tugas lapangan: %w", err)
	}
	return items, nil
}

// GetByID mengembalikan satu tugas menurut ID BIGINT atau kode lama (TGS-...) sesuai cakupan baris.
func (r *Repository) GetByID(ctx context.Context, ref idx.RujukanID, cakupan hargadomain.Cakupan) (*domain.TugasView, error) {
	kondisi, nilai := ref.Kondisi("t.id", "t.kode")
	cakKondisi, cakArgs := kondisiCakupan(cakupan, "p.kabupaten_kode", "t.petugas_id")
	q := r.tugasView(ctx).Where(kondisi, nilai).Where(cakKondisi, cakArgs...)
	var items []domain.TugasView
	if err := q.Limit(1).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get tugas lapangan: %w", err)
	}
	if len(items) == 0 {
		return nil, domain.ErrNotFound
	}
	return &items[0], nil
}

// Rincian membaca nama komoditas dan apakah sudah ada entri harga yang tertaut ke tugas untuk tiap kode.
func (r *Repository) Rincian(ctx context.Context, tugasID idx.ID, kode []string) ([]domain.RincianKomoditas, error) {
	if len(kode) == 0 {
		return nil, nil
	}
	var items []domain.RincianKomoditas
	err := r.db.WithContext(ctx).Raw(`
		SELECT k.kode, k.nama,
		       EXISTS (SELECT 1 FROM tugas_entri te JOIN entri_harga e ON e.id = te.entri_id
		                WHERE te.tugas_id = @tugas AND e.komoditas_kode = k.kode) AS terisi
		  FROM komoditas k
		 WHERE k.kode IN @kode`,
		map[string]any{"tugas": tugasID, "kode": kode},
	).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("rincian komoditas tugas: %w", err)
	}
	return items, nil
}

// pisahRujukan memisahkan rujukan menjadi ID BIGINT dan kode lama untuk klausa WHERE id IN … OR kode IN ….
func pisahRujukan(refs []idx.RujukanID) (ids []idx.ID, kode []string) {
	for _, ref := range refs {
		if ref.ID != 0 {
			ids = append(ids, ref.ID)
		} else if ref.Kode != "" {
			kode = append(kode, ref.Kode)
		}
	}
	return ids, kode
}

// Aksesibel menyaring refs ke baris tugas yang ada dan boleh disentuh pemanggil.
func (r *Repository) Aksesibel(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan) ([]domain.TugasRef, error) {
	ids, kode := pisahRujukan(refs)
	cakKondisi, cakArgs := kondisiCakupan(cakupan, "p.kabupaten_kode", "t.petugas_id")
	q := r.db.WithContext(ctx).Table("tugas_lapangan AS t").
		Joins("JOIN pasar p ON p.kode = t.pasar_id").
		Select("t.id, t.kode").
		Where("(t.id IN ? OR t.kode IN ?)", ids, kode).
		Where(cakKondisi, cakArgs...)
	var found []domain.TugasRef
	if err := q.Scan(&found).Error; err != nil {
		return nil, fmt.Errorf("cek akses tugas lapangan: %w", err)
	}
	return found, nil
}

// TandaiSelesai menutup tugas yang belum selesai dan targetnya sudah terpenuhi.
func (r *Repository) TandaiSelesai(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan, waktu time.Time) ([]domain.TugasRef, error) {
	ids, kode := pisahRujukan(refs)
	var changed []domain.TugasRef
	err := r.db.WithContext(ctx).Raw(`
		UPDATE tugas_lapangan AS t
		   SET status = 'SELESAI', selesai_pada = @waktu
		  FROM pasar p
		 WHERE p.kode = t.pasar_id
		   AND (t.id IN @ids OR t.kode IN @kode)
		   AND t.status <> 'SELESAI'
		   AND t.jumlah_selesai >= t.target
		   AND (@semua OR (@kab <> '' AND p.kabupaten_kode = @kab) OR (@petugas <> '' AND t.petugas_id::text = @petugas))
		RETURNING t.id, t.kode`,
		map[string]any{
			"ids": ids, "kode": kode, "waktu": waktu,
			"semua": cakupan.Semua, "kab": cakupan.KabupatenKode, "petugas": cakupan.PemilikID,
		},
	).Scan(&changed).Error
	if err != nil {
		return nil, fmt.Errorf("tandai tugas selesai: %w", err)
	}
	return changed, nil
}

// Acuan membaca komoditas dan pasar aktif serta median harga resmi (DIVALIDASI atau
// DIPUBLIKASI) per komoditas, kabupaten, dan tipe sejak waktu itu, hanya bila
// sampelnya minimal minSampel.
func (r *Repository) Acuan(ctx context.Context, sejak time.Time, minSampel int) (domain.DataAcuan, error) {
	var data domain.DataAcuan
	tx := r.db.WithContext(ctx)

	if err := tx.Raw(`SELECT kode, nama, satuan, harga_acuan FROM komoditas WHERE status = 'AKTIF' ORDER BY kode`).Scan(&data.Komoditas).Error; err != nil {
		return data, fmt.Errorf("baca komoditas acuan: %w", err)
	}
	if err := tx.Raw(`SELECT kode AS id, nama, kabupaten_kode, lat, lng, radius_meter FROM pasar WHERE aktif ORDER BY kode`).Scan(&data.Pasar).Error; err != nil {
		return data, fmt.Errorf("baca pasar acuan: %w", err)
	}
	err := tx.Raw(`
		SELECT e.komoditas_kode, p.kabupaten_kode, e.tipe::text AS tipe,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY e.harga) AS median, COUNT(*) AS sampel
		  FROM entri_harga e
		  JOIN pasar p ON p.kode = e.pasar_id
		 WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI') AND e.dicatat_pada >= ?
		 GROUP BY e.komoditas_kode, p.kabupaten_kode, e.tipe
		HAVING COUNT(*) >= ?
		 ORDER BY e.komoditas_kode, p.kabupaten_kode, e.tipe`, sejak, minSampel).Scan(&data.Median).Error
	if err != nil {
		return data, fmt.Errorf("baca median acuan: %w", err)
	}
	return data, nil
}

// EntriMilikPetugas memeriksa keberadaan entri_harga dan mengembalikan petugas_id serta kabupaten_kode pasarnya.
func (r *Repository) EntriMilikPetugas(ctx context.Context, entriID idx.ID) (string, string, bool, error) {
	var row struct {
		PetugasID     string `gorm:"column:petugas_id"`
		KabupatenKode string `gorm:"column:kabupaten_kode"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(e.petugas_id::text, '') AS petugas_id,
		       COALESCE(p.kabupaten_kode, '') AS kabupaten_kode
		  FROM entri_harga e
		  LEFT JOIN pasar p ON p.kode = e.pasar_id
		 WHERE e.id = ?`,
		entriID,
	).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("cek pemilik entri: %w", err)
	}
	return row.PetugasID, row.KabupatenKode, true, nil
}

// SimpanLampiranEntri menyimpan baris lampiran_entri dan memperbarui foto_url di entri_harga dalam satu transaksi.
func (r *Repository) SimpanLampiranEntri(ctx context.Context, lampiran *domain.LampiranEntri, fotoURL string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(lampiran).Error; err != nil {
			return fmt.Errorf("simpan lampiran entri: %w", err)
		}
		if err := tx.Exec(`UPDATE entri_harga SET foto_url = ? WHERE id = ?`, fotoURL, lampiran.EntriID).Error; err != nil {
			return fmt.Errorf("perbarui foto_url entri: %w", err)
		}
		return nil
	})
}

// LampiranTerbaru mengambil lampiran terbaru milik entri tertentu.
func (r *Repository) LampiranTerbaru(ctx context.Context, entriID idx.ID) (*domain.LampiranEntri, error) {
	var out domain.LampiranEntri
	err := r.db.WithContext(ctx).
		Where("entri_id = ?", entriID).
		Order("dibuat_pada DESC, id DESC").
		First(&out).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("baca lampiran entri: %w", err)
	}
	return &out, nil
}

// KodeEtikAktif mengembalikan kode etik aktif beserta status persetujuan untuk userID.
func (r *Repository) KodeEtikAktif(ctx context.Context, userID string) (*domain.KodeEtikStatus, error) {
	var ke domain.KodeEtik
	err := r.db.WithContext(ctx).
		Where("aktif = true").
		Order("dibuat_pada DESC").
		First(&ke).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("baca kode etik aktif: %w", err)
	}

	out := &domain.KodeEtikStatus{
		Versi: ke.Versi,
		Judul: ke.Judul,
		Isi:   ke.Isi,
	}

	var p domain.PersetujuanKodeEtik
	err = r.db.WithContext(ctx).
		Where("user_id = ? AND versi = ?", userID, ke.Versi).
		First(&p).Error
	if err == nil {
		out.SudahSetuju = true
		t := p.DisetujuiPada
		out.DisetujuiPada = &t
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("baca persetujuan kode etik: %w", err)
	}
	return out, nil
}

// SetujuiKodeEtik mencatat persetujuan pengguna terhadap versi kode etik tertentu secara idempoten.
func (r *Repository) SetujuiKodeEtik(ctx context.Context, userID, versi string) (*domain.PersetujuanKodeEtik, error) {
	var ke domain.KodeEtik
	if err := r.db.WithContext(ctx).Where("versi = ?", versi).First(&ke).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("cek versi kode etik: %w", err)
	}

	rec := domain.PersetujuanKodeEtik{
		UserID: userID,
		Versi:  versi,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "versi"}},
			DoNothing: true,
		}).
		Create(&rec).Error
	if err != nil {
		return nil, fmt.Errorf("simpan persetujuan kode etik: %w", err)
	}

	var saved domain.PersetujuanKodeEtik
	if err := r.db.WithContext(ctx).Where("user_id = ? AND versi = ?", userID, versi).First(&saved).Error; err != nil {
		return nil, fmt.Errorf("baca persetujuan tersimpan: %w", err)
	}
	return &saved, nil
}

// ListKinerja membaca view kinerja_petugas sesuai cakupan baris pemanggil.
func (r *Repository) ListKinerja(ctx context.Context, cakupan hargadomain.Cakupan) ([]domain.KinerjaPetugas, error) {
	kondisi, args := kondisiCakupan(cakupan, "kabupaten_kode", "petugas_id")
	var out []domain.KinerjaPetugas
	err := r.db.WithContext(ctx).Table("kinerja_petugas").
		Where(kondisi, args...).
		Order("total_entri DESC, nama_petugas ASC").
		Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("baca kinerja petugas: %w", err)
	}
	return out, nil
}
