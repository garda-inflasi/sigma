// Package pihps adalah konektor Pusat Informasi Harga Pangan Strategis Nasional
// (PIHPS) Bank Indonesia: harga konsumen pasar tradisional per kabupaten/kota.
//
// BELUM TERVERIFIKASI TERHADAP ENDPOINT NYATA. Bentuk permintaan dan respons di
// bawah ditulis dari dokumentasi/pengetahuan publik tentang tabel harga PIHPS
// (`WebSite/TabelHarga/GetGridDataDaerah`) tanpa akses jaringan saat dibuat. Parser
// sengaja defensif (nama kolom, format tanggal, dan format angka dilonggarkan)
// dan seluruh perilakunya dikunci dengan fixture di testdata/. Sebelum dipakai di
// produksi, jalankan sekali terhadap layanan nyata, bandingkan payload bronze
// dengan fixture, dan sesuaikan bila bentuknya berbeda.
//
// Konfigurasi sumber (sumber_data.konfigurasi), semua teks atau angka:
//
//	province_id   wajib: id provinsi menurut PIHPS (bukan kode BPS)
//	hari          jendela tanggal ke belakang (bawaan 7)
//	price_type_id jenis pasar PIHPS (bawaan 1 = pasar tradisional)
//	comcat_id     filter komoditas PIHPS (bawaan kosong = semua)
//
// Wilayah yang diambil adalah semua baris pemetaan_sumber jenis WILAYAH sumber ini:
// alias = regency_id PIHPS, kode_master = kode kabupaten/kota BPS.
package pihps

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/connector/kit"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// Kode adalah kode baris sumber_data PIHPS Nasional (sumber_data.kode).
const Kode = "SRC-BI"

const (
	jalurGrid    = "/WebSite/TabelHarga/GetGridDataDaerah"
	hariBawaan   = 7
	hariMaks     = 31
	kunciWilayah = "wilayah="
	// hargaMaks menolak angka yang jelas salah baca (rupiah per kg/liter).
	hargaMaks = 10_000_000
)

type konfigurasi struct {
	ProvinceID  kit.Teks `json:"province_id"`
	Hari        int      `json:"hari"`
	PriceTypeID kit.Teks `json:"price_type_id"`
	ComcatID    kit.Teks `json:"comcat_id"`
}

// Konektor adalah konektor PIHPS.
type Konektor struct{}

// New membuat konektor PIHPS.
func New() *Konektor { return &Konektor{} }

func (*Konektor) Kode() string { return Kode }

func (*Konektor) Permintaan(in domain.Masukan) ([]domain.Permintaan, error) {
	var cfg konfigurasi
	if len(in.Konfigurasi) > 0 {
		if err := json.Unmarshal(in.Konfigurasi, &cfg); err != nil {
			return nil, fmt.Errorf("konfigurasi %s tidak valid: %w", Kode, err)
		}
	}
	if cfg.ProvinceID == "" {
		return nil, fmt.Errorf("konfigurasi.province_id %s belum diisi (id provinsi menurut PIHPS)", Kode)
	}
	if in.URLDasar == "" {
		return nil, fmt.Errorf("url_dasar %s belum diisi", Kode)
	}
	hari := cfg.Hari
	if hari == 0 {
		hari = hariBawaan
	}
	if hari < 1 || hari > hariMaks {
		return nil, fmt.Errorf("konfigurasi.hari %d di luar 1-%d", hari, hariMaks)
	}
	jenisPasar := string(cfg.PriceTypeID)
	if jenisPasar == "" {
		jenisPasar = "1"
	}

	wilayah := in.Pemetaan.SemuaWilayah()
	if len(wilayah) == 0 {
		return nil, fmt.Errorf("pemetaan_sumber WILAYAH untuk %s kosong: petakan regency_id PIHPS ke kode kabupaten/kota", Kode)
	}

	akhir := in.Sekarang
	awal := akhir.AddDate(0, 0, -(hari - 1))
	reqs := make([]domain.Permintaan, len(wilayah))
	for i, w := range wilayah {
		q := url.Values{}
		q.Set("price_type_id", jenisPasar)
		q.Set("comcat_id", string(cfg.ComcatID))
		q.Set("province_id", string(cfg.ProvinceID))
		q.Set("regency_id", w.Alias)
		q.Set("market_id", "")
		q.Set("tipe_laporan", "1")
		q.Set("start_date", awal.Format(time.DateOnly))
		q.Set("end_date", akhir.Format(time.DateOnly))
		reqs[i] = domain.Permintaan{
			Kunci: kunciWilayah + w.Alias,
			URL:   strings.TrimRight(in.URLDasar, "/") + jalurGrid + "?" + q.Encode(),
		}
	}
	return reqs, nil
}

