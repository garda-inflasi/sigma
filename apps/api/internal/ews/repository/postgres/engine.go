package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"gorm.io/gorm"
)

var _ domain.EngineRepository = (*Repository)(nil)

// ukuranRow adalah baris hasil query pengukuran. Komoditas boleh kosong untuk aturan cuaca tanpa komoditas.
type ukuranRow struct {
	KomoditasKode   *string
	KomoditasNama   *string
	SatuanKomoditas *string
	KabupatenKode   string
	KabupatenNama   string
	Nilai           float64
	Terbaru         float64
}

func (u ukuranRow) pengukuran() domain.Pengukuran {
	p := domain.Pengukuran{
		KomoditasKode: u.KomoditasKode, KabupatenKode: u.KabupatenKode, KabupatenNama: u.KabupatenNama,
		Nilai: u.Nilai, Terbaru: u.Terbaru,
	}
	if u.KomoditasNama != nil {
		p.KomoditasNama = *u.KomoditasNama
	}
	if u.SatuanKomoditas != nil {
		p.SatuanKomoditas = *u.SatuanKomoditas
	}
	return p
}

// Query pengukuran. Filter komoditas/kabupaten aturan bersifat opsional (NULL = semua).

// lonjakanSQL membandingkan harga konsumen rata-rata 24 jam terakhir dengan rata-rata
// jendela sebelumnya (hanya entri DIVALIDASI/DIPUBLIKASI); nilai = kenaikan dalam %.
const lonjakanSQL = `
WITH kini AS (
	SELECT e.komoditas_kode, p.kabupaten_kode, AVG(e.harga) AS rata
	  FROM entri_harga e JOIN pasar p ON p.id = e.pasar_id
	 WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI') AND e.tipe = 'KONSUMEN'
	   AND e.dicatat_pada > @kini_dari AND e.dicatat_pada <= @sampai
	   AND (CAST(@komoditas AS text) IS NULL OR e.komoditas_kode = CAST(@komoditas AS text))
	   AND (CAST(@kabupaten AS text) IS NULL OR p.kabupaten_kode = CAST(@kabupaten AS text))
	 GROUP BY e.komoditas_kode, p.kabupaten_kode
), dasar AS (
	SELECT e.komoditas_kode, p.kabupaten_kode, AVG(e.harga) AS rata
	  FROM entri_harga e JOIN pasar p ON p.id = e.pasar_id
	 WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI') AND e.tipe = 'KONSUMEN'
	   AND e.dicatat_pada > @dasar_dari AND e.dicatat_pada <= @kini_dari
	   AND (CAST(@komoditas AS text) IS NULL OR e.komoditas_kode = CAST(@komoditas AS text))
	   AND (CAST(@kabupaten AS text) IS NULL OR p.kabupaten_kode = CAST(@kabupaten AS text))
	 GROUP BY e.komoditas_kode, p.kabupaten_kode
)
SELECT k.komoditas_kode, kom.nama AS komoditas_nama, kom.satuan AS satuan_komoditas,
       k.kabupaten_kode, kab.nama AS kabupaten_nama,
       (k.rata - d.rata) / d.rata * 100 AS nilai, k.rata AS terbaru
  FROM kini k
  JOIN dasar d ON d.komoditas_kode = k.komoditas_kode AND d.kabupaten_kode = k.kabupaten_kode
  JOIN komoditas kom ON kom.kode = k.komoditas_kode
  JOIN kabupaten kab ON kab.kode = k.kabupaten_kode
 WHERE d.rata > 0
 ORDER BY k.kabupaten_kode, k.komoditas_kode`

// stokSQL membandingkan stok layak simpan (lot belum kedaluwarsa di fasilitas
// OPERASIONAL) sekarang dengan stok pada awal jendela; nilai = penurunan dalam %
// (negatif bila stok bertambah). Hanya lot yang masih tercatat yang dihitung.
const stokSQL = `
WITH stok AS (
	SELECT s.komoditas_kode, c.kabupaten_kode,
	       SUM(s.volume_ton) FILTER (WHERE s.masuk_pada <= @awal AND s.kedaluwarsa_pada > @awal) AS awal,
	       COALESCE(SUM(s.volume_ton) FILTER (WHERE s.masuk_pada <= @sampai AND s.kedaluwarsa_pada > @sampai), 0) AS kini
	  FROM stok_cold_storage s JOIN cold_storage c ON c.id = s.cold_storage_id
	 WHERE c.status = 'OPERASIONAL'
	   AND (CAST(@komoditas AS text) IS NULL OR s.komoditas_kode = CAST(@komoditas AS text))
	   AND (CAST(@kabupaten AS text) IS NULL OR c.kabupaten_kode = CAST(@kabupaten AS text))
	 GROUP BY s.komoditas_kode, c.kabupaten_kode
)
SELECT t.komoditas_kode, kom.nama AS komoditas_nama, kom.satuan AS satuan_komoditas,
       t.kabupaten_kode, kab.nama AS kabupaten_nama,
       (t.awal - t.kini) / t.awal * -100 AS nilai, t.kini AS terbaru
  FROM stok t
  JOIN komoditas kom ON kom.kode = t.komoditas_kode
  JOIN kabupaten kab ON kab.kode = t.kabupaten_kode
 WHERE t.awal > 0
 ORDER BY t.kabupaten_kode, t.komoditas_kode`

