package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"time"
)

// Parameter validasi otomatis.
const (
	// RasioProdusen menurunkan harga acuan produsen dari harga acuan konsumen komoditas.
	RasioProdusen = 0.72
	// HariHistori adalah jendela histori harga resmi pembanding.
	HariHistori = 30
	// MinSampelHistori adalah jumlah harga resmi minimal agar median dan outlier dihitung dari histori.
	MinSampelHistori = 5
	// AmbangOutlier adalah batas |z-score| robust (berbasis median dan MAD) sebuah harga disebut outlier.
	AmbangOutlier = 3.5
	// lantaiSebaranPersen mencegah sebaran nol pada histori yang datar: sebaran minimal 2% dari median.
	lantaiSebaranPersen = 0.02
	// madKeSigma mengubah MAD menjadi padanan simpangan baku untuk sebaran normal.
	madKeSigma = 1.4826
	// radiusBumi dalam meter, untuk jarak haversine.
	radiusBumi = 6371e3
)

// Bobot skor anomali (0 sampai 1): sinyal numerik ditambah penalti per bendera.
const (
	skalaDeviasiPct    = 40.0
	skalaZ             = 2 * AmbangOutlier
	penaltiRentang     = 0.5
	penaltiGeofence    = 0.1
	penaltiDuplikat    = 0.2
	penaltiTanpaLokasi = 0.05
)

// FlagValidasi adalah hasil cek otomatis yang disimpan di kolom flag_validasi (JSONB).
type FlagValidasi struct {
	DiLuarRentang bool `json:"di_luar_rentang"`
	Outlier       bool `json:"outlier"`
	Duplikat      bool `json:"duplikat"`
	LuarGeofence  bool `json:"luar_geofence"`
	TanpaLokasi   bool `json:"tanpa_lokasi"`
	// Bentrok menandai entri yang dipertahankan petugas walau harganya berbeda dari petugas lain.
	Bentrok bool `json:"bentrok"`
	// Median adalah harga pembanding (median histori resmi, atau harga acuan master bila histori kurang).
	Median int `json:"median,omitempty"`
	// JarakMeter adalah jarak titik GPS dari pasar.
	JarakMeter int `json:"jarak_meter,omitempty"`
}

// Value menyimpan flag sebagai JSON.
func (f FlagValidasi) Value() (driver.Value, error) {
	raw, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("FlagValidasi: %w", err)
	}
	return string(raw), nil
}

// Scan membaca flag dari kolom JSONB; nilai kosong berarti tanpa bendera.
func (f *FlagValidasi) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*f = FlagValidasi{}
		return nil
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("FlagValidasi: tipe %T tidak didukung", src)
	}
	return json.Unmarshal(raw, f)
}

