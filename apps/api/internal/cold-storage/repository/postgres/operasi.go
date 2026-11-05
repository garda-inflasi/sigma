package postgres

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ domain.OperasiRepository = (*Repository)(nil)

// epsilon menoleransi galat pembulatan saat membandingkan volume (ton).
const epsilon = 1e-6

// telemetriBatch adalah ukuran satu INSERT telemetri.
const telemetriBatch = 200

// round3 membulatkan ke kilogram (3 desimal ton) agar saldo lot tidak menumpuk galat float.
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func (r *Repository) GetFasilitas(ctx context.Context, id string) (*domain.FasilitasView, error) {
	return fasilitasByID(r.db.WithContext(ctx), id)
}

func fasilitasByID(db *gorm.DB, id string) (*domain.FasilitasView, error) {
	var rows []domain.FasilitasView
	if err := db.Raw(fmt.Sprintf(fasilitasSelect, "WHERE c.id = ?"), id).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("ambil fasilitas: %w", err)
	}
	if len(rows) == 0 {
		return nil, domain.ErrFasilitasTidakAda
	}
	return &rows[0], nil
}

// fasilitasLock adalah baris fasilitas yang dikunci selama transaksi.
type fasilitasLock struct {
	ID           string
	KapasitasTon int
	Status       string
}

// lockFasilitas mengunci fasilitas menurut urutan id (mencegah deadlock antar
// transaksi yang menyentuh pasangan fasilitas sama) dan mengembalikannya per id.
func lockFasilitas(tx *gorm.DB, ids ...string) (map[string]fasilitasLock, error) {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	var rows []fasilitasLock
	if err := tx.Raw("SELECT id, kapasitas_ton, status FROM cold_storage WHERE id IN ? ORDER BY id FOR UPDATE", sorted).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("kunci fasilitas: %w", err)
	}
	out := make(map[string]fasilitasLock, len(rows))
	for _, row := range rows {
		out[row.ID] = row
	}
	for _, id := range sorted {
		if _, ok := out[id]; !ok {
			return nil, domain.ErrFasilitasTidakAda
		}
	}
	return out, nil
}

func (r *Repository) UpdateFasilitas(ctx context.Context, id string, patch domain.FasilitasPatch) (before, after domain.FasilitasView, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockFasilitas(tx, id); err != nil {
			return err
		}
		b, err := fasilitasByID(tx, id)
		if err != nil {
			return err
		}
		before = *b

		updates := map[string]any{}
		if patch.KapasitasTon != nil {
			if float64(*patch.KapasitasTon)+epsilon < before.TerisiTon {
				return domain.ErrKapasitasDiBawahStok
			}
			updates["kapasitas_ton"] = *patch.KapasitasTon
		}
		if patch.SuhuMinC != nil {
			updates["suhu_min_c"] = *patch.SuhuMinC
		}
		if patch.SuhuMaksC != nil {
			updates["suhu_maks_c"] = *patch.SuhuMaksC
		}
		if patch.Status != nil {
			updates["status"] = *patch.Status
		}
		if len(updates) == 0 {
			after = before
			return nil
		}
		lo, hi := before.SuhuMinC, before.SuhuMaksC
		if patch.SuhuMinC != nil {
			lo = patch.SuhuMinC
		}
		if patch.SuhuMaksC != nil {
			hi = patch.SuhuMaksC
		}
		if lo != nil && hi != nil && *lo >= *hi {
			return domain.ErrBatasSuhuTidakValid
		}
		if err := tx.Model(&domain.FasilitasColdStorage{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return fmt.Errorf("ubah fasilitas: %w", err)
		}
		a, err := fasilitasByID(tx, id)
		if err != nil {
			return err
		}
		after = *a
		return nil
	})
	return before, after, err
}

