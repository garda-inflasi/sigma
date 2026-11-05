package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"gorm.io/gorm"
)

// wib adalah zona waktu pengelompokan harian: hari pencatatan mengikuti hari di Aceh.
const wib = "Asia/Jakarta"

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

const viewColumns = `
	e.id, e.komoditas_kode, k.nama AS komoditas_nama, e.pasar_id, p.nama AS pasar_nama,
	p.kabupaten_kode, kb.nama AS kabupaten_nama, e.tipe, e.harga, e.satuan, e.kualitas, e.sumber,
	e.status, e.deviasi_pct, e.skor_anomali, e.lat, e.lng, e.dalam_geofence,
	(e.foto_url IS NOT NULL) AS ada_foto, e.foto_url, e.catatan, e.dicatat_pada,
	e.petugas_id, u.nama AS petugas_nama, e.divalidasi_oleh, e.divalidasi_pada,
	e.dipublikasi_oleh, e.dipublikasi_pada, e.dibuat_pada, e.diupdate_pada`

// entriView menyusun query entri beserta nama komoditas, pasar, wilayah, dan petugas.
func (r *Repository) entriView(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("entri_harga AS e").
		Joins("JOIN komoditas k ON k.kode = e.komoditas_kode").
		Joins("JOIN pasar p ON p.id = e.pasar_id").
		Joins("JOIN kabupaten kb ON kb.kode = p.kabupaten_kode").
		Joins("LEFT JOIN pengguna u ON u.id = e.petugas_id")
}

func applyFilter(q *gorm.DB, f domain.ListFilter) *gorm.DB {
	if f.Status != "" {
		q = q.Where("e.status = ?", f.Status)
	}
	if f.KabupatenKode != "" {
		q = q.Where("p.kabupaten_kode = ?", f.KabupatenKode)
	}
	if f.KomoditasKode != "" {
		q = q.Where("e.komoditas_kode = ?", f.KomoditasKode)
	}
	if f.PasarID != "" {
		q = q.Where("e.pasar_id = ?", f.PasarID)
	}
	if f.PetugasID != "" {
		q = q.Where("e.petugas_id = ?", f.PetugasID)
	}
	if f.Dari != nil {
		q = q.Where("e.dicatat_pada >= ?", *f.Dari)
	}
	if f.Sampai != nil {
		q = q.Where("e.dicatat_pada < ?", *f.Sampai)
	}
	return q
}

// List mengembalikan satu halaman entri terbaru lebih dulu beserta jumlah seluruh hasil.
func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.EntriView, int64, error) {
	var total int64
	if err := applyFilter(r.entriView(ctx), filter).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("hitung entri harga: %w", err)
	}

	var items []domain.EntriView
	err := applyFilter(r.entriView(ctx), filter).
		Select(viewColumns).
		Order("e.dicatat_pada DESC, e.id").
		Limit(filter.Limit).Offset((filter.Page - 1) * filter.Limit).
		Scan(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list entri harga: %w", err)
	}
	return items, total, nil
}

// GetByID mengembalikan satu entri beserta nama terkait.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.EntriView, error) {
	var items []domain.EntriView
	if err := r.entriView(ctx).Select(viewColumns).Where("e.id = ?", id).Limit(1).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get entri harga: %w", err)
	}
	if len(items) == 0 {
		return nil, domain.ErrNotFound
	}
	return &items[0], nil
}

// Ringkasan menghitung entri per status sejak waktu tertentu.
func (r *Repository) Ringkasan(ctx context.Context, sejak time.Time) (domain.Ringkasan, error) {
	var out struct {
		Draf, Diajukan, Divalidasi, Dipublikasi, Ditolak, Anomali int64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'DRAF')        AS draf,
			COUNT(*) FILTER (WHERE status = 'DIAJUKAN')    AS diajukan,
			COUNT(*) FILTER (WHERE status = 'DIVALIDASI')  AS divalidasi,
			COUNT(*) FILTER (WHERE status = 'DIPUBLIKASI') AS dipublikasi,
			COUNT(*) FILTER (WHERE status = 'DITOLAK')     AS ditolak,
			COUNT(*) FILTER (WHERE abs(deviasi_pct) > ?)   AS anomali
		FROM entri_harga
		WHERE dicatat_pada >= ?`, domain.AmbangAnomaliPct, sejak).Scan(&out).Error
	if err != nil {
		return domain.Ringkasan{}, fmt.Errorf("ringkasan entri harga: %w", err)
	}
	return domain.Ringkasan(out), nil
}

// Tren merata-ratakan harga resmi (DIVALIDASI atau DIPUBLIKASI) per hari atau per pekan.
func (r *Repository) Tren(ctx context.Context, komoditasKode, kabupatenKode string, sejak time.Time, interval string) ([]domain.TitikTren, error) {
	unit := "day"
	if interval == domain.IntervalMingguan {
		unit = "week"
	}

	var rows []struct {
		Tanggal  time.Time
		Produsen *float64
		Konsumen *float64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			date_trunc(?, e.dicatat_pada AT TIME ZONE ?) AS tanggal,
			AVG(e.harga) FILTER (WHERE e.tipe = 'PRODUSEN') AS produsen,
			AVG(e.harga) FILTER (WHERE e.tipe = 'KONSUMEN') AS konsumen
		FROM entri_harga e
		JOIN pasar p ON p.id = e.pasar_id
		WHERE e.komoditas_kode = ?
		  AND p.kabupaten_kode = ?
		  AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
		  AND e.dicatat_pada >= ?
		GROUP BY 1
		ORDER BY 1`, unit, wib, komoditasKode, kabupatenKode, sejak).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("tren harga: %w", err)
	}

	tren := make([]domain.TitikTren, len(rows))
	for i, row := range rows {
		tren[i] = domain.TitikTren{
			// Hasil date_trunc adalah waktu dinding WIB tanpa zona; pasang zonanya kembali.
			Tanggal:  time.Date(row.Tanggal.Year(), row.Tanggal.Month(), row.Tanggal.Day(), 0, 0, 0, 0, domain.WIB),
			Produsen: row.Produsen,
			Konsumen: row.Konsumen,
		}
	}
	return tren, nil
}

