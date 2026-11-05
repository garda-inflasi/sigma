package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

const (
	// EnvAPIKey mengaktifkan LLM; tanpa nilai, brief disusun dari templat.
	EnvAPIKey = "ANTHROPIC_API_KEY"
	// EnvModel memilih model; EnvBaseURL mengganti alamat API (mis. gateway internal).
	EnvModel   = "LLM_MODEL"
	EnvBaseURL = "ANTHROPIC_BASE_URL"

	ModelBawaan   = "claude-sonnet-5-5"
	BaseURLBawaan = "https://api.anthropic.com"
	// VersiAPI adalah nilai header anthropic-version.
	VersiAPI = "2026-10-21"
	// VersiPrompt dinaikkan setiap prompt atau skema berubah; tersimpan di brief bersama hash prompt.
	VersiPrompt = "brief-v1"

	// maksToken menampung pemikiran dan jawaban; narasi brief sendiri pendek.
	maksToken = 16000
	// effort rendah cukup untuk menyusun narasi dari fakta yang sudah diberikan.
	effort        = "low"
	timeoutBawaan = 3 * time.Minute
	maksCoba      = 3
	jedaAwal      = 2 * time.Second
	maksBadanResp = 8 << 20
)

// ErrDitolakModel berarti model menolak menyusun (stop_reason "refusal").
var ErrDitolakModel = errors.New("model menolak permintaan")

// Claude menyusun narasi lewat Claude Messages API (HTTP langsung). Jawaban dipaksa
// berskema JSON (structured outputs); angka tetap hanya boleh berupa placeholder fakta.
type Claude struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
	// tidur menunggu sebelum percobaan ulang; diganti di tes.
	tidur func(ctx context.Context, d time.Duration) error
}

var _ domain.Generator = (*Claude)(nil)

// NewClaude membuat klien. model/baseURL kosong memakai bawaan; httpClient nil memakai klien dengan timeout bawaan.
func NewClaude(apiKey, model, baseURL string, httpClient *http.Client) *Claude {
	if model == "" {
		model = ModelBawaan
	}
	if baseURL == "" {
		baseURL = BaseURLBawaan
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeoutBawaan}
	}
	return &Claude{apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/"), http: httpClient, tidur: tidurKonteks}
}

// ClaudeDariEnv membuat klien dari ANTHROPIC_API_KEY (dan LLM_MODEL, ANTHROPIC_BASE_URL);
// nil bila kunci tidak ada, yaitu LLM tidak aktif.
func ClaudeDariEnv() domain.Generator {
	kunci := strings.TrimSpace(os.Getenv(EnvAPIKey))
	if kunci == "" {
		return nil
	}
	return NewClaude(kunci, strings.TrimSpace(os.Getenv(EnvModel)), strings.TrimSpace(os.Getenv(EnvBaseURL)), nil)
}

func (c *Claude) Nama() string { return "claude:" + c.model }

func tidurKonteks(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

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

type jawabanPesan struct {
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	StopDetail *struct {
		Category string `json:"category"`
	} `json:"stop_details"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Susun mengirim fakta ke Claude dan mengembalikan narasi ber-placeholder beserta jejak model dan prompt.
func (c *Claude) Susun(ctx context.Context, k domain.Konteks) (*domain.Narasi, error) {
	sistem, pengguna := BangunPrompt(k)
	badan, err := json.Marshal(map[string]any{
		"model":      c.model,
		"max_tokens": maksToken,
		"system":     sistem,
		"messages":   []map[string]any{{"role": "user", "content": pengguna}},
		"output_config": map[string]any{
			"effort": effort,
			"format": map[string]any{"type": "json_schema", "schema": skemaNarasi()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude: encode permintaan: %w", err)
	}

	jawaban, err := c.kirim(ctx, badan)
	if err != nil {
		return nil, err
	}
	switch jawaban.StopReason {
	case "refusal":
		kategori := "tidak disebutkan"
		if jawaban.StopDetail != nil && jawaban.StopDetail.Category != "" {
			kategori = jawaban.StopDetail.Category
		}
		return nil, fmt.Errorf("%w (kategori %s)", ErrDitolakModel, kategori)
	case "max_tokens":
		return nil, errors.New("claude: jawaban terpotong karena batas token")
	}

	var teks strings.Builder
	for _, blok := range jawaban.Content {
		if blok.Type == "text" {
			teks.WriteString(blok.Text)
		}
	}
	var n narasiJSON
	dec := json.NewDecoder(strings.NewReader(teks.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&n); err != nil {
		return nil, fmt.Errorf("claude: jawaban bukan JSON narasi yang valid: %w", err)
	}

	model := jawaban.Model
	if model == "" {
		model = c.model
	}
	return &domain.Narasi{
		Ringkasan: n.Ringkasan, Temuan: n.Temuan, Risiko: n.Risiko,
		RekomendasiPemda: n.RekomendasiPemda, RekomendasiTPID: n.RekomendasiTPID,
		Meta: domain.MetaNarasi{Penyusun: domain.PenyusunLLM, Model: model, PromptVersi: VersiPrompt, PromptHash: HashPrompt(sistem, pengguna)},
	}, nil
}

// kirim mengirim permintaan dengan percobaan ulang untuk galat sementara (jaringan, 429, 5xx, 529).
func (c *Claude) kirim(ctx context.Context, badan []byte) (*jawabanPesan, error) {
	var err error
	for coba := 0; coba < maksCoba; coba++ {
		if coba > 0 {
			if e := c.tidur(ctx, jedaAwal<<(coba-1)); e != nil {
				return nil, fmt.Errorf("claude: dibatalkan sebelum percobaan ulang: %w", e)
			}
		}
		var jawaban *jawabanPesan
		var ulangi bool
		jawaban, ulangi, err = c.sekali(ctx, badan)
		if err == nil {
			return jawaban, nil
		}
		if !ulangi {
			return nil, err
		}
	}
	return nil, err
}

// sekali melakukan satu panggilan; ulangi benar bila galatnya layak dicoba lagi.
func (c *Claude) sekali(ctx context.Context, badan []byte) (jawaban *jawabanPesan, ulangi bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(badan))
	if err != nil {
		return nil, false, fmt.Errorf("claude: buat permintaan: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", VersiAPI)

	res, err := c.http.Do(req)
	if err != nil {
		// Pesan galat jaringan tidak memuat header, jadi kunci API tidak bocor.
		return nil, ctx.Err() == nil, fmt.Errorf("claude: panggilan gagal: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maksBadanResp))
	if err != nil {
		return nil, ctx.Err() == nil, fmt.Errorf("claude: baca jawaban: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		ulangi = res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500
		return nil, ulangi, fmt.Errorf("claude: status %d: %s", res.StatusCode, pesanGalat(raw))
	}
	var j jawabanPesan
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, false, fmt.Errorf("claude: jawaban bukan JSON: %w", err)
	}
	return &j, false, nil
}

// pesanGalat mengambil jenis dan pesan galat dari badan jawaban galat API.
func pesanGalat(raw []byte) string {
	var e struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error.Type != "" {
		return e.Error.Type + ": " + e.Error.Message
	}
	return "badan galat tidak dikenali"
}
