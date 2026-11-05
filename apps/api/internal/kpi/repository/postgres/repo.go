package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/kpi/domain"
	"gorm.io/gorm"
)

// epsilon adalah selisih terkecil yang dianggap nilai berbeda (kolom nilai 4 desimal).
const epsilon = 5e-5

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Definisi(ctx context.Context) ([]domain.Definisi, error) {
	var rows []domain.Definisi
	if err := r.db.WithContext(ctx).Order("urutan, kode").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("daftar definisi kpi: %w", err)
	}
	return rows, nil
}

func (r *Repository) DefinisiByKode(ctx context.Context, kode string) (*domain.Definisi, error) {
	var rows []domain.Definisi
	if err := r.db.WithContext(ctx).Where("kode = ?", kode).Limit(1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("definisi kpi %s: %w", kode, err)
	}
	if len(rows) == 0 {
		return nil, domain.ErrNotFound
	}
	return &rows[0], nil
}

func (r *Repository) TargetBerlaku(ctx context.Context, periode time.Time, wilayah *string) ([]domain.Target, error) {
	var kode any // NULL bila tanpa wilayah: hanya target umum yang cocok
	if wilayah != nil {
		kode = *wilayah
	}
	var rows []domain.Target
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (kpi_kode) *
		  FROM kpi_target
		 WHERE periode_mulai <= ? AND (wilayah_kode IS NULL OR wilayah_kode = ?)
		 ORDER BY kpi_kode, (wilayah_kode IS NULL), periode_mulai DESC`, periode, kode).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("target kpi: %w", err)
	}
	return rows, nil
}

func (r *Repository) NilaiBerlaku(ctx context.Context, dari, sampai time.Time, wilayah *string) ([]domain.Nilai, error) {
	var rows []domain.Nilai
	err := r.nilaiBerlaku(ctx).Where("periode BETWEEN ? AND ?", dari, sampai).
		Scopes(wilayahScope(wilayah)).Order("periode, kpi_kode").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("nilai kpi: %w", err)
	}
	return rows, nil
}

func (r *Repository) RiwayatKPI(ctx context.Context, kode string, wilayah *string, n int) ([]domain.Nilai, error) {
	var rows []domain.Nilai
	err := r.nilaiBerlaku(ctx).Where("kpi_kode = ?", kode).Scopes(wilayahScope(wilayah)).
		Order("periode DESC").Limit(n).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("riwayat kpi %s: %w", kode, err)
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

// nilaiBerlaku membatasi query ke versi terbesar tiap (kpi, wilayah, periode).
func (r *Repository) nilaiBerlaku(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&domain.Nilai{}).Where(`NOT EXISTS (
		SELECT 1 FROM kpi_nilai baru
		 WHERE baru.kpi_kode = kpi_nilai.kpi_kode
		   AND baru.wilayah_kode IS NOT DISTINCT FROM kpi_nilai.wilayah_kode
		   AND baru.periode = kpi_nilai.periode
		   AND baru.versi > kpi_nilai.versi)`)
}

func wilayahScope(wilayah *string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if wilayah == nil {
			return db.Where("wilayah_kode IS NULL")
		}
		return db.Where("wilayah_kode = ?", *wilayah)
	}
}

func (r *Repository) SimpanNilai(ctx context.Context, kode string, periode time.Time, nilai float64, dihitungPada time.Time) (domain.HasilSimpan, error) {
	hasil := domain.Baru
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var terakhir []domain.Nilai
		err := tx.Where("kpi_kode = ? AND wilayah_kode IS NULL AND periode = ?", kode, periode).
			Order("versi DESC").Limit(1).Find(&terakhir).Error
		if err != nil {
			return err
		}
		versi := 1
		if len(terakhir) > 0 {
			if math.Abs(terakhir[0].Nilai-nilai) < epsilon {
				hasil = domain.Tetap
				return nil
			}
			versi, hasil = terakhir[0].Versi+1, domain.Direvisi
		}
		return tx.Create(&domain.Nilai{KpiKode: kode, Periode: periode, Versi: versi, Nilai: nilai, DihitungPada: dihitungPada}).Error
	})
	if err != nil {
		return 0, fmt.Errorf("simpan nilai kpi %s: %w", kode, err)
	}
	return hasil, nil
}

// Penghitung. Tiap query mengembalikan satu baris dengan kolom nilai; NULL berarti
// tidak ada dasar data untuk periode itu, sehingga tidak ada nilai yang ditulis
// (bukan 0). Parameter: @mulai, @akhir (batas bulan WIB, akhir eksklusif), @asof.
// Seluruh batas dikirim sebagai timestamptz; hari dihitung menurut WIB.
var penghitung = map[string]string{
	domain.QProdusenTerverifikasi: `
		SELECT 100.0 * COUNT(DISTINCT kode_pedagang) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI'))
		       / NULLIF(COUNT(DISTINCT kode_pedagang), 0) AS nilai
		  FROM entri_harga
		 WHERE tipe = 'PRODUSEN' AND kode_pedagang IS NOT NULL
		   AND dicatat_pada >= @mulai AND dicatat_pada < @akhir`,

	domain.QKetepatanData: `
		SELECT 100.0 * COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI'))
		       / NULLIF(COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI', 'DITOLAK')), 0) AS nilai
		  FROM entri_harga
		 WHERE dicatat_pada >= @mulai AND dicatat_pada < @akhir`,

	// Rata-rata harian: pasangan (pasar aktif x komoditas pilot) yang punya entri
	// tervalidasi pada hari itu, dibagi seluruh pasangan. Hari yang dihitung hanya
	// yang sudah berlalu pada @asof.
	domain.QKelengkapanHarian: `
		WITH hari AS (
			SELECT d::date AS tgl
			  FROM generate_series(
			       (@mulai AT TIME ZONE 'Asia/Jakarta')::date,
			       LEAST((@akhir AT TIME ZONE 'Asia/Jakarta')::date - 1, (@asof AT TIME ZONE 'Asia/Jakarta')::date),
			       interval '1 day') d
		), pasangan AS (
			SELECT COUNT(*) AS n FROM pasar p CROSS JOIN komoditas k WHERE p.aktif AND k.pilot
		), terisi AS (
			SELECT (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tgl,
			       COUNT(DISTINCT (e.pasar_id, e.komoditas_kode)) AS n
			  FROM entri_harga e
			  JOIN pasar p ON p.id = e.pasar_id AND p.aktif
			  JOIN komoditas k ON k.kode = e.komoditas_kode AND k.pilot
			 WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
			   AND e.dicatat_pada >= @mulai AND e.dicatat_pada < @akhir
			 GROUP BY 1
		)
		SELECT CASE WHEN EXISTS (SELECT 1 FROM terisi) AND (SELECT n FROM pasangan) > 0 AND EXISTS (SELECT 1 FROM hari)
		            THEN 100.0 * (SELECT COALESCE(SUM(n), 0) FROM terisi WHERE tgl IN (SELECT tgl FROM hari))
		                 / ((SELECT COUNT(*) FROM hari) * (SELECT n FROM pasangan))
		       END AS nilai`,

	// Lot dianggap tersimpan dari masuk_pada sampai kedaluwarsa_pada: belum ada
	// catatan stok keluar, jadi ini pendekatan; kapasitas memakai fasilitas operasional saat ini.
	domain.QUtilisasiColdStorage: `
		WITH kapasitas AS (
			SELECT COALESCE(SUM(kapasitas_ton), 0) AS total FROM cold_storage WHERE status = 'OPERASIONAL'
		), stok AS (
			SELECT s.volume_ton
			  FROM stok_cold_storage s JOIN cold_storage c ON c.id = s.cold_storage_id
			 WHERE c.status = 'OPERASIONAL' AND s.masuk_pada <= @asof AND s.kedaluwarsa_pada > @asof
		)
		SELECT CASE WHEN EXISTS (SELECT 1 FROM stok) AND (SELECT total FROM kapasitas) > 0
		            THEN 100.0 * (SELECT SUM(volume_ton) FROM stok) / (SELECT total FROM kapasitas)
		       END AS nilai`,

	domain.QRisikoFoodLoss: `
		WITH stok AS (
			SELECT volume_ton, risiko_food_loss
			  FROM stok_cold_storage WHERE masuk_pada <= @asof AND kedaluwarsa_pada > @asof
		)
		SELECT CASE WHEN (SELECT COALESCE(SUM(volume_ton), 0) FROM stok) > 0
		            THEN 100.0 * (SELECT SUM(volume_ton * risiko_food_loss) FROM stok) / (SELECT SUM(volume_ton) FROM stok)
		       END AS nilai`,

	domain.QRekomendasiAI: `
		SELECT 100.0 * COUNT(*) FILTER (WHERE tahap LIKE '%-selesai') / NULLIF(COUNT(*), 0) AS nilai
		  FROM intervensi
		 WHERE rekomendasi_ai AND dibuat_pada >= @mulai AND dibuat_pada < @akhir`,
}

func (r *Repository) Hitung(ctx context.Context, sumberQuery string, w domain.Jendela) (*float64, error) {
	query, ok := penghitung[sumberQuery]
	if !ok {
		return nil, fmt.Errorf("%w: %q", domain.ErrQueryTakDikenal, sumberQuery)
	}
	var row struct{ Nilai *float64 }
	err := r.db.WithContext(ctx).Raw(query,
		sql.Named("mulai", w.Mulai), sql.Named("akhir", w.Akhir), sql.Named("asof", w.AsOf)).Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("hitung kpi %s: %w", sumberQuery, err)
	}
	if row.Nilai == nil {
		return nil, nil
	}
	nilai := math.Round(*row.Nilai*1e4) / 1e4
	return &nilai, nil
}
