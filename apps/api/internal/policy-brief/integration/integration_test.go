// Package integration menguji API policy brief terhadap PostgreSQL sungguhan hasil
// migrasi, dengan data demo lintas modul (inflasi, harga, peringatan dini, proyeksi) sebagai bahan
// penyusunan brief. LLM tidak pernah dipanggil: tes memakai templat atau Generator palsu.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/internal/policy-brief/generator"
	briefRoutes "github.com/garda-inflasi/api/internal/policy-brief/routes"
	"github.com/garda-inflasi/api/internal/seed"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "brief-integration-secret-key-32chars!"

	// MANAJER_KEBIJAKAN: brief.lihat + brief.susun; STRATEGIS_BI: brief.lihat saja; OPS_MAHASISWA: tanpa izin brief.
	emailPenyusun  = "manajer.kebijakan@gardainflasi.id"
	emailPembaca   = "strategis.bi@gardainflasi.id"
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
)

var (
	pg       *pgtest.Server
	fixedNow = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// Jaga-jaga: kunci API di lingkungan pengembang tidak boleh sampai membuat tes memanggil API sungguhan.
	_ = os.Unsetenv(generator.EnvAPIKey)
	os.Exit(pgtest.Main(m, &pg))
}

type suite struct {
	db     *gorm.DB
	router *gin.Engine
	tokens map[string]string
}

var (
	once   sync.Once
	shared *suite
)

// newSuite membuat router di atas db; gen nil berarti tanpa LLM (brief disusun dari templat).
func newSuite(db *gorm.DB, gen domain.Generator) *suite {
	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	briefRoutes.Register(v1, &briefRoutes.Dependencies{DB: db, Token: tokens, Logger: log, Generator: gen})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() {
		db := pg.CreateDatabase(t, "brief_shared")
		pgtest.MigrateUp(t, db)
		ctx := t.Context()
		require.NoError(t, seed.SeedReference(ctx, db))
		require.NoError(t, seed.SeedDemo(ctx, db))
		shared = newSuite(db, nil)
	})
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
}

// denganGenerator membuat router lain di atas basis data yang sama dengan Generator palsu.
func (s *suite) denganGenerator(gen domain.Generator) *suite { return newSuite(s.db, gen) }

