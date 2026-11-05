// Package postgres adalah repository intervensi di atas PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
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

func (r *Repository) List(ctx context.Context, q domain.ListQuery) ([]domain.Kartu, error) {
	query := r.db.WithContext(ctx).Model(&domain.Intervensi{})
	if q.Siklus != "" {
		query = query.Where("siklus = ?", q.Siklus)
	}
	if q.Tahap != "" {
		query = query.Where("tahap = ?", q.Tahap)
	}
	if q.Prioritas != "" {
		query = query.Where("prioritas = ?", q.Prioritas)
	}

	var items []domain.Intervensi
	if err := query.Order("siklus, tahap, urutan, dibuat_pada, id").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar intervensi: %w", err)
	}
	cards := make([]domain.Kartu, len(items))
	if len(items) == 0 {
		return cards, nil
	}

	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i, it := range items {
		cards[i].Intervensi = it
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

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Intervensi, error) {
	var item domain.Intervensi
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&item).Error; err != nil {
		return nil, fmt.Errorf("ambil intervensi: %w", err)
	}
	return &item, nil
}

func (r *Repository) Create(ctx context.Context, in domain.NewIntervensi, tahap string) (*domain.Intervensi, error) {
	var created domain.Intervensi
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var seq int64
		if err := tx.Raw("SELECT nextval('intervensi_id_seq')").Scan(&seq).Error; err != nil {
			return fmt.Errorf("nomor intervensi: %w", err)
		}
		var urutan int
		err := tx.Raw("SELECT COALESCE(MAX(urutan) + 1, 0) FROM intervensi WHERE siklus = ? AND tahap = ?", in.Siklus, tahap).Scan(&urutan).Error
		if err != nil {
			return fmt.Errorf("urutan intervensi: %w", err)
		}

		created = domain.Intervensi{
			ID: fmt.Sprintf("INT-%d", seq), Siklus: in.Siklus, Tahap: tahap, Urutan: urutan, Judul: in.Judul,
			Deskripsi: in.Deskripsi, KomoditasKode: in.KomoditasKode, KabupatenKode: in.KabupatenKode,
			Prioritas: in.Prioritas, Tenggat: in.Tenggat, EwsAlertID: in.EwsAlertID, DibuatOlehID: in.DibuatOlehID,
		}
		if err := tx.Create(&created).Error; err != nil {
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

func (r *Repository) Move(ctx context.Context, id, tahap string, index int, selesai bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var siklus string
		res := tx.Raw("SELECT siklus FROM intervensi WHERE id = ? FOR UPDATE", id).Scan(&siklus)
		if res.Error != nil {
			return fmt.Errorf("kunci intervensi: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		var column []string
		err := tx.Raw(`
			SELECT id FROM intervensi
			 WHERE siklus = ? AND tahap = ? AND id <> ?
			 ORDER BY urutan, dibuat_pada, id
			   FOR UPDATE`, siklus, tahap, id).Scan(&column).Error
		if err != nil {
			return fmt.Errorf("kolom tujuan: %w", err)
		}
		index = max(0, min(index, len(column)))
		column = slices.Insert(column, index, id)

		err = tx.Exec(`
			UPDATE intervensi
			   SET tahap = ?, urutan = ?,
			       selesai_pada = CASE WHEN ?::boolean THEN COALESCE(selesai_pada, NOW()) ELSE NULL END
			 WHERE id = ?`, tahap, index, selesai, id).Error
		if err != nil {
			return fmt.Errorf("pindahkan kartu: %w", err)
		}
		for pos, other := range column {
			if other == id {
				continue
			}
			if err := tx.Exec("UPDATE intervensi SET urutan = ? WHERE id = ? AND urutan <> ?", pos, other, pos).Error; err != nil {
				return fmt.Errorf("nomori ulang kolom: %w", err)
			}
		}
		return nil
	})
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
