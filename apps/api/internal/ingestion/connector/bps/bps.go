// Package bps adalah konektor WebAPI BPS (webapi.bps.go.id) untuk IHK dan inflasi bulanan
// Aceh (inflasi_bulanan) serta andil komoditas (andil_inflasi). Angka yang masuk bersumber 'BPS'
// dan menggantikan baris contoh (DEMO) pada bulan yang sama.
//
// BELUM TERVERIFIKASI TERHADAP ENDPOINT NYATA. Bentuk respons (var, turvar, vervar, tahun,
// turtahun, datacontent dengan kunci gabungan vervar+var+turvar+tahun+turtahun) ditulis dari
// dokumentasi publik tanpa akses jaringan saat dibuat; perilakunya dikunci dengan fixture di
// testdata/. Id variabel BPS tidak diketahui dan TIDAK ditebak: operator wajib mengisinya lewat
// `gardactl ingest konfigurasi SRC-BPS ...`. Tanpa itu run gagal dengan pesan yang menyebut apa yang kurang.
//
// Konfigurasi sumber (sumber_data.konfigurasi):
//
//	domain         kode domain BPS (Aceh = "1100"; terisi dari seeder)
//	variabel       peta nama -> id variabel BPS; wajib: ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd
//	               (vervar = wilayah); opsional berpasangan: andil, andil_inflasi_pct (vervar = komoditas)
//	wilayah_andil  kode wilayah untuk andil (bawaan "11", provinsi Aceh)
//	tahun_surut    berapa tahun ke belakang ikut diminta (bawaan 1)
//
// Kunci API dari variabel lingkungan BPS_API_KEY (tidak disimpan di basis data). Alias wilayah
// (label vervar) dan komoditas dipetakan lewat pemetaan_sumber.
package bps

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	eksekutif "github.com/garda-inflasi/api/internal/eksekutif/domain"
	"github.com/garda-inflasi/api/internal/ingestion/connector/kit"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// Kode adalah kode baris sumber_data BPS (sumber_data.kode).
const Kode = "SRC-BPS"

// EnvKunci adalah variabel lingkungan berisi kunci API BPS.
const EnvKunci = "BPS_API_KEY"

// Nama metrik pada konfigurasi.variabel.
const (
	MetrikIHK       = "ihk"
	MetrikMtm       = "inflasi_mtm"
	MetrikYoy       = "inflasi_yoy"
	MetrikYtd       = "inflasi_ytd"
	MetrikAndil     = "andil"
	MetrikAndilPct  = "andil_inflasi_pct"
	wilayahAndilDef = "11"
	tahunSurutDef   = 1
	tahunSurutMaks  = 5
	// tahunBPS: id tahun BPS adalah tahun - 1900.
	tahunBPS = 1900
)

var (
	metrikInflasi = []string{MetrikIHK, MetrikMtm, MetrikYoy, MetrikYtd}
	metrikAndil   = []string{MetrikAndil, MetrikAndilPct}
)

type konfigurasi struct {
	Domain       kit.Teks            `json:"domain"`
	Variabel     map[string]kit.Teks `json:"variabel"`
	WilayahAndil kit.Teks            `json:"wilayah_andil"`
	TahunSurut   *int                `json:"tahun_surut"`
}

// Konektor adalah konektor BPS; kunci API diberikan saat dibuat.
type Konektor struct {
	kunci string
}

// New membuat konektor BPS dengan kunci API (boleh kosong; run lalu gagal dengan pesan jelas).
func New(kunciAPI string) *Konektor { return &Konektor{kunci: kunciAPI} }

func (*Konektor) Kode() string { return Kode }

// kunciKonfigurasi adalah kunci konfigurasi yang dibaca konektor ini.
var kunciKonfigurasi = []string{"domain", "variabel", "wilayah_andil", "tahun_surut"}

// KunciKonfigurasi dan PeriksaKonfigurasi memenuhi domain.PemeriksaKonfigurasi. Selain aturan yang dipakai run,
// pemeriksa menolak nama metrik asing di variabel: bacaKonfigurasi mengabaikannya diam-diam, sehingga salah ketik
// seperti "inflasi_mom" baru tampak sebagai "belum lengkap" tanpa petunjuk.
func (*Konektor) KunciKonfigurasi() []string { return slices.Clone(kunciKonfigurasi) }

