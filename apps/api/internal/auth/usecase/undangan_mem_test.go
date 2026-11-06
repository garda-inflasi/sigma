package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/email/emailtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// memUndangan menambahkan penyimpanan undangan dalam memori di atas memRepo (pengguna, sesi, lockout) dengan aturan
// yang sama seperti repositori Postgres: email unik tak peka huruf besar-kecil, penulisan atomik, aktivasi
// bersyarat. Aturan SQL-nya sendiri diuji di paket integration.
type memUndangan struct {
	*memRepo
	undangan []*domain.Undangan
	nextID   int64
	// kabupaten adalah kode yang dianggap ada di master wilayah.
	kabupaten map[string]bool
	// Galat yang dikembalikan metode terkait bila diisi.
	errBuat, errGanti, errAktifkan error
	nBuat                          int
}

func newMemUndangan(users ...*domain.Pengguna) *memUndangan {
	return &memUndangan{memRepo: newMemRepo(users...), kabupaten: map[string]bool{"1171": true, "1108": true}}
}

func (r *memUndangan) emailAda(e string) bool {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, e) {
			return true
		}
	}
	return false
}

func (r *memUndangan) BuatUndangan(_ context.Context, baru []domain.UndanganBaru) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nBuat++
	if r.errBuat != nil {
		return r.errBuat
	}
	// Semua atau tidak sama sekali: periksa seluruh kelompok sebelum menulis apa pun.
	terlihat := map[string]bool{}
	for _, b := range baru {
		kunci := strings.ToLower(b.User.Email)
		if r.emailAda(b.User.Email) || terlihat[kunci] {
			return fmt.Errorf("buat akun undangan: %w", gorm.ErrDuplicatedKey)
		}
		if _, ada := r.users[b.User.ID]; ada {
			return fmt.Errorf("buat akun undangan: %w", gorm.ErrDuplicatedKey)
		}
		terlihat[kunci] = true
	}
	for _, b := range baru {
		u := *b.User
		r.users[u.ID] = &u
		r.nextID++
		b.Undangan.ID, b.Undangan.UserID = r.nextID, u.ID
		salinan := *b.Undangan
		r.undangan = append(r.undangan, &salinan)
	}
	return nil
}

func (r *memUndangan) GantiUndangan(_ context.Context, userID string, u *domain.Undangan, now time.Time) (*domain.Pengguna, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errGanti != nil {
		return nil, r.errGanti
	}
	user, ada := r.users[userID]
	if !ada {
		return nil, fmt.Errorf("kunci akun undangan: %w", gorm.ErrRecordNotFound)
	}
	if user.Status != domain.StatusMenunggu {
		return nil, domain.ErrBukanMenunggu
	}
	for _, lama := range r.undangan {
		if lama.UserID == userID && lama.UsedAt == nil && lama.DicabutPada == nil {
			t := now
			lama.DicabutPada = &t
		}
	}
	r.nextID++
	u.ID, u.UserID, u.Email = r.nextID, userID, user.Email
	salinan := *u
	r.undangan = append(r.undangan, &salinan)
	return r.clone(user), nil
}

func (r *memUndangan) GetUndangan(_ context.Context, tokenHash string) (*domain.Undangan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.undangan {
		if u.TokenHash == tokenHash {
			salinan := *u
			return &salinan, nil
		}
	}
	return nil, fmt.Errorf("get undangan: %w", gorm.ErrRecordNotFound)
}

func (r *memUndangan) Aktifkan(_ context.Context, tokenHash, passwordHash string, now time.Time) (*domain.Pengguna, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errAktifkan != nil {
		return nil, r.errAktifkan
	}
	for _, u := range r.undangan {
		if u.TokenHash != tokenHash {
			continue
		}
		user, ada := r.users[u.UserID]
		if !ada || u.UsedAt != nil || u.DicabutPada != nil || !now.Before(u.ExpiresAt) ||
			user.Status != domain.StatusMenunggu || !strings.EqualFold(user.Email, u.Email) {
			return nil, nil
		}
		t, hash := now, passwordHash
		u.UsedAt = &t
		user.PasswordHash, user.Status, user.WajibGantiPassword, user.PasswordDiubahPada = &hash, "AKTIF", false, &t
		return r.clone(user), nil
	}
	return nil, nil
}

func (r *memUndangan) EmailTerdaftar(_ context.Context, emailKecil []string) (map[string]bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ada := map[string]bool{}
	for _, e := range emailKecil {
		if r.emailAda(e) {
			ada[e] = true
		}
	}
	return ada, nil
}

