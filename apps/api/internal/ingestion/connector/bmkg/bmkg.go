// Package bmkg adalah konektor prakiraan cuaca publik BMKG (api.bmkg.go.id/publik/prakiraan-cuaca):
// prakiraan 3-jaman per desa/kelurahan (kode adm4) diringkas menjadi satu baris per hari per
// kabupaten/kota di cuaca_harian.
//
// Terverifikasi terhadap endpoint nyata (2026-11-10): bentuk respons (data[].cuaca[][] dengan local_datetime, t, hu,
// tp) sama dengan fixture di testdata/. Jendela prakiraan mencakup tiga hari kalender; hari pertama dan terakhir
// hampir selalu parsial dan ditolak, jadi satu pengambilan biasanya menghasilkan satu hari penuh per titik.
// Perilakunya dikunci dengan fixture. Bandingkan payload bronze dengan fixture bila BMKG mengubah API.
//
// Kode adm4 memakai kode Kemendagri, BUKAN kode BPS master: urutan kabupaten berbeda (Simeulue BPS 1101 =
// Kemendagri 11.09; Langsa dan Lhokseumawe tertukar), jadi awalan adm4 tidak boleh diturunkan dari kode master.
// Titik pantau bawaan seed (domain.wilayahBMKG) sudah dicoba satu per satu ke BMKG dan namanya cocok dengan master.
//
// Titik pantau adalah baris pemetaan_sumber jenis WILAYAH sumber ini: alias = kode adm4,
// kode_master = kabupaten/kota BPS. Tidak ada konfigurasi lain.
package bmkg

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/connector/kit"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// Kode adalah kode baris sumber_data BMKG (sumber_data.kode).
const Kode = "SRC-BMKG"

// Sumber adalah nilai kolom cuaca_harian.sumber.
const Sumber = "BMKG"

const (
	jalur        = "/publik/prakiraan-cuaca"
	kunciWilayah = "wilayah="
	// slotPerHari: prakiraan BMKG per 3 jam. Hari dengan slot kurang ditolak agar prakiraan
	// separuh hari tidak menimpa baris hari penuh dari kiriman sebelumnya.
	slotPerHari = 8
	// anomaliCurahMm: curah hujan harian yang ditandai anomali (kategori hujan lebat BMKG, >= 50 mm/hari).
	anomaliCurahMm = 50.0

	alasanHariTakLengkap = "hari prakiraan tidak lengkap atau slot tanpa nilai"
)

// Konektor adalah konektor BMKG.
type Konektor struct{}

// New membuat konektor BMKG.
func New() *Konektor { return &Konektor{} }

func (*Konektor) Kode() string { return Kode }

func (*Konektor) Permintaan(in domain.Masukan) ([]domain.Permintaan, error) {
	if in.URLDasar == "" {
		return nil, fmt.Errorf("url_dasar %s belum diisi", Kode)
	}
	wilayah := in.Pemetaan.SemuaWilayah()
	if len(wilayah) == 0 {
		return nil, fmt.Errorf("pemetaan_sumber WILAYAH untuk %s kosong: petakan kode adm4 BMKG ke kode kabupaten/kota", Kode)
	}
	reqs := make([]domain.Permintaan, len(wilayah))
	for i, w := range wilayah {
		reqs[i] = domain.Permintaan{
			Kunci: kunciWilayah + w.Alias,
			URL:   strings.TrimRight(in.URLDasar, "/") + jalur + "?" + url.Values{"adm4": {w.Alias}}.Encode(),
		}
	}
	return reqs, nil
}

type slot struct {
	Datetime      string   `json:"datetime"`
	LocalDatetime string   `json:"local_datetime"`
	T             kit.Teks `json:"t"`
	Hu            kit.Teks `json:"hu"`
	Tp            kit.Teks `json:"tp"`
}

type respons struct {
	Data []struct {
		Cuaca [][]slot `json:"cuaca"`
	} `json:"data"`
}

