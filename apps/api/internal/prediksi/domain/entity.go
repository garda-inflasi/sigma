package domain

import "time"

// Nilai kolom sumber: keluaran pipeline ML atau data contoh seeder.
const (
	SumberModel = "MODEL"
	SumberDemo  = "DEMO"
)

// Status model di registri (enum status_model).
const (
	StatusProduksi = "PRODUKSI"
	StatusKandidat = "KANDIDAT"
	StatusArsip    = "ARSIP"
)

// PrediksiHarga memetakan tabel prediksi_harga: satu proyeksi harga konsumen
// untuk komoditas, kabupaten, tanggal target, dan horizon tertentu.
type PrediksiHarga struct {
	ID            string    `gorm:"column:id;primaryKey"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	TanggalTarget time.Time `gorm:"column:tanggal_target;type:date"`
	HorizonHari   int       `gorm:"column:horizon_hari"`
	Nilai         int       `gorm:"column:nilai"`
	Bawah         int       `gorm:"column:bawah"`
	Atas          int       `gorm:"column:atas"`
	HargaDasar    int       `gorm:"column:harga_dasar"`
	ModelVersi    string    `gorm:"column:model_versi"`
	// Faktor adalah dokumen JSON berisi larik []Faktor (kontribusi ala SHAP).
	Faktor     string    `gorm:"column:faktor;type:jsonb"`
	Sumber     string    `gorm:"column:sumber;default:MODEL"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (PrediksiHarga) TableName() string {
	return "prediksi_harga"
}

// ModelRegistry memetakan tabel model_registry: satu versi model per komoditas.
type ModelRegistry struct {
	ID                string     `gorm:"column:id;primaryKey"`
	KomoditasKode     string     `gorm:"column:komoditas_kode"`
	Nama              string     `gorm:"column:nama"`
	Versi             string     `gorm:"column:versi"`
	Mape              float64    `gorm:"column:mape"`
	Drift             float64    `gorm:"column:drift"`
	Status            string     `gorm:"column:status;default:KANDIDAT"`
	DilatihPada       time.Time  `gorm:"column:dilatih_pada"`
	Algorithm         *string    `gorm:"column:algorithm"`
	Metrics           string     `gorm:"column:metrics;type:jsonb;default:{}"`
	DeployedAt        *time.Time `gorm:"column:deployed_at"`
	FeatureImportance string     `gorm:"column:feature_importance;type:jsonb;default:[]"`
	Sumber            string     `gorm:"column:sumber;default:MODEL"`
}

func (ModelRegistry) TableName() string {
	return "model_registry"
}

// Faktor adalah kontribusi satu faktor terhadap perubahan harga proyeksi.
type Faktor struct {
	Nama string `json:"nama"`
	// Kontribusi dalam poin persen terhadap perubahan harga.
	Kontribusi float64 `json:"kontribusi"`
	Detail     string  `json:"detail"`
}

// ProyeksiView adalah satu proyeksi beserta turunan yang dihitung saat dibaca:
// nama komoditas dan kabupaten, serta model yang menghasilkannya.
type ProyeksiView struct {
	ID            string    `gorm:"column:id"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KomoditasNama string    `gorm:"column:komoditas_nama"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	KabupatenNama string    `gorm:"column:kabupaten_nama"`
	TanggalTarget time.Time `gorm:"column:tanggal_target"`
	HorizonHari   int       `gorm:"column:horizon_hari"`
	Nilai         int       `gorm:"column:nilai"`
	Bawah         int       `gorm:"column:bawah"`
	Atas          int       `gorm:"column:atas"`
	HargaDasar    int       `gorm:"column:harga_dasar"`
	Faktor        string    `gorm:"column:faktor"`
	Sumber        string    `gorm:"column:sumber"`
	ModelVersi    string    `gorm:"column:model_versi"`
	ModelNama     string    `gorm:"column:model_nama"`
	ModelMape     float64   `gorm:"column:model_mape"`
	ModelDilatih  time.Time `gorm:"column:model_dilatih"`
}

// TanggalDasar adalah hari proyeksi dibuat (tanggal target dikurangi horizon).
func (p ProyeksiView) TanggalDasar() time.Time {
	return p.TanggalTarget.AddDate(0, 0, -p.HorizonHari)
}

// TitikProyeksi adalah satu titik pada jalur proyeksi menuju tanggal target.
type TitikProyeksi struct {
	TanggalTarget time.Time `gorm:"column:tanggal_target"`
	HorizonHari   int       `gorm:"column:horizon_hari"`
	Nilai         int       `gorm:"column:nilai"`
	Bawah         int       `gorm:"column:bawah"`
	Atas          int       `gorm:"column:atas"`
}

// TitikHarga adalah harga konsumen rata-rata satu hari.
type TitikHarga struct {
	Tanggal time.Time `gorm:"column:tanggal"`
	Harga   int       `gorm:"column:harga"`
}

// ModelView adalah satu baris registri model beserta nama komoditasnya.
type ModelView struct {
	ID            string    `gorm:"column:id"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KomoditasNama string    `gorm:"column:komoditas_nama"`
	Nama          string    `gorm:"column:nama"`
	Versi         string    `gorm:"column:versi"`
	Mape          float64   `gorm:"column:mape"`
	Drift         float64   `gorm:"column:drift"`
	Status        string    `gorm:"column:status"`
	DilatihPada   time.Time `gorm:"column:dilatih_pada"`
	Sumber        string    `gorm:"column:sumber"`
}

// Ringkasan adalah satu proyeksi dengan perubahannya terhadap harga dasar
// (persen, satu desimal), untuk tabel proyeksi seluruh komoditas.
type Ringkasan struct {
	Proyeksi     ProyeksiView
	PerubahanPct float64
	BawahPct     float64
	AtasPct      float64
}

// Detail adalah proyeksi lengkap satu komoditas di satu kabupaten: bentuk yang
// dipakai panel XAI.
type Detail struct {
	Proyeksi     ProyeksiView
	PerubahanPct float64
	BawahPct     float64
	AtasPct      float64
	Riwayat      []TitikHarga
	Jalur        []TitikProyeksi
	Faktor       []Faktor
	Narasi       string
	Rekomendasi  []string
}
