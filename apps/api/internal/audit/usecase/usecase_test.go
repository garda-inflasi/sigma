package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	actors  map[string]*domain.Actor
	created []*domain.AuditLog
	rows    []domain.AuditLog
	err     error

	gotLimit  int
	gotAfter  *domain.Cursor
	gotFilter domain.Filter
}

func (f *fakeRepo) Create(_ context.Context, log *domain.AuditLog) error {
	f.created = append(f.created, log)
	return f.err
}

func (f *fakeRepo) FindActor(_ context.Context, id string) (*domain.Actor, error) {
	if a, ok := f.actors[id]; ok {
		return a, nil
	}
	return nil, domain.ErrActorNotFound
}

func (f *fakeRepo) Get(_ context.Context, ref idx.RujukanID) (*domain.AuditLog, error) {
	for i := range f.rows {
		cocokID := ref.ID != 0 && f.rows[i].ID == ref.ID
		cocokKode := ref.Kode != "" && f.rows[i].Kode != nil && *f.rows[i].Kode == ref.Kode
		if cocokID || cocokKode {
			return &f.rows[i], f.err
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) List(_ context.Context, filter domain.Filter, after *domain.Cursor, limit int) ([]domain.AuditLog, error) {
	f.gotFilter, f.gotAfter, f.gotLimit = filter, after, limit
	if limit < len(f.rows) {
		return f.rows[:limit], f.err
	}
	return f.rows, f.err
}

var fixedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func newUseCase(repo *fakeRepo) *UseCase {
	uc := New(repo)
	uc.now = func() time.Time { return fixedNow }
	return uc
}

func TestWrite_SnapshotsActorAndOrigin(t *testing.T) {
	repo := &fakeRepo{actors: map[string]*domain.Actor{uid("u1"): {ID: uid("u1"), Nama: "Rina", Email: "rina@gardainflasi.id", Peran: "MANAJER_PROGRAM"}}}
	ctx := domain.WithOrigin(context.Background(), domain.Origin{IP: "10.0.0.7", UserAgent: "uji", CorrelationID: "req-1"})

	err := newUseCase(repo).Write(ctx, domain.Entry{
		AktorID: uid("u1"), Aksi: "ASSIGN_ROLE", Entitas: "pengguna", EntitasID: uid("u2"), Hasil: domain.HasilBerhasil,
		Sebelum: map[string]any{"peran": "A"}, Sesudah: map[string]any{"peran": "B"}, Metadata: map[string]any{"k": 1},
	})

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	row := repo.created[0]
	assert.Zero(t, row.ID, "ID dibuat basis data (BIGINT identity), bukan aplikasi")
	assert.Nil(t, row.Kode)
	assert.Equal(t, fixedNow, row.Waktu)
	assert.Equal(t, uid("u1"), *row.AktorID)
	assert.Equal(t, "Rina", *row.AktorNama)
	assert.Equal(t, "rina@gardainflasi.id", *row.AktorEmail)
	assert.Equal(t, "MANAJER_PROGRAM", *row.AktorPeran)
	assert.Equal(t, "10.0.0.7", *row.IP)
	assert.Equal(t, "uji", *row.UserAgent)
	assert.Equal(t, "req-1", *row.CorrelationID)
	assert.JSONEq(t, `{"peran":"A"}`, *row.Sebelum)
	assert.JSONEq(t, `{"peran":"B"}`, *row.Sesudah)
	assert.JSONEq(t, `{"k":1}`, *row.Metadata)
}

func TestWrite_WithoutActorOrOriginLeavesColumnsNull(t *testing.T) {
	repo := &fakeRepo{}

	err := newUseCase(repo).Write(context.Background(), domain.Entry{Aksi: "LOGIN", Entitas: "pengguna", EntitasID: "x", Hasil: domain.HasilGagal})

	require.NoError(t, err)
	row := repo.created[0]
	assert.Nil(t, row.AktorID)
	assert.Nil(t, row.IP)
	assert.Nil(t, row.Sebelum)
	assert.Nil(t, row.Metadata)
}

func TestWrite_UnknownActorIsRecordedWithoutActor(t *testing.T) {
	repo := &fakeRepo{}

	require.NoError(t, newUseCase(repo).Write(context.Background(), domain.Entry{AktorID: "hilang", Aksi: "X", Entitas: "e", EntitasID: "1", Hasil: domain.HasilBerhasil}))

	assert.Nil(t, repo.created[0].AktorID)
}

func TestWrite_PropagatesRepositoryError(t *testing.T) {
	repo := &fakeRepo{err: errors.New("db mati")}

	err := newUseCase(repo).Write(context.Background(), domain.Entry{Aksi: "X", Entitas: "e", EntitasID: "1", Hasil: domain.HasilBerhasil})

	assert.Error(t, err)
}

func TestWrite_RejectsUnencodableJSON(t *testing.T) {
	repo := &fakeRepo{}

	err := newUseCase(repo).Write(context.Background(), domain.Entry{Aksi: "X", Entitas: "e", EntitasID: "1", Hasil: domain.HasilBerhasil, Sesudah: make(chan int)})

	assert.Error(t, err)
	assert.Empty(t, repo.created)
}

func rowsAt(n int) []domain.AuditLog {
	rows := make([]domain.AuditLog, n)
	for i := range rows {
		rows[i] = domain.AuditLog{ID: idx.ID(100 - i), Waktu: fixedNow.Add(-time.Duration(i) * time.Minute)}
	}
	return rows
}

func TestList_LimitDefaultsAndIsCapped(t *testing.T) {
	cases := map[string]struct{ in, wantFetch int }{
		"kosong memakai bawaan":  {0, DefaultLimit + 1},
		"negatif memakai bawaan": {-3, DefaultLimit + 1},
		"dalam batas":            {7, 8},
		"melebihi batas":         {5000, MaxLimit + 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}

			_, err := newUseCase(repo).List(context.Background(), Query{Limit: tc.in})

			require.NoError(t, err)
			assert.Equal(t, tc.wantFetch, repo.gotLimit, "satu baris ekstra untuk mendeteksi halaman berikutnya")
		})
	}
}