func (*Konektor) PeriksaKonfigurasi(raw json.RawMessage) error {
	cfg, err := bacaKonfigurasi(raw)
	if err != nil {
		return err
	}
	dikenal := slices.Concat(metrikInflasi, metrikAndil)
	var asing []string
	for nama := range cfg.Variabel {
		if !slices.Contains(dikenal, nama) {
			asing = append(asing, nama)
		}
	}
	if len(asing) > 0 {
		slices.Sort(asing)
		return fmt.Errorf("konfigurasi.variabel %s memuat nama metrik yang tidak dikenal: %s (dikenal: %s)",
			Kode, strings.Join(asing, ", "), strings.Join(dikenal, ", "))
	}
	return nil
}

func bacaKonfigurasi(raw json.RawMessage) (konfigurasi, error) {
	var cfg konfigurasi
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("konfigurasi %s tidak valid: %w", Kode, err)
		}
	}
	if cfg.Domain == "" {
		return cfg, fmt.Errorf("konfigurasi.domain %s belum diisi (Aceh = 1100)", Kode)
	}
	var kurang []string
	for _, m := range metrikInflasi {
		if cfg.Variabel[m] == "" {
			kurang = append(kurang, m)
		}
	}
	if len(kurang) > 0 {
		return cfg, fmt.Errorf("konfigurasi.variabel %s belum lengkap, isi id variabel BPS untuk: %s", Kode, strings.Join(kurang, ", "))
	}
	if (cfg.Variabel[MetrikAndil] == "") != (cfg.Variabel[MetrikAndilPct] == "") {
		return cfg, fmt.Errorf("konfigurasi.variabel %s: %s dan %s harus diisi berpasangan", Kode, MetrikAndil, MetrikAndilPct)
	}
	if cfg.TahunSurut != nil && (*cfg.TahunSurut < 0 || *cfg.TahunSurut > tahunSurutMaks) {
		return cfg, fmt.Errorf("konfigurasi.tahun_surut %d di luar 0-%d", *cfg.TahunSurut, tahunSurutMaks)
	}
	return cfg, nil
}

func (k *Konektor) Permintaan(in domain.Masukan) ([]domain.Permintaan, error) {
	cfg, err := bacaKonfigurasi(in.Konfigurasi)
	if err != nil {
		return nil, err
	}
	if k.kunci == "" {
		return nil, fmt.Errorf("kunci API %s belum diisi: set variabel lingkungan %s", Kode, EnvKunci)
	}
	if in.URLDasar == "" {
		return nil, fmt.Errorf("url_dasar %s belum diisi", Kode)
	}
	surut := tahunSurutDef
	if cfg.TahunSurut != nil {
		surut = *cfg.TahunSurut
	}

	base := strings.TrimRight(in.URLDasar, "/")
	metrik := slices.Clone(metrikInflasi)
	if cfg.Variabel[MetrikAndil] != "" {
		metrik = append(metrik, metrikAndil...)
	}
	var reqs []domain.Permintaan
	for _, m := range metrik {
		for th := in.Sekarang.Year() - surut; th <= in.Sekarang.Year(); th++ {
			reqs = append(reqs, domain.Permintaan{
				Kunci: fmt.Sprintf("var=%s&th=%d", m, th),
				URL: fmt.Sprintf("%s/list/model/data/lang/ind/domain/%s/var/%s/th/%d/key/%s",
					base, cfg.Domain, cfg.Variabel[m], th-tahunBPS, k.kunci),
				Samarkan: []string{k.kunci},
			})
		}
	}
	return reqs, nil
}

type entri struct {
	Val   kit.Teks `json:"val"`
	Label string   `json:"label"`
}

type respons struct {
	Status       string                     `json:"status"`
	Ketersediaan string                     `json:"data-availability"`
	Var          []entri                    `json:"var"`
	Turvar       []entri                    `json:"turvar"`
	Vervar       []entri                    `json:"vervar"`
	Tahun        []entri                    `json:"tahun"`
	Turtahun     []entri                    `json:"turtahun"`
	Datacontent  map[string]json.RawMessage `json:"datacontent"`
}

