// Package bapanas adalah konektor Panel Harga Pangan Badan Pangan Nasional (Bapanas):
// mengumpulkan harga pangan strategis di tingkat konsumen dan produsen per kabupaten/kota
// dan menormalkannya ke tabel harga_eksternal.
package bapanas

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/garda-inflasi/api/internal/ingestion/connector/kit"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// Kode adalah kode baris sumber_data Panel Harga Bapanas (sumber_data.kode).
const Kode = "SRC-BAPANAS"

// TipeHargaDefault adalah tipe harga bawaan (KONSUMEN).
const TipeHargaDefault = "KONSUMEN"

const (
	jalurHarga = "/api/harga-pangan-harian"
	kunciProv  = "provinsi=11" // 11 = Provinsi Aceh
	hargaMaks  = 10_000_000
)

type konfigurasi struct {
	ProvinsiID string `json:"provinsi_id"`
	TipeHarga  string `json:"tipe_harga"`
}

// Konektor adalah adaptor untuk Panel Harga Bapanas.
type Konektor struct{}

// New membuat konektor Bapanas baru.
func New() *Konektor {
	return &Konektor{}
}

func (*Konektor) Kode() string { return Kode }

func (*Konektor) Permintaan(in domain.Masukan) ([]domain.Permintaan, error) {
	if in.URLDasar == "" {
		return nil, fmt.Errorf("url_dasar %s belum diisi", Kode)
	}

	provID := "11"
	if len(in.Konfigurasi) > 0 {
		var cfg konfigurasi
		_ = json.Unmarshal(in.Konfigurasi, &cfg)
		if cfg.ProvinsiID != "" {
			provID = cfg.ProvinsiID
		}
	}

	q := url.Values{"provinsi": {provID}}
	return []domain.Permintaan{
		{
			Kunci: kunciProv,
			URL:   strings.TrimRight(in.URLDasar, "/") + jalurHarga + "?" + q.Encode(),
		},
	}, nil
}

type itemHarga struct {
	NamaKomoditas string   `json:"nama_komoditas"`
	NamaKabupaten string   `json:"nama_kabupaten"`
	KabupatenID   kit.Teks `json:"kabupaten_id"`
	Tanggal       string   `json:"tanggal"`
	Harga         kit.Teks `json:"harga"`
	Satuan        string   `json:"satuan"`
}

type respons struct {
	Status string      `json:"status"`
	Pesan  string      `json:"message"`
	Data   []itemHarga `json:"data"`
}

func (*Konektor) Normalisasi(in domain.Masukan, dok domain.Dokumen) (domain.Hasil, error) {
	var hasil domain.Hasil

	var resp respons
	if err := json.Unmarshal(dok.Konten, &resp); err != nil {
		return hasil, fmt.Errorf("unmarshal JSON: %w", err)
	}

	tipe := TipeHargaDefault
	if len(in.Konfigurasi) > 0 {
		var cfg konfigurasi
		_ = json.Unmarshal(in.Konfigurasi, &cfg)
		if cfg.TipeHarga != "" {
			tipe = cfg.TipeHarga
		}
	}

	for _, item := range resp.Data {
		// 1. Pemetaan Komoditas
		komKode, cariKom := in.Pemetaan.Cari(domain.JenisKomoditas, item.NamaKomoditas)
		if cariKom == domain.Diabaikan {
			continue
		}
		if cariKom == domain.TakTerpetakan {
			hasil.TambahTakTerpetakan(domain.JenisKomoditas, item.NamaKomoditas)
			continue
		}

		// 2. Pemetaan Wilayah (coba alias kabupaten_id dulu, jika gagal coba nama_kabupaten)
		aliasWil := string(item.KabupatenID)
		kabKode, cariWil := in.Pemetaan.Cari(domain.JenisWilayah, aliasWil)
		if cariWil == domain.TakTerpetakan && item.NamaKabupaten != "" {
			aliasWil = item.NamaKabupaten
			kabKode, cariWil = in.Pemetaan.Cari(domain.JenisWilayah, aliasWil)
		}

		if cariWil == domain.Diabaikan {
			continue
		}
		if cariWil == domain.TakTerpetakan {
			hasil.TambahTakTerpetakan(domain.JenisWilayah, aliasWil)
			continue
		}

		// 3. Validasi Harga
		harga, ok := kit.Angka(string(item.Harga))
		if !ok || harga <= 0 || harga > hargaMaks {
			hasil.TambahTidakValid("harga kosong atau tidak wajar")
			continue
		}

		// 4. Validasi Tanggal
		tanggal, ok := kit.Tanggal(item.Tanggal)
		if !ok {
			hasil.TambahTidakValid("tanggal tidak valid")
			continue
		}

		// 5. Satuan
		satuan := in.Pemetaan.Satuan(komKode)
		if satuan == "" {
			satuan = strings.ToLower(strings.TrimSpace(item.Satuan))
		}
		if satuan == "" {
			satuan = "kg"
		}

		hasil.Harga = append(hasil.Harga, domain.HargaEksternal{
			SumberID:      in.SumberID,
			KomoditasKode: komKode,
			KabupatenKode: kabKode,
			Tanggal:       tanggal,
			Tipe:          tipe,
			Harga:         harga,
			Satuan:        satuan,
		})
	}

	return hasil, nil
}
