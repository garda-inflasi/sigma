package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"gorm.io/gorm"
)

const tanggalSQL = "2006-01-02"

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// proyeksi memilih kolom ProyeksiView untuk proyeksi dari model PRODUKSI.
func (r *Repository) proyeksi(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("prediksi_harga AS p").
		Select(`p.id, p.komoditas_kode, k.nama AS komoditas_nama, p.kabupaten_kode, kb.nama AS kabupaten_nama,
			p.tanggal_target, p.horizon_hari, p.nilai, p.bawah, p.atas, p.harga_dasar, p.faktor::text AS faktor,
			p.sumber, p.model_versi, m.nama AS model_nama, m.mape AS model_mape, m.dilatih_pada AS model_dilatih`).
		Joins("JOIN komoditas k ON k.kode = p.komoditas_kode").
		Joins("JOIN kabupaten kb ON kb.kode = p.kabupaten_kode").
		Joins("JOIN model_registry m ON m.komoditas_kode = p.komoditas_kode AND m.versi = p.model_versi").
		Where("m.status = ?", domain.StatusProduksi)
}

func (r *Repository) ListTerbaru(ctx context.Context, kabupaten string, horizon int) ([]domain.ProyeksiView, error) {
	var items []domain.ProyeksiView
	err := r.proyeksi(ctx).
		Where("p.kabupaten_kode = ? AND p.horizon_hari = ?", kabupaten, horizon).
		Order("p.komoditas_kode, p.tanggal_target DESC").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list proyeksi: %w", err)
	}
	return terbaruPerKomoditas(items), nil
}

// terbaruPerKomoditas menyisakan baris pertama tiap komoditas; masukan sudah
// berurut komoditas lalu tanggal target terbaru.
func terbaruPerKomoditas(items []domain.ProyeksiView) []domain.ProyeksiView {
	out := make([]domain.ProyeksiView, 0, len(items))
	for i, it := range items {
		if i == 0 || it.KomoditasKode != items[i-1].KomoditasKode {
			out = append(out, it)
		}
	}
	return out
}

func (r *Repository) GetTerbaru(ctx context.Context, kode, kabupaten string, horizon int) (*domain.ProyeksiView, error) {
	var item domain.ProyeksiView
	res := r.proyeksi(ctx).
		Where("p.komoditas_kode = ? AND p.kabupaten_kode = ? AND p.horizon_hari = ?", kode, kabupaten, horizon).
		Order("p.tanggal_target DESC").Limit(1).
		Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("get proyeksi: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) Jalur(ctx context.Context, p domain.ProyeksiView, maxHorizon int) ([]domain.TitikProyeksi, error) {
	var titik []domain.TitikProyeksi
	err := r.db.WithContext(ctx).Table("prediksi_harga").
		Select("tanggal_target, horizon_hari, nilai, bawah, atas").
		Where("komoditas_kode = ? AND kabupaten_kode = ? AND model_versi = ?", p.KomoditasKode, p.KabupatenKode, p.ModelVersi).
		Where("horizon_hari <= ? AND tanggal_target - horizon_hari = ?::date", maxHorizon, p.TanggalDasar().Format(tanggalSQL)).
		Order("horizon_hari").
		Scan(&titik).Error
	if err != nil {
		return nil, fmt.Errorf("jalur proyeksi: %w", err)
	}
	return titik, nil
}

func (r *Repository) Riwayat(ctx context.Context, kode, kabupaten string, dari, sampai time.Time) ([]domain.TitikHarga, error) {
	var titik []domain.TitikHarga
	err := r.db.WithContext(ctx).Table("entri_harga AS e").
		Select("e.dicatat_pada::date AS tanggal, ROUND(AVG(e.harga))::int AS harga").
		Joins("JOIN pasar ps ON ps.id = e.pasar_id").
		Where("e.komoditas_kode = ? AND ps.kabupaten_kode = ? AND e.tipe = 'KONSUMEN'", kode, kabupaten).
		Where("e.status IN ('DIVALIDASI', 'DIPUBLIKASI')").
		Where("e.dicatat_pada >= ?::date AND e.dicatat_pada < ?::date", dari.Format(tanggalSQL), sampai.Format(tanggalSQL)).
		Group("e.dicatat_pada::date").Order("tanggal").
		Scan(&titik).Error
	if err != nil {
		return nil, fmt.Errorf("riwayat harga: %w", err)
	}
	return titik, nil
}

func (r *Repository) ListModel(ctx context.Context) ([]domain.ModelView, error) {
	var items []domain.ModelView
	err := r.db.WithContext(ctx).Table("model_registry AS m").
		Select("m.id, m.komoditas_kode, k.nama AS komoditas_nama, m.nama, m.versi, m.mape, m.drift, m.status::text AS status, m.dilatih_pada, m.sumber").
		Joins("JOIN komoditas k ON k.kode = m.komoditas_kode").
		Order("m.komoditas_kode, m.dilatih_pada DESC, m.versi DESC").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list model: %w", err)
	}
	return items, nil
}