func bacaRespons(konten []byte) (*respons, error) {
	var r respons
	if err := json.Unmarshal(konten, &r); err != nil {
		return nil, fmt.Errorf("respons BPS tidak terbaca: %w", err)
	}
	if r.Status != "" && !strings.EqualFold(r.Status, "OK") {
		return nil, fmt.Errorf("BPS menjawab status %q", r.Status)
	}
	if strings.Contains(r.Ketersediaan, "not-available") {
		return nil, fmt.Errorf("BPS: data tidak tersedia (%s); periksa id variabel dan tahun", r.Ketersediaan)
	}
	if len(r.Var) != 1 {
		return nil, fmt.Errorf("respons BPS harus memuat tepat satu variabel, ada %d", len(r.Var))
	}
	return &r, nil
}

// Normalisasi hanya memeriksa bahwa dokumen terbaca: baris inflasi baru lengkap setelah
// keempat metrik terkumpul, jadi perakitannya ada di Gabungkan.
func (*Konektor) Normalisasi(_ domain.Masukan, dok domain.Dokumen) (domain.Hasil, error) {
	_, err := bacaRespons(dok.Konten)
	return domain.Hasil{}, err
}

var bulanIndonesia = map[string]time.Month{
	"januari": time.January, "februari": time.February, "maret": time.March, "april": time.April,
	"mei": time.May, "juni": time.June, "juli": time.July, "agustus": time.August,
	"september": time.September, "oktober": time.October, "november": time.November, "desember": time.December,
	"january": time.January, "february": time.February, "march": time.March, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "october": time.October, "december": time.December,
}

// bulan membaca bulan dari entri turtahun: nama bulan, atau id 1-12.
func bulan(e entri) (time.Month, bool) {
	if m, ok := bulanIndonesia[strings.ToLower(strings.TrimSpace(e.Label))]; ok {
		return m, true
	}
	if strings.Contains(strings.ToLower(e.Label), "tahun") {
		return 0, false
	}
	n, err := strconv.Atoi(string(e.Val))
	if err != nil || n < 1 || n > 12 {
		return 0, false
	}
	return time.Month(n), true
}

// sel adalah satu nilai metrik pada satu (baris vervar, bulan).
type sel struct {
	metrik string
	label  string // label vervar
	bulan  time.Time
	nilai  float64
}

// bacaSel mengurai semua sel bernilai pada dokumen. Sel yang tak berangka atau di luar batas
// dilewati (grup yang kekurangan metrik dilaporkan oleh Gabungkan).
func bacaSel(metrik string, r *respons) []sel {
	var hasil []sel
	v, turvar := r.Var[0].Val, kit.Teks("0")
	if len(r.Turvar) > 0 {
		turvar = r.Turvar[0].Val
	}
	for _, th := range r.Tahun {
		tahun, err := strconv.Atoi(strings.TrimSpace(th.Label))
		if err != nil {
			continue
		}
		for _, tt := range r.Turtahun {
			bln, ok := bulan(tt)
			if !ok {
				continue
			}
			for _, vv := range r.Vervar {
				mentah, ada := r.Datacontent[string(vv.Val)+string(v)+string(turvar)+string(th.Val)+string(tt.Val)]
				if !ada {
					continue
				}
				nilai, ok := angkaJSON(mentah)
				if !ok || !dalamBatas(metrik, nilai) {
					continue
				}
				hasil = append(hasil, sel{metrik: metrik, label: vv.Label, bulan: time.Date(tahun, bln, 1, 0, 0, 0, 0, time.UTC), nilai: nilai})
			}
		}
	}
	return hasil
}

