package generator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/anthropics/anthropic-sdk-go/v2/shared/constant"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

const (
	// EnvAPIKey mengaktifkan LLM; tanpa nilai, brief disusun dari templat.
	EnvAPIKey = "ANTHROPIC_API_KEY"
	// EnvModel memilih model; EnvBaseURL mengganti alamat API (mis. gateway internal).
	EnvModel   = "LLM_MODEL"
	EnvBaseURL = "ANTHROPIC_BASE_URL"

	ModelBawaan = "claude-opus-5-5"
	// VersiPrompt dinaikkan setiap prompt atau skema berubah; tersimpan di brief bersama hash prompt.
	VersiPrompt = "brief-v1"

	// maksToken menampung pemikiran dan jawaban; narasi brief sendiri pendek.
	maksToken = 16000
	// timeoutPanggilan membatasi satu percobaan; SDK mengulang galat sementara
	// (jaringan, 408, 409, 429, 5xx) dua kali dengan jeda bertambah.
	timeoutPanggilan = 3 * time.Minute
)

// modelDenganFallback adalah model yang menerima fallback server-side "default": bila
// pengaman model menolak permintaan, API menjalankannya ulang pada model pengganti yang
// direkomendasikan untuk kategori penolakan itu, dalam panggilan yang sama.
var modelDenganFallback = map[string]bool{
	"claude-fable-5-1": true, "claude-opus-5-5": true, "claude-opus-5": true, "claude-sonnet-5-5": true,
}

// ErrDitolakModel berarti model menolak menyusun (stop_reason "refusal").
var ErrDitolakModel = errors.New("model menolak permintaan")

// Claude menyusun narasi lewat Claude Messages API memakai SDK Go resmi. Jawaban dipaksa
// berskema JSON (structured outputs); angka tetap hanya boleh berupa placeholder fakta.
type Claude struct {
	client anthropic.Client
	model  string
}

var _ domain.Generator = (*Claude)(nil)

// NewClaude membuat klien dengan kunci API eksplisit; tidak ada kredensial yang dibaca
// dari lingkungan atau profil. model/baseURL kosong memakai bawaan. opsi tambahan
// (mis. option.WithHTTPClient) diterapkan terakhir.
func NewClaude(apiKey, model, baseURL string, opsi ...option.RequestOption) *Claude {
	if model == "" {
		model = ModelBawaan
	}
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(timeoutPanggilan),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &Claude{client: anthropic.NewClient(append(opts, opsi...)...), model: model}
}

// ClaudeDariEnv membuat klien dari ANTHROPIC_API_KEY (dan LLM_MODEL, ANTHROPIC_BASE_URL);
// nil bila kunci tidak ada, yaitu LLM tidak aktif.
func ClaudeDariEnv() domain.Generator {
	kunci := strings.TrimSpace(os.Getenv(EnvAPIKey))
	if kunci == "" {
		return nil
	}
	return NewClaude(kunci, strings.TrimSpace(os.Getenv(EnvModel)), strings.TrimSpace(os.Getenv(EnvBaseURL)))
}

func (c *Claude) Nama() string { return "claude:" + c.model }

const promptSistem = `Kamu adalah analis kebijakan yang menulis draf policy brief pengendalian inflasi pangan untuk Pemerintah Daerah dan Tim Pengendalian Inflasi Daerah (TPID) di Aceh. Tulis dalam bahasa Indonesia baku, ringkas, dan netral.

Aturan ketat:
1. Semua angka, persentase, nilai rupiah, nama komoditas, nama wilayah, dan nama periode wajib ditulis memakai placeholder berbentuk {{ID}} yang diambil dari daftar fakta. Jangan menulis angka apa pun sendiri, baik berupa digit maupun hasil perhitunganmu, dan jangan menghitung, membulatkan, atau membandingkan angka.
2. Pakai hanya fakta yang diberikan. Jangan menambah fakta, penyebab, angka, atau sumber lain. Bila data untuk sebuah topik tidak ada dalam daftar fakta, katakan bahwa datanya belum tersedia, tanpa angka.
3. Jangan membahas dirimu atau proses penyusunan. Draf ini akan ditelaah manusia.
4. Keluarkan hanya JSON sesuai skema yang diminta.`

const petunjukBagian = `Bagian yang diminta:
- ringkasan: dua sampai empat kalimat.
- temuan: tiga sampai enam butir, tiap butir satu atau dua kalimat.
- risiko: dua sampai lima butir.
- rekomendasi_pemda: tiga sampai lima butir tindakan konkret untuk Pemerintah Daerah.
- rekomendasi_tpid: dua sampai empat butir tindakan konkret untuk TPID.`

