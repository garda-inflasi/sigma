package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	eksekutif "github.com/garda-inflasi/api/internal/eksekutif/domain"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	sumber "github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// kodeUnikDilanggar adalah SQLSTATE unique_violation.
const kodeUnikDilanggar = "23505"

// cuplikanByte adalah panjang cuplikan payload pada SampelMentah.
const cuplikanByte = 2000

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Sumber(ctx context.Context, id string) (*sumber.SumberData, error) {
	var s sumber.SumberData
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSumberTidakAda
	}
	if err != nil {
		return nil, fmt.Errorf("baca sumber %s: %w", id, err)
	}
	return &s, nil
}

func (r *Repository) SumberAktif(ctx context.Context) ([]sumber.SumberData, error) {
	var items []sumber.SumberData
	if err := r.db.WithContext(ctx).Where("status <> ?", sumber.StatusDijeda).Order("id").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("baca sumber aktif: %w", err)
	}
	return items, nil
}

func (r *Repository) Pemetaan(ctx context.Context, sumberID string) (*domain.Pemetaan, error) {
	var rows []domain.PemetaanSumber
	if err := r.db.WithContext(ctx).Where("sumber_id = ?", sumberID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("baca pemetaan %s: %w", sumberID, err)
	}
	var komoditas []struct {
		Kode   string `gorm:"column:kode"`
		Satuan string `gorm:"column:satuan"`
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT kode, satuan FROM komoditas`).Scan(&komoditas).Error; err != nil {
		return nil, fmt.Errorf("baca satuan komoditas: %w", err)
	}
	satuan := make(map[string]string, len(komoditas))
	for _, k := range komoditas {
		satuan[k.Kode] = k.Satuan
	}
	return domain.NewPemetaan(rows, satuan), nil
}

func (r *Repository) MulaiEksekusi(ctx context.Context, sumberID, pemicu string, aktorID *string, macetSebelum time.Time) (*domain.EksekusiSumber, error) {
	var e *domain.EksekusiSumber
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status string
		res := tx.Raw(`SELECT status::text FROM sumber_data WHERE id = ? FOR UPDATE`, sumberID).Scan(&status)
		if res.Error != nil {
			return fmt.Errorf("kunci sumber %s: %w", sumberID, res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrSumberTidakAda
		}
		if status == sumber.StatusDijeda {
			return domain.ErrDijeda
		}

		// Proses yang mati di tengah jalan meninggalkan eksekusi BERJALAN; tanpa
		// penutupan, indeks unik parsial akan menolak eksekusi baru selamanya.
		if err := tx.Exec(`
			UPDATE eksekusi_sumber
			   SET status = 'GAGAL', selesai = now(), galat = 'eksekusi terhenti sebelum selesai (proses berhenti)'
			 WHERE sumber_id = ? AND status = 'BERJALAN' AND mulai < ?`, sumberID, macetSebelum).Error; err != nil {
			return fmt.Errorf("tutup eksekusi macet: %w", err)
		}

		baru := domain.EksekusiSumber{SumberID: sumberID, Pemicu: pemicu, AktorID: aktorID, Status: domain.StatusBerjalan, Mulai: time.Now(), Ringkasan: "{}"}
		if err := tx.Create(&baru).Error; err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == kodeUnikDilanggar {
				return domain.ErrSudahBerjalan
			}
			return fmt.Errorf("catat eksekusi: %w", err)
		}
		e = &baru
		return nil
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *Repository) SimpanMentah(ctx context.Context, m domain.DataMentah) (int64, error) {
	sum := sha256.Sum256([]byte(m.Payload))
	m.Checksum = hex.EncodeToString(sum[:])

	var ids []int64
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO data_mentah (sumber_id, eksekusi_id, kunci_permintaan, tipe_konten, diambil_pada, payload, checksum)
		VALUES (?, ?, ?, ?, ?, ?::jsonb, ?)
		ON CONFLICT (sumber_id, checksum) DO NOTHING
		RETURNING id`,
		m.SumberID, m.EksekusiID, m.KunciPermintaan, m.TipeKonten, m.DiambilPada, m.Payload, m.Checksum).Scan(&ids).Error
	if err != nil {
		return 0, fmt.Errorf("simpan data mentah: %w", err)
	}
	if len(ids) == 1 {
		return ids[0], nil
	}

	var id int64
	err = r.db.WithContext(ctx).Raw(`SELECT id FROM data_mentah WHERE sumber_id = ? AND checksum = ?`, m.SumberID, m.Checksum).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("baca data mentah yang sudah ada: %w", err)
	}
	return id, nil
}

