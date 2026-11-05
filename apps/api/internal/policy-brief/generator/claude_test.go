package generator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kunciUji adalah kunci palsu; tes tidak pernah memanggil API sungguhan.
const kunciUji = "kunci-palsu-untuk-tes"

func konteksUji() domain.Konteks {
	return domain.Konteks{
		Periode: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Fakta: []domain.Fakta{
			{ID: "PERIODE", Label: "Periode brief", Nilai: "November 2026"},
			{ID: "INFLASI_YOY", Label: "Inflasi tahunan", Nilai: "2,84%", Sumber: "inflasi_bulanan (BPS)"},
		},
	}
}

const jawabanNarasi = `{"ringkasan":"Inflasi tahunan {{INFLASI_YOY}}.","temuan":["a"],"risiko":["b"],"rekomendasi_pemda":["c"],"rekomendasi_tpid":["d"]}`

// jawabanAPI membungkus teks keluaran model dalam badan jawaban Messages API.
func jawabanAPI(t *testing.T, stopReason string, bagian ...map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"id": "msg_uji", "type": "message", "role": "assistant", "model": "claude-opus-5-5-diverifikasi",
		"content": bagian, "stop_reason": stopReason,
	})
	require.NoError(t, err)
	return string(raw)
}

func teks(s string) map[string]any { return map[string]any{"type": "text", "text": s} }

// serverUji menjalankan handler palsu dan mencatat setiap permintaan yang masuk.
type serverUji struct {
	*httptest.Server
	mu      sync.Mutex
	header  []http.Header
	badan   [][]byte
	jalur   []string
	respons func(panggilan int) (status int, badan string)
}

func newServerUji(t *testing.T, respons func(panggilan int) (int, string)) *serverUji {
	t.Helper()
	s := &serverUji{respons: respons}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.header = append(s.header, r.Header.Clone())
		s.badan = append(s.badan, raw)
		s.jalur = append(s.jalur, r.Method+" "+r.URL.Path)
		panggilan := len(s.badan)
		s.mu.Unlock()
		status, badan := s.respons(panggilan)
		w.Header().Set("content-type", "application/json")
		// Jeda percobaan ulang SDK dibuat nol agar tes cepat dan deterministik.
		w.Header().Set("retry-after-ms", "0")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, badan)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *serverUji) jumlah() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.badan)
}

func klienUji(s *serverUji) *Claude { return NewClaude(kunciUji, "", s.URL) }

func TestClaude_MengirimPermintaanSesuaiKontrak(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) { return 200, jawabanAPI(t, "end_turn", teks(jawabanNarasi)) })
	c := klienUji(s)
	k := konteksUji()

	n, err := c.Susun(t.Context(), k)

	require.NoError(t, err)
	require.Equal(t, 1, s.jumlah())
	assert.Equal(t, []string{"POST /v1/messages"}, s.jalur)

	h := s.header[0]
	assert.Equal(t, "Bearer "+kunciUji, h.Get("authorization"))
	assert.Equal(t, "2026-10-21", h.Get("anthropic-version"))
	assert.Equal(t, "application/json", h.Get("content-type"))
	assert.Equal(t, "server-side-fallback-2026-07-01", h.Get("anthropic-beta"), "fallback server-side saat ditolak")

	var badan map[string]any
	require.NoError(t, json.Unmarshal(s.badan[0], &badan))
	assert.Equal(t, "claude-opus-5-5", badan["model"], "model bawaan")
	assert.Equal(t, "default", badan["fallbacks"])
	assert.EqualValues(t, maksToken, badan["max_tokens"])
	for _, dilarang := range []string{"temperature", "top_p", "top_k", "thinking", "tool_choice", "tools", "stream"} {
		assert.NotContains(t, badan, dilarang, "parameter %s ditolak/tidak perlu pada model ini", dilarang)
	}
	assert.NotContains(t, string(s.badan[0]), kunciUji, "kunci API tidak boleh ikut ke badan permintaan")

	sistem, pengguna := BangunPrompt(k)
	assert.Equal(t, []any{map[string]any{"type": "text", "text": sistem}}, badan["system"])
	pesan := badan["messages"].([]any)
	require.Len(t, pesan, 1, "tanpa prefill")
	assert.Equal(t, map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": pengguna}}}, pesan[0])

	cfg := badan["output_config"].(map[string]any)
	assert.Equal(t, "low", cfg["effort"])
	format := cfg["format"].(map[string]any)
	assert.Equal(t, "json_schema", format["type"])
	skema := format["schema"].(map[string]any)
	assert.Equal(t, false, skema["additionalProperties"])
	assert.ElementsMatch(t, []any{"ringkasan", "temuan", "risiko", "rekomendasi_pemda", "rekomendasi_tpid"}, skema["required"])

	assert.Equal(t, "Inflasi tahunan {{INFLASI_YOY}}.", n.Ringkasan, "placeholder tetap utuh sampai Render")
	assert.Equal(t, []string{"a"}, n.Temuan)
	assert.Equal(t, []string{"b"}, n.Risiko)
	assert.Equal(t, []string{"c"}, n.RekomendasiPemda)
	assert.Equal(t, []string{"d"}, n.RekomendasiTPID)
	assert.Equal(t, domain.MetaNarasi{
		Penyusun: domain.PenyusunLLM, Model: "claude-opus-5-5-diverifikasi", PromptVersi: VersiPrompt, PromptHash: HashPrompt(sistem, pengguna),
	}, n.Meta, "model dari jawaban API, hash dari prompt yang benar-benar dikirim")
	assert.Len(t, n.Meta.PromptHash, 64)
}