func (s *suite) call(t *testing.T, method, path, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if email != "" {
		req.Header.Set("Authorization", "Bearer "+s.login(t, email))
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *suite) login(t *testing.T, email string) string {
	t.Helper()
	if tok, ok := s.tokens[email]; ok {
		return tok
	}
	body, _ := json.Marshal(map[string]string{"email": email, "password": seed.DemoPassword})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	s.tokens[email] = res.Data.AccessToken
	return res.Data.AccessToken
}

type briefRow struct {
	ID                string `json:"id"`
	Kode              string `json:"kode"`
	Periode           string `json:"periode"`
	Judul             string `json:"judul"`
	Status            string `json:"status"`
	Ringkasan         string `json:"ringkasan"`
	DisusunOleh       string `json:"disusun_oleh"`
	JumlahRekomendasi int    `json:"jumlah_rekomendasi"`
	Halaman           int    `json:"halaman"`
	SumberNarasi      string `json:"sumber_narasi"`
	LabelDraf         string `json:"label_draf"`
}

type versiRow struct {
	ID    string `json:"id"`
	Versi int    `json:"versi"`
	Oleh  string `json:"oleh"`
	Waktu string `json:"waktu"`
}

type detailRow struct {
	briefRow
	Konten       domain.Konten `json:"konten"`
	ModelLLM     *string       `json:"model_llm"`
	PromptVersi  *string       `json:"prompt_versi"`
	PromptHash   *string       `json:"prompt_hash"`
	Galat        *string       `json:"galat"`
	RiwayatVersi []versiRow    `json:"riwayat_versi"`
}

// briefID membaca UUID brief menurut kodenya dari basis data (brief hasil Generate memakai gen_random_uuid()).
func (s *suite) briefID(t *testing.T, kode string) string {
	t.Helper()
	var id string
	require.NoError(t, s.db.Raw(`SELECT id::text FROM policy_brief WHERE kode = ?`, kode).Scan(&id).Error)
	require.NotEmpty(t, id, kode)
	return id
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

// tunggu menunggu penyusunan latar belakang sampai brief berstatus `status`, lalu mengembalikan detailnya.
func (s *suite) tunggu(t *testing.T, id, status string) detailRow {
	t.Helper()
	var d detailRow
	require.Eventually(t, func() bool {
		w := s.call(t, http.MethodGet, "/v1/policy-brief/"+id, emailPembaca, nil)
		if w.Code != http.StatusOK {
			return false
		}
		d = decode[detailRow](t, w)
		return d.Status == status
	}, 30*time.Second, 50*time.Millisecond, "brief %s tidak mencapai status %s (terakhir %s)", id, status, d.Status)
	return d
}

func (s *suite) generate(t *testing.T, periode string) detailRow {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": periode})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	got := decode[briefRow](t, w)
	assert.Equal(t, "DIPROSES", got.Status, "jawaban Generate adalah brief yang baru dimulai")
	return s.tunggu(t, got.ID, "DRAF")
}

func TestPolicyBrief_Authorization(t *testing.T) {
	s := sharedSuite(t)
	generate := map[string]string{"periode": "2027-01"}

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/policy-brief", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/policy-brief", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/policy-brief", emailPembaca, nil).Code)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/policy-brief/PB-2026-10", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/policy-brief/PB-2026-10", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/policy-brief/PB-2026-10", emailPembaca, nil).Code)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/policy-brief/generate", "", generate).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailTanpaIzin, generate).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPembaca, generate).Code,
		"pembaca tanpa brief.susun tidak boleh menyusun")

	var n int64
	require.NoError(t, s.db.Table("policy_brief").Where("kode = ?", "PB-2027-01").Count(&n).Error)
	assert.Zero(t, n, "permintaan yang ditolak tidak boleh menyimpan brief")
}

func TestPolicyBrief_ListReturnsSeededBriefsNewestFirst(t *testing.T) {
	s := sharedSuite(t)

	got := decode[[]briefRow](t, s.call(t, http.MethodGet, "/v1/policy-brief", emailPembaca, nil))

	byID := map[string]briefRow{}
	for _, b := range got {
		byID[b.Kode] = b
		assert.True(t, idx.ValidUUID(b.ID), "ID brief adalah UUID, bukan lagi PB-YYYY-MM: %s", b.ID)
	}
	for _, kode := range []string{"PB-2026-10", "PB-2026-09", "PB-2026-08", "PB-2026-07", "PB-2026-06"} {
		require.Contains(t, byID, kode)
		assert.Equal(t, idx.UUIDStabil("policy_brief", kode), byID[kode].ID, "UUID brief seed = idx.UUIDStabil")
	}
	oct := byID["PB-2026-10"]
	assert.Equal(t, "Oktober 2026", oct.Periode)
	assert.Equal(t, "DRAF", oct.Status)
	assert.Equal(t, 5, oct.JumlahRekomendasi)
	assert.Equal(t, 8, oct.Halaman)
	assert.Equal(t, "Dr. Nurul Aini, S.E., M.E.", oct.DisusunOleh)
	assert.Equal(t, "DIDISTRIBUSIKAN", byID["PB-2026-08"].Status)
	assert.Equal(t, "DEMO", oct.SumberNarasi)
	assert.Equal(t, domain.LabelDemo, oct.LabelDraf)

	for i := 1; i < len(got); i++ {
		assert.Greater(t, got[i-1].Kode, got[i].Kode, "daftar harus berurut periode terbaru lebih dulu (menurut kode, bukan UUID)")
	}
}

func TestPolicyBrief_DetailBriefContohBerlabelDemo(t *testing.T) {
	s := sharedSuite(t)

	d := decode[detailRow](t, s.call(t, http.MethodGet, "/v1/policy-brief/PB-2026-10", emailPembaca, nil))

	assert.Equal(t, "DEMO", d.SumberNarasi)
	assert.Equal(t, domain.LabelDemo, d.Konten.Label)
	assert.Len(t, d.Konten.Butir("rekomendasi"), 5)
	assert.Nil(t, d.ModelLLM)
	assert.Equal(t, "PB-2026-10", d.Kode)
	assert.NotNil(t, d.RiwayatVersi, "riwayat_versi selalu larik (kosong untuk brief contoh tanpa versi)")
	assert.Empty(t, d.RiwayatVersi)

	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/policy-brief/PB-1999-01", emailPembaca, nil).Code)
}

