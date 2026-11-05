package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
)

const tanggalISO = "2006-01-02"

// RingkasanResponse adalah satu baris tabel proyeksi seluruh komoditas.
type RingkasanResponse struct {
	KomoditasKode string  `json:"komoditas_kode"`
	KomoditasNama string  `json:"komoditas_nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	HorizonHari   int     `json:"horizon_hari"`
	HargaKini     int     `json:"harga_kini"`
	Proyeksi      int     `json:"proyeksi"`
	PerubahanPct  float64 `json:"perubahan_pct"`
	BawahPct      float64 `json:"bawah_pct"`
	AtasPct       float64 `json:"atas_pct"`
	ModelVersi    string  `json:"model_versi"`
	Mape          float64 `json:"mape"`
	Sumber        string  `json:"sumber"`
}

// TitikResponse adalah satu titik pada seri grafik: riwayat (aktual) atau
// proyeksi (dengan rentang). Titik sambung memuat keduanya.
type TitikResponse struct {
	Tanggal  string `json:"tanggal"`
	Aktual   *int   `json:"aktual,omitempty"`
	Proyeksi *int   `json:"proyeksi,omitempty"`
	Bawah    *int   `json:"bawah,omitempty"`
	Atas     *int   `json:"atas,omitempty"`
}

type FaktorResponse struct {
	Nama       string  `json:"nama"`
	Kontribusi float64 `json:"kontribusi"`
	Detail     string  `json:"detail"`
}

type ModelInfoResponse struct {
	Nama    string  `json:"nama"`
	Versi   string  `json:"versi"`
	Mape    float64 `json:"mape"`
	Dilatih string  `json:"dilatih"`
}

// ProyeksiResponse adalah proyeksi lengkap satu komoditas untuk panel XAI.
type ProyeksiResponse struct {
	KomoditasKode string            `json:"komoditas_kode"`
	KomoditasNama string            `json:"komoditas_nama"`
	KabupatenKode string            `json:"kabupaten_kode"`
	KabupatenNama string            `json:"kabupaten_nama"`
	HorizonHari   int               `json:"horizon_hari"`
	HargaKini     int               `json:"harga_kini"`
	PerubahanPct  float64           `json:"perubahan_pct"`
	BawahPct      float64           `json:"bawah_pct"`
	AtasPct       float64           `json:"atas_pct"`
	Seri          []TitikResponse   `json:"seri"`
	Faktor        []FaktorResponse  `json:"faktor"`
	Narasi        string            `json:"narasi"`
	Model         ModelInfoResponse `json:"model"`
	Rekomendasi   []string          `json:"rekomendasi"`
	Sumber        string            `json:"sumber"`
}

// ModelResponse adalah satu baris registri model.
type ModelResponse struct {
	ID            string  `json:"id"`
	KomoditasKode string  `json:"komoditas_kode"`
	KomoditasNama string  `json:"komoditas_nama"`
	Nama          string  `json:"nama"`
	Versi         string  `json:"versi"`
	Mape          float64 `json:"mape"`
	Drift         float64 `json:"drift"`
	Status        string  `json:"status"`
	DilatihPada   string  `json:"dilatih_pada"`
	Sumber        string  `json:"sumber"`
}

func ToRingkasan(items []domain.Ringkasan) []RingkasanResponse {
	res := make([]RingkasanResponse, len(items))
	for i, r := range items {
		p := r.Proyeksi
		res[i] = RingkasanResponse{
			KomoditasKode: p.KomoditasKode, KomoditasNama: p.KomoditasNama, KabupatenKode: p.KabupatenKode,
			HorizonHari: p.HorizonHari, HargaKini: p.HargaDasar, Proyeksi: p.Nilai,
			PerubahanPct: r.PerubahanPct, BawahPct: r.BawahPct, AtasPct: r.AtasPct,
			ModelVersi: p.ModelVersi, Mape: p.ModelMape, Sumber: p.Sumber,
		}
	}
	return res
}

func ToModels(items []domain.ModelView) []ModelResponse {
	res := make([]ModelResponse, len(items))
	for i, m := range items {
		res[i] = ModelResponse{
			ID: m.ID, KomoditasKode: m.KomoditasKode, KomoditasNama: m.KomoditasNama, Nama: m.Nama, Versi: m.Versi,
			Mape: m.Mape, Drift: m.Drift, Status: m.Status, DilatihPada: m.DilatihPada.Format(time.RFC3339), Sumber: m.Sumber,
		}
	}
	return res
}

// ToProyeksi menyusun seri grafik: riwayat aktual, titik sambung pada hari dasar
// (harga dasar sebagai aktual sekaligus awal proyeksi), lalu titik proyeksi.
func ToProyeksi(d domain.Detail) ProyeksiResponse {
	p := d.Proyeksi
	dasar := p.TanggalDasar()

	seri := make([]TitikResponse, 0, len(d.Riwayat)+len(d.Jalur)+1)
	for _, r := range d.Riwayat {
		seri = append(seri, TitikResponse{Tanggal: r.Tanggal.Format(tanggalISO), Aktual: ptr(r.Harga)})
	}
	seri = append(seri, TitikResponse{
		Tanggal: dasar.Format(tanggalISO), Aktual: ptr(p.HargaDasar),
		Proyeksi: ptr(p.HargaDasar), Bawah: ptr(p.HargaDasar), Atas: ptr(p.HargaDasar),
	})
	for _, t := range d.Jalur {
		seri = append(seri, TitikResponse{Tanggal: t.TanggalTarget.Format(tanggalISO), Proyeksi: ptr(t.Nilai), Bawah: ptr(t.Bawah), Atas: ptr(t.Atas)})
	}

	faktor := make([]FaktorResponse, len(d.Faktor))
	for i, f := range d.Faktor {
		faktor[i] = FaktorResponse{Nama: f.Nama, Kontribusi: f.Kontribusi, Detail: f.Detail}
	}

	return ProyeksiResponse{
		KomoditasKode: p.KomoditasKode, KomoditasNama: p.KomoditasNama, KabupatenKode: p.KabupatenKode, KabupatenNama: p.KabupatenNama,
		HorizonHari: p.HorizonHari, HargaKini: p.HargaDasar,
		PerubahanPct: d.PerubahanPct, BawahPct: d.BawahPct, AtasPct: d.AtasPct,
		Seri: seri, Faktor: faktor, Narasi: d.Narasi,
		Model:       ModelInfoResponse{Nama: p.ModelNama, Versi: p.ModelVersi, Mape: p.ModelMape, Dilatih: p.ModelDilatih.Format(tanggalISO)},
		Rekomendasi: d.Rekomendasi, Sumber: p.Sumber,
	}
}

func ptr(v int) *int { return &v }