// cuacaSQL menjumlahkan curah hujan harian dalam jendela; komoditas diambil dari aturan.
const cuacaSQL = `
SELECT kom.kode AS komoditas_kode, kom.nama AS komoditas_nama, kom.satuan AS satuan_komoditas,
       kab.kode AS kabupaten_kode, kab.nama AS kabupaten_nama,
       SUM(c.curah_hujan_mm) AS nilai, SUM(c.curah_hujan_mm) AS terbaru
  FROM cuaca_harian c
  JOIN kabupaten kab ON kab.kode = c.kabupaten_kode
  LEFT JOIN komoditas kom ON kom.kode = CAST(@komoditas AS text)
 WHERE c.tanggal >= CAST(@dari AS date) AND c.tanggal <= CAST(@sampai AS date)
   AND (CAST(@kabupaten AS text) IS NULL OR c.kabupaten_kode = CAST(@kabupaten AS text))
 GROUP BY kom.kode, kom.nama, kom.satuan, kab.kode, kab.nama
 ORDER BY kab.kode`

func (r *Repository) AturanAktif(ctx context.Context) ([]domain.AturanEws, error) {
	var items []domain.AturanEws
	err := r.db.WithContext(ctx).Where("aktif").
		Order("(komoditas_kode IS NULL), (kabupaten_kode IS NULL), id").Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("aturan ews aktif: %w", err)
	}
	return items, nil
}

func (r *Repository) Ukur(ctx context.Context, aturan domain.AturanEws, now time.Time) ([]domain.Pengukuran, error) {
	hari := time.Duration(aturan.JendelaHari) * 24 * time.Hour
	args := map[string]any{"komoditas": aturan.KomoditasKode, "kabupaten": aturan.KabupatenKode, "sampai": now}

	var query string
	switch aturan.Jenis {
	case domain.JenisLonjakanHarga:
		query = lonjakanSQL
		args["kini_dari"] = now.Add(-24 * time.Hour)
		args["dasar_dari"] = now.Add(-24*time.Hour - hari)
	case domain.JenisKelangkaanStok:
		query = stokSQL
		args["awal"] = now.Add(-hari)
	case domain.JenisAnomaliCuaca:
		query = cuacaSQL
		args["dari"] = now.AddDate(0, 0, -(aturan.JendelaHari - 1)).Format(time.DateOnly)
		args["sampai"] = now.Format(time.DateOnly)
	default:
		return nil, fmt.Errorf("jenis aturan %q belum didukung rule engine", aturan.Jenis)
	}

	var rows []ukuranRow
	if err := r.db.WithContext(ctx).Raw(query, args).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("ukur aturan %s: %w", aturan.ID, err)
	}
	out := make([]domain.Pengukuran, len(rows))
	for i, row := range rows {
		out[i] = row.pengukuran()
	}
	return out, nil
}