// kolomNama adalah kunci yang mungkin memuat nama komoditas pada satu baris grid.
var kolomNama = []string{"name", "nama", "commodity_name", "komoditas"}

func (*Konektor) Normalisasi(in domain.Masukan, dok domain.Dokumen) (domain.Hasil, error) {
	var hasil domain.Hasil

	alias, ok := strings.CutPrefix(dok.Kunci, kunciWilayah)
	if !ok {
		return hasil, fmt.Errorf("kunci dokumen %q bukan permintaan PIHPS", dok.Kunci)
	}
	kab, st := in.Pemetaan.Cari(domain.JenisWilayah, alias)
	if st != domain.Terpetakan {
		return hasil, fmt.Errorf("wilayah %q tidak terpetakan", alias)
	}

	baris, err := bacaBaris(dok.Konten)
	if err != nil {
		return hasil, err
	}

	for _, row := range baris {
		nama := namaKomoditas(row)
		if nama == "" {
			continue
		}
		kode, st := in.Pemetaan.Cari(domain.JenisKomoditas, nama)
		if st == domain.Diabaikan {
			continue
		}
		satuan := in.Pemetaan.Satuan(kode)

		for _, kunci := range kolomTanggal(row) {
			tanggal, _ := kit.Tanggal(kunci)
			mentah := row[kunci]
			switch {
			case st == domain.TakTerpetakan:
				hasil.TambahTakTerpetakan(domain.JenisKomoditas, domain.NormalisasiAlias(nama))
			case satuan == "":
				hasil.TambahTidakValid("komoditas master tanpa satuan")
			default:
				harga, ada := kit.Angka(kit.NilaiSel(mentah))
				switch {
				case !ada:
					hasil.TambahTidakValid("harga kosong atau bukan angka")
				case harga <= 0 || harga > hargaMaks:
					hasil.TambahTidakValid("harga di luar batas wajar")
				default:
					hasil.Harga = append(hasil.Harga, domain.HargaEksternal{
						SumberID: in.SumberID, KomoditasKode: kode, KabupatenKode: kab, Tanggal: tanggal,
						Tipe: domain.TipeHargaKonsumen, Harga: harga, Satuan: satuan,
					})
				}
			}
		}
	}
	return hasil, nil
}

// bacaBaris membaca larik baris grid dari {"data":[...]} atau larik di puncak.
func bacaBaris(konten []byte) ([]map[string]json.RawMessage, error) {
	var amplop struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(konten, &amplop); err == nil && amplop.Data != nil {
		return amplop.Data, nil
	}
	var larik []map[string]json.RawMessage
	if err := json.Unmarshal(konten, &larik); err == nil {
		return larik, nil
	}
	return nil, errors.New("respons PIHPS bukan {\"data\":[...]} maupun larik baris")
}

// kolomTanggal mengembalikan kunci baris yang berupa tanggal, terurut agar hasil deterministik.
func kolomTanggal(row map[string]json.RawMessage) []string {
	var kunci []string
	for k := range row {
		if _, ok := kit.Tanggal(k); ok {
			kunci = append(kunci, k)
		}
	}
	slices.Sort(kunci)
	return kunci
}

func namaKomoditas(row map[string]json.RawMessage) string {
	for _, k := range kolomNama {
		if raw, ok := row[k]; ok {
			if nama := strings.TrimSpace(kit.NilaiSel(raw)); nama != "" {
				return nama
			}
		}
	}
	return ""
}
