package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// pgForeignKeyViolation adalah SQLSTATE pelanggaran kunci asing.
const pgForeignKeyViolation = "23503"

// RepoReferensi mengimplementasikan domain.RepositoryReferensi di PostgreSQL.
type RepoReferensi struct {
	db *gorm.DB
}

// NewReferensi membuat repositori data referensi master.
func NewReferensi(db *gorm.DB) *RepoReferensi {
	return &RepoReferensi{db: db}
}

var _ domain.RepositoryReferensi = (*RepoReferensi)(nil)

func (r *RepoReferensi) ListKecamatan(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.KecamatanRef, int64, error) {
	query := r.db.WithContext(ctx).Table("kecamatan AS k").Joins("JOIN kabupaten kab ON kab.kode = k.kabupaten_kode")
	if kabupatenKode != "" {
		query = query.Where("k.kabupaten_kode = ?", kabupatenKode)
	}
	var items []domain.KecamatanRef
	total, err := daftar(query, q, []string{"k.kode", "k.nama"}, selectKecamatan, "k.kode ASC", &items)
	return items, total, err
}

const selectKecamatan = "k.id, k.kode, k.nama, k.kabupaten_kode, kab.nama AS kabupaten_nama, k.kode_dagri"

func (r *RepoReferensi) GetKecamatan(ctx context.Context, kode string) (domain.KecamatanRef, error) {
	var row domain.KecamatanRef
	err := r.db.WithContext(ctx).Table("kecamatan AS k").Joins("JOIN kabupaten kab ON kab.kode = k.kabupaten_kode").
		Select(selectKecamatan).Where("k.id::text = ? OR k.kode = ?", kode, kode).Take(&row).Error
	return row, tidakDitemukan(err, "kecamatan "+kode)
}

func (r *RepoReferensi) ListSatuan(ctx context.Context, q domain.ListQuery) ([]domain.SatuanRef, int64, error) {
	query := r.db.WithContext(ctx).Table("satuan AS s")
	var items []domain.SatuanRef
	total, err := daftar(query, q, []string{"s.kode", "s.nama"},
		"s.id, s.kode, s.nama, (SELECT count(*) FROM komoditas k WHERE k.satuan = s.kode) AS jumlah_komoditas", "s.kode ASC", &items)
	return items, total, err
}

func (r *RepoReferensi) ListPedagang(ctx context.Context, f domain.PedagangFilter, q domain.ListQuery) ([]domain.PedagangRef, int64, error) {
	query := r.db.WithContext(ctx).Table("pedagang AS p").Joins("JOIN pasar ps ON ps.kode = p.pasar_id")
	if f.PasarID != "" {
		query = query.Where("p.pasar_id = ?", f.PasarID)
	}
	if f.Aktif != nil {
		query = query.Where("p.aktif = ?", *f.Aktif)
	}
	var items []domain.PedagangRef
	// jumlah_entri dihitung per baris halaman: subkueri select-list dievaluasi sesudah LIMIT.
	total, err := daftar(query, q, []string{"p.kode", "p.nama"},
		`p.id, p.pasar_id, ps.nama AS pasar_nama, p.kode, p.nama, p.aktif,
		 (SELECT count(*) FROM entri_harga e WHERE e.pasar_id = p.pasar_id AND e.kode_pedagang = p.kode) AS jumlah_entri`,
		"p.pasar_id ASC, p.kode ASC", &items)
	return items, total, err
}

func (r *RepoReferensi) GetKomoditas(ctx context.Context, kode string) (domain.Komoditas, error) {
	var k domain.Komoditas
	err := r.db.WithContext(ctx).Where("id::text = ? OR kode = ?", kode, kode).Take(&k).Error
	return k, tidakDitemukan(err, "komoditas "+kode)
}

