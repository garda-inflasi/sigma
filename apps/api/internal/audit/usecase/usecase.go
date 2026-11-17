// Package usecase menulis dan membaca jejak audit.
package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

const (
	// DefaultLimit dan MaxLimit membatasi ukuran satu halaman daftar audit.
	DefaultLimit = 50
	MaxLimit     = 200
	// DefaultLimitSaya dan MaxLimitSaya membatasi satu halaman riwayat aktivitas akun sendiri.
	DefaultLimitSaya = 20
	MaxLimitSaya     = 100
)

const (
	// ExportBatch adalah ukuran satu pembacaan saat mengekspor; MaxExportRows
	// membatasi total baris satu ekspor agar tidak membebani basis data.
	ExportBatch   = 500
	MaxExportRows = 20000
)

// ErrInvalidCursor dikembalikan bila cursor bukan hasil halaman sebelumnya.
var ErrInvalidCursor = errors.New("cursor tidak valid")

type UseCase struct {
	repo domain.Repository
	now  func() time.Time
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo, now: time.Now}
}

var _ domain.Writer = (*UseCase)(nil)

// Write mencatat satu baris audit lengkap dengan snapshot aktor dan origin permintaan.
func (uc *UseCase) Write(ctx context.Context, entry domain.Entry) error {
	row, err := uc.buildRow(ctx, entry)
	if err != nil {
		return err
	}
	return uc.repo.Create(ctx, row)
}

func (uc *UseCase) buildRow(ctx context.Context, entry domain.Entry) (*domain.AuditLog, error) {
	waktu := entry.Waktu
	if waktu.IsZero() {
		waktu = uc.now()
	}
	origin := domain.OriginFrom(ctx)
	// ID tidak diisi: basis data memberinya (BIGINT identity).
	row := &domain.AuditLog{
		Waktu:         waktu,
		Aksi:          entry.Aksi,
		Entitas:       entry.Entitas,
		EntitasID:     entry.EntitasID,
		Hasil:         entry.Hasil,
		IP:            nonEmpty(origin.IP),
		UserAgent:     nonEmpty(origin.UserAgent),
		CorrelationID: nonEmpty(origin.CorrelationID),
	}

	if err := uc.fillActor(ctx, row, entry.AktorID); err != nil {
		return nil, err
	}

	metadata := any(nil)
	if len(entry.Metadata) > 0 {
		metadata = entry.Metadata
	}
	for _, field := range []struct {
		value any
		dst   **string
	}{{entry.Sebelum, &row.Sebelum}, {entry.Sesudah, &row.Sesudah}, {metadata, &row.Metadata}} {
		encoded, err := marshalOptional(field.value)
		if err != nil {
			return nil, fmt.Errorf("encode audit: %w", err)
		}
		*field.dst = encoded
	}
	return row, nil
}