func TestClaude_ModelDariKonfigurasiDanCadanganBilaJawabanTanpaModel(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) {
		return 200, `{"stop_reason":"end_turn","content":[{"type":"text","text":` + jsonString(jawabanNarasi) + `}]}`
	})
	c := NewClaude(kunciUji, "claude-model-lain", s.URL)

	n, err := c.Susun(t.Context(), konteksUji())

	require.NoError(t, err)
	assert.Equal(t, "claude-model-lain", n.Meta.Model)
	var badan map[string]any
	require.NoError(t, json.Unmarshal(s.badan[0], &badan))
	assert.Equal(t, "claude-model-lain", badan["model"])
	assert.NotContains(t, badan, "fallbacks", "fallback hanya untuk model yang mendukungnya")
	assert.Empty(t, s.header[0].Get("anthropic-beta"))
	assert.Equal(t, "claude:claude-model-lain", c.Nama())
}

func jsonString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func TestClaude_MengabaikanBlokThinkingDanMembacaTeks(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) {
		return 200, jawabanAPI(t, "end_turn", map[string]any{"type": "thinking", "thinking": ""}, teks(jawabanNarasi))
	})
	c := klienUji(s)

	n, err := c.Susun(t.Context(), konteksUji())

	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, n.Temuan)
}

func TestClaude_PenolakanModel(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"stop_reason": "refusal", "stop_details": map[string]any{"type": "refusal", "category": "general_harms"},
		"content": []map[string]any{teks("")},
	})
	require.NoError(t, err)
	s := newServerUji(t, func(int) (int, string) { return 200, string(raw) })
	c := klienUji(s)

	_, err = c.Susun(t.Context(), konteksUji())

	require.ErrorIs(t, err, ErrDitolakModel)
	assert.ErrorContains(t, err, "general_harms")
	assert.Equal(t, 1, s.jumlah(), "penolakan tidak diulang")
}

func TestClaude_JawabanTerpotongAtauRusakDitolak(t *testing.T) {
	kasus := map[string]string{
		"terpotong":        jawabanAPI(t, "max_tokens", teks(`{"ringkasan":"x`)),
		"bukan JSON":       jawabanAPI(t, "end_turn", teks("Berikut draf brief:")),
		"properti asing":   jawabanAPI(t, "end_turn", teks(`{"ringkasan":"x","temuan":[],"risiko":[],"rekomendasi_pemda":[],"rekomendasi_tpid":[],"tambahan":1}`)),
		"tanpa blok teks":  jawabanAPI(t, "end_turn"),
		"badan bukan JSON": `<html>`,
	}
	for nama, badan := range kasus {
		t.Run(nama, func(t *testing.T) {
			s := newServerUji(t, func(int) (int, string) { return 200, badan })
			c := klienUji(s)

			n, err := c.Susun(t.Context(), konteksUji())

			assert.Error(t, err)
			assert.Nil(t, n)
		})
	}
}

func TestClaude_GalatSementaraDiulang(t *testing.T) {
	s := newServerUji(t, func(panggilan int) (int, string) {
		switch panggilan {
		case 1:
			return 429, `{"type":"error","error":{"type":"rate_limit_error","message":"terlalu banyak"}}`
		case 2:
			return 529, `{"type":"error","error":{"type":"overloaded_error","message":"sibuk"}}`
		}
		return 200, jawabanAPI(t, "end_turn", teks(jawabanNarasi))
	})

	n, err := klienUji(s).Susun(t.Context(), konteksUji())

	require.NoError(t, err)
	assert.NotNil(t, n)
	assert.Equal(t, 3, s.jumlah())
}

func TestClaude_BerhentiSetelahBatasPercobaan(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) {
		return 500, `{"type":"error","error":{"type":"api_error","message":"galat internal"}}`
	})

	_, err := klienUji(s).Susun(t.Context(), konteksUji())

	require.Error(t, err)
	assert.Equal(t, 3, s.jumlah(), "satu panggilan dan dua percobaan ulang")
	assert.ErrorContains(t, err, "status 500: api_error: galat internal")
	assert.NotContains(t, err.Error(), kunciUji)
}

