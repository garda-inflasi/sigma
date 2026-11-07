// Package postgres adalah repository intervensi di atas PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// pgForeignKeyViolation adalah SQLSTATE pelanggaran kunci asing.
const pgForeignKeyViolation = "23503"

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// kartuRow adalah baris daftar: kartu beserta kode tampilan alert EWS pemicunya.
type kartuRow struct {
	domain.Intervensi `gorm:"embedded"`
	EwsAlertKode      *string `gorm:"column:ews_alert_kode;->"`
}

func (r *Repository) List(ctx context.Context, q domain.ListQuery) ([]domain.Kartu, error) {
	query := r.db.WithContext(ctx).Table("intervensi AS i").
		Select("i.*, a.kode AS ews_alert_kode").
		Joins("LEFT JOIN ews_alert a ON a.id = i.ews_alert_id")
	if q.Siklus != "" {
		query = query.Where("i.siklus = ?", q.Siklus)
	}
	if q.Tahap != "" {
		query = query.Where("i.tahap = ?", q.Tahap)
	}
	if q.Prioritas != "" {
		query = query.Where("i.prioritas = ?", q.Prioritas)
	}

	var items []kartuRow
	if err := query.Order("i.siklus, i.tahap, i.urutan, i.dibuat_pada, i.id").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar intervensi: %w", err)
	}
	cards := make([]domain.Kartu, len(items))
	if len(items) == 0 {
		return cards, nil
	}

	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i, it := range items {
		cards[i].Intervensi, cards[i].EwsAlertKode = it.Intervensi, it.EwsAlertKode
		ids[i] = it.ID
		index[it.ID] = i
	}

	var pj []struct {
		IntervensiID string
		PenggunaID   string
		Nama         string
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT p.intervensi_id, u.id AS pengguna_id, u.nama
		  FROM intervensi_pj p
		  JOIN pengguna u ON u.id = p.pengguna_id
		 WHERE p.intervensi_id IN ?
		 ORDER BY u.nama, u.id`, ids).Scan(&pj).Error
	if err != nil {
		return nil, fmt.Errorf("penanggung jawab intervensi: %w", err)
	}
	for _, row := range pj {
		i := index[row.IntervensiID]
		cards[i].PenanggungJawab = append(cards[i].PenanggungJawab, domain.PenanggungJawab{ID: row.PenggunaID, Nama: row.Nama})
	}
	return cards, nil
}

func (r *Repository) ListTahap(ctx context.Context) ([]domain.TahapDef, error) {
	var defs []domain.TahapDef
	if err := r.db.WithContext(ctx).Order("siklus, urutan").Find(&defs).Error; err != nil {
		return nil, fmt.Errorf("daftar tahap intervensi: %w", err)
	}
	return defs, nil
}

func (r *Repository) GetByID(ctx context.Context, ref idx.RujukanUUID) (*domain.Intervensi, error) {
	var item domain.Intervensi
	if err := r.db.WithContext(ctx).Where(ref.Kondisi("id", "kode")).Take(&item).Error; err != nil {
		return nil, fmt.Errorf("ambil intervensi: %w", err)
	}
	return &item, nil
}

func (r *Repository) Create(ctx context.Context, in domain.NewIntervensi, tahap string) (*domain.Intervensi, error) {
	var created domain.Intervensi
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// ID (UUID) dibuat basis data. Kode tampilan INT-n berasal dari sequence khusus kode, jadi tidak bisa
		// bentrok antar-instance.
		var seq int64
		if err := tx.Raw("SELECT nextval('intervensi_kode_seq')").Scan(&seq).Error; err != nil {
			return fmt.Errorf("nomor intervensi: %w", err)
		}
		kode := fmt.Sprintf("INT-%d", seq)
		var urutan int
		err := tx.Raw("SELECT COALESCE(MAX(urutan) + 1, 0) FROM intervensi WHERE siklus = ? AND tahap = ?", in.Siklus, tahap).Scan(&urutan).Error
		if err != nil {
			return fmt.Errorf("urutan intervensi: %w", err)
		}

		created = domain.Intervensi{
			Kode: &kode, Siklus: in.Siklus, Tahap: tahap, Urutan: urutan, Judul: in.Judul,
			Deskripsi: in.Deskripsi, KomoditasKode: in.KomoditasKode, KabupatenKode: in.KabupatenKode,
			Prioritas: in.Prioritas, Tenggat: in.Tenggat, EwsAlertID: in.EwsAlertID, DibuatOlehID: idx.PtrAtauNil(in.DibuatOlehID),
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := catatRiwayat(tx, created.ID, domain.AksiBuat, nil, &tahap, in.DibuatOlehID, ""); err != nil {
			return err
		}
		var pj []domain.IntervensiPJ
		for _, id := range uniqueStrings(in.PenanggungJawabIDs) {
			pj = append(pj, domain.IntervensiPJ{IntervensiID: created.ID, PenggunaID: id})
		}
		if len(pj) > 0 {
			return tx.Create(&pj).Error
		}
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return nil, domain.ErrInvalidReference
		}
		return nil, fmt.Errorf("buat intervensi: %w", err)
	}
	return &created, nil
}

func (r *Repository) Move(ctx context.Context, id, tahap string, index int, selesai bool, versi int, oleh string) (int, error) {
	var versiBaru int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		kini, err := kunciKartu(tx, id)
		if err != nil {
			return err
		}
		if kini.Versi != versi {
			return domain.ErrVersiUsang
		}
		versiBaru, err = pindahKartu(tx, id, kini, tahap, index, selesai, oleh, "")
		return err
	})
	if err != nil {
		return 0, err
	}
	return versiBaru, nil
}

// posisiKartu adalah keadaan kartu yang terkunci dalam transaksi.
type posisiKartu struct {
	Siklus     string
	Tahap      string
	Versi      int
	Selesai    bool
	Dieskalasi bool
}

// kunciKartu mengunci baris kartu (domain.ErrNotFound bila tidak ada).
func kunciKartu(tx *gorm.DB, id string) (posisiKartu, error) {
	var kini posisiKartu
	res := tx.Raw(`SELECT siklus, tahap, versi, selesai_pada IS NOT NULL AS selesai,
		       dieskalasi_pada IS NOT NULL AS dieskalasi
		  FROM intervensi WHERE id = ? FOR UPDATE`, id).Scan(&kini)
	if res.Error != nil {
		return kini, fmt.Errorf("kunci intervensi: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return kini, domain.ErrNotFound
	}
	return kini, nil
}

// pindahKartu memindahkan kartu yang sudah terkunci ke tahap dan posisi baru, menomori ulang
// kolom tujuan, menaikkan versi, dan mencatat riwayat bila tahap berganti. Mengembalikan versi baru.
func pindahKartu(tx *gorm.DB, id string, kini posisiKartu, tahap string, index int, selesai bool, oleh, alasan string) (int, error) {
	var column []string
	err := tx.Raw(`
		SELECT id FROM intervensi
		 WHERE siklus = ? AND tahap = ? AND id <> ?
		 ORDER BY urutan, dibuat_pada, id
		   FOR UPDATE`, kini.Siklus, tahap, id).Scan(&column).Error
	if err != nil {
		return 0, fmt.Errorf("kolom tujuan: %w", err)
	}
	index = max(0, min(index, len(column)))
	column = slices.Insert(column, index, id)

	var versiBaru int
	err = tx.Raw(`
		UPDATE intervensi
		   SET tahap_sejak = CASE WHEN tahap <> @tahap THEN NOW() ELSE tahap_sejak END,
		       tahap = @tahap, urutan = @urutan, versi = versi + 1,
		       selesai_pada = CASE WHEN CAST(@selesai AS boolean) THEN COALESCE(selesai_pada, NOW()) ELSE NULL END
		 WHERE id = @id
		RETURNING versi`, map[string]any{"tahap": tahap, "urutan": index, "selesai": selesai, "id": id}).Scan(&versiBaru).Error
	if err != nil {
		return 0, fmt.Errorf("pindahkan kartu: %w", err)
	}
	for pos, other := range column {
		if other == id {
			continue
		}
		if err := tx.Exec("UPDATE intervensi SET urutan = ? WHERE id = ? AND urutan <> ?", pos, other, pos).Error; err != nil {
			return 0, fmt.Errorf("nomori ulang kolom: %w", err)
		}
	}
	if kini.Tahap != tahap {
		if err := catatRiwayat(tx, id, domain.AksiPindahTahap, &kini.Tahap, &tahap, oleh, alasan); err != nil {
			return 0, err
		}
	}
	return versiBaru, nil
}

// catatRiwayat menambah satu peristiwa pada riwayat kartu dalam transaksi tx;
// oleh kosong berarti sistem (disimpan NULL).
func catatRiwayat(tx *gorm.DB, id, aksi string, dari, ke *string, oleh, alasan string) error {
	var pelaku *string
	if oleh != "" {
		pelaku = &oleh
	}
	err := tx.Exec(`
		INSERT INTO intervensi_riwayat (intervensi_id, aksi, dari, ke, oleh, alasan)
		VALUES (?, ?, ?, ?, ?, ?)`, id, aksi, dari, ke, pelaku, alasan).Error
	if err != nil {
		return fmt.Errorf("riwayat intervensi: %w", err)
	}
	return nil
}

func (r *Repository) BulkAksi(ctx context.Context, aksi domain.BulkAksi, ids []string, oleh string) ([]domain.HasilKartu, error) {
	hasil := make([]domain.HasilKartu, len(ids))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			alasan, err := terapkanAksi(tx, aksi, id, oleh)
			if err != nil {
				return err
			}
			hasil[i] = domain.HasilKartu{ID: id, Berhasil: alasan == "", Alasan: alasan}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("aksi massal %s: %w", aksi, err)
	}
	return hasil, nil
}

// terapkanAksi menerapkan aksi pada satu kartu. Alasan tidak kosong bila kartu tak dapat
// diproses (bukan galat sistem); kartu seperti itu dibiarkan utuh.
func terapkanAksi(tx *gorm.DB, aksi domain.BulkAksi, id, oleh string) (alasan string, err error) {
	kartu, err := kunciKartu(tx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Kartu tidak ditemukan", nil
	case err != nil:
		return "", err
	case kartu.Selesai:
		return "Kartu sudah selesai", nil
	}

	switch aksi {
	case domain.BulkTugaskan:
		ins := tx.Exec("INSERT INTO intervensi_pj (intervensi_id, pengguna_id) VALUES (?, ?) ON CONFLICT DO NOTHING", id, oleh)
		if ins.Error != nil {
			return "", fmt.Errorf("tugaskan: %w", ins.Error)
		}
		if ins.RowsAffected == 0 {
			return "Anda sudah menjadi penanggung jawab", nil
		}
		if err := tx.Exec("UPDATE intervensi SET versi = versi + 1 WHERE id = ?", id).Error; err != nil {
			return "", fmt.Errorf("naikkan versi: %w", err)
		}
		return "", catatRiwayat(tx, id, domain.AksiTugaskan, nil, nil, oleh, "")
	case domain.BulkEskalasi:
		if kartu.Dieskalasi {
			return "Kartu sudah dieskalasi", nil
		}
		return "", eskalasiKartu(tx, id, oleh, "Dieskalasi ke Dewan Pengarah")
	}
	return "", fmt.Errorf("aksi %q tidak dikenal", aksi)
}

// eskalasiKartu menandai kartu terkunci dieskalasi (prioritas TINGGI, versi naik), mencatat riwayat,
// dan membuka permintaan persetujuan eskalasi bila ada alur aktifnya. oleh kosong = sistem.
func eskalasiKartu(tx *gorm.DB, id, oleh, alasan string) error {
	var pelaku *string
	if oleh != "" {
		pelaku = &oleh
	}
	err := tx.Exec(`UPDATE intervensi
		   SET dieskalasi_pada = NOW(), dieskalasi_oleh_id = ?, prioritas = 'TINGGI', versi = versi + 1
		 WHERE id = ?`, pelaku, id).Error
	if err != nil {
		return fmt.Errorf("eskalasi: %w", err)
	}
	if err := catatRiwayat(tx, id, domain.AksiEskalasi, nil, nil, oleh, alasan); err != nil {
		return err
	}
	_, err = bukaPersetujuan(tx, domain.AjukanInput{KartuID: id, Jenis: domain.JenisEskalasi, Oleh: oleh, Catatan: alasan})
	if errors.Is(err, domain.ErrTanpaAlur) || errors.Is(err, domain.ErrPersetujuanTerbuka) {
		return nil // tanpa alur aktif, atau sudah ada permintaan terbuka: eskalasi tetap tercatat
	}
	return err
}

func (r *Repository) ListRiwayat(ctx context.Context, id string) ([]domain.Riwayat, error) {
	var items []domain.Riwayat
	err := r.db.WithContext(ctx).Table("intervensi_riwayat AS r").
		Select("r.*, u.nama AS oleh_nama").
		Joins("LEFT JOIN pengguna u ON u.id = r.oleh").
		Where("r.intervensi_id = ?", id).
		Order("r.waktu, r.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("riwayat intervensi: %w", err)
	}
	return items, nil
}

// uniqueStrings membuang nilai ganda dengan tetap menjaga urutan.
func uniqueStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