// angkaJSON membaca angka JSON (atau teks berangka) dengan titik desimal.
func angkaJSON(raw json.RawMessage) (float64, bool) {
	t := strings.TrimSpace(kit.NilaiSel(raw))
	v, err := strconv.ParseFloat(t, 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// dalamBatas menjaga nilai tetap muat di kolom numerik dan masuk akal.
func dalamBatas(metrik string, v float64) bool {
	switch metrik {
	case MetrikIHK:
		return v > 0 && v < 1000
	case MetrikAndilPct:
		return math.Abs(v) < 1000
	default: // mtm, yoy, ytd, andil
		return math.Abs(v) <= 100
	}
}

type kunciGrup struct {
	kode  string // kode wilayah (inflasi) atau komoditas (andil)
	bulan time.Time
}

// Gabungkan merakit baris inflasi_bulanan dan andil_inflasi dari semua dokumen sebuah run.
func (*Konektor) Gabungkan(in domain.Masukan, dokumen []domain.Dokumen) (domain.Hasil, error) {
	var hasil domain.Hasil
	cfg, err := bacaKonfigurasi(in.Konfigurasi)
	if err != nil {
		return hasil, err
	}
	wilayahAndil := string(cfg.WilayahAndil)
	if wilayahAndil == "" {
		wilayahAndil = wilayahAndilDef
	}

	inflasi := map[kunciGrup]map[string]float64{}
	andil := map[kunciGrup]map[string]float64{}
	for _, d := range dokumen {
		metrik, ok := metrikDariKunci(d.Kunci)
		if !ok {
			return hasil, fmt.Errorf("kunci dokumen %q bukan permintaan BPS", d.Kunci)
		}
		r, err := bacaRespons(d.Konten)
		if err != nil {
			return hasil, fmt.Errorf("%s: %w", d.Kunci, err)
		}
		jenis, tujuan := domain.JenisWilayah, inflasi
		if slices.Contains(metrikAndil, metrik) {
			jenis, tujuan = domain.JenisKomoditas, andil
		}
		for _, s := range bacaSel(metrik, r) {
			kode, st := in.Pemetaan.Cari(jenis, s.label)
			switch st {
			case domain.Diabaikan:
				continue
			case domain.TakTerpetakan:
				hasil.TambahTakTerpetakan(jenis, domain.NormalisasiAlias(s.label))
				continue
			}
			k := kunciGrup{kode: kode, bulan: s.bulan}
			if tujuan[k] == nil {
				tujuan[k] = map[string]float64{}
			}
			tujuan[k][s.metrik] = s.nilai
		}
	}

	for _, k := range urut(inflasi) {
		m := inflasi[k]
		if !lengkap(m, metrikInflasi) {
			hasil.TambahTidakValid(alasanTakLengkap)
			continue
		}
		hasil.Inflasi = append(hasil.Inflasi, eksekutif.InflasiBulanan{
			WilayahKode: k.kode, Periode: k.bulan, IHK: m[MetrikIHK],
			InflasiMtm: m[MetrikMtm], InflasiYoy: m[MetrikYoy], InflasiYtd: m[MetrikYtd], Sumber: eksekutif.SumberBPS,
		})
	}
	for _, k := range urut(andil) {
		m := andil[k]
		if !lengkap(m, metrikAndil) {
			hasil.TambahTidakValid(alasanTakLengkap)
			continue
		}
		hasil.Andil = append(hasil.Andil, eksekutif.AndilInflasi{
			Periode: k.bulan, WilayahKode: wilayahAndil, KomoditasKode: k.kode,
			Andil: m[MetrikAndil], InflasiPct: m[MetrikAndilPct], Sumber: eksekutif.SumberBPS,
		})
	}
	return hasil, nil
}

const alasanTakLengkap = "metrik inflasi tidak lengkap untuk wilayah/komoditas dan bulan ini"

// metrikDariKunci membaca nama metrik dari kunci "var=<metrik>&th=<tahun>".
func metrikDariKunci(kunci string) (string, bool) {
	rest, ok := strings.CutPrefix(kunci, "var=")
	if !ok {
		return "", false
	}
	metrik, _, _ := strings.Cut(rest, "&")
	return metrik, metrik != ""
}

func lengkap(m map[string]float64, wajib []string) bool {
	for _, w := range wajib {
		if _, ada := m[w]; !ada {
			return false
		}
	}
	return true
}

// urut mengembalikan kunci grup terurut (bulan, lalu kode) agar hasil deterministik.
func urut(m map[kunciGrup]map[string]float64) []kunciGrup {
	keys := make([]kunciGrup, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b kunciGrup) int {
		if c := a.bulan.Compare(b.bulan); c != 0 {
			return c
		}
		return strings.Compare(a.kode, b.kode)
	})
	return keys
}
