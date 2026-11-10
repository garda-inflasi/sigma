package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Siklus ekonomi (enum siklus).
const (
	SiklusNormal  = "NORMAL"
	SiklusTekanan = "TEKANAN"
	SiklusSurplus = "SURPLUS"
)

// WilayahBawaan adalah kode provinsi Aceh: siklus dominan yang mewarnai seluruh antarmuka.
const WilayahBawaan = "11"

// Indikator aturan penentuan siklus (kolom aturan_siklus.indikator).
const (
	// IndikatorAlertEws: jumlah alert EWS yang belum SELESAI di wilayah menurut jenis dan tingkat minimum.
	IndikatorAlertEws = "ALERT_EWS"
	// IndikatorUtilisasiStok: persen terisi cold storage OPERASIONAL di wilayah.
	IndikatorUtilisasiStok = "UTILISASI_STOK"
)

// SiklusEkonomi memetakan tabel siklus_ekonomi. PK-nya ID BIGINT identity; Kode memuat ID lama
// (mis. SE-11-DEMO) dan menjadi kunci idempotensi seeder demo. wilayah_kode -> provinsi (RESTRICT).
type SiklusEkonomi struct {
	ID          idx.ID     `gorm:"column:id;primaryKey;autoIncrement"`
	Kode        *string    `gorm:"column:kode"`
	WilayahKode string     `gorm:"column:wilayah_kode"`
	Siklus      string     `gorm:"column:siklus"`
	MulaiPada   time.Time  `gorm:"column:mulai_pada"`
	AkhirPada   *time.Time `gorm:"column:akhir_pada"`
	DibuatPada  time.Time  `gorm:"column:dibuat_pada;default:now()"`
	// Otomatis menandai baris yang dibuka mesin penentuan siklus; false untuk seed atau penetapan manual.
	Otomatis bool `gorm:"column:otomatis"`
	// Alasan adalah foto keputusan mesin (dokumen JSON bentuk Alasan) saat siklus dibuka; kosong untuk baris tak otomatis.
	Alasan *string `gorm:"column:alasan;type:jsonb"`
	// PulihSejak adalah sejak kapan tidak ada aturan siklus ini yang memicu (penanda masa pulih). Hanya ada di
	// baris aktif; kosong selama siklus masih dipicu.
	PulihSejak *time.Time `gorm:"column:pulih_sejak"`
}

func (SiklusEkonomi) TableName() string {
	return "siklus_ekonomi"
}

// AturanSiklus memetakan tabel aturan_siklus: satu aturan penentuan siklus sebagai data. Tiap aturan menyebut siklus yang
// dipicunya (TEKANAN atau SURPLUS), indikatornya, dan parameter indikator itu: ALERT_EWS memakai JenisEws, SeverityMin,
// dan JumlahMin; UTILISASI_STOK memakai Ambang (persen). Kolom yang tidak dipakai indikatornya bernilai NULL (CHECK).
type AturanSiklus struct {
	ID idx.ID `gorm:"column:id;primaryKey;autoIncrement"`
	// Kode (SKL-...) adalah kunci idempoten seed reference.
	Kode      string `gorm:"column:kode"`
	Siklus    string `gorm:"column:siklus"`
	Indikator string `gorm:"column:indikator"`
	// JenisEws, SeverityMin, dan JumlahMin hanya untuk ALERT_EWS: aturan memicu bila jumlah alert belum SELESAI di
	// wilayah dengan jenis itu dan tingkat >= SeverityMin mencapai JumlahMin.
	JenisEws    *string `gorm:"column:jenis_ews"`
	SeverityMin *string `gorm:"column:severity_min"`
	JumlahMin   *int    `gorm:"column:jumlah_min"`
	// Ambang hanya untuk UTILISASI_STOK: aturan memicu bila utilisasi (persen) >= Ambang.
	Ambang *float64 `gorm:"column:ambang"`
	// Prioritas: angka kecil lebih diutamakan saat aturan siklus berbeda memicu bersamaan.
	Prioritas int `gorm:"column:prioritas"`
	// PulihJam adalah lama (jam) tidak ada aturan siklus ini yang memicu sebelum siklus dilepas.
	PulihJam   int       `gorm:"column:pulih_jam"`
	Aktif      bool      `gorm:"column:aktif"`
	Keterangan string    `gorm:"column:keterangan"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (AturanSiklus) TableName() string {
	return "aturan_siklus"
}

// Memicu menilai apakah pengukuran memenuhi aturan. Pengukuran yang tidak terukur (tidak ada data) tidak pernah
// memicu. Batasnya inklusif: jumlah alert tepat JumlahMin, atau utilisasi tepat Ambang, sudah memicu.
func (a AturanSiklus) Memicu(p Pengukuran) bool {
	if !p.Terukur {
		return false
	}
	switch a.Indikator {
	case IndikatorAlertEws:
		return a.JumlahMin != nil && p.Nilai >= float64(*a.JumlahMin)
	case IndikatorUtilisasiStok:
		return a.Ambang != nil && p.Nilai >= *a.Ambang
	}
	return false
}

// RankSiklus mengurutkan siklus pemicu untuk pemutus seri prioritas: TEKANAN lebih dulu dari SURPLUS. NORMAL (bukan
// pemicu) paling akhir.
func RankSiklus(siklus string) int {
	switch siklus {
	case SiklusTekanan:
		return 1
	case SiklusSurplus:
		return 2
	}
	return 3
}
