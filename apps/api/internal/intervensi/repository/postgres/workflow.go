package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"gorm.io/gorm"
)

var _ domain.WorkflowRepository = (*Repository)(nil)

func (r *Repository) ListAlur(ctx context.Context) ([]domain.Alur, error) {
	var alur []domain.AlurPersetujuan
	if err := r.db.WithContext(ctx).Order("jenis, siklus, tahap_tujuan, kode").Find(&alur).Error; err != nil {
		return nil, fmt.Errorf("daftar alur: %w", err)
	}
	var langkah []domain.AlurLangkah
	if err := r.db.WithContext(ctx).Order("alur_kode, urutan").Find(&langkah).Error; err != nil {
		return nil, fmt.Errorf("langkah alur: %w", err)
	}
	out := make([]domain.Alur, len(alur))
	index := make(map[string]int, len(alur))
	for i, a := range alur {
		out[i] = domain.Alur{AlurPersetujuan: a}
		index[a.Kode] = i
	}
	for _, l := range langkah {
		if i, ok := index[l.AlurKode]; ok {
			out[i].Langkah = append(out[i].Langkah, l)
		}
	}
	return out, nil
}

// rantai adalah satu langkah hasil menggabungkan alur-alur yang dilalui sebuah permintaan.
type rantai struct {
	AlurKode       string
	Label          string
	IzinDibutuhkan string
}