func (r *RepoReferensi) RiwayatVersiKomoditas(ctx context.Context, kode string) ([]domain.KomoditasVersiRef, error) {
	var rows []domain.KomoditasVersiRef
	// Urutan menurun: baris sebelumnya dalam urutan ini adalah versi yang lebih baru, jadi LAG(berlaku_dari)
	// adalah akhir masa berlaku baris ini. Versi terbaru tidak punya pendahulu (NULL = masih berlaku).
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, versi, nama, kategori, satuan, harga_acuan, shelf_life_hari, pilot, berlaku_dari,
		       LAG(berlaku_dari) OVER w AS berlaku_sampai, perubahan_id
		  FROM komoditas_versi
		 WHERE komoditas_kode = (SELECT kode FROM komoditas WHERE id::text = ? OR kode = ? LIMIT 1)
		WINDOW w AS (ORDER BY berlaku_dari DESC, id DESC)
		 ORDER BY berlaku_dari DESC, id DESC`, kode, kode).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("riwayat versi komoditas %s: %w", kode, err)
	}
	return rows, nil
}

func (r *RepoReferensi) GetWilayah(ctx context.Context, kode string) (domain.WilayahDetail, error) {
	var d domain.WilayahDetail
	db := r.db.WithContext(ctx)
	if err := tidakDitemukan(db.Where("id::text = ? OR kode = ?", kode, kode).Take(&d.Kabupaten).Error, "wilayah "+kode); err != nil {
		return domain.WilayahDetail{}, err
	}
	err := db.Raw(`
		SELECT (SELECT count(*) FROM kecamatan WHERE kabupaten_kode = ?) AS jumlah_kecamatan,
		       (SELECT count(*) FROM pasar WHERE kabupaten_kode = ?) AS jumlah_pasar`, d.Kabupaten.Kode, d.Kabupaten.Kode).
		Row().Scan(&d.JumlahKecamatan, &d.JumlahPasar)
	if err != nil {
		return domain.WilayahDetail{}, fmt.Errorf("cacah wilayah %s: %w", kode, err)
	}
	return d, nil
}

func (r *RepoReferensi) GetPasar(ctx context.Context, kode string) (domain.PasarDetail, error) {
	var d domain.PasarDetail
	db := r.db.WithContext(ctx)
	if err := tidakDitemukan(db.Where("id::text = ? OR kode = ?", kode, kode).Take(&d.Pasar).Error, "pasar "+kode); err != nil {
		return domain.PasarDetail{}, err
	}
	err := db.Raw(`
		SELECT (SELECT nama FROM kabupaten WHERE kode = ?) AS kabupaten_nama,
		       (SELECT nama FROM kecamatan WHERE kode = ?) AS kecamatan_nama,
		       (SELECT count(*) FROM pedagang WHERE pasar_id = ?) AS jumlah_pedagang`,
		d.Pasar.KabupatenKode, d.Pasar.KecamatanKode, d.Pasar.Kode).
		Row().Scan(&d.KabupatenNama, &d.Pasar.KecamatanNama, &d.JumlahPedagang)
	if err != nil {
		return domain.PasarDetail{}, fmt.Errorf("detail pasar %s: %w", kode, err)
	}
	return d, nil
}

func (r *RepoReferensi) HapusKomoditas(ctx context.Context, kode string) (domain.Komoditas, error) {
	var row domain.Komoditas
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Baris dikunci agar salinan untuk audit sama dengan yang dihapus.
		if err := tidakDitemukan(tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text = ? OR kode = ?", kode, kode).Take(&row).Error, "komoditas "+kode); err != nil {
			return err
		}
		if err := pastikanTanpaUsulan(tx, domain.EntitasKomoditas, row.Kode); err != nil {
			return err
		}
		// komoditas_versi ikut terhapus lewat ON DELETE CASCADE.
		return tx.Where("id = ?", row.ID).Delete(&domain.Komoditas{}).Error
	})
	return row, terjemahkanHapus(err, "komoditas "+kode)
}

func (r *RepoReferensi) HapusPasar(ctx context.Context, kode string) (domain.Pasar, error) {
	var row domain.Pasar
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tidakDitemukan(tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text = ? OR kode = ?", kode, kode).Take(&row).Error, "pasar "+kode); err != nil {
			return err
		}
		if err := pastikanTanpaUsulan(tx, domain.EntitasPasar, row.Kode); err != nil {
			return err
		}
		return tx.Where("id = ?", row.ID).Delete(&domain.Pasar{}).Error
	})
	return row, terjemahkanHapus(err, "pasar "+kode)
}

// pastikanTanpaUsulan mengembalikan ErrAdaUsulan bila perubahan_master punya usulan MENUNGGU untuk entitas itu.
func pastikanTanpaUsulan(tx *gorm.DB, entitas, kode string) error {
	var ada bool
	err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM perubahan_master WHERE entitas = ? AND entitas_id = ? AND status = 'MENUNGGU')`,
		entitas, kode).Scan(&ada).Error
	if err != nil {
		return fmt.Errorf("cek usulan %s %s: %w", entitas, kode, err)
	}
	if ada {
		return domain.ErrAdaUsulan
	}
	return nil
}

