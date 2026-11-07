package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/internal/spasial/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
)

// MarkerResponse adalah satu wilayah di peta. Status: defisit | waspada | normal | surplus.
type MarkerResponse struct {
	Kode       string  `json:"kode"`
	Nama       string  `json:"nama"`
	Level      string  `json:"level"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Delta      float64 `json:"delta"`
	Komoditas  string  `json:"komoditas"`
	Status     string  `json:"status"`
	PunyaPasar bool    `json:"punya_pasar"`
}

// NeracaResponse adalah neraca satu komoditas di satu wilayah.
type NeracaResponse struct {
	KomoditasKode     string  `json:"komoditas_kode"`
	KomoditasNama     string  `json:"komoditas_nama"`
	KabupatenKode     string  `json:"kabupaten_kode"`
	Wilayah           string  `json:"wilayah"`
	Periode           string  `json:"periode"`
	SurplusDefisitTon float64 `json:"surplus_defisit_ton"`
	DeltaHargaPct     float64 `json:"delta_harga_pct"`
	Sumber            string  `json:"sumber"`
}

// RuteResponse adalah satu usulan rute distribusi.
type RuteResponse struct {
	// ID adalah ID BIGINT (string desimal); Kode adalah ID lama (RT-1106-1171) pada data seed dan migrasi.
	ID            idx.ID    `json:"id"`
	Kode          *string   `json:"kode"`
	AsalKode      string    `json:"asal_kode"`
	Asal          string    `json:"asal"`
	TujuanKode    string    `json:"tujuan_kode"`
	Tujuan        string    `json:"tujuan"`
	KomoditasKode string    `json:"komoditas_kode"`
	KomoditasNama string    `json:"komoditas_nama"`
	VolumeTon     float64   `json:"volume_ton"`
	JarakKm       int       `json:"jarak_km"`
	JamTempuh     float64   `json:"jam_tempuh"`
	Biaya         int64     `json:"biaya"`
	Status        string    `json:"status"`
	Sumber        string    `json:"sumber"`
	DibuatPada    time.Time `json:"dibuat_pada"`
}

// KandidatResponse adalah satu kandidat lokasi cold storage.
type KandidatResponse struct {
	// ID adalah ID BIGINT (string desimal); Kode adalah ID lama (KC-1173) pada data seed dan migrasi.
	ID            idx.ID             `json:"id"`
	Kode          *string            `json:"kode"`
	KabupatenKode string             `json:"kabupaten_kode"`
	Wilayah       string             `json:"wilayah"`
	Lokasi        string             `json:"lokasi"`
	Skor          int                `json:"skor"`
	KomponenSkor  map[string]float64 `json:"komponen_skor"`
	Alasan        string             `json:"alasan"`
	Status        string             `json:"status"`
	Sumber        string             `json:"sumber"`
}

func ToMarkerResponses(items []domain.Marker) []MarkerResponse {
	out := make([]MarkerResponse, len(items))
	for i, m := range items {
		komoditas := m.Komoditas
		if komoditas == "" {
			komoditas = "—"
		}
		out[i] = MarkerResponse{
			Kode: m.Kode, Nama: m.Nama, Level: m.Level, Lat: m.Lat, Lng: m.Lng, Delta: m.Delta,
			Komoditas: komoditas, Status: domain.StatusDariDelta(m.Delta), PunyaPasar: m.PunyaPasar,
		}
	}
	return out
}

func ToNeracaResponses(items []domain.Neraca) []NeracaResponse {
	out := make([]NeracaResponse, len(items))
	for i, n := range items {
		out[i] = NeracaResponse{
			KomoditasKode: n.KomoditasKode, KomoditasNama: n.KomoditasNama, KabupatenKode: n.KabupatenKode,
			Wilayah: n.KabupatenNama, Periode: n.Periode.Format("2006-01-02"),
			SurplusDefisitTon: n.SurplusDefisitTon, DeltaHargaPct: n.DeltaHargaPct, Sumber: n.Sumber,
		}
	}
	return out
}

func ToRuteResponses(items []domain.Rute) []RuteResponse {
	out := make([]RuteResponse, len(items))
	for i, r := range items {
		out[i] = RuteResponse{
			ID: r.ID, Kode: r.Kode, AsalKode: r.AsalKode, Asal: r.AsalNama, TujuanKode: r.TujuanKode, Tujuan: r.TujuanNama,
			KomoditasKode: r.KomoditasKode, KomoditasNama: r.KomoditasNama, VolumeTon: r.VolumeTon,
			JarakKm: r.JarakKm, JamTempuh: r.JamTempuh, Biaya: r.Biaya, Status: r.Status, Sumber: r.Sumber,
			DibuatPada: r.DibuatPada,
		}
	}
	return out
}

// ToKandidatResponses gagal bila komponen_skor bukan JSON objek bilangan (tak mungkin
// terjadi bila data ditulis lewat seeder, tetapi basis data tidak menjaminnya).
func ToKandidatResponses(items []domain.Kandidat) ([]KandidatResponse, error) {
	out := make([]KandidatResponse, len(items))
	for i, c := range items {
		var komponen map[string]float64
		if err := json.Unmarshal([]byte(c.KomponenSkor), &komponen); err != nil {
			return nil, err
		}
		out[i] = KandidatResponse{
			ID: c.ID, Kode: c.Kode, KabupatenKode: c.KabupatenKode, Wilayah: c.KabupatenNama, Lokasi: c.Lokasi, Skor: c.Skor,
			KomponenSkor: komponen, Alasan: c.Alasan, Status: c.Status, Sumber: c.Sumber,
		}
	}
	return out, nil
}

// FiturResponse adalah satu wilayah dalam FeatureCollection GeoJSON.
type FiturResponse struct {
	Type       string                  `json:"type"`
	ID         string                  `json:"id"`
	Properties FiturPropertiesResponse `json:"properties"`
	Geometry   json.RawMessage         `json:"geometry"`
}

// FiturPropertiesResponse menjelaskan wilayah. Perkiraan benar bila geometri
// bukan batas administratif resmi (sumber DEMO).
type FiturPropertiesResponse struct {
	Kode       string `json:"kode"`
	Nama       string `json:"nama"`
	Level      string `json:"level"`
	Sumber     string `json:"sumber"`
	Perkiraan  bool   `json:"perkiraan"`
	Keterangan string `json:"keterangan"`
}

// GeometriResponse adalah FeatureCollection GeoJSON seluruh wilayah yang diminta.
type GeometriResponse struct {
	Type     string          `json:"type"`
	Features []FiturResponse `json:"features"`
}

// LokasiResponse adalah wilayah untuk satu titik.
type LokasiResponse struct {
	Kode         string  `json:"kode"`
	Nama         string  `json:"nama"`
	DiDalam      bool    `json:"di_dalam"`
	JarakPusatKm float64 `json:"jarak_pusat_km"`
}

func ToGeometriResponse(items []usecase.Fitur) GeometriResponse {
	features := make([]FiturResponse, len(items))
	for i, f := range items {
		features[i] = FiturResponse{
			Type: "Feature", ID: f.KabupatenKode, Geometry: f.Geometri,
			Properties: FiturPropertiesResponse{
				Kode: f.KabupatenKode, Nama: f.Nama, Level: f.Level, Sumber: f.Sumber,
				Perkiraan: f.Sumber != domain.SumberResmi, Keterangan: f.Keterangan,
			},
		}
	}
	return GeometriResponse{Type: "FeatureCollection", Features: features}
}

// RuteOptimasiResponse adalah satu rute dalam rencana optimasi.
type RuteOptimasiResponse struct {
	AsalKode   string  `json:"asal_kode"`
	Asal       string  `json:"asal"`
	TujuanKode string  `json:"tujuan_kode"`
	Tujuan     string  `json:"tujuan"`
	VolumeTon  float64 `json:"volume_ton"`
	JarakKm    float64 `json:"jarak_km"`
	JamTempuh  float64 `json:"jam_tempuh"`
	Biaya      int64   `json:"biaya"`
}

// SisaResponse adalah volume yang tersisa pada satu wilayah.
type SisaResponse struct {
	Kode string  `json:"kode"`
	Nama string  `json:"nama"`
	Ton  float64 `json:"ton"`
}

// OptimasiResponse adalah rencana distribusi untuk satu komoditas.
type OptimasiResponse struct {
	KomoditasKode  string                 `json:"komoditas_kode"`
	KomoditasNama  string                 `json:"komoditas_nama"`
	Periode        string                 `json:"periode"`
	Rute           []RuteOptimasiResponse `json:"rute"`
	TotalVolumeTon float64                `json:"total_volume_ton"`
	TotalBiaya     int64                  `json:"total_biaya"`
	// BelumTerpenuhi adalah defisit yang tidak tertutup; SurplusSisa surplus yang tidak tersalurkan.
	BelumTerpenuhi []SisaResponse `json:"belum_terpenuhi"`
	SurplusSisa    []SisaResponse `json:"surplus_sisa"`
	// BiayaPembanding adalah biaya rencana serakah (pasangan terdekat dulu) untuk
	// volume yang sama; null bila volumenya berbeda. PenghematanPersen: penghematan rencana optimal terhadapnya.
	BiayaPembanding   *int64   `json:"biaya_pembanding"`
	PenghematanPersen *float64 `json:"penghematan_persen"`
	// OptimasiID dan OptimasiKode terisi (bukan null) bila rencana disimpan: ID BIGINT (string desimal) dan
	// kode tampilannya (OPT-000042).
	OptimasiID   *idx.ID `json:"optimasi_id"`
	OptimasiKode *string `json:"optimasi_kode"`
}

// RencanaOptimasiResponse adalah ringkasan satu rencana tersimpan.
type RencanaOptimasiResponse struct {
	// ID adalah ID BIGINT (string desimal); Kode adalah kode tampilan (OPT-000042, atau ID lama pada data migrasi).
	ID                idx.ID    `json:"id"`
	Kode              *string   `json:"kode"`
	KomoditasKode     string    `json:"komoditas_kode"`
	KomoditasNama     string    `json:"komoditas_nama"`
	Periode           string    `json:"periode"`
	TotalVolumeTon    float64   `json:"total_volume_ton"`
	TotalBiaya        int64     `json:"total_biaya"`
	BelumTerpenuhiTon float64   `json:"belum_terpenuhi_ton"`
	DibuatOleh        *string   `json:"dibuat_oleh"`
	DibuatPada        time.Time `json:"dibuat_pada"`
}

func sisaResponses(in []usecase.Sisa) []SisaResponse {
	out := make([]SisaResponse, len(in))
	for i, s := range in {
		out[i] = SisaResponse{Kode: s.Kode, Nama: s.Nama, Ton: s.Ton}
	}
	return out
}

func ToOptimasiResponse(o usecase.OptimasiOutput) OptimasiResponse {
	rute := make([]RuteOptimasiResponse, len(o.Rute))
	for i, r := range o.Rute {
		rute[i] = RuteOptimasiResponse{
			AsalKode: r.AsalKode, Asal: r.AsalNama, TujuanKode: r.TujuanKode, Tujuan: r.TujuanNama,
			VolumeTon: r.VolumeTon, JarakKm: r.JarakKm, JamTempuh: r.JamTempuh, Biaya: r.Biaya,
		}
	}
	res := OptimasiResponse{
		KomoditasKode: o.KomoditasKode, KomoditasNama: o.KomoditasNama, Periode: o.Periode.Format("2006-01-02"),
		Rute: rute, TotalVolumeTon: o.TotalVolumeTon, TotalBiaya: o.TotalBiaya,
		BelumTerpenuhi: sisaResponses(o.BelumTerpenuhi), SurplusSisa: sisaResponses(o.SurplusSisa),
		BiayaPembanding: o.BiayaPembanding, PenghematanPersen: o.PenghematanPersen,
	}
	if o.OptimasiID != 0 {
		id, kode := o.OptimasiID, o.OptimasiKode
		res.OptimasiID = &id
		if kode != "" {
			res.OptimasiKode = &kode
		}
	}
	return res
}

func ToRencanaOptimasiResponses(items []domain.RencanaOptimasi) []RencanaOptimasiResponse {
	out := make([]RencanaOptimasiResponse, len(items))
	for i, r := range items {
		out[i] = RencanaOptimasiResponse{
			ID: r.ID, Kode: r.Kode, KomoditasKode: r.KomoditasKode, KomoditasNama: r.KomoditasNama, Periode: r.Periode.Format("2006-01-02"),
			TotalVolumeTon: r.TotalVolumeTon, TotalBiaya: r.TotalBiaya, BelumTerpenuhiTon: r.BelumTerpenuhiTon,
			DibuatOleh: r.DibuatOlehNama, DibuatPada: r.DibuatPada,
		}
	}
	return out
}