func (r *Repository) SimpanTelemetri(ctx context.Context, rows []domain.TelemetriSuhu) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.FasilitasID)
	}

	var inserted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Mengunci fasilitas menyerialkan penerimaan telemetri per fasilitas,
		// sehingga "terbaru" yang dihitung di bawah tidak terlewat oleh transaksi lain.
		if _, err := lockFasilitas(tx, ids...); err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fasilitas_id"}, {Name: "waktu"}},
			DoNothing: true,
		}).CreateInBatches(&rows, telemetriBatch)
		if res.Error != nil {
			return fmt.Errorf("simpan telemetri: %w", res.Error)
		}
		inserted = res.RowsAffected

		// suhu_aktual_c adalah turunan dari pembacaan terbaru.
		if err := tx.Exec(`
			UPDATE cold_storage c
			   SET suhu_aktual_c = t.suhu_c, kelembapan_aktual_pct = t.kelembapan_pct, telemetri_pada = t.waktu
			  FROM (SELECT DISTINCT ON (fasilitas_id) fasilitas_id, waktu, suhu_c, kelembapan_pct
			          FROM telemetri_suhu WHERE fasilitas_id IN ?
			         ORDER BY fasilitas_id, waktu DESC) t
			 WHERE c.id = t.fasilitas_id`, ids).Error; err != nil {
			return fmt.Errorf("perbarui suhu aktual: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return inserted, nil
}

func (r *Repository) ListTelemetri(ctx context.Context, fasilitasID string, sejak time.Time, limit int) ([]domain.TelemetriSuhu, error) {
	var rows []domain.TelemetriSuhu
	err := r.db.WithContext(ctx).
		Where("fasilitas_id = ? AND waktu >= ?", fasilitasID, sejak).
		Order("waktu DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("daftar telemetri: %w", err)
	}
	return rows, nil
}

func (r *Repository) StokFasilitas(ctx context.Context, fasilitasID string) ([]domain.StokView, error) {
	items := []domain.StokView{}
	err := r.db.WithContext(ctx).Table("stok_cold_storage AS s").
		Select(`s.id, s.cold_storage_id, c.nama AS cold_storage_nama, s.komoditas_kode,
			k.nama AS komoditas_nama, s.volume_ton, s.masuk_pada, s.kedaluwarsa_pada,
			s.risiko_food_loss, s.pemilik, s.status`).
		Joins("JOIN cold_storage c ON c.id = s.cold_storage_id").
		Joins("JOIN komoditas k ON k.kode = s.komoditas_kode").
		Where("s.cold_storage_id = ? AND s.status <> ?", fasilitasID, domain.StatusLotHabis).
		Order("s.kedaluwarsa_pada ASC, s.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("stok fasilitas: %w", err)
	}
	return items, nil
}

func (r *Repository) ListTransaksi(ctx context.Context, fasilitasID string, limit int) ([]domain.TransaksiView, error) {
	items := []domain.TransaksiView{}
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.id, t.lot_id, t.lot_baru_id, t.jenis, t.volume_ton, s.komoditas_kode, k.nama AS komoditas_nama,
		       t.fasilitas_asal_id, t.fasilitas_tujuan_id, p.nama AS oleh_nama, t.waktu, t.alasan
		  FROM transaksi_stok t
		  JOIN stok_cold_storage s ON s.id = t.lot_id
		  JOIN komoditas k ON k.kode = s.komoditas_kode
		  LEFT JOIN pengguna p ON p.id = t.oleh_id
		 WHERE t.fasilitas_asal_id = ? OR t.fasilitas_tujuan_id = ?
		 ORDER BY t.waktu DESC, t.id DESC
		 LIMIT ?`, fasilitasID, fasilitasID, limit).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("daftar transaksi: %w", err)
	}
	return items, nil
}

func (r *Repository) TandaiPrioritasJual(ctx context.Context, ids []string) ([]domain.BarisBulk, error) {
	var rows []struct {
		ID        string
		VolumeTon float64
	}
	err := r.db.WithContext(ctx).Raw(`
		UPDATE stok_cold_storage SET status = ?
		 WHERE id IN ? AND status <> ?
		RETURNING id, volume_ton`, domain.StatusLotPrioritasJual, ids, domain.StatusLotHabis).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("tandai prioritas jual: %w", err)
	}
	out := make([]domain.BarisBulk, len(rows))
	for i, row := range rows {
		out[i] = domain.BarisBulk{ID: row.ID, Berhasil: true, VolumeTon: row.VolumeTon}
	}
	return out, nil
}

// ── Transaksi stok ────────────────────────────────────────────────────

func (r *Repository) CatatTransaksi(ctx context.Context, in domain.TransaksiInput) (domain.TransaksiHasil, error) {
	var out domain.TransaksiHasil
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		switch in.Jenis {
		case domain.TransaksiMasuk:
			out, err = masuk(tx, in)
		case domain.TransaksiKeluar:
			out, err = keluar(tx, in)
		case domain.TransaksiPindah:
			out, err = pindah(tx, in)
		default:
			err = fmt.Errorf("jenis transaksi tidak dikenal: %q", in.Jenis)
		}
		return err
	})
	return out, err
}

// kunciLot mengunci dan membaca lot; lot yang sudah habis dianggap tidak dapat ditransaksikan.
func kunciLot(tx *gorm.DB, id string) (*domain.StokColdStorage, error) {
	var lots []domain.StokColdStorage
	if err := tx.Raw("SELECT * FROM stok_cold_storage WHERE id = ? FOR UPDATE", id).Scan(&lots).Error; err != nil {
		return nil, fmt.Errorf("kunci lot: %w", err)
	}
	if len(lots) == 0 {
		return nil, domain.ErrLotTidakAda
	}
	if lots[0].Status == domain.StatusLotHabis {
		return nil, domain.ErrLotHabis
	}
	return &lots[0], nil
}

// terisiTon menjumlahkan volume lot yang belum habis pada fasilitas.
func terisiTon(tx *gorm.DB, fasilitasID string) (float64, error) {
	var total float64
	err := tx.Raw("SELECT COALESCE(SUM(volume_ton), 0) FROM stok_cold_storage WHERE cold_storage_id = ? AND status <> ?",
		fasilitasID, domain.StatusLotHabis).Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("hitung stok terisi: %w", err)
	}
	return total, nil
}

// pastikanMuat memeriksa fasilitas beroperasi dan masih memuat tambahan volume.
func pastikanMuat(tx *gorm.DB, fas fasilitasLock, tambahan float64) error {
	if fas.Status != domain.StatusFasilitasOperasional {
		return domain.ErrFasilitasTidakAktif
	}
	terisi, err := terisiTon(tx, fas.ID)
	if err != nil {
		return err
	}
	if terisi+tambahan > float64(fas.KapasitasTon)+epsilon {
		return domain.ErrKapasitasTidakCukup
	}
	return nil
}

func catat(tx *gorm.DB, in domain.TransaksiInput, volume float64, asal, tujuan string, lotBaru *string) error {
	row := domain.TransaksiStok{
		ID: in.ID, LotID: in.LotID, LotBaruID: lotBaru, Jenis: in.Jenis, VolumeTon: volume,
		Waktu: in.Waktu, Alasan: in.Alasan, Sumber: domain.SumberApp,
	}
	if asal != "" {
		row.FasilitasAsalID = &asal
	}
	if tujuan != "" {
		row.FasilitasTujuanID = &tujuan
	}
	if in.OlehID != "" {
		row.OlehID = &in.OlehID
	}
	if err := tx.Create(&row).Error; err != nil {
		return fmt.Errorf("catat transaksi: %w", err)
	}
	return nil
}

func masuk(tx *gorm.DB, in domain.TransaksiInput) (domain.TransaksiHasil, error) {
	fas, err := lockFasilitas(tx, in.FasilitasID)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}
	var shelfLife []int
	if err := tx.Raw("SELECT shelf_life_hari FROM komoditas WHERE kode = ?", in.KomoditasKode).Scan(&shelfLife).Error; err != nil {
		return domain.TransaksiHasil{}, fmt.Errorf("cari komoditas: %w", err)
	}
	if len(shelfLife) == 0 {
		return domain.TransaksiHasil{}, domain.ErrKomoditasTidakAda
	}
	if err := pastikanMuat(tx, fas[in.FasilitasID], in.VolumeTon); err != nil {
		return domain.TransaksiHasil{}, err
	}

	kedaluwarsa := in.Waktu.Add(time.Duration(shelfLife[0]) * 24 * time.Hour)
	if in.KedaluwarsaPada != nil {
		kedaluwarsa = *in.KedaluwarsaPada
	}
	volume := round3(in.VolumeTon)
	lot := domain.StokColdStorage{
		ID: in.LotID, ColdStorageID: in.FasilitasID, KomoditasKode: in.KomoditasKode, VolumeTon: volume,
		MasukPada: in.Waktu, KedaluwarsaPada: kedaluwarsa, Pemilik: in.Pemilik,
		RisikoFoodLoss: domain.RisikoAwalLot, Status: domain.StatusLotAktif,
	}
	if err := tx.Create(&lot).Error; err != nil {
		return domain.TransaksiHasil{}, fmt.Errorf("buat lot: %w", err)
	}
	if err := catat(tx, in, volume, "", in.FasilitasID, nil); err != nil {
		return domain.TransaksiHasil{}, err
	}
	return domain.TransaksiHasil{
		TransaksiID: in.ID, Jenis: in.Jenis, LotID: lot.ID, VolumeTon: volume, VolumeSisaTon: volume,
		StatusLot: lot.Status,
		Sesudah:   domain.LotSnapshot{FasilitasID: lot.ColdStorageID, VolumeTon: volume, Status: lot.Status},
	}, nil
}

// volumeBergerak menentukan volume transaksi atas lot: seluruhnya (Semua, atau
// permintaan sebesar sisa) atau sebagian.
func volumeBergerak(lot *domain.StokColdStorage, in domain.TransaksiInput) (volume float64, semua bool, err error) {
	if in.Semua || math.Abs(in.VolumeTon-lot.VolumeTon) <= epsilon {
		return lot.VolumeTon, true, nil
	}
	if in.VolumeTon > lot.VolumeTon {
		return 0, false, domain.ErrStokTidakCukup
	}
	return round3(in.VolumeTon), false, nil
}

func keluar(tx *gorm.DB, in domain.TransaksiInput) (domain.TransaksiHasil, error) {
	lot, err := kunciLot(tx, in.LotID)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}
	volume, semua, err := volumeBergerak(lot, in)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}
	sebelum := domain.LotSnapshot{FasilitasID: lot.ColdStorageID, VolumeTon: lot.VolumeTon, Status: lot.Status}

	sisa, status := round3(lot.VolumeTon-volume), lot.Status
	if semua {
		sisa, status = 0, domain.StatusLotHabis
	}
	if err := tx.Model(&domain.StokColdStorage{}).Where("id = ?", lot.ID).
		Updates(map[string]any{"volume_ton": sisa, "status": status}).Error; err != nil {
		return domain.TransaksiHasil{}, fmt.Errorf("kurangi lot: %w", err)
	}
	if err := catat(tx, in, volume, lot.ColdStorageID, "", nil); err != nil {
		return domain.TransaksiHasil{}, err
	}
	return domain.TransaksiHasil{
		TransaksiID: in.ID, Jenis: in.Jenis, LotID: lot.ID, VolumeTon: volume, VolumeSisaTon: sisa, StatusLot: status,
		Sebelum: &sebelum,
		Sesudah: domain.LotSnapshot{FasilitasID: lot.ColdStorageID, VolumeTon: sisa, Status: status},
	}, nil
}

func pindah(tx *gorm.DB, in domain.TransaksiInput) (domain.TransaksiHasil, error) {
	lot, err := kunciLot(tx, in.LotID)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}
	asal, tujuan := lot.ColdStorageID, in.FasilitasTujuanID
	if asal == tujuan {
		return domain.TransaksiHasil{}, domain.ErrFasilitasSama
	}
	volume, semua, err := volumeBergerak(lot, in)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}
	fas, err := lockFasilitas(tx, asal, tujuan)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}
	if err := pastikanMuat(tx, fas[tujuan], volume); err != nil {
		return domain.TransaksiHasil{}, err
	}
	sebelum := domain.LotSnapshot{FasilitasID: asal, VolumeTon: lot.VolumeTon, Status: lot.Status}

	if semua {
		if err := tx.Model(&domain.StokColdStorage{}).Where("id = ?", lot.ID).Update("cold_storage_id", tujuan).Error; err != nil {
			return domain.TransaksiHasil{}, fmt.Errorf("pindahkan lot: %w", err)
		}
		if err := catat(tx, in, volume, asal, tujuan, nil); err != nil {
			return domain.TransaksiHasil{}, err
		}
		return domain.TransaksiHasil{
			TransaksiID: in.ID, Jenis: in.Jenis, LotID: lot.ID, VolumeTon: volume, VolumeSisaTon: volume, StatusLot: lot.Status,
			Sebelum: &sebelum,
			Sesudah: domain.LotSnapshot{FasilitasID: tujuan, VolumeTon: volume, Status: lot.Status},
		}, nil
	}

	// Pindah sebagian: lot sumber berkurang, lot baru terbentuk di tujuan dengan atribut yang sama.
	sisa := round3(lot.VolumeTon - volume)
	if err := tx.Model(&domain.StokColdStorage{}).Where("id = ?", lot.ID).Update("volume_ton", sisa).Error; err != nil {
		return domain.TransaksiHasil{}, fmt.Errorf("kurangi lot sumber: %w", err)
	}
	baru := *lot
	baru.ID, baru.ColdStorageID, baru.VolumeTon = in.LotBaruID, tujuan, volume
	if err := tx.Create(&baru).Error; err != nil {
		return domain.TransaksiHasil{}, fmt.Errorf("buat lot tujuan: %w", err)
	}
	if err := catat(tx, in, volume, asal, tujuan, &baru.ID); err != nil {
		return domain.TransaksiHasil{}, err
	}
	return domain.TransaksiHasil{
		TransaksiID: in.ID, Jenis: in.Jenis, LotID: lot.ID, LotBaruID: &baru.ID, VolumeTon: volume, VolumeSisaTon: sisa,
		StatusLot: lot.Status,
		Sebelum:   &sebelum,
		Sesudah:   domain.LotSnapshot{FasilitasID: asal, VolumeTon: sisa, Status: lot.Status},
	}, nil
}
