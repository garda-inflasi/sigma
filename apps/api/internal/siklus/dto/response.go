package dto

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

var wib = time.FixedZone("WIB", 7*60*60)

// SiklusAktifResponse is the response for the active cycle endpoint.
// Siklus berhuruf kecil (normal, tekanan, surplus) dan Sejak berformat YYYY-MM-DD waktu Aceh. Otomatis dan Alasan
// menjelaskan siapa yang menetapkan siklus: Alasan kosong (null) untuk siklus dari seed atau penetapan manual.
type SiklusAktifResponse struct {
	Siklus   string          `json:"siklus"`
	Sejak    string          `json:"sejak"`
	Otomatis bool            `json:"otomatis"`
	Alasan   *AlasanResponse `json:"alasan"`
}

// AlasanResponse adalah foto keputusan mesin saat siklus dibuka.
type AlasanResponse struct {
	Ringkasan string           `json:"ringkasan"`
	Pemicu    []PemicuResponse `json:"pemicu"`
	Ditekan   []PemicuResponse `json:"ditekan"`
}

// PemicuResponse adalah satu aturan di dalam alasan: nilai ukur terhadap ambangnya.
type PemicuResponse struct {
	Aturan    string   `json:"aturan"`
	Siklus    string   `json:"siklus"`
	Indikator string   `json:"indikator"`
	Nilai     float64  `json:"nilai"`
	Ambang    float64  `json:"ambang"`
	Satuan    string   `json:"satuan"`
	Prioritas int      `json:"prioritas"`
	Rujukan   []string `json:"rujukan"`
	Rincian   string   `json:"rincian"`
}

// ToSiklusAktifResponse converts a domain entity to the response DTO.
func ToSiklusAktifResponse(siklus string, mulaiPada time.Time) SiklusAktifResponse {
	return SiklusAktifResponse{
		Siklus: strings.ToLower(siklus),
		Sejak:  mulaiPada.In(wib).Format("2006-01-02"),
	}
}

// FromSiklus mengubah siklus aktif menjadi respons lengkap. Alasan yang tidak terbaca (bukan dokumen alasan)
// dijawab null, bukan galat: siklus tetap valid dan antarmuka hanya kehilangan penjelasannya.
func FromSiklus(s *domain.SiklusEkonomi) SiklusAktifResponse {
	resp := ToSiklusAktifResponse(s.Siklus, s.MulaiPada)
	resp.Otomatis = s.Otomatis
	if s.Alasan != nil {
		var alasan domain.Alasan
		if err := json.Unmarshal([]byte(*s.Alasan), &alasan); err == nil {
			resp.Alasan = toAlasanResponse(alasan)
		}
	}
	return resp
}

func toAlasanResponse(a domain.Alasan) *AlasanResponse {
	return &AlasanResponse{Ringkasan: a.Ringkasan, Pemicu: toPemicuList(a.Pemicu), Ditekan: toPemicuList(a.Ditekan)}
}

func toPemicuList(items []domain.PemicuAlasan) []PemicuResponse {
	out := make([]PemicuResponse, len(items))
	for i, p := range items {
		rujukan := p.Rujukan
		if rujukan == nil {
			rujukan = []string{}
		}
		out[i] = PemicuResponse{
			Aturan: p.Aturan, Siklus: strings.ToLower(p.Siklus), Indikator: p.Indikator, Nilai: p.Nilai, Ambang: p.Ambang,
			Satuan: p.Satuan, Prioritas: p.Prioritas, Rujukan: rujukan, Rincian: p.Rincian,
		}
	}
	return out
}

// AturanResponse adalah satu aturan penentuan siklus. Siklus berhuruf kecil; kolom yang tidak dipakai indikatornya null.
type AturanResponse struct {
	ID          idx.ID   `json:"id"`
	Kode        string   `json:"kode"`
	Siklus      string   `json:"siklus"`
	Indikator   string   `json:"indikator"`
	JenisEws    *string  `json:"jenis_ews"`
	SeverityMin *string  `json:"severity_min"`
	JumlahMin   *int     `json:"jumlah_min"`
	Ambang      *float64 `json:"ambang"`
	Prioritas   int      `json:"prioritas"`
	PulihJam    int      `json:"pulih_jam"`
	Aktif       bool     `json:"aktif"`
	Keterangan  string   `json:"keterangan"`
}

// ToAturanResponse mengubah aturan menjadi respons.
func ToAturanResponse(a domain.AturanSiklus) AturanResponse {
	return AturanResponse{
		ID: a.ID, Kode: a.Kode, Siklus: strings.ToLower(a.Siklus), Indikator: a.Indikator, JenisEws: a.JenisEws,
		SeverityMin: a.SeverityMin, JumlahMin: a.JumlahMin, Ambang: a.Ambang, Prioritas: a.Prioritas, PulihJam: a.PulihJam,
		Aktif: a.Aktif, Keterangan: a.Keterangan,
	}
}

