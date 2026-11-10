package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// Ringkasan operasional (08-B): aktivitas per petugas, stok, produsen, dan logistik. Semua larik berbentuk `[]` (tidak
// pernah null), blok opsional berbentuk objek atau null, dan tidak ada data pribadi selain nama tampilan petugas.

// ── Aktivitas per petugas ─────────────────────────────────────────────

// AktivitasPetugasItem adalah ringkasan satu petugas lapangan. ID adalah UUID pengguna.
type AktivitasPetugasItem struct {
	ID            string `json:"id"`
	Nama          string `json:"nama"`
	KabupatenKode string `json:"kabupaten_kode"`
	KabupatenNama string `json:"kabupaten_nama"`

	EntriHariIni     int `json:"entri_hari_ini"`
	TargetHarian     int `json:"target_harian"`
	MenungguValidasi int `json:"menunggu_validasi"`
	DitolakJendela   int `json:"ditolak_jendela"`
	// EntriTerakhirPada null bila petugas belum pernah mencatat entri.
	EntriTerakhirPada *time.Time `json:"entri_terakhir_pada"`

	TugasAktif     int `json:"tugas_aktif"`
	TugasSelesai   int `json:"tugas_selesai"`
	TugasTerlambat int `json:"tugas_terlambat"`
}

// AktivitasPetugasResponse: Tanggal adalah hari ini menurut waktu Aceh; JendelaHari adalah panjang jendela untuk
// DitolakJendela dan tugas.
type AktivitasPetugasResponse struct {
	Tanggal     string                 `json:"tanggal"`
	JendelaHari int                    `json:"jendela_hari"`
	Petugas     []AktivitasPetugasItem `json:"petugas"`
}

func ToAktivitasPetugasResponse(h *domain.AktivitasPetugasHasil) AktivitasPetugasResponse {
	items := make([]AktivitasPetugasItem, len(h.Petugas))
	for i, p := range h.Petugas {
		items[i] = AktivitasPetugasItem{
			ID: p.ID, Nama: p.Nama, KabupatenKode: p.KabupatenKode, KabupatenNama: p.KabupatenNama,
			EntriHariIni: p.EntriHariIni, TargetHarian: p.TargetHarian, MenungguValidasi: p.MenungguValidasi,
			DitolakJendela: p.DitolakJendela, EntriTerakhirPada: p.EntriTerakhirPada,
			TugasAktif: p.TugasAktif, TugasSelesai: p.TugasSelesai, TugasTerlambat: p.TugasTerlambat,
		}
	}
	return AktivitasPetugasResponse{Tanggal: h.Tanggal.Format(domain.TanggalLayout), JendelaHari: domain.JendelaOperasionalHari, Petugas: items}
}

// ── Stok ──────────────────────────────────────────────────────────────

// StokKomoditasItem adalah stok berlaku satu komoditas.
type StokKomoditasItem struct {
	KomoditasKode           string  `json:"komoditas_kode"`
	KomoditasNama           string  `json:"komoditas_nama"`
	TotalTon                float64 `json:"total_ton"`
	JumlahLot               int     `json:"jumlah_lot"`
	JumlahFasilitas         int     `json:"jumlah_fasilitas"`
	MendekatiKedaluwarsaTon float64 `json:"mendekati_kedaluwarsa_ton"`
	RisikoTinggiTon         float64 `json:"risiko_tinggi_ton"`
}

// StokRingkasan adalah stok seluruh cakupan dan filter.
type StokRingkasan struct {
	TotalTon                float64 `json:"total_ton"`
	JumlahLot               int     `json:"jumlah_lot"`
	JumlahFasilitas         int     `json:"jumlah_fasilitas"`
	KapasitasTon            int64   `json:"kapasitas_ton"`
	FasilitasOperasional    int     `json:"fasilitas_operasional"`
	UtilisasiPct            float64 `json:"utilisasi_pct"`
	MendekatiKedaluwarsaTon float64 `json:"mendekati_kedaluwarsa_ton"`
	RisikoTinggiTon         float64 `json:"risiko_tinggi_ton"`
	KedaluwarsaLot          int     `json:"kedaluwarsa_lot"`
	KedaluwarsaTon          float64 `json:"kedaluwarsa_ton"`
	DiLuarOperasionalTon    float64 `json:"di_luar_operasional_ton"`
}

