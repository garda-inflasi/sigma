package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	e.id, e.kode, e.komoditas_kode, k.nama AS komoditas_nama, e.pasar_id, p.nama AS pasar_nama,
	p.kabupaten_kode, kb.nama AS kabupaten_nama, e.tipe, e.harga, e.satuan, e.kualitas, e.sumber,
	e.status, e.deviasi_pct, e.skor_anomali, e.lat, e.lng, e.dalam_geofence,
	(e.foto_url IS NOT NULL) AS ada_foto, e.foto_url, e.catatan, e.kode_pedagang, e.dicatat_pada,
	e.petugas_id, u.nama AS petugas_nama, e.divalidasi_oleh, e.divalidasi_pada,
	e.dipublikasi_oleh, e.dipublikasi_pada, e.alasan_penolakan, e.ditolak_oleh, e.ditolak_pada, e.flag_validasi,
	e.dibuat_pada, e.diupdate_pada`

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
		Order(urutan(filter.Urut)).
		Limit(filter.Limit).Offset((filter.Page - 1) * filter.Limit).
		Scan(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list entri harga: %w", err)
	}
	return items, total, nil
}

// urutan menerjemahkan nama urutan daftar ke klausa ORDER BY; id menjaga halaman tetap stabil.
func urutan(urut string) string {
	if urut == domain.UrutSkor {
		return "e.skor_anomali DESC, e.dicatat_pada DESC, e.id"
	}
	return "e.dicatat_pada DESC, e.id"
}

// GetByID mengembalikan satu entri beserta nama terkait. ref berisi ID BIGINT, atau kode lama (HRG-..., UUID dari
// aplikasi lama) sebagai alias tautan lama.
func (r *Repository) GetByID(ctx context.Context, ref idx.RujukanID) (*domain.EntriView, error) {
	kondisi, nilai := ref.Kondisi("e.id", "e.kode")
	var items []domain.EntriView
	if err := r.entriView(ctx).Select(viewColumns).Where(kondisi, nilai).Limit(1).Scan(&items).Error; err != nil {
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

// Simpan menyimpan entri dalam satu transaksi: kiriman ulang (kunci idempotensi
// yang sama) mengembalikan entri semula, entri petugas lain yang bentrok menolak
// penyimpanan, dan entri baru ditautkan ke tugas lapangan yang dipenuhinya.
func (r *Repository) Simpan(ctx context.Context, kirim domain.Pengiriman) (domain.HasilSimpan, error) {
	entri := kirim.Entri
	var hasil domain.HasilSimpan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hasil = domain.HasilSimpan{}

		ada, err := entriDenganKunci(tx, entri)
		if err != nil {
			return err
		}
		if ada != nil {
			hasil.Entri, hasil.Diulang = ada, true
			return nil
		}

		bentrok, err := cariBentrok(tx, entri)
		if err != nil {
			return err
		}
		if bentrok != nil {
			if !kirim.Pertahankan {
				hasil.Bentrok = bentrok
				return nil
			}
			entri.FlagValidasi.Bentrok = true
		}

		// Dua kiriman serentak dengan kunci sama: yang kalah tidak menambah baris.
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "petugas_id"}, {Name: "idempotency_key"}},
			DoNothing: true,
		}).Create(entri)
		if res.Error != nil {
			return fmt.Errorf("simpan entri harga: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			if ada, err = entriDenganKunci(tx, entri); err != nil {
				return err
			}
			hasil.Entri, hasil.Diulang = ada, true
			return nil
		}

		hasil.Entri = entri
		if err := catatRiwayatAwal(tx, entri); err != nil {
			return err
		}
		return tautkanTugas(tx, entri)
	})
	if err != nil {
		return domain.HasilSimpan{}, err
	}
	return hasil, nil
}

// entriDenganKunci mengembalikan entri petugas yang sama dengan kunci idempotensi
// yang sama, atau nil bila belum ada (atau entri tidak membawa kunci).
func entriDenganKunci(tx *gorm.DB, entri *domain.EntriHarga) (*domain.EntriHarga, error) {
	if entri.IdempotencyKey == nil || entri.PetugasID == nil {
		return nil, nil
	}
	var ada []domain.EntriHarga
	err := tx.Where("petugas_id = ? AND idempotency_key = ?", *entri.PetugasID, *entri.IdempotencyKey).Limit(1).Find(&ada).Error
	if err != nil {
		return nil, fmt.Errorf("cari entri berkunci: %w", err)
	}
	if len(ada) == 0 {
		return nil, nil
	}
	return &ada[0], nil
}

// cariBentrok mengembalikan entri petugas lain, terdekat dalam waktu, yang
// harganya berbeda pada pasar, komoditas, dan tipe yang sama dalam JendelaBentrok.
func cariBentrok(tx *gorm.DB, entri *domain.EntriHarga) (*domain.Bentrok, error) {
	var found []domain.Bentrok
	err := tx.Raw(`
		SELECT e.id, e.harga, e.dicatat_pada, e.petugas_id, COALESCE(u.nama, '') AS petugas_nama
		  FROM entri_harga e
		  LEFT JOIN pengguna u ON u.id = e.petugas_id
		 WHERE e.pasar_id = @pasar AND e.komoditas_kode = @komoditas AND e.tipe = @tipe
		   AND e.petugas_id IS DISTINCT FROM @petugas
		   AND e.status NOT IN ('DRAF', 'DITOLAK')
		   AND e.harga <> @harga
		   AND e.dicatat_pada BETWEEN @dari AND @sampai
		 ORDER BY abs(extract(epoch FROM e.dicatat_pada - CAST(@dicatat AS timestamptz))), e.id
		 LIMIT 1`, map[string]any{
		"pasar": entri.PasarID, "komoditas": entri.KomoditasKode, "tipe": entri.Tipe,
		"petugas": entri.PetugasID, "harga": entri.Harga, "dicatat": entri.DicatatPada,
		"dari": entri.DicatatPada.Add(-domain.JendelaBentrok), "sampai": entri.DicatatPada.Add(domain.JendelaBentrok),
	}).Scan(&found).Error
	if err != nil {
		return nil, fmt.Errorf("cari bentrok harga: %w", err)
	}
	if len(found) == 0 {
		return nil, nil
	}
	return &found[0], nil
}

// catatRiwayatAwal mencatat kelahiran entri sebagai perpindahan status pertama.
func catatRiwayatAwal(tx *gorm.DB, entri *domain.EntriHarga) error {
	riwayat := domain.RiwayatStatusEntri{EntriID: entri.ID, Ke: entri.Status, Oleh: entri.PetugasID, Waktu: entri.DibuatPada}
	if err := tx.Create(&riwayat).Error; err != nil {
		return fmt.Errorf("catat riwayat awal: %w", err)
	}
	return nil
}

// tautkanTugas menautkan entri ke tugas belum selesai milik petugas yang meminta
// pasar, tipe, dan komoditas ini, lalu menambah progres tugas satu komoditas
// (sekali per komoditas, paling banyak sebesar target).
func tautkanTugas(tx *gorm.DB, entri *domain.EntriHarga) error {
	if entri.PetugasID == nil {
		return nil
	}
	var tugas []idx.ID
	err := tx.Raw(`
		SELECT id FROM tugas_lapangan
		 WHERE petugas_id = ? AND pasar_id = ? AND tipe = ? AND status <> 'SELESAI'
		   AND ? = ANY(komoditas) AND dibuat_pada <= ?
		 ORDER BY tenggat, id
		 LIMIT 1
		 FOR UPDATE`, *entri.PetugasID, entri.PasarID, entri.Tipe, entri.KomoditasKode, entri.DicatatPada).Scan(&tugas).Error
	if err != nil {
		return fmt.Errorf("cari tugas lapangan: %w", err)
	}
	if len(tugas) == 0 {
		return nil
	}

	if err := tx.Create(&domain.TugasEntri{TugasID: tugas[0], EntriID: entri.ID}).Error; err != nil {
		return fmt.Errorf("tautkan entri ke tugas: %w", err)
	}
	err = tx.Exec(`
		UPDATE tugas_lapangan t
		   SET jumlah_selesai = LEAST(t.target, t.jumlah_selesai + 1)
		 WHERE t.id = ?
		   AND NOT EXISTS (
		       SELECT 1 FROM tugas_entri te JOIN entri_harga e ON e.id = te.entri_id
		        WHERE te.tugas_id = t.id AND e.komoditas_kode = ? AND te.entri_id <> ?)`,
		tugas[0], entri.KomoditasKode, entri.ID).Error
	if err != nil {
		return fmt.Errorf("perbarui progres tugas: %w", err)
	}
	return nil
}

// perubahanTransisi adalah klausa SET tiap aksi (selain status dan diupdate_pada).
var perubahanTransisi = map[domain.BulkAction]string{
	domain.BulkAjukan:     "",
	domain.BulkValidasi:   ", divalidasi_oleh = @aktor, divalidasi_pada = @waktu",
	domain.BulkPublikasi:  ", dipublikasi_oleh = @aktor, dipublikasi_pada = @waktu",
	domain.BulkTolak:      ", alasan_penolakan = @alasan, ditolak_oleh = @aktor, ditolak_pada = @waktu",
	domain.BulkKembalikan: ", divalidasi_oleh = NULL, divalidasi_pada = NULL",
}

// Transisi mengunci entri yang diminta, menilai tiap id dengan domain.Evaluasi, menerapkan
// aksi pada yang lolos, dan mencatat riwayat statusnya; semuanya dalam satu transaksi.
func (r *Repository) Transisi(ctx context.Context, ids []idx.ID, aksi domain.BulkAction, aktorID, alasan string, waktu time.Time) ([]domain.HasilTransisi, error) {
	set, ok := perubahanTransisi[aksi]
	if !ok {
		return nil, errors.New("aksi massal tidak dikenal: " + string(aksi))
	}

	hasil := make([]domain.HasilTransisi, len(ids))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Pelaku diberitahukan ke pemicu versi entri (migrasi 000118) lewat GUC lokal-transaksi, sehingga
		// versi KEMBALIKAN, yang tidak punya kolom pelaku di entri_harga, tetap mencatat pelakunya.
		if err := tx.Exec("SELECT set_config(?, ?, true)", domain.AktorGUC, aktorID).Error; err != nil {
			return fmt.Errorf("catat pelaku transaksi: %w", err)
		}
		var rows []struct {
			ID        idx.ID  `gorm:"column:id"`
			Status    string  `gorm:"column:status"`
			PetugasID *string `gorm:"column:petugas_id;type:uuid"`
		}
		if err := tx.Raw("SELECT id, status::text AS status, petugas_id FROM entri_harga WHERE id IN ? ORDER BY id FOR UPDATE", ids).Scan(&rows).Error; err != nil {
			return fmt.Errorf("kunci entri: %w", err)
		}
		ada := make(map[idx.ID]int, len(rows))
		for i, row := range rows {
			ada[row.ID] = i
		}

		var lolos []idx.ID
		var riwayat []domain.RiwayatStatusEntri
		for i, id := range ids {
			hasil[i] = domain.HasilTransisi{ID: id, Sebab: domain.SebabTidakDitemukan}
			idx, found := ada[id]
			if !found {
				continue
			}
			row := rows[idx]
			if hasil[i].Sebab = domain.Evaluasi(aksi, row.Status, row.PetugasID, aktorID); hasil[i].Sebab != "" {
				continue
			}
			dari, oleh := row.Status, aktorID
			entry := domain.RiwayatStatusEntri{EntriID: id, Dari: &dari, Ke: aksi.StatusTujuan(), Oleh: &oleh, Waktu: waktu}
			if alasan != "" {
				entry.Alasan = &alasan
			}
			riwayat = append(riwayat, entry)
			lolos = append(lolos, id)
		}
		if len(lolos) == 0 {
			return nil
		}

		err := tx.Exec("UPDATE entri_harga SET status = @ke, diupdate_pada = @waktu"+set+" WHERE id IN @ids", map[string]any{
			"ke": aksi.StatusTujuan(), "waktu": waktu, "aktor": aktorID, "alasan": alasan, "ids": lolos,
		}).Error
		if err != nil {
			return fmt.Errorf("aksi massal %s: %w", aksi, err)
		}
		if err := tx.Create(&riwayat).Error; err != nil {
			return fmt.Errorf("catat riwayat status: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hasil, nil
}

// Riwayat mengembalikan perpindahan status entri, yang terlama lebih dulu.
func (r *Repository) Riwayat(ctx context.Context, entriID idx.ID) ([]domain.RiwayatView, error) {
	var items []domain.RiwayatView
	err := r.db.WithContext(ctx).Raw(`
		SELECT h.dari::text AS dari, h.ke::text AS ke, h.oleh, u.nama AS oleh_nama, h.waktu, h.alasan
		  FROM riwayat_status_entri h
		  LEFT JOIN pengguna u ON u.id = h.oleh
		 WHERE h.entri_id = ?
		 ORDER BY h.waktu, h.id`, entriID).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("riwayat status entri: %w", err)
	}
	return items, nil
}

// Versi mengembalikan seluruh versi isi entri (migrasi 000118), yang terlama lebih dulu.
func (r *Repository) Versi(ctx context.Context, entriID idx.ID) ([]domain.VersiView, error) {
	var items []domain.VersiView
	err := r.db.WithContext(ctx).Raw(`
		SELECT v.versi, v.aksi, v.status::text AS status, v.harga, v.oleh, u.nama AS oleh_nama, v.waktu, v.perubahan, v.data
		  FROM entri_harga_versi v
		  LEFT JOIN pengguna u ON u.id = v.oleh
		 WHERE v.entri_id = ?
		 ORDER BY v.versi`, entriID).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("versi entri harga: %w", err)
	}
	return items, nil
}

// Lineage melengkapi entri dengan jejak asalnya: nama pelaku validasi, publikasi, dan penolakan, tugas lapangan
// yang dipenuhi, seluruh versi, riwayat status, dan rujukan audit trail.
func (r *Repository) Lineage(ctx context.Context, entri domain.EntriView) (domain.Lineage, error) {
	out := domain.Lineage{Entri: entri}
	db := r.db.WithContext(ctx)

	err := db.Raw(`
		SELECT (SELECT nama FROM pengguna WHERE id = e.divalidasi_oleh)  AS divalidasi_nama,
		       (SELECT nama FROM pengguna WHERE id = e.dipublikasi_oleh) AS dipublikasi_nama,
		       (SELECT nama FROM pengguna WHERE id = e.ditolak_oleh)     AS ditolak_nama
		  FROM entri_harga e
		 WHERE e.id = ?`, entri.ID).Scan(&out.Nama).Error
	if err != nil {
		return out, fmt.Errorf("nama pelaku entri harga: %w", err)
	}

	err = db.Raw(`
		SELECT t.id, t.kode, t.judul, te.ditautkan_pada
		  FROM tugas_entri te
		  JOIN tugas_lapangan t ON t.id = te.tugas_id
		 WHERE te.entri_id = ?
		 ORDER BY te.ditautkan_pada, t.id`, entri.ID).Scan(&out.Tugas).Error
	if err != nil {
		return out, fmt.Errorf("tugas lapangan entri harga: %w", err)
	}

	if out.Versi, err = r.Versi(ctx, entri.ID); err != nil {
		return out, err
	}
	for _, v := range out.Versi {
		out.VersiTerkini = max(out.VersiTerkini, v.Versi)
	}
	if out.Riwayat, err = r.Riwayat(ctx, entri.ID); err != nil {
		return out, err
	}
	if out.Audit, err = r.rujukanAudit(ctx, entri); err != nil {
		return out, err
	}
	return out, nil
}

// rujukanAudit mengambil baris audit_log yang menyebut entri: baris dengan entitas_id sama dengan ID (atau kode
// lama) entri, dan aksi massal yang memuatnya di metadata.ids. Batas bawah waktu (satu hari sebelum entri dibuat)
// membiarkan PostgreSQL memangkas partisi audit_log yang lebih tua; kueri tidak bergantung pada kunci primer
// audit_log. Hanya kolom ringkas yang dibaca; operator jsonb `?` tidak dipakai karena bentrok dengan placeholder GORM.
func (r *Repository) rujukanAudit(ctx context.Context, entri domain.EntriView) ([]domain.RujukanAudit, error) {
	rujukan := []string{entri.ID.String()}
	if entri.Kode != nil && *entri.Kode != "" {
		rujukan = append(rujukan, *entri.Kode)
	}

	kondisi := "a.entitas_id IN ?"
	args := []any{"entri_harga", entri.DibuatPada.Add(-24 * time.Hour), rujukan}
	for _, ref := range rujukan {
		kondisi += " OR a.metadata->'ids' @> jsonb_build_array(CAST(? AS text))"
		args = append(args, ref)
	}
	args = append(args, domain.BatasRujukanAudit)

	var items []domain.RujukanAudit
	err := r.db.WithContext(ctx).Raw(`
		SELECT a.id, a.waktu, a.aksi, a.aktor_nama, a.aktor_peran, a.hasil, a.correlation_id
		  FROM audit_log a
		 WHERE a.entitas = ? AND a.waktu >= ? AND (`+kondisi+`)
		 ORDER BY a.waktu, a.id
		 LIMIT ?`, args...).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("rujukan audit entri harga: %w", err)
	}
	return items, nil
}

// KonteksValidasi membaca batas harga yang berlaku, geofence pasar, harga acuan komoditas, histori
// harga resmi sesama kabupaten, dan ada tidaknya entri duplikat untuk entri yang akan disimpan.
func (r *Repository) KonteksValidasi(ctx context.Context, entri *domain.EntriHarga) (domain.KonteksValidasi, error) {
	var k domain.KonteksValidasi
	tx := r.db.WithContext(ctx)

	var pasar []struct {
		Lat           float64 `gorm:"column:lat"`
		Lng           float64 `gorm:"column:lng"`
		RadiusMeter   int     `gorm:"column:radius_meter"`
		KabupatenKode string  `gorm:"column:kabupaten_kode"`
		HargaAcuan    int     `gorm:"column:harga_acuan"`
	}
	err := tx.Raw(`
		SELECT p.lat, p.lng, p.radius_meter, p.kabupaten_kode, k.harga_acuan
		  FROM pasar p, komoditas k
		 WHERE p.id = ? AND k.kode = ?`, entri.PasarID, entri.KomoditasKode).Scan(&pasar).Error
	if err != nil {
		return k, fmt.Errorf("baca pasar dan komoditas: %w", err)
	}
	if len(pasar) == 0 {
		return k, domain.ErrReferensiTidakAda
	}
	k.PasarLat, k.PasarLng, k.RadiusMeter = pasar[0].Lat, pasar[0].Lng, pasar[0].RadiusMeter
	k.HargaAcuan = domain.AcuanTipe(pasar[0].HargaAcuan, entri.Tipe)

	var batas []domain.BatasHarga
	err = tx.Raw(`
		SELECT id, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari
		  FROM batas_harga
		 WHERE komoditas_kode = ? AND (kabupaten_kode = ? OR kabupaten_kode IS NULL)
		   AND berlaku_dari <= (CAST(? AS timestamptz) AT TIME ZONE ?)::date
		 ORDER BY (kabupaten_kode IS NULL), berlaku_dari DESC
		 LIMIT 1`, entri.KomoditasKode, pasar[0].KabupatenKode, entri.DicatatPada, wib).Scan(&batas).Error
	if err != nil {
		return k, fmt.Errorf("baca batas harga: %w", err)
	}
	if len(batas) > 0 {
		k.Batas = &batas[0]
	}

	err = tx.Raw(`
		SELECT e.harga
		  FROM entri_harga e
		  JOIN pasar p ON p.id = e.pasar_id
		 WHERE e.komoditas_kode = @komoditas AND e.tipe = @tipe AND p.kabupaten_kode = @kabupaten
		   AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
		   AND e.dicatat_pada BETWEEN @dari AND @sampai
		 ORDER BY e.dicatat_pada DESC
		 LIMIT 500`, map[string]any{
		"komoditas": entri.KomoditasKode, "tipe": entri.Tipe, "kabupaten": pasar[0].KabupatenKode,
		"dari": entri.DicatatPada.AddDate(0, 0, -domain.HariHistori), "sampai": entri.DicatatPada,
	}).Scan(&k.Histori).Error
	if err != nil {
		return k, fmt.Errorf("baca histori harga: %w", err)
	}

	err = tx.Raw(`
		SELECT EXISTS (
		    SELECT 1 FROM entri_harga e
		     WHERE e.pasar_id = @pasar AND e.komoditas_kode = @komoditas AND e.tipe = @tipe
		       AND e.kode_pedagang IS NOT DISTINCT FROM CAST(@pedagang AS varchar)
		       AND e.status NOT IN ('DRAF', 'DITOLAK')
		       AND e.dicatat_pada BETWEEN @dari AND @sampai)`, map[string]any{
		"pasar": entri.PasarID, "komoditas": entri.KomoditasKode, "tipe": entri.Tipe, "pedagang": entri.KodePedagang,
		"dari": entri.DicatatPada.Add(-domain.JendelaBentrok), "sampai": entri.DicatatPada.Add(domain.JendelaBentrok),
	}).Scan(&k.Duplikat).Error
	if err != nil {
		return k, fmt.Errorf("cek duplikat harga: %w", err)
	}
	return k, nil
}
