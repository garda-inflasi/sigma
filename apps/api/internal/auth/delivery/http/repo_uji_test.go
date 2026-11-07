package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// repoUji adalah repositori tiruan untuk tes handler pengguna (CRUD, aksi admin, undangan, impor/ekspor, izin).
// Setiap metode yang dipakai alur itu punya field fungsi; metode yang tidak diisi menggagalkan tes dengan jelas
// ("tidak diharapkan") alih-alih diam-diam mengembalikan nilai nol. domain.Repository dan domain.IzinRepository
// yang tertanam bernilai nil: UseCase hanya mengenali penyimpanan izin berbasis basis data lewat
// IzinRepository, sehingga tertanamnya menjadikan ListRoles/izin langsung mengikuti field izin di bawah.
type repoUji struct {
	domain.Repository
	domain.IzinRepository
	t *testing.T

	pengguna   func(id string) (*domain.Pengguna, error)
	byEmail    func(email string) (*domain.Pengguna, error)
	buat       func(u *domain.Pengguna) error
	patch      func(id string, p domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error)
	hapus      func(id string) error
	setSandi   func(id string) (bool, error)
	cabutSemua func(id string) error
	cabutSesi  func(id string) ([]string, error)
	bulk       func(ids []string, aksi domain.BulkUserAction) (int64, error)
	daftar     func(q domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error)
	kunciAktif func(hash []string) (map[string]time.Time, error)
	hapusKunci func(hash string) (*domain.KunciLogin, error)

	kabupaten     func(kode []string) (map[string]bool, error)
	emailAda      func(email []string) (map[string]bool, error)
	buatUndangan  func(baru []domain.UndanganBaru) error
	gantiUndangan func(id string) (*domain.Pengguna, error)
	getUndangan   func(hash string) (*domain.Undangan, error)
	aktifkan      func() (*domain.Pengguna, error)
	ekspor        func(f domain.FilterPengguna, batas int) ([]domain.Pengguna, bool, error)

	// pengirim, bila diisi, menggantikan pengirim email yang tidak dikonfigurasi (SMTP palsu emailtest).
	pengirim *email.Sender

	listPeran    func() ([]domain.PeranDetail, error)
	listIzin     func() ([]domain.Izin, error)
	setIzinPeran func(p domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error)
	listLangsung func(id string) (*domain.IzinPengguna, error)
	berikanIzin  func(g domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error)
	cabutIzin    func(id, izin string) (*domain.IzinLangsung, error)
}

func (r *repoUji) tak(nama string) {
	r.t.Helper()
	r.t.Fatalf("repoUji: %s dipanggil tetapi tidak diharapkan tes ini", nama)
}

func (r *repoUji) GetUserByID(_ context.Context, id string) (*domain.Pengguna, error) {
	if r.pengguna == nil {
		r.tak("GetUserByID")
	}
	return r.pengguna(id)
}

func (r *repoUji) GetUserByEmail(_ context.Context, e string) (*domain.Pengguna, error) {
	if r.byEmail == nil {
		r.tak("GetUserByEmail")
	}
	return r.byEmail(e)
}

func (r *repoUji) CreateUser(_ context.Context, u *domain.Pengguna) error {
	if r.buat == nil {
		r.tak("CreateUser")
	}
	return r.buat(u)
}

func (r *repoUji) PatchUser(_ context.Context, id string, p domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
	if r.patch == nil {
		r.tak("PatchUser")
	}
	return r.patch(id, p)
}

func (r *repoUji) DeleteUser(_ context.Context, id string) error {
	if r.hapus == nil {
		r.tak("DeleteUser")
	}
	return r.hapus(id)
}

func (r *repoUji) SetPassword(_ context.Context, id string, _ domain.GantiPassword) (bool, error) {
	if r.setSandi == nil {
		r.tak("SetPassword")
	}
	return r.setSandi(id)
}

func (r *repoUji) RevokeAllUserRefreshTokens(_ context.Context, id string) error {
	if r.cabutSemua == nil {
		r.tak("RevokeAllUserRefreshTokens")
	}
	return r.cabutSemua(id)
}

func (r *repoUji) RevokeSessions(_ context.Context, id, _ string, _ time.Time) ([]string, error) {
	if r.cabutSesi == nil {
		r.tak("RevokeSessions")
	}
	return r.cabutSesi(id)
}