func TestClaude_GalatPermintaanTidakDiulang(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404} {
		s := newServerUji(t, func(int) (int, string) {
			return status, `{"type":"error","error":{"type":"invalid_request_error","message":"model tidak didukung"}}`
		})

		_, err := klienUji(s).Susun(t.Context(), konteksUji())

		require.Error(t, err, status)
		assert.Equal(t, 1, s.jumlah(), "status %d tidak layak diulang", status)
		assert.ErrorContains(t, err, "invalid_request_error: model tidak didukung")
		assert.NotContains(t, err.Error(), kunciUji)
	}
}

func TestClaude_BadanGalatTakDikenaliTetapDilaporkan(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) { return 400, "teks biasa" })

	_, err := klienUji(s).Susun(t.Context(), konteksUji())

	assert.ErrorContains(t, err, "status 400: badan galat tidak dikenali")
}

func TestClaude_LayananMatiGagalTanpaMembocorkanKunci(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) { return 200, "" })
	url := s.URL
	s.Close()
	c := NewClaude(kunciUji, "", url, option.WithMaxRetries(0))

	_, err := c.Susun(t.Context(), konteksUji())

	require.Error(t, err)
	assert.ErrorContains(t, err, "claude: panggilan gagal")
	assert.NotContains(t, err.Error(), kunciUji)
}

func TestClaude_KonteksBatalMenghentikanPanggilan(t *testing.T) {
	s := newServerUji(t, func(int) (int, string) { return 200, jawabanAPI(t, "end_turn", teks(jawabanNarasi)) })
	ctx, batal := context.WithCancel(t.Context())
	batal()

	_, err := klienUji(s).Susun(ctx, konteksUji())

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, s.jumlah())
}

func TestClaudeDariEnv(t *testing.T) {
	t.Run("tanpa kunci LLM tidak aktif", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "")
		t.Setenv(EnvModel, "claude-apa-saja")
		assert.Nil(t, ClaudeDariEnv())
	})
	t.Run("kunci berisi spasi saja tetap tidak aktif", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "   ")
		assert.Nil(t, ClaudeDariEnv())
	})
	t.Run("bawaan model", func(t *testing.T) {
		t.Setenv(EnvAPIKey, kunciUji)
		t.Setenv(EnvModel, "")
		t.Setenv(EnvBaseURL, "")

		c, ok := ClaudeDariEnv().(*Claude)

		require.True(t, ok)
		assert.Equal(t, "claude-opus-5-5", c.model)
		assert.Equal(t, "claude:claude-opus-5-5", c.Nama())
	})
	t.Run("model, kunci, dan alamat dari env", func(t *testing.T) {
		s := newServerUji(t, func(int) (int, string) { return 200, jawabanAPI(t, "end_turn", teks(jawabanNarasi)) })
		t.Setenv(EnvAPIKey, " "+kunciUji+" ")
		t.Setenv(EnvModel, " claude-lain ")
		t.Setenv(EnvBaseURL, s.URL)

		c, ok := ClaudeDariEnv().(*Claude)
		require.True(t, ok)
		_, err := c.Susun(t.Context(), konteksUji())

		require.NoError(t, err)
		assert.Equal(t, "claude-lain", c.model)
		require.Equal(t, 1, s.jumlah(), "permintaan dikirim ke alamat dari env")
		assert.Equal(t, "Bearer "+kunciUji, s.header[0].Get("authorization"))
	})
}

func TestBangunPrompt_MemuatSemuaFaktaSebagaiPlaceholderDanDeterministik(t *testing.T) {
	k := konteksUji()

	sistem1, pengguna1 := BangunPrompt(k)
	sistem2, pengguna2 := BangunPrompt(k)

	assert.Equal(t, sistem1, sistem2)
	assert.Equal(t, pengguna1, pengguna2)
	assert.Contains(t, pengguna1, "{{PERIODE}} | Periode brief | November 2026 | -")
	assert.Contains(t, pengguna1, "{{INFLASI_YOY}} | Inflasi tahunan | 2,84% | inflasi_bulanan (BPS)")
	assert.Contains(t, sistem1, "Jangan menulis angka apa pun sendiri")
	assert.Contains(t, sistem1, "Pakai hanya fakta yang diberikan")
	assert.NotEqual(t, HashPrompt(sistem1, pengguna1), HashPrompt(sistem1, pengguna1+" "))

	k.Fakta[1].Nilai = "3,01%"
	_, pengguna3 := BangunPrompt(k)
	assert.NotEqual(t, HashPrompt(sistem1, pengguna1), HashPrompt(sistem1, pengguna3), "hash berubah bila data berubah")
}