// isiNamaKecamatan mengisi Pasar.KecamatanNama untuk daftar pasar dengan satu kueri.
func isiNamaKecamatan(db *gorm.DB, items []domain.Pasar) error {
	kode := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, p := range items {
		if p.KecamatanKode != nil && !seen[*p.KecamatanKode] {
			seen[*p.KecamatanKode] = true
			kode = append(kode, *p.KecamatanKode)
		}
	}
	if len(kode) == 0 {
		return nil
	}
	var rows []struct {
		Kode string `gorm:"column:kode"`
		Nama string `gorm:"column:nama"`
	}
	if err := db.Table("kecamatan").Select("kode, nama").Where("kode IN ?", kode).Scan(&rows).Error; err != nil {
		return fmt.Errorf("nama kecamatan pasar: %w", err)
	}
	nama := make(map[string]string, len(rows))
	for _, row := range rows {
		nama[row.Kode] = row.Nama
	}
	for i := range items {
		if items[i].KecamatanKode == nil {
			continue
		}
		if n, ok := nama[*items[i].KecamatanKode]; ok {
			items[i].KecamatanNama = &n
		}
	}
	return nil
}

// daftar menerapkan pencarian dan paginasi pada query dasar (Table + Joins + filter), menghitung seluruh hasil,
// lalu memindai kolom selectSQL ke dest. Kolom cari harus sudah berkualifikasi tabel.
func daftar(query *gorm.DB, q domain.ListQuery, searchColumns []string, selectSQL, orderBy string, dest any) (int64, error) {
	if search := strings.TrimSpace(q.Search); search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		conditions := make([]string, len(searchColumns))
		args := make([]any, len(searchColumns))
		for i, column := range searchColumns {
			conditions[i] = "LOWER(" + column + ") LIKE ?"
			args[i] = pattern
		}
		query = query.Where(strings.Join(conditions, " OR "), args...)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("hitung master: %w", err)
	}

	query = query.Select(selectSQL).Order(orderBy)
	if q.Limit > 0 {
		query = query.Limit(q.Limit).Offset((max(q.Page, 1) - 1) * q.Limit)
	}
	if err := query.Scan(dest).Error; err != nil {
		return 0, fmt.Errorf("daftar master: %w", err)
	}
	return total, nil
}

// tidakDitemukan memetakan gorm.ErrRecordNotFound ke domain.ErrTidakDitemukan; galat lain dibungkus.
func tidakDitemukan(err error, apa string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domain.ErrTidakDitemukan
	}
	return fmt.Errorf("ambil %s: %w", apa, err)
}

// terjemahkanHapus memetakan pelanggaran kunci asing (23503) ke domain.ErrDirujuk; galat domain dilewatkan.
func terjemahkanHapus(err error, apa string) error {
	var pgErr *pgconn.PgError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrTidakDitemukan), errors.Is(err, domain.ErrAdaUsulan):
		return err
	case errors.Is(err, gorm.ErrForeignKeyViolated), errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation:
		return domain.ErrDirujuk
	}
	return fmt.Errorf("hapus %s: %w", apa, err)
}