// StokResponse: Tanggal adalah hari ini menurut waktu Aceh (acuan lot berlaku); AmbangKedaluwarsaHari dan
// AmbangRisiko adalah batas "mendekati kedaluwarsa" (hari) dan "risiko tinggi" (pecahan 0-1, risiko di ATAS batas).
type StokResponse struct {
	Tanggal               string              `json:"tanggal"`
	AmbangKedaluwarsaHari int                 `json:"ambang_kedaluwarsa_hari"`
	AmbangRisiko          float64             `json:"ambang_risiko"`
	Ringkasan             StokRingkasan       `json:"ringkasan"`
	Komoditas             []StokKomoditasItem `json:"komoditas"`
}

func ToStokResponse(s *domain.Stok) StokResponse {
	r := s.Ringkasan
	komoditas := make([]StokKomoditasItem, len(s.Komoditas))
	for i, k := range s.Komoditas {
		komoditas[i] = StokKomoditasItem{
			KomoditasKode: k.KomoditasKode, KomoditasNama: k.KomoditasNama, TotalTon: k.TotalTon, JumlahLot: k.JumlahLot,
			JumlahFasilitas: k.JumlahFasilitas, MendekatiKedaluwarsaTon: k.MendekatiKedaluwarsaTon, RisikoTinggiTon: k.RisikoTinggiTon,
		}
	}
	return StokResponse{
		Tanggal: s.Tanggal.Format(domain.TanggalLayout), AmbangKedaluwarsaHari: domain.AmbangMendekatiKedaluwarsaHari,
		AmbangRisiko: domain.AmbangRisikoTinggi,
		Ringkasan: StokRingkasan{
			TotalTon: r.TotalTon, JumlahLot: r.JumlahLot, JumlahFasilitas: r.JumlahFasilitas, KapasitasTon: r.KapasitasTon,
			FasilitasOperasional: r.FasilitasOperasional, UtilisasiPct: r.UtilisasiPct,
			MendekatiKedaluwarsaTon: r.MendekatiKedaluwarsaTon, RisikoTinggiTon: r.RisikoTinggiTon,
			KedaluwarsaLot: r.KedaluwarsaLot, KedaluwarsaTon: r.KedaluwarsaTon, DiLuarOperasionalTon: r.DiLuarOperasionalTon,
		},
		Komoditas: komoditas,
	}
}

// ── Produsen ──────────────────────────────────────────────────────────

// SisiHargaResponse adalah harga rata-rata (rupiah per satuan, tertimbang jumlah entri) satu sisi pada jendela.
type SisiHargaResponse struct {
	Harga           float64 `json:"harga"`
	JumlahEntri     int     `json:"jumlah_entri"`
	JumlahKabupaten int     `json:"jumlah_kabupaten"`
}

// MarginResponse: Selisih dalam rupiah per satuan; PersenProdusen terhadap harga produsen, PersenKonsumen terhadap harga
// konsumen (definisi EKO-MARGIN).
type MarginResponse struct {
	Selisih        float64 `json:"selisih"`
	PersenProdusen float64 `json:"persen_produsen"`
	PersenKonsumen float64 `json:"persen_konsumen"`
}

// ProduksiResponse adalah produksi neraca pangan periode terakhir; Demo benar bila angkanya contoh (sumber DEMO).
type ProduksiResponse struct {
	Ton     float64 `json:"ton"`
	Periode string  `json:"periode"`
	Sumber  string  `json:"sumber"`
	Demo    bool    `json:"demo"`
}

