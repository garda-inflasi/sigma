package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// ListKecamatanRequest: filter kabupaten ditambah q/page/limit dari ListRequest.
type ListKecamatanRequest struct {
	ListRequest
	KabupatenKode string `form:"kabupaten_kode"`
}

// ListPedagangRequest: filter pasar dan aktif ditambah q/page/limit. Aktif memakai string agar nilai selain
// true/false ditolak dengan 400, bukan diam-diam dianggap kosong.
type ListPedagangRequest struct {
	ListRequest
	PasarID string `form:"pasar_id"`
	Aktif   string `form:"aktif" binding:"omitempty,oneof=true false"`
}

// VersiKomoditasRequest: pada opsional (RFC3339 atau YYYY-MM-DD, akhir hari WIB).
type VersiKomoditasRequest struct {
	Pada string `form:"pada"`
}

// KecamatanRowResponse adalah satu kecamatan. kode_dagri hilang bila tidak ada.
type KecamatanRowResponse struct {
	Kode          string  `json:"kode"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	KodeDagri     *string `json:"kode_dagri,omitempty"`
}

// SatuanRowResponse adalah satu satuan beserta jumlah komoditas yang memakainya.
type SatuanRowResponse struct {
	Kode            string `json:"kode"`
	Nama            string `json:"nama"`
	JumlahKomoditas int64  `json:"jumlah_komoditas"`
}

// PedagangRowResponse adalah satu pedagang. id berupa string desimal (BIGINT); nama hilang bila belum diisi.
type PedagangRowResponse struct {
	ID          idx.ID  `json:"id"`
	PasarID     string  `json:"pasar_id"`
	PasarNama   string  `json:"pasar_nama"`
	Kode        string  `json:"kode"`
	Nama        *string `json:"nama,omitempty"`
	Aktif       bool    `json:"aktif"`
	JumlahEntri int64   `json:"jumlah_entri"`
}

// KomoditasVersiResponse adalah satu versi definisi komoditas. berlaku_sampai null untuk versi yang kini berlaku;
// perubahan_id hilang bila versi tidak berasal dari usulan perubahan (mis. seed).
type KomoditasVersiResponse struct {
	ID            idx.ID     `json:"id"`
	Versi         int        `json:"versi"`
	Nama          string     `json:"nama"`
	Kategori      string     `json:"kategori"`
	Satuan        string     `json:"satuan"`
	HargaAcuan    int        `json:"harga_acuan"`
	ShelfLifeHari int        `json:"shelf_life_hari"`
	Pilot         bool       `json:"pilot"`
	BerlakuDari   time.Time  `json:"berlaku_dari"`
	BerlakuSampai *time.Time `json:"berlaku_sampai"`
	PerubahanID   *idx.ID    `json:"perubahan_id,omitempty"`
}

// KomoditasDetailResponse adalah baris komoditas yang sama dengan daftar ditambah riwayat versi (terbaru dulu).
// Struct ini sengaja datar dan mudah diperluas (mis. perubahan_menunggu).
type KomoditasDetailResponse struct {
	MasterKomoditasRowResponse
	RiwayatVersi []KomoditasVersiResponse `json:"riwayat_versi"`
	// PerubahanMenunggu adalah usulan yang menunggu persetujuan untuk komoditas ini (null bila tidak ada).
	PerubahanMenunggu *PerubahanResponse `json:"perubahan_menunggu"`
}

// WilayahDetailResponse adalah baris wilayah ditambah cacah kecamatan dan pasar.
type WilayahDetailResponse struct {
	MasterWilayahRowResponse
	JumlahKecamatan int64 `json:"jumlah_kecamatan"`
	JumlahPasar     int64 `json:"jumlah_pasar"`
}

// PasarDetailResponse adalah baris pasar ditambah nama kabupaten dan cacah pedagang terdaftar. kecamatan_kode dan
// kecamatan_nama (null bila pasar belum punya kecamatan) sudah ada pada baris pasar.
type PasarDetailResponse struct {
	MasterPasarRowResponse
	KabupatenNama  string `json:"kabupaten_nama"`
	JumlahPedagang int64  `json:"jumlah_pedagang"`
	// PerubahanMenunggu adalah usulan yang menunggu persetujuan untuk pasar ini (null bila tidak ada).
	PerubahanMenunggu *PerubahanResponse `json:"perubahan_menunggu"`
}

func ToKecamatanRow(k domain.KecamatanRef) KecamatanRowResponse {
	return KecamatanRowResponse{Kode: k.Kode, Nama: k.Nama, KabupatenKode: k.KabupatenKode, KabupatenNama: k.KabupatenNama, KodeDagri: k.KodeDagri}
}

func ToKecamatanRows(items []domain.KecamatanRef) []KecamatanRowResponse {
	rows := make([]KecamatanRowResponse, len(items))
	for i, k := range items {
		rows[i] = ToKecamatanRow(k)
	}
	return rows
}

func ToSatuanRows(items []domain.SatuanRef) []SatuanRowResponse {
	rows := make([]SatuanRowResponse, len(items))
	for i, s := range items {
		rows[i] = SatuanRowResponse{Kode: s.Kode, Nama: s.Nama, JumlahKomoditas: s.JumlahKomoditas}
	}
	return rows
}

func ToPedagangRows(items []domain.PedagangRef) []PedagangRowResponse {
	rows := make([]PedagangRowResponse, len(items))
	for i, p := range items {
		rows[i] = PedagangRowResponse{
			ID: p.ID, PasarID: p.PasarID, PasarNama: p.PasarNama, Kode: p.Kode, Nama: p.Nama, Aktif: p.Aktif, JumlahEntri: p.JumlahEntri,
		}
	}
	return rows
}

// ToKomoditasVersiRow memetakan satu versi; waktu ditulis dalam UTC agar bentuknya tetap.
func ToKomoditasVersiRow(v domain.KomoditasVersiRef) KomoditasVersiResponse {
	var sampai *time.Time
	if v.BerlakuSampai != nil {
		t := v.BerlakuSampai.UTC()
		sampai = &t
	}
	return KomoditasVersiResponse{
		ID: v.ID, Versi: v.Versi, Nama: v.Nama, Kategori: v.Kategori, Satuan: v.Satuan, HargaAcuan: v.HargaAcuan,
		ShelfLifeHari: v.ShelfLifeHari, Pilot: v.Pilot, BerlakuDari: v.BerlakuDari.UTC(), BerlakuSampai: sampai, PerubahanID: v.PerubahanID,
	}
}

func ToKomoditasVersiRows(items []domain.KomoditasVersiRef) []KomoditasVersiResponse {
	rows := make([]KomoditasVersiResponse, len(items))
	for i, v := range items {
		rows[i] = ToKomoditasVersiRow(v)
	}
	return rows
}

func ToKomoditasDetail(d domain.KomoditasDetail) KomoditasDetailResponse {
	return KomoditasDetailResponse{
		MasterKomoditasRowResponse: ToKomoditasRows([]domain.Komoditas{d.Komoditas})[0],
		RiwayatVersi:               ToKomoditasVersiRows(d.RiwayatVersi),
	}
}

func ToWilayahDetail(d domain.WilayahDetail) WilayahDetailResponse {
	return WilayahDetailResponse{
		MasterWilayahRowResponse: ToWilayahRows([]domain.Kabupaten{d.Kabupaten})[0],
		JumlahKecamatan:          d.JumlahKecamatan,
		JumlahPasar:              d.JumlahPasar,
	}
}

func ToPasarDetail(d domain.PasarDetail) PasarDetailResponse {
	return PasarDetailResponse{
		MasterPasarRowResponse: ToPasarRows([]domain.Pasar{d.Pasar})[0],
		KabupatenNama:          d.KabupatenNama,
		JumlahPedagang:         d.JumlahPedagang,
	}
}
