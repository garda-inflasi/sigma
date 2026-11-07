package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

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
// untuk komoditas, kabupaten, tanggal target, dan horizon tertentu. ID BIGINT identity diberikan basis data.
type PrediksiHarga struct {
	ID idx.ID `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// Kode adalah ID lama proyeksi (kunci idempoten seeder demo); NULL untuk proyeksi hasil pipeline ML, yang
	// di-upsert menurut kunci alaminya (komoditas, kabupaten, tanggal target, horizon, versi).
	Kode          *string   `gorm:"column:kode" json:"kode"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	TanggalTarget time.Time `gorm:"column:tanggal_target;type:date"`
	HorizonHari   int       `gorm:"column:horizon_hari"`
	Nilai         int       `gorm:"column:nilai"`
	Bawah         int       `gorm:"column:bawah"`
	Atas          int       `gorm:"column:atas"`
	HargaDasar    int       `gorm:"column:harga_dasar"`
	// ModelID adalah model yang menghasilkan proyeksi (FK ke model_registry). ModelVersi adalah versi model itu,
	// disimpan sebagai informasi dan bagian kunci unik; keduanya wajib ditulis konsisten.
	ModelID    idx.ID `gorm:"column:model_id"`
	ModelVersi string `gorm:"column:model_versi"`
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
	ID idx.ID `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// Kode adalah kode tampilan model (MDL-<komoditas>-<versi>) dan alias rujukan; ID lama sebelum migrasi 000114.
	Kode              *string    `gorm:"column:kode" json:"kode"`
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
	// ArtefakURI menandai implementasi yang menghasilkan model (layanan ML dan versinya).
	ArtefakURI *string `gorm:"column:artefak_uri"`
	Catatan    string  `gorm:"column:catatan"`
	// DipromosikanOleh dan DipromosikanPada mencatat promosi terakhir ke PRODUKSI.
	DipromosikanOleh *string    `gorm:"column:dipromosikan_oleh;type:uuid"`
	DipromosikanPada *time.Time `gorm:"column:dipromosikan_pada"`
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
	ID idx.ID `gorm:"column:id"`
	// ModelID adalah model yang menghasilkan proyeksi ini (FK model_id).
	ModelID       idx.ID    `gorm:"column:model_id"`
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

// PrediksiEvaluasi memetakan tabel prediksi_evaluasi: galat nyata satu proyeksi
// setelah tanggal targetnya lewat.
type PrediksiEvaluasi struct {
	ID             idx.ID    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PrediksiID     idx.ID    `gorm:"column:prediksi_id"`
	Realisasi      int       `gorm:"column:realisasi"`
	GalatPct       float64   `gorm:"column:galat_pct"`
	DievaluasiPada time.Time `gorm:"column:dievaluasi_pada;default:now()"`
}

func (PrediksiEvaluasi) TableName() string {
	return "prediksi_evaluasi"
}

// Aksi pada riwayat model_promosi.
const (
	AksiPromosi = "PROMOSI"
	AksiArsip   = "ARSIP"
)

// ModelPromosi memetakan tabel model_promosi: satu keputusan promosi atau pengarsipan model.
type ModelPromosi struct {
	ID idx.ID `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// Kode adalah ID lama riwayat (sebelum migrasi 000114); NULL untuk riwayat baru.
	Kode              *string   `gorm:"column:kode" json:"kode"`
	ModelID           idx.ID    `gorm:"column:model_id"`
	KomoditasKode     string    `gorm:"column:komoditas_kode"`
	Aksi              string    `gorm:"column:aksi"`
	StatusLama        string    `gorm:"column:status_lama"`
	StatusBaru        string    `gorm:"column:status_baru"`
	ModelDigantikanID *idx.ID   `gorm:"column:model_digantikan_id"`
	Oleh              string    `gorm:"column:oleh;type:uuid"`
	Waktu             time.Time `gorm:"column:waktu;default:now()"`
}

func (ModelPromosi) TableName() string {
	return "model_promosi"
}

// SkenarioSimulasi memetakan tabel skenario_simulasi: satu simulasi what-if yang
// dihitung server. Parameter dan Hasil adalah dokumen JSON.
type SkenarioSimulasi struct {
	ID idx.ID `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// Kode adalah ID lama skenario (UUID, sebelum migrasi 000114) dan alias rujukan; NULL untuk skenario baru.
	Kode          *string   `gorm:"column:kode" json:"kode"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	HorizonHari   int       `gorm:"column:horizon_hari"`
	Parameter     string    `gorm:"column:parameter;type:jsonb"`
	Hasil         string    `gorm:"column:hasil;type:jsonb"`
	DibuatOleh    string    `gorm:"column:dibuat_oleh;type:uuid"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (SkenarioSimulasi) TableName() string {
	return "skenario_simulasi"
}

// ModelView adalah satu baris registri model beserta nama komoditasnya.
type ModelView struct {
	ID idx.ID `gorm:"column:id"`
	// Kode adalah kode tampilan model (MDL-<komoditas>-<versi>); nil untuk model tanpa kode.
	Kode          *string   `gorm:"column:kode"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KomoditasNama string    `gorm:"column:komoditas_nama"`
	Nama          string    `gorm:"column:nama"`
	Versi         string    `gorm:"column:versi"`
	Mape          float64   `gorm:"column:mape"`
	Drift         float64   `gorm:"column:drift"`
	Status        string    `gorm:"column:status"`
	DilatihPada   time.Time `gorm:"column:dilatih_pada"`
	Sumber        string    `gorm:"column:sumber"`
	Catatan       string    `gorm:"column:catatan"`
	// BaselineMape adalah MAPE baseline naif pada backtest yang sama (nil bila tidak diukur,
	// mis. model demo).
	BaselineMape *float64 `gorm:"column:baseline_mape"`
	// MapeRealisasi adalah rata-rata |galat| proyeksi model yang sudah jatuh tempo dan
	// dievaluasi; nil bila belum ada. NEvaluasi adalah jumlah proyeksi yang dievaluasi.
	MapeRealisasi *float64 `gorm:"column:mape_realisasi"`
	NEvaluasi     int      `gorm:"column:n_evaluasi"`
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
