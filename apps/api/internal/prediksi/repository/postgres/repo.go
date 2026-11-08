package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const tanggalSQL = "2006-01-02"

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// proyeksi memilih kolom ProyeksiView untuk proyeksi dari model PRODUKSI. Model dicari lewat FK model_id,
// dan versinya dibaca dari registri (sumber kebenaran), bukan dari kolom informasi prediksi_harga.model_versi.
func (r *Repository) proyeksi(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("prediksi_harga AS p").
		Select(`p.id, p.model_id, p.komoditas_kode, k.nama AS komoditas_nama, p.kabupaten_kode, kb.nama AS kabupaten_nama,
			p.tanggal_target, p.horizon_hari, p.nilai, p.bawah, p.atas, p.harga_dasar, p.faktor::text AS faktor,
			p.sumber, m.versi AS model_versi, m.nama AS model_nama, m.mape AS model_mape, m.dilatih_pada AS model_dilatih`).
		Joins("JOIN komoditas k ON k.kode = p.komoditas_kode").
		Joins("JOIN kabupaten kb ON kb.kode = p.kabupaten_kode").
		Joins("JOIN model_registry m ON m.id = p.model_id").
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
		Where("komoditas_kode = ? AND kabupaten_kode = ? AND model_id = ?", p.KomoditasKode, p.KabupatenKode, p.ModelID).
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
		Joins("JOIN pasar ps ON (ps.kode = e.pasar_id OR ps.id::text = e.pasar_id)").
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

// modelView memilih kolom ModelView: registri digabung nama komoditas, MAPE baseline
// dari metrik pelatihan, dan MAPE realisasi dari evaluasi proyeksi model itu.
func (r *Repository) modelView(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("model_registry AS m").
		Select(`m.id, m.kode, m.komoditas_kode, k.nama AS komoditas_nama, m.nama, m.versi, m.mape, m.drift,
			m.status::text AS status, m.dilatih_pada, m.sumber, m.catatan,
			(m.metrics->>'baseline_mape')::float8 AS baseline_mape,
			ev.mape_realisasi, COALESCE(ev.n_evaluasi, 0) AS n_evaluasi`).
		Joins("JOIN komoditas k ON k.kode = m.komoditas_kode").
		Joins(`LEFT JOIN (
				SELECT p.model_id, AVG(ABS(e.galat_pct))::float8 AS mape_realisasi, COUNT(*)::int AS n_evaluasi
				FROM prediksi_evaluasi e JOIN prediksi_harga p ON p.id = e.prediksi_id
				GROUP BY p.model_id
			) ev ON ev.model_id = m.id`)
}

func (r *Repository) ListModel(ctx context.Context) ([]domain.ModelView, error) {
	var items []domain.ModelView
	err := r.modelView(ctx).Order("m.komoditas_kode, m.dilatih_pada DESC, m.versi DESC").Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list model: %w", err)
	}
	return items, nil
}

func (r *Repository) GetModelView(ctx context.Context, id idx.ID) (*domain.ModelView, error) {
	var item domain.ModelView
	res := r.modelView(ctx).Where("m.id = ?", id).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("get model: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) SeriKonsumen(ctx context.Context, kode string, dari time.Time) ([]domain.SeriHarian, error) {
	var rows []struct {
		KabupatenKode string    `gorm:"column:kabupaten_kode"`
		Tanggal       time.Time `gorm:"column:tanggal"`
		Harga         int       `gorm:"column:harga"`
	}
	err := r.db.WithContext(ctx).Table("entri_harga AS e").
		Select("ps.kabupaten_kode, e.dicatat_pada::date AS tanggal, ROUND(AVG(e.harga))::int AS harga").
		Joins("JOIN pasar ps ON (ps.kode = e.pasar_id OR ps.id::text = e.pasar_id)").
		Where("e.komoditas_kode = ? AND e.tipe = 'KONSUMEN' AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')", kode).
		Where("e.dicatat_pada >= ?::date", dari.Format(tanggalSQL)).
		Group("ps.kabupaten_kode, e.dicatat_pada::date").Order("ps.kabupaten_kode, tanggal").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("seri harga konsumen: %w", err)
	}

	var seri []domain.SeriHarian
	for _, row := range rows {
		if len(seri) == 0 || seri[len(seri)-1].KabupatenKode != row.KabupatenKode {
			seri = append(seri, domain.SeriHarian{KabupatenKode: row.KabupatenKode})
		}
		s := &seri[len(seri)-1]
		s.Titik = append(s.Titik, domain.TitikHarga{Tanggal: row.Tanggal, Harga: row.Harga})
	}
	return seri, nil
}