// BatasHarga memetakan tabel batas_harga: rentang harga wajar per komoditas (dan kabupaten bila diisi).
type BatasHarga struct {
	ID            string    `gorm:"column:id;primaryKey"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KabupatenKode *string   `gorm:"column:kabupaten_kode"`
	BatasMin      int       `gorm:"column:batas_min"`
	BatasMaks     int       `gorm:"column:batas_maks"`
	BerlakuDari   time.Time `gorm:"column:berlaku_dari;type:date"`
}

func (BatasHarga) TableName() string {
	return "batas_harga"
}

// KonteksValidasi adalah data pembanding yang dibaca dari basis data untuk satu entri baru.
type KonteksValidasi struct {
	// Batas adalah rentang wajar yang berlaku; nil bila belum ada.
	Batas *BatasHarga
	// PasarLat, PasarLng, dan RadiusMeter adalah titik dan geofence pasar.
	PasarLat, PasarLng float64
	RadiusMeter        int
	// HargaAcuan adalah harga acuan konsumen komoditas (master).
	HargaAcuan int
	// Histori adalah harga resmi (DIVALIDASI atau DIPUBLIKASI) komoditas dan tipe yang sama di kabupaten
	// yang sama selama HariHistori sebelum waktu pencatatan.
	Histori []int
	// Duplikat true bila entri lain (bukan DRAF atau DITOLAK) memiliki pasar, komoditas, tipe, dan kode
	// pedagang yang sama dalam JendelaBentrok.
	Duplikat bool
}

// HasilValidasi adalah isian kolom hasil cek otomatis entri.
type HasilValidasi struct {
	DeviasiPct    float64
	SkorAnomali   float64
	DalamGeofence bool
	Flag          FlagValidasi
}

// Validasi menjalankan cek otomatis: rentang wajar, outlier terhadap histori, duplikasi, dan geofence.
// Fungsi murni: seluruh masukan datang dari entri dan konteks.
func Validasi(e EntriHarga, k KonteksValidasi) HasilValidasi {
	harga := float64(e.Harga)
	var flag FlagValidasi

	if k.Batas != nil {
		flag.DiLuarRentang = e.Harga < k.Batas.BatasMin || e.Harga > k.Batas.BatasMaks
	}
	flag.Duplikat = k.Duplikat

	median, z, outlierBerbasisHistori := banding(harga, k)
	flag.Outlier = outlierBerbasisHistori && math.Abs(z) > AmbangOutlier
	flag.Median = int(math.Round(median))

	hasil := HasilValidasi{DalamGeofence: true}
	if median > 0 {
		hasil.DeviasiPct = math.Round((harga-median)/median*1000) / 10
	}

	switch {
	case e.Lat == nil || e.Lng == nil:
		flag.TanpaLokasi = true
	default:
		jarak := jarakMeter(*e.Lat, *e.Lng, k.PasarLat, k.PasarLng)
		flag.JarakMeter = int(math.Round(jarak))
		flag.LuarGeofence = jarak > float64(k.RadiusMeter)
		hasil.DalamGeofence = !flag.LuarGeofence
	}

	skor := math.Max(math.Abs(hasil.DeviasiPct)/skalaDeviasiPct, math.Abs(z)/skalaZ)
	skor += bobot(flag.DiLuarRentang, penaltiRentang) + bobot(flag.LuarGeofence, penaltiGeofence) +
		bobot(flag.Duplikat, penaltiDuplikat) + bobot(flag.TanpaLokasi, penaltiTanpaLokasi)
	hasil.SkorAnomali = math.Round(math.Min(1, skor)*100) / 100
	hasil.Flag = flag
	return hasil
}

func bobot(aktif bool, nilai float64) float64 {
	if aktif {
		return nilai
	}
	return 0
}

// banding mengembalikan harga pembanding dan z-score robust. Dengan histori cukup, pembandingnya median
// histori dan z memakai MAD (sebaran minimal 2% median); selain itu pembandingnya harga acuan master dan
// z nol, sehingga outlier tidak ditandai tanpa dasar histori.
func banding(harga float64, k KonteksValidasi) (median, z float64, berbasisHistori bool) {
	if len(k.Histori) >= MinSampelHistori {
		nilai := make([]float64, len(k.Histori))
		for i, h := range k.Histori {
			nilai[i] = float64(h)
		}
		median = medianDari(nilai)
		deviasi := make([]float64, len(nilai))
		for i, v := range nilai {
			deviasi[i] = math.Abs(v - median)
		}
		sebaran := math.Max(madKeSigma*medianDari(deviasi), lantaiSebaranPersen*median)
		return median, (harga - median) / sebaran, true
	}
	return float64(k.HargaAcuan), 0, false
}

func medianDari(nilai []float64) float64 {
	urut := slices.Clone(nilai)
	slices.Sort(urut)
	n := len(urut)
	if n%2 == 1 {
		return urut[n/2]
	}
	return (urut[n/2-1] + urut[n/2]) / 2
}

// jarakMeter adalah jarak haversine dua titik dalam meter.
func jarakMeter(lat1, lng1, lat2, lng2 float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLng := rad(lat2-lat1), rad(lng2-lng1)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * radiusBumi * math.Asin(math.Sqrt(h))
}

// AcuanTipe menurunkan harga acuan komoditas untuk tipe harga: produsen sebagian dari konsumen.
func AcuanTipe(hargaAcuan int, tipe string) int {
	if tipe == TipeProdusen {
		return int(math.Round(float64(hargaAcuan) * RasioProdusen))
	}
	return hargaAcuan
}