// BangunPrompt menyusun prompt sistem dan pesan pengguna dari Konteks. Murni dan deterministik.
func BangunPrompt(k domain.Konteks) (sistem, pengguna string) {
	var b strings.Builder
	b.WriteString("Susun draf policy brief dari fakta berikut. Setiap baris fakta berbentuk: {{ID}} | label | nilai | sumber. ")
	b.WriteString("Sebut nilai hanya dengan menulis {{ID}}-nya; nilai di bawah ini hanya untuk membantumu memahami arah dan konteks.\n\nFAKTA:\n")
	for _, f := range k.Fakta {
		sumber := f.Sumber
		if sumber == "" {
			sumber = "-"
		}
		fmt.Fprintf(&b, "{{%s}} | %s | %s | %s\n", f.ID, f.Label, f.Nilai, sumber)
	}
	b.WriteString("\n")
	b.WriteString(petunjukBagian)
	return promptSistem, b.String()
}

// HashPrompt adalah SHA-256 (heksadesimal) dari prompt sistem dan pesan pengguna yang dikirim.
func HashPrompt(sistem, pengguna string) string {
	h := sha256.Sum256([]byte(sistem + "\n\n" + pengguna))
	return hex.EncodeToString(h[:])
}

// skemaNarasi adalah skema JSON jawaban (structured outputs): semua bagian wajib, tanpa properti lain.
func skemaNarasi() map[string]any {
	daftar := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ringkasan":         map[string]any{"type": "string"},
			"temuan":            daftar,
			"risiko":            daftar,
			"rekomendasi_pemda": daftar,
			"rekomendasi_tpid":  daftar,
		},
		"required":             []string{"ringkasan", "temuan", "risiko", "rekomendasi_pemda", "rekomendasi_tpid"},
		"additionalProperties": false,
	}
}

type narasiJSON struct {
	Ringkasan        string   `json:"ringkasan"`
	Temuan           []string `json:"temuan"`
	Risiko           []string `json:"risiko"`
	RekomendasiPemda []string `json:"rekomendasi_pemda"`
	RekomendasiTPID  []string `json:"rekomendasi_tpid"`
}

// Susun mengirim fakta ke Claude dan mengembalikan narasi ber-placeholder beserta jejak model dan prompt.
func (c *Claude) Susun(ctx context.Context, k domain.Konteks) (*domain.Narasi, error) {
	sistem, pengguna := BangunPrompt(k)
	params := anthropic.BetaMessageNewParams{
		Model:     c.model,
		MaxTokens: maksToken,
		System:    []anthropic.BetaTextBlockParam{{Text: sistem}},
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(pengguna))},
		OutputConfig: anthropic.BetaOutputConfigParam{
			// Effort rendah cukup untuk menyusun narasi dari fakta yang sudah diberikan.
			Effort: anthropic.BetaOutputConfigEffortLow,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: skemaNarasi()},
		},
	}
	if modelDenganFallback[c.model] {
		params.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
	}

	jawaban, err := c.client.Beta.Messages.New(ctx, params)
	if err != nil {
		return nil, galatPanggilan(err)
	}
	switch jawaban.StopReason {
	case anthropic.BetaStopReasonRefusal:
		kategori := string(jawaban.StopDetails.Category)
		if kategori == "" {
			kategori = "tidak disebutkan"
		}
		return nil, fmt.Errorf("%w (kategori %s)", ErrDitolakModel, kategori)
	case anthropic.BetaStopReasonMaxTokens:
		return nil, errors.New("claude: jawaban terpotong karena batas token")
	}

	var teks strings.Builder
	for _, blok := range jawaban.Content {
		if t, ok := blok.AsAny().(anthropic.BetaTextBlock); ok {
			teks.WriteString(t.Text)
		}
	}
	var n narasiJSON
	dec := json.NewDecoder(strings.NewReader(teks.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&n); err != nil {
		return nil, fmt.Errorf("claude: jawaban bukan JSON narasi yang valid: %w", err)
	}

	model := string(jawaban.Model)
	if model == "" {
		model = c.model
	}
	return &domain.Narasi{
		Ringkasan: n.Ringkasan, Temuan: n.Temuan, Risiko: n.Risiko,
		RekomendasiPemda: n.RekomendasiPemda, RekomendasiTPID: n.RekomendasiTPID,
		Meta: domain.MetaNarasi{Penyusun: domain.PenyusunLLM, Model: model, PromptVersi: VersiPrompt, PromptHash: HashPrompt(sistem, pengguna)},
	}, nil
}

// galatPanggilan merumuskan galat SDK tanpa memuat kunci API: galat dari API menyebut
// status, jenis, dan pesannya; galat lain (jaringan, konteks batal) dibungkus apa adanya.
func galatPanggilan(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		jenis := string(apiErr.Type())
		if jenis == "" {
			return fmt.Errorf("claude: status %d: badan galat tidak dikenali", apiErr.StatusCode)
		}
		return fmt.Errorf("claude: status %d: %s: %s", apiErr.StatusCode, jenis, apiErr.Message)
	}
	return fmt.Errorf("claude: panggilan gagal: %w", err)
}