func TestList_NextCursorOnlyWhenMoreRows(t *testing.T) {
	t.Run("masih ada halaman berikutnya", func(t *testing.T) {
		repo := &fakeRepo{rows: rowsAt(6)}

		page, err := newUseCase(repo).List(context.Background(), Query{Limit: 5})

		require.NoError(t, err)
		assert.Len(t, page.Items, 5)
		require.NotEmpty(t, page.NextCursor)

		cursor, err := decodeCursor(page.NextCursor)
		require.NoError(t, err)
		assert.Equal(t, idx.ID(96), cursor.ID, "cursor menunjuk baris terakhir yang dikembalikan")
		assert.True(t, cursor.Waktu.Equal(page.Items[4].Waktu))
	})

	t.Run("halaman terakhir", func(t *testing.T) {
		repo := &fakeRepo{rows: rowsAt(5)}

		page, err := newUseCase(repo).List(context.Background(), Query{Limit: 5})

		require.NoError(t, err)
		assert.Len(t, page.Items, 5)
		assert.Empty(t, page.NextCursor)
	})
}

func TestList_PassesDecodedCursorAndFilter(t *testing.T) {
	repo := &fakeRepo{}
	cursor := encodeCursor(domain.Cursor{Waktu: fixedNow, ID: 91})

	_, err := newUseCase(repo).List(context.Background(), Query{Cursor: cursor, Filter: domain.Filter{Aksi: "LOGIN"}})

	require.NoError(t, err)
	require.NotNil(t, repo.gotAfter)
	assert.Equal(t, idx.ID(91), repo.gotAfter.ID)
	assert.Equal(t, "LOGIN", repo.gotFilter.Aksi)
}

func TestList_InvalidCursor(t *testing.T) {
	bad := []string{
		"%%%",
		base64.RawURLEncoding.EncodeToString([]byte("tanpa-pemisah")),
		base64.RawURLEncoding.EncodeToString([]byte("bukan-waktu|12")),
		base64.RawURLEncoding.EncodeToString([]byte(fixedNow.Format(time.RFC3339Nano) + "|")),
		// Cursor lama (ID berupa UUID) dan ID yang bukan bilangan bulat positif ditolak.
		base64.RawURLEncoding.EncodeToString([]byte(fixedNow.Format(time.RFC3339Nano) + "|" + uid("u1"))),
		base64.RawURLEncoding.EncodeToString([]byte(fixedNow.Format(time.RFC3339Nano) + "|0")),
		base64.RawURLEncoding.EncodeToString([]byte(fixedNow.Format(time.RFC3339Nano) + "|-5")),
		base64.RawURLEncoding.EncodeToString([]byte(fixedNow.Format(time.RFC3339Nano) + "|007")),
	}
	for _, cursor := range bad {
		_, err := newUseCase(&fakeRepo{}).List(context.Background(), Query{Cursor: cursor})
		assert.ErrorIs(t, err, ErrInvalidCursor, cursor)
	}
}

func TestList_PropagatesRepositoryError(t *testing.T) {
	_, err := newUseCase(&fakeRepo{err: errors.New("db mati")}).List(context.Background(), Query{})

	assert.Error(t, err)
}
