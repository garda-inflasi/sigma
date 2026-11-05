package domain

import (
	"context"
	"time"
)

// Label draf yang tampil di UI dan tersimpan di konten, menurut asal narasinya.
const (
	LabelDrafAI      = "Draf AI — wajib ditelaah manusia"
	LabelDrafTemplat = "Draf otomatis dari templat — wajib ditelaah manusia"
	LabelDemo        = "Data demo — bukan brief resmi"
)

// LabelPenyusun mengembalikan label draf untuk asal narasi (kolom sumber_narasi).
func LabelPenyusun(penyusun string) string {
	switch penyusun {
	case PenyusunLLM:
		return LabelDrafAI
	case PenyusunDemo:
		return LabelDemo
	default:
		return LabelDrafTemplat
	}
}

// Fakta adalah satu angka atau istilah dari data nyata yang boleh dipakai narasi.
// Narasi menyebutnya hanya lewat placeholder {{ID}}; angka tidak pernah ditulis
// bebas oleh LLM atau templat.
type Fakta struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Nilai adalah teks yang disisipkan ke narasi, sudah diformat (mis. "2,84%").
	Nilai string `json:"nilai"`
	// Sumber menyebut asal data (tabel dan jenis sumbernya); kosong untuk fakta penanda
	// seperti periode dan wilayah, yang tidak masuk lampiran sitasi.
	Sumber string `json:"sumber,omitempty"`
}

// Konteks adalah ringkasan deterministik data nyata untuk satu periode; satu-satunya
// bahan yang boleh dipakai Generator.
type Konteks struct {
	// Periode adalah awal bulan brief.
	Periode time.Time `json:"periode"`
	Fakta   []Fakta   `json:"fakta"`
}

// Cari mengembalikan fakta menurut id.
func (k Konteks) Cari(id string) (Fakta, bool) {
	for _, f := range k.Fakta {
		if f.ID == id {
			return f, true
		}
	}
	return Fakta{}, false
}

// MetaNarasi adalah jejak penyusun narasi.
type MetaNarasi struct {
	Penyusun    string
	Model       string
	PromptVersi string
	PromptHash  string
}

// Narasi adalah keluaran Generator: teks per bagian yang masih berisi placeholder {{ID}}.
type Narasi struct {
	Ringkasan        string
	Temuan           []string
	Risiko           []string
	RekomendasiPemda []string
	RekomendasiTPID  []string
	Meta             MetaNarasi
}

// BagianKonten adalah satu bagian brief yang sudah final: butir-butir teks.
type BagianKonten struct {
	Kode  string   `json:"kode"`
	Judul string   `json:"judul"`
	Butir []string `json:"butir"`
}

// Konten adalah isi brief yang tersimpan (policy_brief.konten): narasi yang placeholder-nya
// sudah diganti nilai fakta, dan lampiran sitasi sumber untuk fakta yang dipakai.
type Konten struct {
	Label    string         `json:"label"`
	Penyusun string         `json:"penyusun"`
	Catatan  string         `json:"catatan,omitempty"`
	Bagian   []BagianKonten `json:"bagian"`
	Lampiran []Fakta        `json:"lampiran"`
}

// Kode bagian brief.
const (
	BagianRingkasan        = "ringkasan"
	BagianTemuan           = "temuan"
	BagianRisiko           = "risiko"
	BagianRekomendasiPemda = "rekomendasi_pemda"
	BagianRekomendasiTPID  = "rekomendasi_tpid"
)

// Butir mengembalikan butir-butir bagian menurut kode; nil bila tidak ada.
func (k Konten) Butir(kode string) []string {
	for _, b := range k.Bagian {
		if b.Kode == kode {
			return b.Butir
		}
	}
	return nil
}

// Generator menyusun narasi brief dari Konteks. Implementasi: templat deterministik
// (cadangan, selalu tersedia) dan LLM (aktif hanya bila kunci API dikonfigurasi).
type Generator interface {
	// Nama dipakai di log dan audit, mis. "templat" atau "claude:claude-sonnet-5-5".
	Nama() string
	Susun(ctx context.Context, k Konteks) (*Narasi, error)
}