// ToAturanResponses mengubah daftar aturan; daftar kosong menjadi [] (bukan null).
func ToAturanResponses(items []domain.AturanSiklus) []AturanResponse {
	out := make([]AturanResponse, len(items))
	for i, a := range items {
		out[i] = ToAturanResponse(a)
	}
	return out
}

// EvaluasiResponse adalah hasil satu putaran mesin (evaluasi manual). Siklus dan Sejak mengacu pada siklus aktif sesudah
// putaran (kosong bila dilewati dan belum ada siklus); Sebelumnya kosong (null) bila belum ada siklus sebelumnya.
type EvaluasiResponse struct {
	WilayahKode string `json:"wilayah_kode"`
	// Dilewati true bila evaluasi tidak dijalankan (SebabDilewati); tidak ada yang berubah.
	Dilewati      bool    `json:"dilewati"`
	SebabDilewati *string `json:"sebab_dilewati"`
	Berubah       bool    `json:"berubah"`
	Sebelumnya    *string `json:"sebelumnya"`
	Siklus        *string `json:"siklus"`
	Sejak         *string `json:"sejak"`
	Otomatis      bool    `json:"otomatis"`
	Ringkasan     string  `json:"ringkasan"`
	// Aturan memuat hasil ukur tiap aturan aktif.
	Aturan []HasilAturanResponse `json:"aturan"`
}

// HasilAturanResponse adalah hasil ukur satu aturan pada satu putaran.
type HasilAturanResponse struct {
	Kode      string  `json:"kode"`
	Siklus    string  `json:"siklus"`
	Indikator string  `json:"indikator"`
	Terukur   bool    `json:"terukur"`
	Nilai     float64 `json:"nilai"`
	Ambang    float64 `json:"ambang"`
	Satuan    string  `json:"satuan"`
	Memicu    bool    `json:"memicu"`
	Prioritas int     `json:"prioritas"`
	// Rujukan memuat kode alert contoh (ALERT_EWS); Rincian keterangan angka (UTILISASI_STOK).
	Rujukan []string `json:"rujukan"`
	Rincian string   `json:"rincian"`
}

// ToEvaluasiResponse mengubah hasil evaluasi menjadi respons.
func ToEvaluasiResponse(h domain.HasilEvaluasi) EvaluasiResponse {
	resp := EvaluasiResponse{
		WilayahKode: h.WilayahKode, Dilewati: h.Dilewati, Berubah: h.Berubah, Ringkasan: h.Ringkasan,
		Aturan: make([]HasilAturanResponse, len(h.Aturan)),
	}
	if h.SebabDilewati != "" {
		resp.SebabDilewati = &h.SebabDilewati
	}
	if h.Sebelumnya != "" {
		sebelumnya := strings.ToLower(h.Sebelumnya)
		resp.Sebelumnya = &sebelumnya
	}
	if h.Aktif != nil {
		aktif := ToSiklusAktifResponse(h.Aktif.Siklus, h.Aktif.MulaiPada)
		resp.Siklus, resp.Sejak, resp.Otomatis = &aktif.Siklus, &aktif.Sejak, h.Aktif.Otomatis
	}
	for i, r := range h.Aturan {
		p := pemicuDari(r)
		resp.Aturan[i] = HasilAturanResponse{
			Kode: r.Aturan.Kode, Siklus: strings.ToLower(r.Aturan.Siklus), Indikator: r.Aturan.Indikator, Terukur: r.Pengukuran.Terukur,
			Nilai: r.Pengukuran.Nilai, Ambang: p.Ambang, Satuan: p.Satuan, Memicu: r.Memicu, Prioritas: r.Aturan.Prioritas,
			Rujukan: p.Rujukan, Rincian: r.Pengukuran.Rincian,
		}
	}
	return resp
}

// pemicuDari menurunkan ambang dan satuan aturan menurut indikatornya.
func pemicuDari(r domain.HasilAturan) PemicuResponse {
	out := PemicuResponse{Rujukan: r.Pengukuran.Rujukan}
	if out.Rujukan == nil {
		out.Rujukan = []string{}
	}
	switch r.Aturan.Indikator {
	case domain.IndikatorAlertEws:
		out.Satuan = "alert"
		if r.Aturan.JumlahMin != nil {
			out.Ambang = float64(*r.Aturan.JumlahMin)
		}
	case domain.IndikatorUtilisasiStok:
		out.Satuan = "%"
		if r.Aturan.Ambang != nil {
			out.Ambang = *r.Aturan.Ambang
		}
	}
	return out
}