// ProdusenKomoditasItem adalah satu komoditas; Produsen, Konsumen, Margin, dan Produksi null bila tidak punya data.
type ProdusenKomoditasItem struct {
	KomoditasKode string             `json:"komoditas_kode"`
	KomoditasNama string             `json:"komoditas_nama"`
	Satuan        string             `json:"satuan"`
	Produsen      *SisiHargaResponse `json:"produsen"`
	Konsumen      *SisiHargaResponse `json:"konsumen"`
	Margin        *MarginResponse    `json:"margin"`
	Produksi      *ProduksiResponse  `json:"produksi"`
}

// ProdusenResponse: Dari dan Sampai adalah jendela harga (tanggal Aceh, inklusif).
type ProdusenResponse struct {
	Dari      string                  `json:"dari"`
	Sampai    string                  `json:"sampai"`
	Komoditas []ProdusenKomoditasItem `json:"komoditas"`
}

func ToProdusenResponse(p *domain.Produsen) ProdusenResponse {
	sisi := func(s *domain.SisiHarga) *SisiHargaResponse {
		if s == nil {
			return nil
		}
		return &SisiHargaResponse{Harga: s.Harga, JumlahEntri: s.JumlahEntri, JumlahKabupaten: s.JumlahKabupaten}
	}
	items := make([]ProdusenKomoditasItem, len(p.Komoditas))
	for i, k := range p.Komoditas {
		item := ProdusenKomoditasItem{
			KomoditasKode: k.KomoditasKode, KomoditasNama: k.KomoditasNama, Satuan: k.Satuan,
			Produsen: sisi(k.Produsen), Konsumen: sisi(k.Konsumen),
		}
		if k.Margin != nil {
			item.Margin = &MarginResponse{Selisih: k.Margin.Selisih, PersenProdusen: k.Margin.PersenProdusen, PersenKonsumen: k.Margin.PersenKonsumen}
		}
		if k.Produksi != nil {
			item.Produksi = &ProduksiResponse{
				Ton: k.Produksi.Ton, Periode: k.Produksi.Periode.Format(domain.TanggalLayout),
				Sumber: k.Produksi.Sumber, Demo: k.Produksi.Demo(),
			}
		}
		items[i] = item
	}
	return ProdusenResponse{Dari: p.Dari.Format(domain.TanggalLayout), Sampai: p.Sampai.Format(domain.TanggalLayout), Komoditas: items}
}

// ── Logistik ──────────────────────────────────────────────────────────

// GerakanJenisResponse adalah jumlah transaksi dan tonase satu jenis.
type GerakanJenisResponse struct {
	Jumlah    int     `json:"jumlah"`
	VolumeTon float64 `json:"volume_ton"`
}

// GerakanRingkasanResponse adalah total tiap jenis; ketiganya selalu ada.
type GerakanRingkasanResponse struct {
	Masuk  GerakanJenisResponse `json:"masuk"`
	Keluar GerakanJenisResponse `json:"keluar"`
	Pindah GerakanJenisResponse `json:"pindah"`
}

// GerakanKomoditasItem adalah pergerakan satu komoditas per jenis.
type GerakanKomoditasItem struct {
	KomoditasKode string               `json:"komoditas_kode"`
	KomoditasNama string               `json:"komoditas_nama"`
	Masuk         GerakanJenisResponse `json:"masuk"`
	Keluar        GerakanJenisResponse `json:"keluar"`
	Pindah        GerakanJenisResponse `json:"pindah"`
}