func (r *memUndangan) KabupatenAda(_ context.Context, kode []string) (map[string]bool, error) {
	ada := map[string]bool{}
	for _, k := range kode {
		if r.kabupaten[k] {
			ada[k] = true
		}
	}
	return ada, nil
}

func (r *memUndangan) ListUsersUntukEkspor(_ context.Context, f domain.FilterPengguna, _ time.Time, batas int) ([]domain.Pengguna, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var cocok []domain.Pengguna
	for _, u := range r.users {
		if f.Cari != "" {
			s := strings.ToLower(f.Cari)
			if !strings.Contains(strings.ToLower(u.Nama), s) && !strings.Contains(strings.ToLower(u.Email), s) &&
				!strings.Contains(strings.ToLower(u.Peran), s) && !strings.Contains(strings.ToLower(u.Lembaga), s) {
				continue
			}
		}
		if len(f.IDs) > 0 {
			ditemukan := false
			for _, id := range f.IDs {
				ditemukan = ditemukan || id == u.ID
			}
			if !ditemukan {
				continue
			}
		}
		cocok = append(cocok, *u)
	}
	sort.Slice(cocok, func(i, j int) bool {
		if !cocok[i].DibuatPada.Equal(cocok[j].DibuatPada) {
			return cocok[i].DibuatPada.After(cocok[j].DibuatPada)
		}
		return cocok[i].ID < cocok[j].ID
	})
	if len(cocok) > batas {
		return cocok[:batas], true, nil
	}
	return cocok, false, nil
}

// undanganAktif mengembalikan undangan akun yang masih hidup (belum dipakai dan belum dicabut).
func (r *memUndangan) undanganAktif(userID string) []*domain.Undangan {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Undangan
	for _, u := range r.undangan {
		if u.UserID == userID && u.UsedAt == nil && u.DicabutPada == nil {
			salinan := *u
			out = append(out, &salinan)
		}
	}
	return out
}

func (r *memUndangan) pengguna(id string) *domain.Pengguna {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[id]; ok {
		return r.clone(u)
	}
	return nil
}

// ── Fixture ───────────────────────────────────────────────────────────

type undanganFixture struct {
	t     *testing.T
	uc    *UseCase
	repo  *memUndangan
	audit *audittest.Recorder
	smtp  *emailtest.Server
	now   *time.Time
}

const baseURLUji = "https://garda.test"

var adminUndangID = uid("admin-1")

func adminPengguna() *domain.Pengguna {
	return &domain.Pengguna{ID: adminUndangID, Nama: "Admin", Email: "admin@garda.test", Peran: "ADMIN_SISTEM", Lapis: "SISTEM", Lembaga: "BPPSDMP", Status: "AKTIF", DibuatPada: awalUji.Add(-24 * time.Hour)}
}

// newUndanganFixture menyiapkan usecase di atas memUndangan dengan SMTP palsu yang menangkap email.
func newUndanganFixture(t *testing.T, users ...*domain.Pengguna) *undanganFixture {
	t.Helper()
	smtp := emailtest.Start(t)
	f := &undanganFixture{
		t:     t,
		repo:  newMemUndangan(append([]*domain.Pengguna{adminPengguna()}, users...)...),
		audit: &audittest.Recorder{},
		smtp:  smtp,
		now:   jamBeku(t, awalUji),
	}
	cfg := DefaultConfig()
	cfg.MFAKey = testKeyring()
	f.uc = New(f.repo, token.NewManager("test-secret-key-32-chars!!"), cfg, logger.NewDefault(), email.New(smtp.Config(baseURLUji)), f.audit)
	return f
}

// masukanUndang adalah isian undangan yang sah.
func masukanUndang() UndangInput {
	return UndangInput{Nama: "Budi Santoso", Email: "budi@garda.test", Peran: "OPS_MAHASISWA", Lembaga: "FMIPA USK", KabupatenKode: "1171"}
}

func (f *undanganFixture) undang(in UndangInput) *HasilUndangan {
	f.t.Helper()
	h, err := f.uc.UndangPengguna(context.Background(), in, adminUndangID)
	require.NoError(f.t, err)
	return h
}

// tokenEmail menunggu email terbaru untuk penerima dan mengembalikan token di tautannya.
func (f *undanganFixture) tokenEmail(penerima string, n int) string {
	f.t.Helper()
	pesan := f.smtp.Tunggu(f.t, penerima, n)
	token := pesan[len(pesan)-1].Token()
	require.NotEmpty(f.t, token, "tautan aktivasi tidak ada di email: %s", pesan[len(pesan)-1].Data)
	return token
}