func (r *Repository) SimpanSilver(ctx context.Context, h domain.Hasil) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if rows := unik(h.Harga, func(x domain.HargaEksternal) string {
			return fmt.Sprint(x.SumberID, "|", x.KomoditasKode, "|", x.KabupatenKode, "|", x.Tanggal.Format(time.DateOnly), "|", x.Tipe)
		}); len(rows) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "sumber_id"}, {Name: "komoditas_kode"}, {Name: "kabupaten_kode"}, {Name: "tanggal"}, {Name: "tipe"}},
				DoUpdates: clause.Assignments(map[string]any{"harga": gorm.Expr("EXCLUDED.harga"), "satuan": gorm.Expr("EXCLUDED.satuan"), "data_mentah_id": gorm.Expr("EXCLUDED.data_mentah_id"), "eksekusi_id": gorm.Expr("EXCLUDED.eksekusi_id"), "diperbarui_pada": gorm.Expr("now()")}),
			}).Create(&rows).Error
			if err != nil {
				return fmt.Errorf("upsert harga_eksternal: %w", err)
			}
		}

		// Angka resmi menggantikan baris contoh (DEMO) dan memperbarui kiriman sumber yang sama,
		// tetapi tidak menimpa angka resmi dari sumber lain.
		if rows := unik(h.Inflasi, func(x eksekutif.InflasiBulanan) string {
			return x.WilayahKode + "|" + x.Periode.Format(time.DateOnly)
		}); len(rows) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "wilayah_kode"}, {Name: "periode"}},
				DoUpdates: clause.Assignments(map[string]any{"ihk": gorm.Expr("EXCLUDED.ihk"), "inflasi_mtm": gorm.Expr("EXCLUDED.inflasi_mtm"), "inflasi_yoy": gorm.Expr("EXCLUDED.inflasi_yoy"), "inflasi_ytd": gorm.Expr("EXCLUDED.inflasi_ytd"), "sumber": gorm.Expr("EXCLUDED.sumber"), "diimpor_pada": gorm.Expr("now()")}),
				Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "inflasi_bulanan.sumber IN (?, EXCLUDED.sumber)", Vars: []any{eksekutif.SumberDemo}}}},
			}).Create(&rows).Error
			if err != nil {
				return fmt.Errorf("upsert inflasi_bulanan: %w", err)
			}
		}

		if rows := unik(h.Andil, func(x eksekutif.AndilInflasi) string {
			return x.WilayahKode + "|" + x.Periode.Format(time.DateOnly) + "|" + x.KomoditasKode
		}); len(rows) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "periode"}, {Name: "wilayah_kode"}, {Name: "komoditas_kode"}},
				DoUpdates: clause.Assignments(map[string]any{"andil": gorm.Expr("EXCLUDED.andil"), "inflasi_pct": gorm.Expr("EXCLUDED.inflasi_pct"), "sumber": gorm.Expr("EXCLUDED.sumber"), "diimpor_pada": gorm.Expr("now()")}),
				Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "andil_inflasi.sumber IN (?, EXCLUDED.sumber)", Vars: []any{eksekutif.SumberDemo}}}},
			}).Create(&rows).Error
			if err != nil {
				return fmt.Errorf("upsert andil_inflasi: %w", err)
			}
		}
		return nil
	})
}