// susunRantai menentukan langkah persetujuan sebuah permintaan. Pemindahan melewati alur aktif
// tiap tahap persetujuan dari tahap kartu (tidak termasuk) sampai tahap tujuan (termasuk), berurutan.
func susunRantai(tx *gorm.DB, kartuID string, in domain.AjukanInput) ([]rantai, error) {
	if in.Jenis == domain.JenisEskalasi {
		var langkah []rantai
		err := tx.Raw(`
			SELECT a.kode AS alur_kode, l.label, l.izin_dibutuhkan
			  FROM alur_persetujuan a JOIN alur_persetujuan_langkah l ON l.alur_kode = a.kode
			 WHERE a.aktif AND a.jenis = 'ESKALASI' ORDER BY l.urutan`).Scan(&langkah).Error
		if err != nil {
			return nil, fmt.Errorf("alur eskalasi: %w", err)
		}
		if len(langkah) == 0 {
			return nil, domain.ErrTanpaAlur
		}
		return langkah, nil
	}

	var stage struct {
		Siklus string
		Asal   int
		Tujuan *int
	}
	res := tx.Raw(`
		SELECT i.siklus,
		       (SELECT urutan FROM tahap_intervensi_def WHERE siklus = i.siklus AND kode_tahap = i.tahap) AS asal,
		       (SELECT urutan FROM tahap_intervensi_def WHERE siklus = i.siklus AND kode_tahap = ?) AS tujuan
		  FROM intervensi i WHERE i.id = ?`, in.TahapTujuan, kartuID).Scan(&stage)
	if res.Error != nil {
		return nil, fmt.Errorf("tahap kartu: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	if stage.Tujuan == nil {
		return nil, domain.ErrTahapTidakValid
	}
	if *stage.Tujuan <= stage.Asal {
		return nil, domain.ErrTanpaPersetujuan
	}

	var langkah []rantai
	err := tx.Raw(`
		SELECT a.kode AS alur_kode, l.label, l.izin_dibutuhkan
		  FROM alur_persetujuan a
		  JOIN alur_persetujuan_langkah l ON l.alur_kode = a.kode
		  JOIN tahap_intervensi_def d ON d.siklus = a.siklus AND d.kode_tahap = a.tahap_tujuan
		 WHERE a.aktif AND a.jenis = 'PINDAH_TAHAP' AND a.siklus = ? AND d.urutan > ? AND d.urutan <= ?
		 ORDER BY d.urutan, l.urutan`, stage.Siklus, stage.Asal, *stage.Tujuan).Scan(&langkah).Error
	if err != nil {
		return nil, fmt.Errorf("alur pemindahan: %w", err)
	}
	if len(langkah) == 0 {
		return nil, domain.ErrTanpaPersetujuan
	}
	return langkah, nil
}

// bukaPersetujuan membuat permintaan persetujuan untuk kartu yang sudah terkunci dalam tx dan
// mengembalikan id-nya. oleh kosong berarti sistem.
func bukaPersetujuan(tx *gorm.DB, in domain.AjukanInput) (int64, error) {
	var terbuka bool
	err := tx.Raw("SELECT EXISTS (SELECT 1 FROM intervensi_persetujuan WHERE intervensi_id = ? AND status = 'MENUNGGU')", in.KartuID).Scan(&terbuka).Error
	if err != nil {
		return 0, fmt.Errorf("periksa permintaan terbuka: %w", err)
	}
	if terbuka {
		return 0, domain.ErrPersetujuanTerbuka
	}
	langkah, err := susunRantai(tx, in.KartuID, in)
	if err != nil {
		return 0, err
	}

	var tujuan, pengaju *string
	if in.TahapTujuan != "" {
		tujuan = &in.TahapTujuan
	}
	if in.Oleh != "" {
		pengaju = &in.Oleh
	}
	var id int64
	err = tx.Raw(`
		INSERT INTO intervensi_persetujuan (intervensi_id, alur_kode, jenis, tahap_tujuan, diajukan_oleh, catatan)
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		in.KartuID, langkah[len(langkah)-1].AlurKode, in.Jenis, tujuan, pengaju, in.Catatan).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("simpan permintaan persetujuan: %w", err)
	}
	rows := make([]domain.PersetujuanLangkah, len(langkah))
	for i, l := range langkah {
		rows[i] = domain.PersetujuanLangkah{PersetujuanID: id, Urutan: i + 1, Label: l.Label, IzinDibutuhkan: l.IzinDibutuhkan}
	}
	if err := tx.Create(&rows).Error; err != nil {
		return 0, fmt.Errorf("simpan langkah persetujuan: %w", err)
	}
	return id, catatRiwayat(tx, in.KartuID, domain.AksiAjukan, nil, tujuan, in.Oleh, in.Catatan)
}

func (r *Repository) Ajukan(ctx context.Context, in domain.AjukanInput) (*domain.Persetujuan, error) {
	var id int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		kartu, err := kunciKartu(tx, in.KartuID)
		if err != nil {
			return err
		}
		if kartu.Selesai {
			return domain.ErrKartuSelesai
		}
		id, err = bukaPersetujuan(tx, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetPersetujuan(ctx, id)
}

// persetujuanQuery menggabungkan ringkasan kartu dan nama pengaju.
func persetujuanQuery(db *gorm.DB) *gorm.DB {
	return db.Table("intervensi_persetujuan AS p").
		Select("p.*, i.judul AS kartu_judul, i.tahap AS kartu_tahap, u.nama AS diajukan_oleh_nama").
		Joins("JOIN intervensi i ON i.id = p.intervensi_id").
		Joins("LEFT JOIN pengguna u ON u.id = p.diajukan_oleh")
}

// isiLangkah memuat rantai langkah semua permintaan sekaligus.
func isiLangkah(db *gorm.DB, items []domain.Persetujuan) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	index := make(map[int64]int, len(items))
	for i, p := range items {
		ids[i] = p.ID
		index[p.ID] = i
	}
	var langkah []domain.LangkahPersetujuan
	err := db.Table("intervensi_persetujuan_langkah AS l").
		Select("l.*, u.nama AS oleh_nama").
		Joins("LEFT JOIN pengguna u ON u.id = l.oleh").
		Where("l.persetujuan_id IN ?", ids).Order("l.persetujuan_id, l.urutan").Scan(&langkah).Error
	if err != nil {
		return fmt.Errorf("langkah persetujuan: %w", err)
	}
	for _, l := range langkah {
		i := index[l.PersetujuanID]
		items[i].Langkah = append(items[i].Langkah, l)
	}
	return nil
}

func (r *Repository) GetPersetujuan(ctx context.Context, id int64) (*domain.Persetujuan, error) {
	return getPersetujuan(r.db.WithContext(ctx), id)
}

func getPersetujuan(db *gorm.DB, id int64) (*domain.Persetujuan, error) {
	var items []domain.Persetujuan
	if err := persetujuanQuery(db).Where("p.id = ?", id).Limit(1).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("ambil persetujuan: %w", err)
	}
	if len(items) == 0 {
		return nil, domain.ErrPersetujuanNotFound
	}
	if err := isiLangkah(db, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// batasDaftarPersetujuan membatasi satu daftar permintaan.
const batasDaftarPersetujuan = 200

func (r *Repository) ListPersetujuan(ctx context.Context, status string) ([]domain.Persetujuan, error) {
	db := r.db.WithContext(ctx)
	q := persetujuanQuery(db)
	if status != "" {
		q = q.Where("p.status = ?", status)
	}
	var items []domain.Persetujuan
	if err := q.Order("p.diajukan_pada DESC, p.id DESC").Limit(batasDaftarPersetujuan).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar persetujuan: %w", err)
	}
	if err := isiLangkah(db, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) Putus(ctx context.Context, in domain.PutusInput) (*domain.Persetujuan, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Kunci kartu lebih dulu (urutan kunci sama dengan Ajukan), baru permintaannya.
		var kartuID string
		res := tx.Raw("SELECT intervensi_id FROM intervensi_persetujuan WHERE id = ?", in.PersetujuanID).Scan(&kartuID)
		if res.Error != nil {
			return fmt.Errorf("cari permintaan: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrPersetujuanNotFound
		}
		kartu, err := kunciKartu(tx, kartuID)
		if err != nil {
			return err
		}

		var p domain.IntervensiPersetujuan
		if err := tx.Raw("SELECT * FROM intervensi_persetujuan WHERE id = ? FOR UPDATE", in.PersetujuanID).Scan(&p).Error; err != nil {
			return fmt.Errorf("kunci permintaan: %w", err)
		}
		if p.Status != domain.PersetujuanMenunggu || p.LangkahKini != in.LangkahKini {
			return domain.ErrPersetujuanSelesai
		}

		err = tx.Exec(`
			UPDATE intervensi_persetujuan_langkah
			   SET keputusan = ?, oleh = ?, waktu = NOW(), catatan = ?
			 WHERE persetujuan_id = ? AND urutan = ?`, in.Keputusan, in.Oleh, in.Catatan, p.ID, p.LangkahKini).Error
		if err != nil {
			return fmt.Errorf("simpan keputusan: %w", err)
		}
		var jumlahLangkah int
		if err := tx.Raw("SELECT COUNT(*) FROM intervensi_persetujuan_langkah WHERE persetujuan_id = ?", p.ID).Scan(&jumlahLangkah).Error; err != nil {
			return fmt.Errorf("hitung langkah: %w", err)
		}

		switch {
		case in.Keputusan == domain.KeputusanTolak:
			return tolakPersetujuan(tx, p, in)
		case p.LangkahKini < jumlahLangkah:
			err := tx.Exec("UPDATE intervensi_persetujuan SET langkah_kini = langkah_kini + 1 WHERE id = ?", p.ID).Error
			if err != nil {
				return fmt.Errorf("lanjut ke langkah berikutnya: %w", err)
			}
			return catatRiwayat(tx, p.IntervensiID, domain.AksiSetuju, nil, p.TahapTujuan, in.Oleh, in.Catatan)
		default:
			return selesaikanPersetujuan(tx, p, kartu, in)
		}
	})
	if err != nil {
		return nil, err
	}
	return r.GetPersetujuan(ctx, in.PersetujuanID)
}

// tolakPersetujuan menutup permintaan sebagai DITOLAK. Menolak eskalasi mencabut tanda eskalasi kartu.
func tolakPersetujuan(tx *gorm.DB, p domain.IntervensiPersetujuan, in domain.PutusInput) error {
	err := tx.Exec("UPDATE intervensi_persetujuan SET status = 'DITOLAK', selesai_pada = NOW() WHERE id = ?", p.ID).Error
	if err != nil {
		return fmt.Errorf("tolak permintaan: %w", err)
	}
	if p.Jenis == domain.JenisEskalasi {
		err := tx.Exec(`UPDATE intervensi SET dieskalasi_pada = NULL, dieskalasi_oleh_id = NULL, versi = versi + 1 WHERE id = ?`, p.IntervensiID).Error
		if err != nil {
			return fmt.Errorf("cabut eskalasi: %w", err)
		}
	}
	return catatRiwayat(tx, p.IntervensiID, domain.AksiTolak, nil, p.TahapTujuan, in.Oleh, in.Catatan)
}

// selesaikanPersetujuan menutup permintaan sebagai DISETUJUI setelah langkah terakhir; permintaan
// pemindahan memindahkan kartu ke tahap tujuan (di akhir kolom) atas nama pemutus terakhir.
func selesaikanPersetujuan(tx *gorm.DB, p domain.IntervensiPersetujuan, kartu posisiKartu, in domain.PutusInput) error {
	err := tx.Exec("UPDATE intervensi_persetujuan SET status = 'DISETUJUI', selesai_pada = NOW() WHERE id = ?", p.ID).Error
	if err != nil {
		return fmt.Errorf("setujui permintaan: %w", err)
	}
	if err := catatRiwayat(tx, p.IntervensiID, domain.AksiSetuju, nil, p.TahapTujuan, in.Oleh, in.Catatan); err != nil {
		return err
	}
	if p.Jenis != domain.JenisPindahTahap || p.TahapTujuan == nil || kartu.Tahap == *p.TahapTujuan {
		return nil
	}
	if kartu.Selesai {
		return domain.ErrKartuSelesai
	}
	var penutup bool
	err = tx.Raw(`
		SELECT urutan = (SELECT MAX(urutan) FROM tahap_intervensi_def WHERE siklus = ?)
		  FROM tahap_intervensi_def WHERE siklus = ? AND kode_tahap = ?`, kartu.Siklus, kartu.Siklus, *p.TahapTujuan).Scan(&penutup).Error
	if err != nil {
		return fmt.Errorf("periksa tahap penutup: %w", err)
	}
	alasan := fmt.Sprintf("Disetujui melalui permintaan #%d", p.ID)
	_, err = pindahKartu(tx, p.IntervensiID, kartu, *p.TahapTujuan, math.MaxInt32, penutup, in.Oleh, alasan)
	return err
}

func (r *Repository) KartuMelewatiSLA(ctx context.Context, now time.Time) ([]domain.KartuMelewatiSLA, error) {
	var out []domain.KartuMelewatiSLA
	err := r.db.WithContext(ctx).Raw(`
		SELECT i.id, i.tahap AS tahap_kode, d.label, d.sla_hari
		  FROM intervensi i
		  JOIN tahap_intervensi_def d ON d.siklus = i.siklus AND d.kode_tahap = i.tahap
		 WHERE i.selesai_pada IS NULL AND i.dieskalasi_pada IS NULL AND d.sla_hari IS NOT NULL
		   AND i.tahap_sejak + d.sla_hari * INTERVAL '1 day' < @now
		 ORDER BY i.tahap_sejak, i.id`, map[string]any{"now": now}).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("kartu melewati SLA: %w", err)
	}
	return out, nil
}

func (r *Repository) EskalasiSLA(ctx context.Context, id, alasan string) (bool, error) {
	var eskalasi bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		kartu, err := kunciKartu(tx, id)
		if err != nil {
			return err
		}
		if kartu.Selesai || kartu.Dieskalasi {
			return nil
		}
		eskalasi = true
		return eskalasiKartu(tx, id, "", alasan)
	})
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	return eskalasi, err
}

func (r *Repository) BuatKartuOtomatis(ctx context.Context, in domain.KartuOtomatis) (*domain.Intervensi, error) {
	var id string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ada bool
		if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM intervensi WHERE ews_alert_id = ?)", in.AlertID).Scan(&ada).Error; err != nil {
			return fmt.Errorf("periksa kartu alert: %w", err)
		}
		if ada {
			return nil
		}
		var awal struct {
			KodeTahap string
			SLAHari   *int
		}
		res := tx.Raw("SELECT kode_tahap, sla_hari FROM tahap_intervensi_def WHERE siklus = ? ORDER BY urutan LIMIT 1", in.Siklus).Scan(&awal)
		if res.Error != nil {
			return fmt.Errorf("tahap awal: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("siklus %s tidak punya tahap: %w", in.Siklus, domain.ErrTahapTidakValid)
		}
		hari := in.HariTenggatCadangan
		if awal.SLAHari != nil {
			hari = *awal.SLAHari
		}

		var ids []string
		err := tx.Raw(`
			INSERT INTO intervensi (id, siklus, tahap, urutan, judul, deskripsi, komoditas_kode, kabupaten_kode, prioritas,
			                        tenggat, rekomendasi_ai, ews_alert_id, dibuat_oleh_id, dibuat_pada, tahap_sejak)
			SELECT 'INT-' || nextval('intervensi_id_seq')::text, CAST(@siklus AS siklus), @tahap,
			       COALESCE((SELECT MAX(urutan) + 1 FROM intervensi WHERE siklus = CAST(@siklus AS siklus) AND tahap = @tahap), 0),
			       @judul, @deskripsi, @komoditas, @kabupaten, CAST(@prioritas AS prioritas),
			       @tenggat, false, @alert, NULL, @now, @now
			ON CONFLICT (ews_alert_id) WHERE dibuat_oleh_id IS NULL AND ews_alert_id IS NOT NULL DO NOTHING
			RETURNING id`, map[string]any{
			"siklus": in.Siklus, "tahap": awal.KodeTahap, "judul": in.Judul, "deskripsi": in.Deskripsi,
			"komoditas": in.KomoditasKode, "kabupaten": in.KabupatenKode, "prioritas": in.Prioritas,
			"tenggat": in.Sekarang.AddDate(0, 0, hari), "alert": in.AlertID, "now": in.Sekarang,
		}).Scan(&ids).Error
		if err != nil {
			return fmt.Errorf("simpan kartu otomatis: %w", err)
		}
		if len(ids) == 0 { // instance lain membuatnya lebih dulu
			return nil
		}
		id = ids[0]

		if len(in.PeranPenanggungJawab) > 0 {
			err := tx.Exec(`
				INSERT INTO intervensi_pj (intervensi_id, pengguna_id)
				SELECT ?, id FROM pengguna WHERE status = 'AKTIF' AND peran::text IN ?
				ON CONFLICT DO NOTHING`, id, in.PeranPenanggungJawab).Error
			if err != nil {
				return fmt.Errorf("tugaskan penanggung jawab otomatis: %w", err)
			}
		}
		return catatRiwayat(tx, id, domain.AksiBuat, nil, &awal.KodeTahap, "", "Dibuat otomatis dari alert "+in.AlertID)
	})
	if err != nil || id == "" {
		return nil, err
	}
	return r.GetByID(ctx, id)
}