func (r *repoUji) BulkUpdateUsers(_ context.Context, ids []string, aksi domain.BulkUserAction) (int64, error) {
	if r.bulk == nil {
		r.tak("BulkUpdateUsers")
	}
	return r.bulk(ids, aksi)
}

func (r *repoUji) ListUsers(_ context.Context, q domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
	if r.daftar == nil {
		r.tak("ListUsers")
	}
	return r.daftar(q)
}

func (r *repoUji) KunciAktif(_ context.Context, hash []string, _ time.Time) (map[string]time.Time, error) {
	if r.kunciAktif == nil {
		return map[string]time.Time{}, nil
	}
	return r.kunciAktif(hash)
}

func (r *repoUji) HapusKunci(_ context.Context, hash string) (*domain.KunciLogin, error) {
	if r.hapusKunci == nil {
		r.tak("HapusKunci")
	}
	return r.hapusKunci(hash)
}

// KabupatenAda tanpa field kabupaten menjawab "tidak ada kabupaten yang dikenal".
func (r *repoUji) KabupatenAda(_ context.Context, kode []string) (map[string]bool, error) {
	if r.kabupaten == nil {
		return map[string]bool{}, nil
	}
	return r.kabupaten(kode)
}

func (r *repoUji) EmailTerdaftar(_ context.Context, e []string) (map[string]bool, error) {
	if r.emailAda == nil {
		return map[string]bool{}, nil
	}
	return r.emailAda(e)
}

func (r *repoUji) BuatUndangan(_ context.Context, baru []domain.UndanganBaru) error {
	if r.buatUndangan == nil {
		r.tak("BuatUndangan")
	}
	return r.buatUndangan(baru)
}

func (r *repoUji) GantiUndangan(_ context.Context, id string, _ *domain.Undangan, _ time.Time) (*domain.Pengguna, error) {
	if r.gantiUndangan == nil {
		r.tak("GantiUndangan")
	}
	return r.gantiUndangan(id)
}

func (r *repoUji) GetUndangan(_ context.Context, hash string) (*domain.Undangan, error) {
	if r.getUndangan == nil {
		r.tak("GetUndangan")
	}
	return r.getUndangan(hash)
}

func (r *repoUji) Aktifkan(context.Context, string, string, time.Time) (*domain.Pengguna, error) {
	if r.aktifkan == nil {
		r.tak("Aktifkan")
	}
	return r.aktifkan()
}

func (r *repoUji) ListUsersUntukEkspor(_ context.Context, f domain.FilterPengguna, _ time.Time, batas int) ([]domain.Pengguna, bool, error) {
	if r.ekspor == nil {
		r.tak("ListUsersUntukEkspor")
	}
	return r.ekspor(f, batas)
}

func (r *repoUji) ListPeran(context.Context) ([]domain.PeranDetail, error) {
	if r.listPeran == nil {
		r.tak("ListPeran")
	}
	return r.listPeran()
}

func (r *repoUji) ListIzin(context.Context) ([]domain.Izin, error) {
	if r.listIzin == nil {
		r.tak("ListIzin")
	}
	return r.listIzin()
}

func (r *repoUji) SetIzinPeran(_ context.Context, p domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
	if r.setIzinPeran == nil {
		r.tak("SetIzinPeran")
	}
	return r.setIzinPeran(p)
}

func (r *repoUji) ListIzinLangsung(_ context.Context, id string) (*domain.IzinPengguna, error) {
	if r.listLangsung == nil {
		r.tak("ListIzinLangsung")
	}
	return r.listLangsung(id)
}

func (r *repoUji) BerikanIzinLangsung(_ context.Context, g domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
	if r.berikanIzin == nil {
		r.tak("BerikanIzinLangsung")
	}
	return r.berikanIzin(g)
}

func (r *repoUji) CabutIzinLangsung(_ context.Context, id, izin string) (*domain.IzinLangsung, error) {
	if r.cabutIzin == nil {
		r.tak("CabutIzinLangsung")
	}
	return r.cabutIzin(id, izin)
}

// ── Perangkat tes bersama ────────────────────────────────────────────

// penggunaAktif adalah pengguna contoh yang dipakai tes.
func penggunaAktif(id string) *domain.Pengguna {
	return &domain.Pengguna{
		ID: id, Nama: "Ani", Email: id + "@garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL",
		Lembaga: "BPPSDMP", Status: "AKTIF", DibuatPada: time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
	}
}