func (r *Repository) SimpanModel(ctx context.Context, m *domain.ModelRegistry, proyeksi []domain.PrediksiHarga) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrModelSudahAda
			}
			return fmt.Errorf("simpan model: %w", err)
		}
		// ID model baru diketahui setelah INSERT; proyeksi ditautkan ke sana dan versinya ditulis konsisten.
		for i := range proyeksi {
			proyeksi[i].ModelID, proyeksi[i].ModelVersi = m.ID, m.Versi
		}
		return upsertProyeksi(tx, proyeksi)
	})
}

func (r *Repository) SimpanProyeksi(ctx context.Context, proyeksi []domain.PrediksiHarga) error {
	return upsertProyeksi(r.db.WithContext(ctx), proyeksi)
}

// upsertProyeksi menulis proyeksi dengan kunci alami (komoditas, kabupaten, tanggal target, horizon, versi model):
// menulis ulang proyeksi yang sama memperbarui barisnya, bukan menambah. ID dan kode tidak ikut diubah.
func upsertProyeksi(db *gorm.DB, proyeksi []domain.PrediksiHarga) error {
	if len(proyeksi) == 0 {
		return nil
	}
	for _, p := range proyeksi {
		if p.ModelID == 0 || p.ModelVersi == "" {
			return fmt.Errorf("simpan proyeksi: proyeksi %s/%s horizon %d tanpa model", p.KomoditasKode, p.KabupatenKode, p.HorizonHari)
		}
	}
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "komoditas_kode"}, {Name: "kabupaten_kode"}, {Name: "tanggal_target"}, {Name: "horizon_hari"}, {Name: "model_versi"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"nilai", "bawah", "atas", "harga_dasar", "faktor", "sumber", "dibuat_pada", "model_id"}),
	}).CreateInBatches(&proyeksi, 200).Error
	if err != nil {
		return fmt.Errorf("simpan proyeksi: %w", err)
	}
	return nil
}

func (r *Repository) ModelProduksi(ctx context.Context, kode string) (*domain.ModelRegistry, error) {
	var m domain.ModelRegistry
	err := r.db.WithContext(ctx).Where("komoditas_kode = ? AND status = ?", kode, domain.StatusProduksi).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) EvaluasiJatuhTempo(ctx context.Context, hariIni time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Exec(`
		INSERT INTO prediksi_evaluasi (prediksi_id, realisasi, galat_pct)
		SELECT p.id, a.harga, ROUND((p.nilai - a.harga)::numeric / a.harga * 100, 2)
		FROM prediksi_harga p
		JOIN LATERAL (
			SELECT ROUND(AVG(e.harga))::int AS harga
			FROM entri_harga e JOIN pasar ps ON (ps.kode = e.pasar_id OR ps.id::text = e.pasar_id)
			WHERE e.komoditas_kode = p.komoditas_kode AND ps.kabupaten_kode = p.kabupaten_kode
			  AND e.tipe = 'KONSUMEN' AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
			  AND e.dicatat_pada::date = p.tanggal_target
		) a ON a.harga IS NOT NULL
		WHERE p.sumber = 'MODEL' AND p.tanggal_target < ?::date
		ON CONFLICT (prediksi_id) DO NOTHING`, hariIni.Format(tanggalSQL))
	if res.Error != nil {
		return 0, fmt.Errorf("evaluasi proyeksi: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *Repository) GetModel(ctx context.Context, ref idx.RujukanID) (*domain.ModelRegistry, error) {
	var m domain.ModelRegistry
	err := r.db.WithContext(ctx).Where(ref.Kondisi("id", "kode")).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrModelTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get model: %w", err)
	}
	return &m, nil
}

// kunciModel membaca satu model (menurut ID atau kode) dengan kunci baris (FOR UPDATE) di dalam transaksi.
func kunciModel(tx *gorm.DB, ref idx.RujukanID) (*domain.ModelRegistry, error) {
	var m domain.ModelRegistry
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(ref.Kondisi("id", "kode")).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrModelTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("kunci model: %w", err)
	}
	return &m, nil
}

