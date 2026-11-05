package domain

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// Status tugas (enum status_tugas).
const (
	StatusBelum     = "BELUM"
	StatusBerjalan  = "BERJALAN"
	StatusSelesai   = "SELESAI"
	StatusTerlambat = "TERLAMBAT"
)

// Tipe harga yang disurvei (enum tipe_harga).
const (
	TipeProdusen = "PRODUSEN"
	TipeKonsumen = "KONSUMEN"
)

// KodeKomoditas adalah kolom TEXT[] berisi kode komoditas yang harus disurvei.
// Kode komoditas hanya memuat huruf, angka, dan tanda hubung, jadi format
// array teks PostgreSQL cukup dibaca tanpa penanganan escape.
type KodeKomoditas []string

// Scan membaca nilai array teks PostgreSQL, mis. {CBM-KRT,BWM}.
func (k *KodeKomoditas) Scan(src any) error {
	var raw string
	switch v := src.(type) {
	case nil:
		*k = nil
		return nil
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("KodeKomoditas: tipe %T tidak didukung", src)
	}

	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return fmt.Errorf("KodeKomoditas: bukan array PostgreSQL: %q", raw)
	}
	body := raw[1 : len(raw)-1]
	if body == "" {
		*k = KodeKomoditas{}
		return nil
	}
	parts := strings.Split(body, ",")
	for i, p := range parts {
		parts[i] = strings.Trim(p, `"`)
	}
	*k = parts
	return nil
}

// Value menulis array teks PostgreSQL. Kode yang memuat karakter di luar
// huruf, angka, dan tanda hubung ditolak.
func (k KodeKomoditas) Value() (driver.Value, error) {
	for _, kode := range k {
		if kode == "" || strings.ContainsAny(kode, `{},"\ `) {
			return nil, fmt.Errorf("KodeKomoditas: kode %q tidak valid", kode)
		}
	}
	return "{" + strings.Join(k, ",") + "}", nil
}

// TugasLapangan memetakan tabel tugas_lapangan.
type TugasLapangan struct {
	ID        string `gorm:"column:id;primaryKey"`
	Judul     string `gorm:"column:judul"`
	PasarID   string `gorm:"column:pasar_id"`
	PetugasID string `gorm:"column:petugas_id"`
	// type:_text adalah nama tipe array teks di pg_type (udt_name), dipakai tes kontrak skema.
	Komoditas     KodeKomoditas `gorm:"column:komoditas;type:_text"`
	Tipe          string        `gorm:"column:tipe"`
	Tenggat       time.Time     `gorm:"column:tenggat"`
	Status        string        `gorm:"column:status;default:BELUM"`
	Target        int           `gorm:"column:target"`
	DibuatPada    time.Time     `gorm:"column:dibuat_pada;default:now()"`
	DibuatOlehID  *string       `gorm:"column:dibuat_oleh_id"`
	SelesaiPada   *time.Time    `gorm:"column:selesai_pada"`
	JumlahSelesai int           `gorm:"column:jumlah_selesai"`
}

func (TugasLapangan) TableName() string {
	return "tugas_lapangan"
}

// TugasView adalah tugas beserta nama pasar, siap ditampilkan.
type TugasView struct {
	ID            string        `gorm:"column:id"`
	Judul         string        `gorm:"column:judul"`
	PasarID       string        `gorm:"column:pasar_id"`
	PasarNama     string        `gorm:"column:pasar_nama"`
	PetugasID     string        `gorm:"column:petugas_id"`
	Komoditas     KodeKomoditas `gorm:"column:komoditas"`
	Tipe          string        `gorm:"column:tipe"`
	Tenggat       time.Time     `gorm:"column:tenggat"`
	Status        string        `gorm:"column:status"`
	Target        int           `gorm:"column:target"`
	JumlahSelesai int           `gorm:"column:jumlah_selesai"`
}

// RincianKomoditas adalah satu komoditas dalam tugas: Terisi benar bila sudah ada entri harga
// yang tertaut ke tugas untuk komoditas itu.
type RincianKomoditas struct {
	Kode   string `gorm:"column:kode"`
	Nama   string `gorm:"column:nama"`
	Terisi bool   `gorm:"column:terisi"`
}

// StatusEfektif menghitung status yang ditampilkan dari data tersimpan:
// tugas belum selesai yang lewat tenggat adalah TERLAMBAT, dan tugas yang sudah
// mulai diisi adalah BERJALAN.
func (t TugasView) StatusEfektif(now time.Time) string {
	switch {
	case t.Status == StatusSelesai:
		return StatusSelesai
	case t.Tenggat.Before(now):
		return StatusTerlambat
	case t.JumlahSelesai > 0:
		return StatusBerjalan
	default:
		return t.Status
	}
}

// BulkAction adalah aksi massal pada tugas lapangan.
type BulkAction string

const (
	BulkUnduhDataAcuan BulkAction = "unduh_data_acuan"
	BulkTandaiSelesai  BulkAction = "tandai_selesai"
)

// Valid memeriksa nama aksi.
func (a BulkAction) Valid() bool {
	return a == BulkUnduhDataAcuan || a == BulkTandaiSelesai
}

// AcuanKomoditas adalah komoditas aktif beserta harga acuan dasarnya.
type AcuanKomoditas struct {
	Kode       string `gorm:"column:kode"`
	Nama       string `gorm:"column:nama"`
	Satuan     string `gorm:"column:satuan"`
	HargaAcuan int    `gorm:"column:harga_acuan"`
}

// AcuanPasar adalah pasar aktif beserta titik dan radius geofence-nya.
type AcuanPasar struct {
	ID            string  `gorm:"column:id"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	RadiusMeter   int     `gorm:"column:radius_meter"`
}

// MedianHarga adalah median harga resmi satu komoditas dan tipe di satu kabupaten.
type MedianHarga struct {
	KomoditasKode string  `gorm:"column:komoditas_kode"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Tipe          string  `gorm:"column:tipe"`
	Median        float64 `gorm:"column:median"`
	Sampel        int     `gorm:"column:sampel"`
}

// DataAcuan adalah bahan validasi harga di perangkat saat offline.
type DataAcuan struct {
	Komoditas []AcuanKomoditas
	Pasar     []AcuanPasar
	Median    []MedianHarga
}