// ada selalu menemukan pengguna dengan id yang diminta.
func ada(id string) (*domain.Pengguna, error) { return penggunaAktif(id), nil }

// rekamAudit memasang pencatat audit tiruan; tes membaca entri yang tercatat lewatnya.
type rekamAudit = audittest.Recorder

// routerPengguna memasang semua handler pengguna di atas UseCase yang memakai repo tiruan. Pemanggil di
// "admin-1" (c.Set("user_id")), tanpa middleware izin: yang diuji adalah pemetaan galat handler.
func routerPengguna(t *testing.T, repo *repoUji, opts ...func(*usecase.Config)) (*gin.Engine, *rekamAudit) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo.t = t
	cfg := usecase.DefaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	rec := &audittest.Recorder{}
	pengirim := email.New(email.Config{})
	if repo.pengirim != nil {
		pengirim = repo.pengirim
	}
	uc := usecase.New(repo, token.NewManager("test-secret-key-32-chars!!"), cfg, logger.NewDefault(), pengirim, rec)
	h := New(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uid("admin-1")) })
	r.POST("/users", h.CreateUser)
	r.GET("/users", h.ListUsers)
	r.POST("/users/bulk", h.BulkUser)
	r.POST("/users/undang", h.UndangPengguna)
	r.POST("/users/impor", h.ImporPengguna)
	r.GET("/users/ekspor", h.EksporPengguna)
	r.POST("/users/:id/undang-ulang", h.UndangUlang)
	r.GET("/users/:id", h.GetUser)
	r.PATCH("/users/:id", h.UpdateUser)
	r.DELETE("/users/:id", h.DeleteUser)
	r.POST("/users/:id/reset-password", h.ResetPasswordAdmin)
	r.POST("/users/:id/buka-kunci", h.BukaKunci)
	r.POST("/users/:id/revoke-sessions", h.ForceLogoutUser)
	r.PUT("/users/:id/roles", h.AssignRole)
	r.POST("/users/:id/reset-mfa", h.ResetMFAPengguna)
	r.GET("/users/:id/riwayat-peran", h.RiwayatPeranPengguna)
	r.GET("/users/:id/permissions", h.ListUserPermissions)
	r.POST("/users/:id/permissions", h.GrantUserPermission)
	r.DELETE("/users/:id/permissions/:izin", h.RevokeUserPermission)
	r.GET("/roles", h.ListRoles)
	r.PUT("/roles/:id/permissions", h.UpdateRolePermissions)
	r.GET("/permissions", h.ListPermissions)
	r.POST("/auth/aktivasi", h.AktivasiAkun)
	return r, rec
}

// hasil adalah jawaban handler: status, badan JSON (bila JSON), dan rekaman mentah.
type hasil struct {
	*httptest.ResponseRecorder
	JSON map[string]any
}

// panggilJSON mengirim badan JSON (string mentah, supaya tes bisa mengirim JSON rusak) dan mengurai jawaban.
func panggilJSON(t *testing.T, r *gin.Engine, method, path, badan string) hasil {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(badan))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := hasil{ResponseRecorder: w}
	if w.Body.Len() > 0 && w.Header().Get("Content-Type") != "" && json.Valid(w.Body.Bytes()) {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out.JSON), w.Body.String())
	}
	return out
}

// data mengambil objek `data` dari jawaban.
func (h hasil) data(t *testing.T) map[string]any {
	t.Helper()
	d, ok := h.JSON["data"].(map[string]any)
	require.True(t, ok, "jawaban tanpa objek data: %s", h.Body.String())
	return d
}

// kode mengambil `code` amplop galat.
func (h hasil) kode() string {
	s, _ := h.JSON["code"].(string)
	return s
}

// pesan mengambil `message` amplop.
func (h hasil) pesan() string {
	s, _ := h.JSON["message"].(string)
	return s
}

func js(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("js: %v", err))
	}
	return string(b)
}

// routerKecil memasang satu handler tanpa UseCase, untuk menguji fungsi pembantu respons.
func routerKecil(h gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", h)
	return r
}

// snap mengambil snapshot map dari bidang Sebelum/Sesudah entri audit.
func snap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