func (r *Repository) Terapkan(ctx context.Context, in domain.NewAlert, now time.Time) (domain.HasilTulis, *domain.Alert, error) {
	var (
		hasil   domain.HasilTulis
		alertID string
	)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var aktif struct{ ID, Severity, Status string }
		res := tx.Raw(`
			SELECT id, severity, status FROM ews_alert
			 WHERE dedup_key = ? AND status <> 'SELESAI' FOR UPDATE`, in.DedupKey).Scan(&aktif)
		if res.Error != nil {
			return fmt.Errorf("cari alert aktif: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			naik, err := naikkan(tx, aktif.ID, aktif.Status, aktif.Severity, in)
			hasil, alertID = domain.HasilDilewati, aktif.ID
			if err != nil || !naik {
				return err
			}
			hasil = domain.HasilDinaikkan
			return catatEvent(tx, aktif.ID)
		}

		var diredam bool
		err := tx.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM ews_alert
				 WHERE dedup_key = @key AND status = 'SELESAI'
				   AND COALESCE(selesai_pada, dibuat_pada) > @batas)`,
			map[string]any{"key": in.DedupKey, "batas": now.Add(-time.Duration(in.CooldownJam) * time.Hour)}).
			Scan(&diredam).Error
		if err != nil {
			return fmt.Errorf("periksa peredaman: %w", err)
		}
		if diredam {
			hasil = domain.HasilDiredam
			return nil
		}

		var ids []string
		err = tx.Raw(`
			INSERT INTO ews_alert (id, severity, jenis, judul, pesan, komoditas_kode, kabupaten_kode, nilai,
			                       satuan_nilai, skor_dampak, sumber, dibuat_pada, aturan_id, dedup_key)
			VALUES ('EWS-' || nextval('ews_alert_id_seq')::text, CAST(@severity AS severity), CAST(@jenis AS jenis_ews),
			        @judul, @pesan, @komoditas, @kabupaten, @nilai, @satuan, @skor, @sumber, @waktu, @aturan, @key)
			ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL AND status <> 'SELESAI' DO NOTHING
			RETURNING id`, map[string]any{
			"severity": in.Severity, "jenis": in.Jenis, "judul": in.Judul, "pesan": in.Pesan,
			"komoditas": in.KomoditasKode, "kabupaten": in.KabupatenKode, "nilai": in.Nilai, "satuan": in.SatuanNilai,
			"skor": in.SkorDampak, "sumber": in.Sumber, "waktu": now, "aturan": in.AturanID, "key": in.DedupKey,
		}).Scan(&ids).Error
		if err != nil {
			return fmt.Errorf("simpan alert: %w", err)
		}
		if len(ids) == 0 { // instance lain menulis kondisi yang sama lebih dulu
			hasil = domain.HasilDilewati
			return nil
		}
		hasil, alertID = domain.HasilDibuat, ids[0]
		err = tx.Exec(`
			INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, waktu, catatan)
			VALUES (?, NULL, 'BARU', NULL, ?, 'Dibuat oleh rule engine')`, alertID, now).Error
		if err != nil {
			return fmt.Errorf("riwayat alert baru: %w", err)
		}
		return catatEvent(tx, alertID)
	})
	if err != nil {
		return "", nil, err
	}
	if hasil != domain.HasilDibuat && hasil != domain.HasilDinaikkan {
		return hasil, nil, nil
	}
	alert, err := r.GetByID(ctx, alertID)
	if err != nil {
		return "", nil, err
	}
	return hasil, alert, nil
}

// naikkan memperbarui alert aktif bila tingkatnya lebih tinggi dari yang tersimpan.
func naikkan(tx *gorm.DB, id, status, severityLama string, in domain.NewAlert) (bool, error) {
	if domain.SeverityRank(in.Severity) <= domain.SeverityRank(severityLama) {
		return false, nil
	}
	err := tx.Exec(`
		UPDATE ews_alert
		   SET severity = CAST(@severity AS severity), pesan = @pesan, nilai = @nilai, skor_dampak = @skor, aturan_id = @aturan
		 WHERE id = @id`, map[string]any{
		"severity": in.Severity, "pesan": in.Pesan, "nilai": in.Nilai, "skor": in.SkorDampak, "aturan": in.AturanID, "id": id,
	}).Error
	if err != nil {
		return false, fmt.Errorf("naikkan alert: %w", err)
	}
	catatan := fmt.Sprintf("Tingkat naik dari %s ke %s", severityLama, in.Severity)
	err = tx.Exec(`
		INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, catatan)
		VALUES (?, CAST(? AS status_ews), CAST(? AS status_ews), NULL, ?)`, id, status, status, catatan).Error
	if err != nil {
		return false, fmt.Errorf("riwayat kenaikan alert: %w", err)
	}
	return true, nil
}

// kunciEvent menserialkan penulisan ews_event: kunci advisory bertahan sampai
// transaksi commit, sehingga id event yang lebih besar tidak pernah terlihat
// pembaca lebih dulu daripada id yang lebih kecil (pembaca memakai kursor id).
const kunciEvent = 710_001

// catatEvent mencatat peristiwa alert untuk aliran SSE dalam transaksi tx.
func catatEvent(tx *gorm.DB, alertID string) error {
	if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", kunciEvent).Error; err != nil {
		return fmt.Errorf("kunci event ews: %w", err)
	}
	if err := tx.Exec("INSERT INTO ews_event (alert_id) VALUES (?)", alertID).Error; err != nil {
		return fmt.Errorf("catat event ews: %w", err)
	}
	return nil
}