func TestPolicyBrief_DetailMenerimaUUIDMaupunKodeAlias(t *testing.T) {
	s := sharedSuite(t)
	uuid := idx.UUIDStabil("policy_brief", "PB-2026-09")

	lewatKode := decode[detailRow](t, s.call(t, http.MethodGet, "/v1/policy-brief/PB-2026-09", emailPembaca, nil))
	lewatUUID := decode[detailRow](t, s.call(t, http.MethodGet, "/v1/policy-brief/"+uuid, emailPembaca, nil))
	lewatUUIDBesar := decode[detailRow](t, s.call(t, http.MethodGet, "/v1/policy-brief/"+strings.ToUpper(uuid), emailPembaca, nil))

	assert.Equal(t, uuid, lewatKode.ID)
	assert.Equal(t, lewatKode, lewatUUID, "ID dan kode menunjuk brief yang sama")
	assert.Equal(t, lewatKode, lewatUUIDBesar, "UUID huruf besar dinormalkan")

	t.Run("tidak ada atau salah bentuk dijawab 404 tanpa menyentuh kolom uuid", func(t *testing.T) {
		for _, path := range []string{
			"/v1/policy-brief/00000000-0000-4000-8000-000000000000", // UUID sah yang tidak ada
			"/v1/policy-brief/PB-1999-01",                           // kode yang tidak ada
			"/v1/policy-brief/bukan-uuid",                           // bukan UUID dan bukan kode
			"/v1/policy-brief/12345",                                // angka murni bukan kode brief
			"/v1/policy-brief/" + strings.Repeat("P", 65),           // lebih panjang dari kolom kode
			"/v1/policy-brief/%20",                                  // spasi
			"/v1/policy-brief/urn:uuid:" + uuid,                     // ejaan UUID non-kanonik
			"/v1/policy-brief/" + strings.ReplaceAll(uuid, "-", ""), // tanpa tanda hubung
		} {
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, path, emailPembaca, nil).Code, path)
		}
	})
}

