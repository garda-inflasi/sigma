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
	"github.com/google/uuid"
)

const (
	// DefaultLimit dan MaxLimit membatasi ukuran satu halaman daftar audit.
	DefaultLimit = 50
	MaxLimit     = 200
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
	row := &domain.AuditLog{
		ID:            uuid.NewString(),
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

// List mengembalikan satu halaman audit, terbaru lebih dulu.
func (uc *UseCase) List(ctx context.Context, q Query) (domain.Page, error) {
	limit := q.Limit
	if limit < 1 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)

	var after *domain.Cursor
	if q.Cursor != "" {
		cursor, err := decodeCursor(q.Cursor)
		if err != nil {
			return domain.Page{}, err
		}
		after = &cursor
	}

	// Satu baris ekstra menunjukkan masih ada halaman berikutnya.
	items, err := uc.repo.List(ctx, q.Filter, after, limit+1)
	if err != nil {
		return domain.Page{}, err
	}
	if len(items) <= limit {
		return domain.Page{Items: items}, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	return domain.Page{Items: items, NextCursor: encodeCursor(domain.Cursor{Waktu: last.Waktu, ID: last.ID})}, nil
}

func encodeCursor(c domain.Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.Waktu.UTC().Format(time.RFC3339Nano) + "|" + c.ID))
}

func decodeCursor(raw string) (domain.Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return domain.Cursor{}, ErrInvalidCursor
	}
	waktu, id, found := strings.Cut(string(decoded), "|")
	if !found || id == "" {
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