// SatuanKomoditas mengembalikan satuan komoditas aktif dan memastikan pasar aktif ada.
func (r *Repository) SatuanKomoditas(ctx context.Context, komoditasKode, pasarID string) (string, error) {
	var satuan []string
	if err := r.db.WithContext(ctx).Raw(`
		SELECT k.satuan FROM komoditas k
		 WHERE k.kode = ? AND k.status = 'AKTIF'
		   AND EXISTS (SELECT 1 FROM pasar p WHERE p.id = ? AND p.aktif)`, komoditasKode, pasarID).Scan(&satuan).Error; err != nil {
		return "", fmt.Errorf("cari komoditas dan pasar: %w", err)
	}
	if len(satuan) == 0 {
		return "", domain.ErrReferensiTidakAda
	}
	return satuan[0], nil
}

// Create menyimpan entri baru.
func (r *Repository) Create(ctx context.Context, entri *domain.EntriHarga) error {
	if err := r.db.WithContext(ctx).Create(entri).Error; err != nil {
		return fmt.Errorf("simpan entri harga: %w", err)
	}
	return nil
}

// transisi memuat pernyataan UPDATE per aksi. Syarat status dan pemisahan tugas
// (aktor bukan petugas pencatat) ada di klausa WHERE, sehingga baris yang tidak
// memenuhi syarat dilewati dan tidak muncul di RETURNING.
var transisi = map[domain.BulkAction]string{
	domain.BulkValidasi: `
		UPDATE entri_harga
		   SET status = 'DIVALIDASI', divalidasi_oleh = @aktor, divalidasi_pada = @waktu, diupdate_pada = @waktu
		 WHERE id IN @ids AND status = 'DIAJUKAN' AND petugas_id IS DISTINCT FROM @aktor
		RETURNING id`,
	domain.BulkPublikasi: `
		UPDATE entri_harga
		   SET status = 'DIPUBLIKASI', dipublikasi_oleh = @aktor, dipublikasi_pada = @waktu, diupdate_pada = @waktu
		 WHERE id IN @ids AND status = 'DIVALIDASI' AND petugas_id IS DISTINCT FROM @aktor
		RETURNING id`,
	domain.BulkTolak: `
		UPDATE entri_harga
		   SET status = 'DITOLAK', diupdate_pada = @waktu
		 WHERE id IN @ids AND status = 'DIAJUKAN'
		RETURNING id`,
	domain.BulkKembalikan: `
		UPDATE entri_harga
		   SET status = 'DRAF', divalidasi_oleh = NULL, divalidasi_pada = NULL, diupdate_pada = @waktu
		 WHERE id IN @ids AND status IN ('DIAJUKAN', 'DIVALIDASI')
		RETURNING id`,
}

// Transisi menerapkan aksi massal dan mengembalikan id yang berubah.
func (r *Repository) Transisi(ctx context.Context, ids []string, aksi domain.BulkAction, aktorID string, waktu time.Time) ([]string, error) {
	query, ok := transisi[aksi]
	if !ok {
		return nil, errors.New("aksi massal tidak dikenal: " + string(aksi))
	}
	var changed []string
	err := r.db.WithContext(ctx).Raw(query, map[string]any{"ids": ids, "aktor": aktorID, "waktu": waktu}).Scan(&changed).Error
	if err != nil {
		return nil, fmt.Errorf("aksi massal %s: %w", aksi, err)
	}
	return changed, nil
}