func TestPolicyBrief_GenerateTanpaLLMMenyusunDrafDariDataNyata(t *testing.T) {
	s := sharedSuite(t)

	d := s.generate(t, "2026-11")

	assert.Equal(t, "PB-2026-11", d.Kode)
	assert.True(t, idx.ValidUUID(d.ID), "ID brief baru adalah UUID dari basis data: %s", d.ID)
	assert.Equal(t, "November 2026", d.Periode)
	assert.Equal(t, "Dr. Nurul Aini, S.E., M.E.", d.DisusunOleh, "penyusun diambil dari token")
	assert.Equal(t, "TEMPLAT", d.SumberNarasi)
	assert.Equal(t, domain.LabelDrafTemplat, d.LabelDraf)
	assert.Equal(t, domain.LabelDrafTemplat, d.Konten.Label)
	assert.Nil(t, d.ModelLLM, "tanpa kunci API tidak ada jejak model")
	assert.Nil(t, d.Galat)
	assert.Positive(t, d.Halaman)
	require.Len(t, d.Konten.Bagian, 5)

	t.Run("angka berasal dari basis data, bukan karangan", func(t *testing.T) {
		var inflasi struct {
			Mtm, Yoy, Ytd float64
			Sumber        string
		}
		require.NoError(t, s.db.Raw(`SELECT inflasi_mtm AS mtm, inflasi_yoy AS yoy, inflasi_ytd AS ytd, sumber FROM inflasi_bulanan
			WHERE wilayah_kode = '11' AND periode = '2026-10-01'`).Scan(&inflasi).Error)
		require.Equal(t, "DEMO", inflasi.Sumber)
		assert.InDelta(t, 2.84, inflasi.Yoy, 1e-9)

		assert.Contains(t, d.Ringkasan, "Inflasi tahunan Oktober 2026 tercatat 2,84% (bulanan 0,41%)")
		assert.Contains(t, d.Ringkasan, "sebagian angka berasal dari data demo", "data demo diberi catatan keandalan")

		fakta := map[string]domain.Fakta{}
		for _, f := range d.Konten.Lampiran {
			fakta[f.ID] = f
		}
		assert.Equal(t, "inflasi_bulanan (DEMO)", fakta["INFLASI_YOY"].Sumber)
		assert.Equal(t, "2,84%", fakta["INFLASI_YOY"].Nilai)
		assert.Equal(t, "2,13%", fakta["INFLASI_YTD"].Nilai)
		assert.Contains(t, fakta, "ANDIL_1_KOMODITAS")
		assert.Contains(t, fakta, "EWS_TOTAL")
		assert.Contains(t, fakta, "PROYEKSI_1_PERUBAHAN")
		for id, f := range fakta {
			assert.NotEmpty(t, f.Sumber, "tiap angka di lampiran bersitasi sumber: %s", id)
		}

		// Harga: rata-rata bulan ini komoditas teratas dihitung ulang langsung dari entri_harga.
		var rata int
		require.NoError(t, s.db.Raw(`SELECT ROUND(AVG(e.harga))::int FROM entri_harga e JOIN komoditas k ON k.kode = e.komoditas_kode
			WHERE k.nama = ? AND e.tipe = 'KONSUMEN' AND e.status IN ('DIVALIDASI','DIPUBLIKASI')
			AND e.dicatat_pada >= '2026-11-01T00:00:00Z' AND e.dicatat_pada < '2026-12-01T00:00:00Z'`, fakta["HARGA_1_KOMODITAS"].Nilai).Scan(&rata).Error)
		require.Positive(t, rata)
		assert.Equal(t, "Rp"+ribuan(rata), fakta["HARGA_1_RATA"].Nilai)
	})

	t.Run("snapshot, riwayat versi, job, dan audit tersimpan", func(t *testing.T) {
		var snapshot struct {
			Fakta []domain.Fakta `json:"fakta"`
		}
		var raw string
		require.NoError(t, s.db.Raw(`SELECT snapshot_data::text FROM policy_brief WHERE kode = 'PB-2026-11'`).Scan(&raw).Error)
		require.NoError(t, json.Unmarshal([]byte(raw), &snapshot))
		assert.Greater(t, len(snapshot.Fakta), len(d.Konten.Lampiran), "snapshot memuat semua fakta, lampiran hanya yang dipakai")

		var jumlah int64
		require.NoError(t, s.db.Table("policy_brief_versi").Where("brief_id = ? AND versi = 1 AND oleh = garda_uuid_stabil('pengguna', 'manajer_kebijakan')", d.ID).Count(&jumlah).Error)
		assert.EqualValues(t, 1, jumlah)
		var job struct{ Status string }
		require.NoError(t, s.db.Raw(`SELECT status FROM brief_job WHERE brief_id = ? AND kode IS NULL`, d.ID).Scan(&job).Error)
		assert.Equal(t, "SELESAI", job.Status)

		// Respons detail mengekspos versi_id (BIGINT) supaya telaah dan revisi bisa merujuknya.
		require.Len(t, d.RiwayatVersi, 1)
		assert.Equal(t, 1, d.RiwayatVersi[0].Versi)
		assert.Regexp(t, `^[1-9][0-9]*$`, d.RiwayatVersi[0].ID)
		assert.Equal(t, "Dr. Nurul Aini, S.E., M.E.", d.RiwayatVersi[0].Oleh)
		var idVersi string
		require.NoError(t, s.db.Raw(`SELECT id::text FROM policy_brief_versi WHERE brief_id = ? AND versi = 1`, d.ID).Scan(&idVersi).Error)
		assert.Equal(t, idVersi, d.RiwayatVersi[0].ID, "versi_id di respons = id baris policy_brief_versi")

		for aksi, hasil := range map[string]string{"GENERATE_BRIEF": auditdomain.HasilBerhasil, "BRIEF_DRAF_SELESAI": auditdomain.HasilBerhasil} {
			var trail auditdomain.AuditLog
			require.NoError(t, s.db.Where("aksi = ? AND entitas_id = ?", aksi, d.ID).First(&trail).Error, aksi)
			assert.Equal(t, "policy_brief", trail.Entitas)
			assert.Equal(t, hasil, trail.Hasil)
			require.NotNil(t, trail.AktorID)
			assert.Equal(t, uid("manajer_kebijakan"), *trail.AktorID)
		}
	})

	t.Run("brief baru tampil di daftar dengan label dan jumlah rekomendasi", func(t *testing.T) {
		listed := decode[[]briefRow](t, s.call(t, http.MethodGet, "/v1/policy-brief", emailPembaca, nil))
		assert.Equal(t, "PB-2026-11", listed[0].Kode)
		assert.Equal(t, d.ID, listed[0].ID)
		assert.Equal(t, "DRAF", listed[0].Status)
		assert.Equal(t, len(d.Konten.Butir(domain.BagianRekomendasiPemda))+len(d.Konten.Butir(domain.BagianRekomendasiTPID)), listed[0].JumlahRekomendasi)
		assert.Equal(t, domain.LabelDrafTemplat, listed[0].LabelDraf)
	})
}