// fillActor menyalin identitas aktor. Akun yang tidak ada (mis. sudah dihapus)
// dicatat tanpa aktor daripada kehilangan barisnya: kolom aktor_id adalah FK.
func (uc *UseCase) fillActor(ctx context.Context, row *domain.AuditLog, actorID string) error {
	if actorID == "" {
		return nil
	}
	actor, err := uc.repo.FindActor(ctx, actorID)
	if errors.Is(err, domain.ErrActorNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	row.AktorID, row.AktorNama, row.AktorEmail, row.AktorPeran = &actor.ID, &actor.Nama, &actor.Email, &actor.Peran
	return nil
}

// Query adalah kriteria satu halaman daftar audit.
type Query struct {
	Filter domain.Filter
	Limit  int
	Cursor string
}

// List mengembalikan satu halaman audit, terbaru lebih dulu. Seperti Get dan Export, baris disamarkan data
// pribadinya saat disajikan (domain.SamarkanBaris): baris lama yang tersimpan dengan nomor telepon utuh tidak tampil
// utuh. Data di basis data dan hash rantai tidak berubah.
func (uc *UseCase) List(ctx context.Context, q Query) (domain.Page, error) {
	return uc.listPage(ctx, q.Filter, q.Limit, q.Cursor, DefaultLimit, MaxLimit)
}

// listPage membaca satu halaman dengan paginasi cursor; limit dijepit ke 1..maks (bawaan bila kurang dari 1).
func (uc *UseCase) listPage(ctx context.Context, filter domain.Filter, limit int, rawCursor string, bawaan, maks int) (domain.Page, error) {
	if limit < 1 {
		limit = bawaan
	}
	limit = min(limit, maks)

	var after *domain.Cursor
	if rawCursor != "" {
		cursor, err := decodeCursor(rawCursor)
		if err != nil {
			return domain.Page{}, err
		}
		after = &cursor
	}

	// Satu baris ekstra menunjukkan masih ada halaman berikutnya.
	items, err := uc.repo.List(ctx, filter, after, limit+1)
	if err != nil {
		return domain.Page{}, err
	}
	if len(items) <= limit {
		return domain.Page{Items: samarkanBaris(items)}, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	return domain.Page{Items: samarkanBaris(items), NextCursor: encodeCursor(domain.Cursor{Waktu: last.Waktu, ID: last.ID})}, nil
}

// AktivitasSaya adalah satu baris riwayat aktivitas akun, disajikan kepada pemiliknya sendiri. Sengaja sempit:
// tanpa snapshot sebelum/sesudah, metadata, atau correlation id, dan tanpa identitas pelaku lain (hanya
// perannya), karena baris sasaran dibuat oleh orang lain.
type AktivitasSaya struct {
	ID        idx.ID
	Waktu     time.Time
	Aksi      string
	Entitas   string
	EntitasID string
	Hasil     string
	Sebagai   domain.Sebagai
	// IP dan UserAgent hanya terisi pada baris pelaku: itu perangkat pemiliknya sendiri.
	IP        *string
	UserAgent *string
	// OlehPeran adalah peran pelaku pada baris sasaran.
	OlehPeran *string
}

// HalamanAktivitas adalah satu halaman riwayat aktivitas; NextCursor kosong berarti halaman terakhir.
type HalamanAktivitas struct {
	Items      []AktivitasSaya
	NextCursor string
}

// QuerySaya adalah kriteria riwayat aktivitas milik pengguna yang sedang masuk. Filter.PenggunaID dan
// Filter.Sebagai diisi usecase; isi dari pemanggil diabaikan, jadi riwayat akun lain tidak bisa diminta.
type QuerySaya struct {
	Filter  domain.Filter
	Sebagai domain.Sebagai
	Limit   int
	Cursor  string
}

// ErrSebagaiTidakValid dikembalikan bila Sebagai bukan pelaku, sasaran, atau semua.
var ErrSebagaiTidakValid = errors.New("nilai sebagai tidak valid")

// ListSaya mengembalikan riwayat aktivitas akun penggunaID (aksi yang ia lakukan dan aksi orang lain terhadap
// akunnya), terbaru lebih dulu. Cakupan dikunci ke penggunaID, yang berasal dari token, bukan dari masukan.
func (uc *UseCase) ListSaya(ctx context.Context, penggunaID string, q QuerySaya) (HalamanAktivitas, error) {
	if !q.Sebagai.Sah() {
		return HalamanAktivitas{}, ErrSebagaiTidakValid
	}
	filter := q.Filter
	// Filter akun lain tidak boleh melonggarkan cakupan.
	filter.Aktor, filter.Peran, filter.CorrelationID, filter.Q = "", "", "", ""
	filter.PenggunaID, filter.Sebagai = penggunaID, q.Sebagai
	if filter.Sebagai == "" {
		filter.Sebagai = domain.SebagaiSemua
	}
	// Entitas dan id entitas mengikuti filter pemanggil, tetapi tetap di dalam cakupan di atas.
	halaman, err := uc.listPage(ctx, filter, q.Limit, q.Cursor, DefaultLimitSaya, MaxLimitSaya)
	if err != nil {
		return HalamanAktivitas{}, err
	}
	out := HalamanAktivitas{Items: make([]AktivitasSaya, len(halaman.Items)), NextCursor: halaman.NextCursor}
	for i, row := range halaman.Items {
		out.Items[i] = keAktivitasSaya(row, penggunaID)
	}
	return out, nil
}

func keAktivitasSaya(row domain.AuditLog, penggunaID string) AktivitasSaya {
	a := AktivitasSaya{ID: row.ID, Waktu: row.Waktu, Aksi: row.Aksi, Entitas: row.Entitas, EntitasID: row.EntitasID, Hasil: row.Hasil, Sebagai: domain.SebagaiPelaku}
	if row.AktorID != nil && *row.AktorID == penggunaID {
		a.IP, a.UserAgent = row.IP, row.UserAgent
		return a
	}
	a.Sebagai, a.OlehPeran = domain.SebagaiSasaran, row.AktorPeran
	return a
}

// samarkanBaris menyamarkan data pribadi pada setiap baris (salinan; masukan tidak diubah).
func samarkanBaris(items []domain.AuditLog) []domain.AuditLog {
	out := make([]domain.AuditLog, len(items))
	for i, row := range items {
		out[i] = domain.SamarkanBaris(row)
	}
	return out
}

// Detail adalah satu entri audit lengkap dengan perubahan sebelum → sesudah.
type Detail struct {
	Row       domain.AuditLog
	Perubahan []domain.Perubahan
}

// Get mengembalikan satu entri beserta diff sebelum/sesudah; domain.ErrNotFound
// bila ID (atau kode ID lama) tidak ada. Selisih dihitung dari data mentah, lalu nilai data pribadinya disamarkan:
// perubahan nomor telepon tetap tampil sebagai bidang yang berubah tanpa membuka digitnya.
func (uc *UseCase) Get(ctx context.Context, ref idx.RujukanID) (Detail, error) {
	row, err := uc.repo.Get(ctx, ref)
	if err != nil {
		return Detail{}, err
	}
	changes, err := domain.DiffState(row.Sebelum, row.Sesudah)
	if err != nil {
		return Detail{}, fmt.Errorf("diff audit %s: %w", row.ID, err)
	}
	return Detail{Row: domain.SamarkanBaris(*row), Perubahan: domain.SamarkanPerubahan(row.Entitas, changes)}, nil
}

// Export menyerahkan baris yang cocok dengan filter ke emit, terbaru lebih dulu,
// per ExportBatch baris sampai paling banyak MaxExportRows. Mengembalikan jumlah
// baris yang diserahkan dan apakah ada baris lagi yang terpotong oleh batas.
func (uc *UseCase) Export(ctx context.Context, filter domain.Filter, emit func(domain.AuditLog) error) (n int, truncated bool, err error) {
	var after *domain.Cursor
	for n < MaxExportRows {
		batch := min(ExportBatch, MaxExportRows-n)
		// Satu baris ekstra membedakan "tepat habis" dari "terpotong".
		items, err := uc.repo.List(ctx, filter, after, batch+1)
		if err != nil {
			return n, false, err
		}
		more := len(items) > batch
		if more {
			items = items[:batch]
		}
		for _, row := range items {
			if err := emit(domain.SamarkanBaris(row)); err != nil {
				return n, false, err
			}
			n++
		}
		if !more {
			return n, false, nil
		}
		last := items[len(items)-1]
		after = &domain.Cursor{Waktu: last.Waktu, ID: last.ID}
	}
	return n, true, nil
}

func encodeCursor(c domain.Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.Waktu.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()))
}

func decodeCursor(raw string) (domain.Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return domain.Cursor{}, ErrInvalidCursor
	}
	waktu, teksID, found := strings.Cut(string(decoded), "|")
	if !found {
		return domain.Cursor{}, ErrInvalidCursor
	}
	// Cursor lama (sebelum ID menjadi BIGINT) membawa UUID teks dan ditolak di sini: cursor hanya berlaku untuk
	// satu penelusuran, klien memulai lagi dari halaman pertama.
	id, err := idx.Parse(teksID)
	if err != nil {
		return domain.Cursor{}, ErrInvalidCursor
	}
	t, err := time.Parse(time.RFC3339Nano, waktu)
	if err != nil {
		return domain.Cursor{}, ErrInvalidCursor
	}
	return domain.Cursor{Waktu: t, ID: id}, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// marshalOptional men-marshal v ke JSON; nil menghasilkan nil.
func marshalOptional(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	s := string(raw)
	return &s, nil
}