// unik membuang duplikat kunci dalam satu kiriman (INSERT ... ON CONFLICT DO UPDATE
// menolak dua baris berkunci sama dalam satu pernyataan); yang terakhir menang.
func unik[T any](rows []T, kunci func(T) string) []T {
	if len(rows) < 2 {
		return rows
	}
	indeks := make(map[string]int, len(rows))
	res := make([]T, 0, len(rows))
	for _, row := range rows {
		k := kunci(row)
		if i, ada := indeks[k]; ada {
			res[i] = row
			continue
		}
		indeks[k] = len(res)
		res = append(res, row)
	}
	return res
}

func (r *Repository) SelesaikanEksekusi(ctx context.Context, e *domain.EksekusiSumber) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Exec(`
			UPDATE eksekusi_sumber
			   SET status = ?, selesai = ?, durasi_ms = ?, latensi_ms = ?, dokumen = ?, baris_dibaca = ?, baris_diambil = ?,
			       baris_tak_terpetakan = ?, baris_tidak_valid = ?, galat = ?, ringkasan = ?::jsonb
			 WHERE id = ? AND status = 'BERJALAN'`,
			e.Status, e.Selesai, e.DurasiMs, e.LatensiMs, e.Dokumen, e.BarisDibaca, e.BarisDiambil,
			e.BarisTakTerpetakan, e.BarisTidakValid, e.Galat, e.Ringkasan, e.ID).Error
		if err != nil {
			return fmt.Errorf("simpan hasil eksekusi: %w", err)
		}

		// Statistik sumber dihitung dari riwayat eksekusi nyata (bukan contoh) 7 hari terakhir.
		// Status DIJEDA tidak ditimpa: jeda dari operator lebih kuat daripada hasil run yang sedang berjalan.
		err = tx.Exec(`
			UPDATE sumber_data s
			   SET terakhir_jalan = ?,
			       status = CASE WHEN s.status = 'DIJEDA' THEN s.status ELSE ?::status_sumber END,
			       latensi_ms = COALESCE(?, s.latensi_ms),
			       keberhasilan_7h = (
			           SELECT ROUND(100.0 * COUNT(*) FILTER (WHERE x.status = 'SUKSES') / COUNT(*), 2)
			             FROM eksekusi_sumber x
			            WHERE x.sumber_id = s.id AND x.pemicu <> 'DEMO' AND x.status <> 'BERJALAN'
			              AND x.mulai >= ?::timestamptz - interval '7 days')
			 WHERE s.id = ?`,
			e.Selesai, statusSumber(e.Status), e.LatensiMs, e.Selesai, e.SumberID).Error
		if err != nil {
			return fmt.Errorf("perbarui statistik sumber: %w", err)
		}
		return nil
	})
}

// statusSumber memetakan hasil eksekusi ke status katalog.
func statusSumber(eksekusi string) string {
	switch eksekusi {
	case domain.StatusSukses:
		return sumber.StatusSehat
	case domain.StatusSebagian:
		return sumber.StatusTertunda
	default:
		return sumber.StatusGagal
	}
}

func (r *Repository) Eksekusi(ctx context.Context, sumberID string, limit int) ([]domain.EksekusiSumber, error) {
	var items []domain.EksekusiSumber
	err := r.db.WithContext(ctx).Where("sumber_id = ?", sumberID).Order("mulai DESC, id DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("baca riwayat eksekusi %s: %w", sumberID, err)
	}
	return items, nil
}

func (r *Repository) SampelMentah(ctx context.Context, sumberID string, limit int) ([]domain.SampelMentah, error) {
	var items []domain.SampelMentah
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, eksekusi_id, kunci_permintaan, diambil_pada,
		       length(payload::text) AS ukuran, left(payload::text, ?) AS cuplikan
		  FROM data_mentah WHERE sumber_id = ? ORDER BY diambil_pada DESC, id DESC LIMIT ?`,
		cuplikanByte, sumberID, limit).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("baca sampel mentah %s: %w", sumberID, err)
	}
	return items, nil
}