// ribuan menulis n dengan titik pemisah ribuan, mis. 56000 menjadi "56.000".
func ribuan(n int) string {
	digit := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range digit {
		if i > 0 && (len(digit)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// llmPalsu adalah Generator palsu yang menyusun narasi ber-placeholder atau gagal.
type llmPalsu struct {
	narasi  *domain.Narasi
	err     error
	konteks domain.Konteks
	mu      sync.Mutex
}

func (g *llmPalsu) Nama() string { return "llm-palsu" }

func (g *llmPalsu) Susun(_ context.Context, k domain.Konteks) (*domain.Narasi, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.konteks = k
	return g.narasi, g.err
}

func narasiLLM() *domain.Narasi {
	return &domain.Narasi{
		Ringkasan:        "Inflasi tahunan {{INFLASI_PERIODE}} tercatat {{INFLASI_YOY}}, {{STATUS_SASARAN}}.",
		Temuan:           []string{"{{ANDIL_1_KOMODITAS}} menyumbang {{ANDIL_1_NILAI}} terhadap inflasi."},
		Risiko:           []string{"Harga {{PROYEKSI_1_KOMODITAS}} diproyeksikan {{PROYEKSI_1_ARAH}} {{PROYEKSI_1_PERUBAHAN}}."},
		RekomendasiPemda: []string{"Siapkan operasi pasar untuk {{PROYEKSI_1_KOMODITAS}}."},
		RekomendasiTPID:  []string{"Bahas pasokan {{PROYEKSI_1_KOMODITAS}} dalam rapat TPID."},
		Meta:             domain.MetaNarasi{Penyusun: domain.PenyusunLLM, Model: "claude-sonnet-5-5", PromptVersi: "brief-v1", PromptHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
}

func TestPolicyBrief_GenerateDenganLLMMenyimpanJejakModelDanLabelDrafAI(t *testing.T) {
	gen := &llmPalsu{narasi: narasiLLM()}
	s := sharedSuite(t).denganGenerator(gen)

	d := s.generate(t, "2026-12")

	assert.Equal(t, "LLM", d.SumberNarasi)
	assert.Equal(t, "Draf AI — wajib ditelaah manusia", d.LabelDraf)
	assert.Equal(t, domain.LabelDrafAI, d.Konten.Label)
	require.NotNil(t, d.ModelLLM)
	assert.Equal(t, "claude-sonnet-5-5", *d.ModelLLM)
	assert.Equal(t, "brief-v1", *d.PromptVersi)
	assert.Equal(t, narasiLLM().Meta.PromptHash, *d.PromptHash)
	assert.Nil(t, d.Galat)

	// Placeholder sudah diganti nilai dari data; LLM tidak pernah menulis angka sendiri.
	assert.Equal(t, "Inflasi tahunan Oktober 2026 tercatat 2,84%, berada dalam koridor sasaran inflasi.", d.Ringkasan)
	assert.Contains(t, d.Konten.Butir(domain.BagianTemuan)[0], "poin persen")
	assert.NotContains(t, d.Konten.Butir(domain.BagianRisiko)[0], "{{")

	// LLM hanya menerima konteks hasil rangkuman deterministik dari basis data.
	gen.mu.Lock()
	defer gen.mu.Unlock()
	var ada = map[string]string{}
	for _, f := range gen.konteks.Fakta {
		ada[f.ID] = f.Nilai
	}
	assert.Equal(t, "2,84%", ada["INFLASI_YOY"])
	assert.Equal(t, "Desember 2026", ada["PERIODE"])

	var catatan auditdomain.AuditLog
	require.NoError(t, s.db.Where("aksi = 'BRIEF_DRAF_SELESAI' AND entitas_id = ?", d.ID).First(&catatan).Error)
	require.NotNil(t, catatan.Metadata)
	assert.Contains(t, *catatan.Metadata, "claude-sonnet-5-5")
	assert.Contains(t, *catatan.Metadata, narasiLLM().Meta.PromptHash)
}

func TestPolicyBrief_LLMGagalAtauMengarangAngkaDigantikanTemplat(t *testing.T) {
	mengarang := narasiLLM()
	mengarang.Ringkasan = "Inflasi tahunan melonjak 9 persen pada kuartal ini."

	kasus := []struct {
		periode string
		gen     *llmPalsu
	}{
		{"2027-03", &llmPalsu{err: errors.New("429: rate limit")}},
		{"2027-04", &llmPalsu{narasi: mengarang}},
	}
	for _, k := range kasus {
		t.Run(k.periode, func(t *testing.T) {
			s := sharedSuite(t).denganGenerator(k.gen)

			d := s.generate(t, k.periode)

			assert.Equal(t, "TEMPLAT", d.SumberNarasi, "narasi LLM yang ditolak tidak tersimpan")
			assert.Nil(t, d.ModelLLM)
			require.NotNil(t, d.Galat)
			assert.Contains(t, *d.Galat, "llm-palsu")
			assert.Contains(t, *d.Galat, "draf memakai templat")
			assert.Equal(t, domain.LabelDrafTemplat, d.Konten.Label)
			assert.Contains(t, d.Konten.Catatan, "draf memakai templat")

			var teks string
			require.NoError(t, s.db.Raw(`SELECT konten::text FROM policy_brief WHERE kode = ?`, "PB-"+k.periode).Scan(&teks).Error)
			assert.NotContains(t, teks, "melonjak")
		})
	}
}

func TestPolicyBrief_GagalMengumpulkanDataLaluDiulang(t *testing.T) {
	s := sharedSuite(t)
	// Merusak tabel sumber data sementara: pengumpulan konteks gagal, brief GAGAL, lalu pulih dan diulang.
	require.NoError(t, s.db.Exec(`ALTER TABLE ews_alert RENAME TO ews_alert_rusak`).Error)
	pulihkan := func() { _ = s.db.Exec(`ALTER TABLE IF EXISTS ews_alert_rusak RENAME TO ews_alert`).Error }
	t.Cleanup(pulihkan)

	w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2027-05"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	gagal := s.tunggu(t, "PB-2027-05", "GAGAL")
	require.NotNil(t, gagal.Galat)
	assert.Contains(t, *gagal.Galat, "kumpulkan data")
	var job struct{ Status string }
	require.NoError(t, s.db.Raw(`SELECT status FROM brief_job WHERE brief_id = ?`, gagal.ID).Scan(&job).Error)
	assert.Equal(t, "GAGAL", job.Status)
	var trail auditdomain.AuditLog
	require.NoError(t, s.db.Where("aksi = 'BRIEF_DRAF_GAGAL' AND entitas_id = ?", gagal.ID).First(&trail).Error)
	assert.Equal(t, auditdomain.HasilGagal, trail.Hasil)

	// Brief GAGAL tidak bisa ditelaah.
	bulk := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": []string{"PB-2027-05"}})
	assert.Equal(t, bulkRes{Gagal: 1}, decode[bulkRes](t, bulk))

	pulihkan()
	ulang := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2027-05"})
	require.Equal(t, http.StatusCreated, ulang.Code, "brief GAGAL boleh disusun ulang: %s", ulang.Body.String())
	pulih := s.tunggu(t, "PB-2027-05", "DRAF")
	assert.Nil(t, pulih.Galat, "galat lama dibersihkan")
	assert.Equal(t, gagal.ID, pulih.ID, "diulang pada brief yang sama (satu periode, satu brief)")

	var versi, jobs, brief int64
	require.NoError(t, s.db.Table("policy_brief_versi").Where("brief_id = ?", gagal.ID).Count(&versi).Error)
	require.NoError(t, s.db.Table("brief_job").Where("brief_id = ?", gagal.ID).Count(&jobs).Error)
	require.NoError(t, s.db.Table("policy_brief").Where("kode = 'PB-2027-05'").Count(&brief).Error)
	assert.EqualValues(t, 1, versi)
	assert.EqualValues(t, 2, jobs, "tiap percobaan punya job")
	assert.EqualValues(t, 1, brief, "mengulang tidak membuat brief kedua")
}

func TestPolicyBrief_GenerateRejectsDuplicateAndInvalidPeriod(t *testing.T) {
	s := sharedSuite(t)

	t.Run("periode yang sudah punya brief ditolak dan brief lama utuh", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2026-09"})
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

		var status string
		require.NoError(t, s.db.Raw("SELECT status::text FROM policy_brief WHERE kode = ?", "PB-2026-09").Scan(&status).Error)
		assert.Equal(t, "DISETUJUI", status)
		var n int64
		require.NoError(t, s.db.Table("policy_brief").Where("kode = ?", "PB-2026-09").Count(&n).Error)
		assert.EqualValues(t, 1, n, "satu periode hanya punya satu brief")
	})

	t.Run("brief yang sedang diproses tidak disusun ganda", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`INSERT INTO policy_brief (kode, periode, judul, status, ringkasan, rekomendasi, halaman, disusun_oleh_id)
			VALUES ('PB-2027-06', 'Juni 2027', 'x', 'DIPROSES', 'x', '[]', 1, garda_uuid_stabil('pengguna', 'manajer_kebijakan'))`).Error)

		w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2027-06"})

		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		var jobs int64
		require.NoError(t, s.db.Table("brief_job").Where("brief_id = ?", s.briefID(t, "PB-2027-06")).Count(&jobs).Error)
		assert.Zero(t, jobs)
	})

	t.Run("brief diproses yang prosesnya mati boleh diulang", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`INSERT INTO policy_brief (kode, periode, judul, status, ringkasan, rekomendasi, halaman, disusun_oleh_id, diperbarui)
			VALUES ('PB-2027-07', 'Juli 2027', 'x', 'DIPROSES', 'x', '[]', 1, garda_uuid_stabil('pengguna', 'manajer_kebijakan'), now() - interval '10 minutes')`).Error)

		w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2027-07"})

		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		s.tunggu(t, "PB-2027-07", "DRAF")
	})

	t.Run("periode tidak valid", func(t *testing.T) {
		for _, periode := range []string{"Oktober 2026", "2026-13", "2026-1", "26-10", "2026-10-01"} {
			w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": periode})
			assert.Equal(t, http.StatusBadRequest, w.Code, periode)
		}
	})

	t.Run("periode kosong", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestPolicyBrief_GenerateBersamaanSatuBriefPerPeriode: permintaan bersamaan untuk periode yang sama menghasilkan
// satu brief (kode PB-YYYY-MM unik; ON CONFLICT (kode) DO NOTHING), satu job, dan sisanya 409.
func TestPolicyBrief_GenerateBersamaanSatuBriefPerPeriode(t *testing.T) {
	s := sharedSuite(t)
	tok := s.login(t, emailPenyusun) // login sekali dulu: cache token tidak aman untuk goroutine

	const pemanggil = 6
	kode := make(chan int, pemanggil)
	for i := 0; i < pemanggil; i++ {
		go func() {
			body, _ := json.Marshal(map[string]string{"periode": "2027-08"})
			req := httptest.NewRequest(http.MethodPost, "/v1/policy-brief/generate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			s.router.ServeHTTP(w, req)
			kode <- w.Code
		}()
	}
	hitung := map[int]int{}
	for i := 0; i < pemanggil; i++ {
		hitung[<-kode]++
	}

	assert.Equal(t, 1, hitung[http.StatusCreated], "tepat satu permintaan membuat brief: %v", hitung)
	assert.Equal(t, pemanggil-1, hitung[http.StatusConflict], "sisanya 409: %v", hitung)
	var brief, jobs int64
	require.NoError(t, s.db.Table("policy_brief").Where("kode = 'PB-2027-08'").Count(&brief).Error)
	require.NoError(t, s.db.Table("brief_job").Where("brief_id = (SELECT id FROM policy_brief WHERE kode = 'PB-2027-08')").Count(&jobs).Error)
	assert.EqualValues(t, 1, brief)
	assert.EqualValues(t, 1, jobs, "satu job untuk satu penyusunan")
	s.tunggu(t, "PB-2027-08", "DRAF")
}

func TestPolicyBrief_BulkMovesStatusWithPermissionsAndAudit(t *testing.T) {
	s := sharedSuite(t)
	const emailPenyetuju = "strategis.djpb@gardainflasi.id" // brief.setujui, tanpa brief.susun

	d := s.generate(t, "2027-02")
	ids := []string{"PB-2027-02", "tidak-ada"} // kode (alias ID lama) dan rujukan yang tidak ada

	statusBrief := func() string {
		var status string
		require.NoError(t, s.db.Raw("SELECT status::text FROM policy_brief WHERE kode = ?", "PB-2027-02").Scan(&status).Error)
		return status
	}

	t.Run("izin per aksi ditegakkan", func(t *testing.T) {
		telaah := map[string]any{"action": "telaah", "ids": ids}
		setujui := map[string]any{"action": "setujui", "ids": ids}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", "", telaah).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPembaca, telaah).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyetuju, telaah).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, setujui).Code)
		assert.Equal(t, "DRAF", statusBrief())
	})

	t.Run("aksi dan id tidak valid", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "hapus", "ids": ids}).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": []string{}}).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": []string{""}}).Code, "elemen kosong")
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": []string{strings.Repeat("P", 65)}}).Code, "lebih dari 64 karakter")
		assert.Equal(t, "DRAF", statusBrief())
	})

	t.Run("setujui dari DRAF dilewati", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyetuju, map[string]any{"action": "setujui", "ids": ids})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, bulkRes{Gagal: 2}, decode[bulkRes](t, w))
		assert.Equal(t, "DRAF", statusBrief())
	})

	t.Run("telaah lalu setujui, sekali jalan per tahap", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": ids})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, bulkRes{Berhasil: 1, Gagal: 1}, decode[bulkRes](t, w))
		assert.Equal(t, "TELAAH", statusBrief())

		again := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": ids})
		assert.Equal(t, bulkRes{Gagal: 2}, decode[bulkRes](t, again), "mengulang tidak mengubah apa pun")

		// Persetujuan lewat UUID (bukan kode): ID dan kode sama-sama sah sebagai rujukan.
		w = s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyetuju, map[string]any{"action": "setujui", "ids": []string{d.ID, "tidak-ada"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, bulkRes{Berhasil: 1, Gagal: 1}, decode[bulkRes](t, w))
		assert.Equal(t, "DISETUJUI", statusBrief())
	})

	t.Run("setiap perubahan tercatat tepat sekali", func(t *testing.T) {
		for aksi, aktor := range map[string]string{"BRIEF_TELAAH": uid("manajer_kebijakan"), "BRIEF_SETUJUI": uid("strategis_djpb")} {
			var rows []auditdomain.AuditLog
			require.NoError(t, s.db.Where("aksi = ? AND entitas_id = ?", aksi, d.ID).Find(&rows).Error)
			require.Len(t, rows, 1, aksi)
			require.NotNil(t, rows[0].AktorID)
			assert.Equal(t, aktor, *rows[0].AktorID)
		}
	})
}

type bulkRes struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}
