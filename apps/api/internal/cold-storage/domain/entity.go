package domain

import "time"

// FasilitasColdStorage maps to the fasilitas_cold_storage table.
type FasilitasColdStorage struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Nama             string    `gorm:"column:nama"`
	Lokasi           string    `gorm:"column:lokasi"`
	Kapasitas        float64   `gorm:"column:kapasitas"`
	KapasitasTerpakai float64  `gorm:"column:kapasitas_terpakai"`
	Status           string    `gorm:"column:status"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (FasilitasColdStorage) TableName() string {
	return "fasilitas_cold_storage"
}

// StokColdStorage maps to the stok_cold_storage table.
type StokColdStorage struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	FasilitasID   int64      `gorm:"column:fasilitas_id;index"`
	KomoditasKode string     `gorm:"column:komoditas_kode"`
	Jumlah        float64    `gorm:"column:jumlah"`
	Satuan        string     `gorm:"column:satuan"`
	MasukPada     time.Time  `gorm:"column:masuk_pada"`
	KeluarPada    *time.Time `gorm:"column:keluar_pada"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (StokColdStorage) TableName() string {
	return "stok_cold_storage"
}
