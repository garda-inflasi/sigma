package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	eksekutif "github.com/garda-inflasi/api/internal/eksekutif/domain"
)

// Permintaan adalah satu permintaan HTTP GET yang dirancang konektor. Konektor
// murni: ia hanya merancang permintaan dan mem-parse respons; pengambilan (retry,
// pembatasan laju, circuit breaker, robots.txt) dilakukan engine lewat Fetcher.
type Permintaan struct {
	// Kunci mengenali permintaan di bronze dan log (tanpa rahasia), mis. "wilayah=1171".
	Kunci string
	URL   string
	// Header tambahan; kunci API sebaiknya di URL/Header, bukan di Kunci.
	Header http.Header
	// Samarkan memuat potongan rahasia pada URL (mis. kunci API di jalur) yang harus
	// disamarkan dari pesan galat dan log.
	Samarkan []string
	// SumberID (UUID sumber_data) dan PatuhRobots diisi engine.
	SumberID    string
	PatuhRobots bool
}

// Dokumen adalah satu respons yang sudah diambil.
type Dokumen struct {
	Kunci  string
	Konten []byte
	// DiambilPada adalah waktu respons diterima (untuk menghitung umur data).
	DiambilPada time.Time
}

// Fetcher mengambil badan respons. Implementasi nyata: engine.HTTPFetcher; tes
// menyuntikkan fixture sehingga tidak memanggil internet.
type Fetcher interface {
	Get(ctx context.Context, p Permintaan) ([]byte, error)
}

// Masukan adalah bahan konektor untuk satu eksekusi.
type Masukan struct {
	// SumberID adalah UUID baris sumber_data yang sedang dijalankan; konektor menulisnya pada baris silver
	// yang bersumber langsung dari sumber itu (HargaEksternal.SumberID).
	SumberID string
	Sekarang time.Time
	// URLDasar dari sumber_data.url_dasar.
	URLDasar string
	// Konfigurasi dari sumber_data.konfigurasi.
	Konfigurasi json.RawMessage
	Pemetaan    *Pemetaan
}

// Hasil adalah keluaran normalisasi satu dokumen. Baris yang tak bisa dipakai
// dihitung, tidak dibuang diam-diam: lihat TakTerpetakan dan TidakValid.
type Hasil struct {
	Harga   []HargaEksternal
	Cuaca   []CuacaHarian
	Inflasi []eksekutif.InflasiBulanan
	Andil   []eksekutif.AndilInflasi

	// TakTerpetakan: "JENIS:alias" -> jumlah baris yang aliasnya tidak ada di pemetaan_sumber.
	TakTerpetakan map[string]int
	// TidakValid: alasan -> jumlah baris (nilai kosong, bukan angka, di luar batas, hari tak lengkap, dst).
	TidakValid map[string]int
}

// TambahTakTerpetakan mencatat satu baris dengan alias yang belum dipetakan.
func (h *Hasil) TambahTakTerpetakan(jenis, alias string) {
	if h.TakTerpetakan == nil {
		h.TakTerpetakan = map[string]int{}
	}
	h.TakTerpetakan[jenis+":"+alias]++
}

// TambahTidakValid mencatat satu baris yang ditolak karena alasan tertentu.
func (h *Hasil) TambahTidakValid(alasan string) {
	if h.TidakValid == nil {
		h.TidakValid = map[string]int{}
	}
	h.TidakValid[alasan]++
}

// Gabung menambahkan isi o ke h.
func (h *Hasil) Gabung(o Hasil) {
	h.Harga = append(h.Harga, o.Harga...)
	h.Cuaca = append(h.Cuaca, o.Cuaca...)
	h.Inflasi = append(h.Inflasi, o.Inflasi...)
	h.Andil = append(h.Andil, o.Andil...)
	for k, n := range o.TakTerpetakan {
		if h.TakTerpetakan == nil {
			h.TakTerpetakan = map[string]int{}
		}
		h.TakTerpetakan[k] += n
	}
	for k, n := range o.TidakValid {
		if h.TidakValid == nil {
			h.TidakValid = map[string]int{}
		}
		h.TidakValid[k] += n
	}
}

// Diambil adalah jumlah baris silver yang dihasilkan.
func (h *Hasil) Diambil() int { return len(h.Harga) + len(h.Cuaca) + len(h.Inflasi) + len(h.Andil) }