func ubahStatus(tx *gorm.DB, id idx.ID, status string, kolom map[string]any) error {
	kolom["status"] = status
	if err := tx.Model(&domain.ModelRegistry{}).Where("id = ?", id).Updates(kolom).Error; err != nil {
		return fmt.Errorf("ubah status model: %w", err)
	}
	return nil
}

// catatPromosi menambah riwayat promosi; ID BIGINT diberikan basis data dan kode dibiarkan kosong.
func catatPromosi(tx *gorm.DB, p domain.ModelPromosi) error {
	if err := tx.Create(&p).Error; err != nil {
		return fmt.Errorf("catat riwayat model: %w", err)
	}
	return nil
}

func (r *Repository) Promosikan(ctx context.Context, ref idx.RujukanID, oleh string, waktu time.Time) (*domain.HasilPromosi, error) {
	var hasil domain.HasilPromosi
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		m, err := kunciModel(tx, ref)
		if err != nil {
			return err
		}
		if m.Status == domain.StatusProduksi {
			return domain.ErrStatusModelTidakValid
		}
		var proyeksi int64
		err = tx.Model(&domain.PrediksiHarga{}).Where("model_id = ?", m.ID).Count(&proyeksi).Error
		if err != nil {
			return fmt.Errorf("hitung proyeksi model: %w", err)
		}
		if proyeksi == 0 {
			return domain.ErrModelTanpaProyeksi
		}

		// Arsipkan dulu model produksi lama: indeks unik hanya mengizinkan satu PRODUKSI per komoditas.
		var lama []domain.ModelRegistry
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("komoditas_kode = ? AND status = ?", m.KomoditasKode, domain.StatusProduksi).Find(&lama).Error
		if err != nil {
			return fmt.Errorf("cari model produksi: %w", err)
		}
		promosi := domain.ModelPromosi{
			ModelID: m.ID, KomoditasKode: m.KomoditasKode, Aksi: domain.AksiPromosi,
			StatusLama: m.Status, StatusBaru: domain.StatusProduksi, Oleh: oleh, Waktu: waktu,
		}
		if len(lama) > 0 {
			if err := ubahStatus(tx, lama[0].ID, domain.StatusArsip, map[string]any{}); err != nil {
				return err
			}
			lama[0].Status = domain.StatusArsip
			hasil.Digantikan = &lama[0]
			promosi.ModelDigantikanID = &lama[0].ID
		}

		dariStatus := m.Status
		err = ubahStatus(tx, m.ID, domain.StatusProduksi, map[string]any{
			"deployed_at": waktu, "dipromosikan_oleh": oleh, "dipromosikan_pada": waktu,
		})
		if err != nil {
			return err
		}
		m.Status, m.DipromosikanOleh, m.DipromosikanPada, m.DeployedAt = domain.StatusProduksi, &oleh, &waktu, &waktu
		hasil.Model, hasil.DariStatus = *m, dariStatus
		return catatPromosi(tx, promosi)
	})
	if err != nil {
		return nil, err
	}
	return &hasil, nil
}

func (r *Repository) Arsipkan(ctx context.Context, ref idx.RujukanID, oleh string, waktu time.Time) (*domain.ModelRegistry, error) {
	var m *domain.ModelRegistry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if m, err = kunciModel(tx, ref); err != nil {
			return err
		}
		if m.Status != domain.StatusKandidat {
			return domain.ErrStatusModelTidakValid
		}
		if err := ubahStatus(tx, m.ID, domain.StatusArsip, map[string]any{}); err != nil {
			return err
		}
		m.Status = domain.StatusArsip
		return catatPromosi(tx, domain.ModelPromosi{
			ModelID: m.ID, KomoditasKode: m.KomoditasKode, Aksi: domain.AksiArsip,
			StatusLama: domain.StatusKandidat, StatusBaru: domain.StatusArsip, Oleh: oleh, Waktu: waktu,
		})
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *Repository) SimpanSkenario(ctx context.Context, s *domain.SkenarioSimulasi) error {
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("simpan skenario: %w", err)
	}
	return nil
}