// FasilitasResponse adalah fasilitas cold storage pada sebuah pergerakan. ID adalah UUID; Kode adalah kode tampilan
// (CS-01) dan boleh null.
type FasilitasResponse struct {
	ID            string  `json:"id"`
	Kode          *string `json:"kode"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
}

// PergerakanItem adalah satu transaksi stok. ID adalah ID BIGINT (string desimal). Asal null untuk MASUK, Tujuan null
// untuk KELUAR. Tidak memuat pelaku maupun alasan.
type PergerakanItem struct {
	ID            idx.ID             `json:"id"`
	Waktu         time.Time          `json:"waktu"`
	Jenis         string             `json:"jenis"`
	KomoditasKode string             `json:"komoditas_kode"`
	KomoditasNama string             `json:"komoditas_nama"`
	VolumeTon     float64            `json:"volume_ton"`
	Asal          *FasilitasResponse `json:"asal"`
	Tujuan        *FasilitasResponse `json:"tujuan"`
}

// RuteItem adalah satu rute distribusi aktif; bentuknya sama dengan RuteResponse modul spasial.
type RuteItem struct {
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

// RuteRingkasResponse: Jumlah, VolumeTon, dan Biaya dihitung atas SEMUA rute aktif dalam cakupan, Items hanya yang
// terbesar biayanya.
type RuteRingkasResponse struct {
	Jumlah    int        `json:"jumlah"`
	VolumeTon float64    `json:"volume_ton"`
	Biaya     int64      `json:"biaya"`
	Items     []RuteItem `json:"items"`
}

// LogistikResponse: Dari dan Sampai adalah jendela (tanggal Aceh, inklusif).
type LogistikResponse struct {
	Dari      string                   `json:"dari"`
	Sampai    string                   `json:"sampai"`
	Ringkasan GerakanRingkasanResponse `json:"ringkasan"`
	Komoditas []GerakanKomoditasItem   `json:"komoditas"`
	Terbaru   []PergerakanItem         `json:"terbaru"`
	Rute      RuteRingkasResponse      `json:"rute"`
}

func ToLogistikResponse(l *domain.Logistik) LogistikResponse {
	jenis := func(g domain.GerakanJenis) GerakanJenisResponse {
		return GerakanJenisResponse{Jumlah: g.Jumlah, VolumeTon: g.VolumeTon}
	}
	fasilitas := func(f *domain.FasilitasRingkas) *FasilitasResponse {
		if f == nil {
			return nil
		}
		return &FasilitasResponse{ID: f.ID, Kode: f.Kode, Nama: f.Nama, KabupatenKode: f.KabupatenKode, KabupatenNama: f.KabupatenNama}
	}

	komoditas := make([]GerakanKomoditasItem, len(l.Komoditas))
	for i, k := range l.Komoditas {
		komoditas[i] = GerakanKomoditasItem{
			KomoditasKode: k.KomoditasKode, KomoditasNama: k.KomoditasNama,
			Masuk: jenis(k.Masuk), Keluar: jenis(k.Keluar), Pindah: jenis(k.Pindah),
		}
	}
	terbaru := make([]PergerakanItem, len(l.Terbaru))
	for i, p := range l.Terbaru {
		terbaru[i] = PergerakanItem{
			ID: p.ID, Waktu: p.Waktu, Jenis: p.Jenis, KomoditasKode: p.KomoditasKode, KomoditasNama: p.KomoditasNama,
			VolumeTon: p.VolumeTon, Asal: fasilitas(p.Asal()), Tujuan: fasilitas(p.Tujuan()),
		}
	}
	rute := make([]RuteItem, len(l.Rute.Items))
	for i, r := range l.Rute.Items {
		rute[i] = RuteItem{
			ID: r.ID, Kode: r.Kode, AsalKode: r.AsalKode, Asal: r.Asal, TujuanKode: r.TujuanKode, Tujuan: r.Tujuan,
			KomoditasKode: r.KomoditasKode, KomoditasNama: r.KomoditasNama, VolumeTon: r.VolumeTon, JarakKm: r.JarakKm,
			JamTempuh: r.JamTempuh, Biaya: r.Biaya, Status: r.Status, Sumber: r.Sumber, DibuatPada: r.DibuatPada,
		}
	}
	return LogistikResponse{
		Dari: l.Dari.Format(domain.TanggalLayout), Sampai: l.Sampai.Format(domain.TanggalLayout),
		Ringkasan: GerakanRingkasanResponse{Masuk: jenis(l.Ringkasan.Masuk), Keluar: jenis(l.Ringkasan.Keluar), Pindah: jenis(l.Ringkasan.Pindah)},
		Komoditas: komoditas, Terbaru: terbaru,
		Rute: RuteRingkasResponse{Jumlah: l.Rute.Jumlah, VolumeTon: l.Rute.VolumeTon, Biaya: l.Rute.Biaya, Items: rute},
	}
}