// JumlahTakTerpetakan dan JumlahTidakValid menjumlah baris yang ditolak per kategori.
func (h *Hasil) JumlahTakTerpetakan() int { return jumlah(h.TakTerpetakan) }
func (h *Hasil) JumlahTidakValid() int    { return jumlah(h.TidakValid) }

func jumlah(m map[string]int) int {
	total := 0
	for _, n := range m {
		total += n
	}
	return total
}

// Konektor adalah adaptor satu sumber data.
type Konektor interface {
	// Kode adalah kode baris sumber_data yang dilayani (sumber_data.kode, mis. SRC-BI): kunci konfigurasi
	// konektor, bukan ID (UUID) baris.
	Kode() string
	// Permintaan merancang permintaan yang harus diambil. Galat bila konfigurasi
	// atau kredensial belum lengkap (pesan menjelaskan apa yang kurang).
	Permintaan(in Masukan) ([]Permintaan, error)
	// Normalisasi mengubah satu respons menjadi baris silver. Galat hanya untuk
	// respons yang tak bisa dibaca sama sekali; baris cacat dilaporkan di Hasil.
	Normalisasi(in Masukan, dok Dokumen) (Hasil, error)
}

// Penggabung adalah Konektor yang barisnya baru lengkap setelah semua dokumen sebuah run
// terkumpul (mis. beberapa variabel yang membentuk satu baris). Engine tetap menyimpan tiap
// dokumen ke bronze dan memanggil Normalisasi per dokumen untuk memeriksanya; baris silver
// dirakit sekali lewat Gabungkan dari dokumen yang lolos pemeriksaan.
type Penggabung interface {
	Konektor
	Gabungkan(in Masukan, dok []Dokumen) (Hasil, error)
}

// Pencarian adalah hasil mencari alias di pemetaan.
type Pencarian int

const (
	// TakTerpetakan: alias belum ada di pemetaan_sumber.
	TakTerpetakan Pencarian = iota
	// Terpetakan: alias dipetakan ke kode master.
	Terpetakan
	// Diabaikan: alias sengaja diabaikan (mis. baris induk kategori).
	Diabaikan
)

// Pemetaan menyimpan alias sumber -> kode master untuk satu sumber.
type Pemetaan struct {
	entri  map[string]PemetaanSumber
	satuan map[string]string
}

// NormalisasiAlias menyeragamkan alias: huruf kecil, spasi tunggal, tanpa spasi tepi.
func NormalisasiAlias(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// NewPemetaan membangun Pemetaan dari baris pemetaan_sumber dan satuan komoditas master.
func NewPemetaan(rows []PemetaanSumber, satuanKomoditas map[string]string) *Pemetaan {
	p := &Pemetaan{entri: make(map[string]PemetaanSumber, len(rows)), satuan: satuanKomoditas}
	for _, r := range rows {
		p.entri[r.Jenis+"\x00"+NormalisasiAlias(r.Alias)] = r
	}
	return p
}

// Cari mengembalikan kode master untuk alias. Kode kosong kecuali Terpetakan.
func (p *Pemetaan) Cari(jenis, alias string) (string, Pencarian) {
	r, ok := p.entri[jenis+"\x00"+NormalisasiAlias(alias)]
	switch {
	case !ok:
		return "", TakTerpetakan
	case r.Diabaikan || r.KodeMaster == nil:
		return "", Diabaikan
	default:
		return *r.KodeMaster, Terpetakan
	}
}

// Satuan adalah satuan komoditas di master; kosong bila komoditas tak dikenal.
func (p *Pemetaan) Satuan(kodeKomoditas string) string { return p.satuan[kodeKomoditas] }

// Wilayah adalah satu alias wilayah yang terpetakan.
type Wilayah struct {
	Alias string
	Kode  string
}

// SemuaWilayah mengembalikan alias wilayah yang terpetakan (bukan yang diabaikan),
// terurut menurut alias agar rancangan permintaan deterministik.
func (p *Pemetaan) SemuaWilayah() []Wilayah {
	var res []Wilayah
	for _, r := range p.entri {
		if r.Jenis == JenisWilayah && !r.Diabaikan && r.KodeMaster != nil {
			res = append(res, Wilayah{Alias: r.Alias, Kode: *r.KodeMaster})
		}
	}
	slices.SortFunc(res, func(a, b Wilayah) int { return strings.Compare(a.Alias, b.Alias) })
	return res
}
