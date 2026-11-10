package domain

import "time"

// ProdusenQuery adalah permintaan agregasi produsen. Jendela harga adalah tanggal Aceh Dari..Sampai (keduanya inklusif).
type ProdusenQuery struct {
	// Cakupan membatasi kabupaten yang boleh dibaca (02-B); nilai nol tertutup: tidak ada baris.
	Cakupan Cakupan
	// KabupatenKode dan KomoditasKode adalah filter permintaan, digabung (AND) dengan Cakupan. Kosong: tanpa filter.
	KabupatenKode string
	KomoditasKode string
	Dari, Sampai  time.Time
}

// ProdusenProduksi adalah produksi satu komoditas dari neraca_pangan pada periode terakhir untuk wilayah yang relevan:
// provinsi (kode wilayah 11) untuk tampilan seluruh Aceh tanpa filter kabupaten, selain itu kabupaten efektifnya.
// Tidak pernah menjumlah baris provinsi dengan baris kabupaten.
type ProdusenProduksi struct {
	Ton     float64
	Periode time.Time
	Sumber  string
}

// Demo benar bila angkanya contoh (sumber DEMO), bukan data resmi.
func (p ProdusenProduksi) Demo() bool { return p.Sumber == SumberNeracaDemo }

// ProdusenFakta adalah fakta mentah satu komoditas dari repository: rata-rata harga TERTIMBANG jumlah entri sepanjang
// jendela (bukan rata-rata rata-rata harian), belum dibulatkan. Pointer nil berarti sisi itu tidak punya data.
//
// Harga konsumen hanya memuat kabupaten yang juga punya harga produsen untuk komoditas itu pada jendela yang sama:
// margin membandingkan wilayah yang sama (seperti EKO-MARGIN di modul KPI), bukan harga produsen sentra dengan harga
// konsumen kabupaten yang tidak punya produsen.
type ProdusenFakta struct {
	KomoditasKode string   `gorm:"column:komoditas_kode"`
	KomoditasNama string   `gorm:"column:komoditas_nama"`
	Satuan        string   `gorm:"column:satuan"`
	Pilot         bool     `gorm:"column:pilot"`
	HargaProdusen *float64 `gorm:"column:harga_produsen"`
	EntriProdusen int      `gorm:"column:entri_produsen"`
	KabProdusen   int      `gorm:"column:kab_produsen"`
	HargaKonsumen *float64 `gorm:"column:harga_konsumen"`
	EntriKonsumen int      `gorm:"column:entri_konsumen"`
	KabKonsumen   int      `gorm:"column:kab_konsumen"`

	ProduksiTon     *float64   `gorm:"column:produksi_ton"`
	ProduksiPeriode *time.Time `gorm:"column:produksi_periode"`
	ProduksiSumber  *string    `gorm:"column:produksi_sumber"`
}

// Produksi mengembalikan produksi neraca bila ada.
func (f ProdusenFakta) Produksi() *ProdusenProduksi {
	if f.ProduksiTon == nil || f.ProduksiPeriode == nil || f.ProduksiSumber == nil {
		return nil
	}
	return &ProdusenProduksi{Ton: *f.ProduksiTon, Periode: *f.ProduksiPeriode, Sumber: *f.ProduksiSumber}
}

// SisiHarga adalah harga rata-rata satu sisi (produsen atau konsumen) dalam jendela, dibulatkan rupiah penuh.
type SisiHarga struct {
	Harga           float64
	JumlahEntri     int
	JumlahKabupaten int
}

// Margin adalah selisih harga konsumen terhadap harga produsen (rupiah per satuan, dari harga yang sudah dibulatkan
// sehingga angka yang tampil selalu saling menjumlah). Dua persentase disediakan dengan nama yang tegas karena
// "margin persen" punya dua pembilang yang lazim:
//   - PersenProdusen: selisih terhadap harga produsen (markup).
//   - PersenKonsumen: selisih terhadap harga konsumen, yaitu bagian harga konsumen yang bukan diterima produsen
//     (definisi KPI EKO-MARGIN di modul KPI).
type Margin struct {
	Selisih        float64
	PersenProdusen float64
	PersenKonsumen float64
}

// ProdusenKomoditas adalah satu komoditas pada agregasi produsen. Setiap blok nil bila tidak punya data.
// Margin hanya ada bila harga produsen dan harga konsumen sama-sama ada.
type ProdusenKomoditas struct {
	KomoditasKode string
	KomoditasNama string
	Satuan        string
	Produsen      *SisiHarga
	Konsumen      *SisiHarga
	Margin        *Margin
	Produksi      *ProdusenProduksi
}

// Produsen adalah hasil GET /produsen.
type Produsen struct {
	// Dari dan Sampai adalah jendela harga (DATE waktu Aceh, inklusif).
	Dari, Sampai time.Time
	// Komoditas memuat komoditas yang punya harga produsen atau produksi; komoditas pilot lebih dulu, lalu menurut nama.
	// Tidak pernah nil.
	Komoditas []ProdusenKomoditas
}
