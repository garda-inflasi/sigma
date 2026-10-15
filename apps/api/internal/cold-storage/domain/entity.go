package domain

import "time"

// FasilitasColdStorage maps to the cold_storage table.
type FasilitasColdStorage struct {
	ID            string  `gorm:"column:id;primaryKey"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	KapasitasTon  int     `gorm:"column:kapasitas_ton"`
	SuhuTargetC   float64 `gorm:"column:suhu_target_c"`
	Status        string  `gorm:"column:status"`
}

func (FasilitasColdStorage) TableName() string {
	return "cold_storage"
}

// StokColdStorage maps to the stok_cold_storage table.
type StokColdStorage struct {
	ID             string    `gorm:"column:id;primaryKey"`
	ColdStorageID  string    `gorm:"column:cold_storage_id;index"`
	KomoditasKode  string    `gorm:"column:komoditas_kode"`
	VolumeTon      float64   `gorm:"column:volume_ton"`
	MasukPada      time.Time `gorm:"column:masuk_pada"`
	Pemilik        string    `gorm:"column:pemilik"`
	RisikoFoodLoss float64   `gorm:"column:risiko_food_loss"`
}

func (StokColdStorage) TableName() string {
	return "stok_cold_storage"
}

// BulkAction represents a bulk action on cold storage.
type BulkAction string

const (
	BulkRencanakanDistribusi BulkAction = "rencanakan_distribusi"
	BulkRekomendasiOlahan    BulkAction = "rekomendasi_olahan"
)