// hari mengumpulkan slot satu tanggal kalender lokal.
type hari struct {
	tanggal         time.Time
	curah, suhu, hu float64
	slot            int
	valid           bool
}

func (*Konektor) Normalisasi(in domain.Masukan, dok domain.Dokumen) (domain.Hasil, error) {
	var hasil domain.Hasil

	alias, ok := strings.CutPrefix(dok.Kunci, kunciWilayah)
	if !ok {
		return hasil, fmt.Errorf("kunci dokumen %q bukan permintaan BMKG", dok.Kunci)
	}
	kab, st := in.Pemetaan.Cari(domain.JenisWilayah, alias)
	if st != domain.Terpetakan {
		return hasil, fmt.Errorf("wilayah %q tidak terpetakan", alias)
	}

	var res respons
	if err := json.Unmarshal(dok.Konten, &res); err != nil {
		return hasil, fmt.Errorf("respons BMKG tidak terbaca: %w", err)
	}
	if len(res.Data) == 0 {
		return hasil, fmt.Errorf("respons BMKG tanpa data prakiraan")
	}

	// Prakiraan dikelompokkan menurut tanggal lokal, bukan menurut larik hari di respons,
	// agar bergantung pada waktu yang tertulis dan bukan pada urutan larik.
	perHari := map[string]*hari{}
	var urutan []string
	for _, blok := range res.Data {
		for _, harian := range blok.Cuaca {
			for _, s := range harian {
				tgl, ok := tanggalLokal(s)
				if !ok {
					hasil.TambahTidakValid(alasanHariTakLengkap)
					continue
				}
				kunci := tgl.Format(time.DateOnly)
				h := perHari[kunci]
				if h == nil {
					h = &hari{tanggal: tgl, valid: true}
					perHari[kunci] = h
					urutan = append(urutan, kunci)
				}
				h.slot++
				t, okT := desimal(s.T)
				hu, okH := desimal(s.Hu)
				tp, okP := desimal(s.Tp)
				if !okT || !okH || !okP || tp < 0 || hu < 0 || hu > 100 {
					h.valid = false
					continue
				}
				h.suhu += t
				h.hu += hu
				h.curah += tp
			}
		}
	}

	slices.Sort(urutan)
	for _, kunci := range urutan {
		h := perHari[kunci]
		if !h.valid || h.slot != slotPerHari {
			hasil.TambahTidakValid(alasanHariTakLengkap)
			continue
		}
		curah := round(h.curah, 1)
		hasil.Cuaca = append(hasil.Cuaca, domain.CuacaHarian{
			KabupatenKode: kab, Tanggal: h.tanggal, Sumber: Sumber,
			CurahHujanMm: curah, SuhuC: round(h.suhu/slotPerHari, 1), Kelembapan: int(math.Round(h.hu / slotPerHari)),
			Anomali: curah >= anomaliCurahMm,
		})
	}
	return hasil, nil
}

// tanggalLokal membaca tanggal kalender lokal sebuah slot: local_datetime bila ada, selain itu
// datetime (UTC) dikonversi ke WIB (Aceh).
func tanggalLokal(s slot) (time.Time, bool) {
	if s.LocalDatetime != "" {
		return kit.Tanggal(s.LocalDatetime)
	}
	utc, err := time.Parse(time.RFC3339, s.Datetime)
	if err != nil {
		return time.Time{}, false
	}
	wib := utc.In(time.FixedZone("WIB", 7*60*60))
	return time.Date(wib.Year(), wib.Month(), wib.Day(), 0, 0, 0, 0, time.UTC), true
}

// desimal membaca angka JSON apa adanya (titik desimal), tanpa heuristik pemisah ribuan.
func desimal(t kit.Teks) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(string(t)), 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func round(v float64, digit int) float64 {
	p := math.Pow10(digit)
	return math.Round(v*p) / p
}
