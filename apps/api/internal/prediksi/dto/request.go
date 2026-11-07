package dto

type GetProyeksiRequest struct {
	Kode string `uri:"kode" binding:"required"`
}

// ProyeksiQuery memilih kabupaten dan horizon proyeksi; kosong berarti bawaan
// (Kota Banda Aceh, 14 hari).
type ProyeksiQuery struct {
	Kabupaten string `form:"kabupaten" binding:"omitempty,len=4,numeric"`
	Horizon   *int   `form:"horizon" binding:"omitempty,min=1,max=365"`
}

// KomoditasRequest memilih komoditas untuk melatih atau menyegarkan model.
type KomoditasRequest struct {
	KomoditasKode string `json:"komoditas_kode" binding:"required,max=50"`
}

// BulkModelRequest adalah aksi massal pada registri model. Setiap elemen ids adalah ID BIGINT model atau
// kodenya (mis. MDL-CBM-KRT-v2.3.1, alias ID lama); elemen kosong atau lebih dari 64 karakter ditolak (400),
// sedangkan rujukan yang tidak ada dilaporkan per model sebagai tidak ditemukan.
type BulkModelRequest struct {
	Action string   `json:"action" binding:"required,oneof=promosi arsip"`
	IDs    []string `json:"ids" binding:"required,min=1,max=100,dive,required,max=64"`
}

// SimulasiRequest adalah skenario what-if. Field yang tidak diisi berarti netral:
// curah hujan normal (100), tanpa tambahan pasokan, tanpa subsidi.
type SimulasiRequest struct {
	KomoditasKode string   `json:"komoditas_kode" binding:"required,max=50"`
	KabupatenKode string   `json:"kabupaten_kode" binding:"omitempty,len=4,numeric"`
	HorizonHari   *int     `json:"horizon_hari" binding:"omitempty,min=1,max=365"`
	HujanPct      *float64 `json:"hujan_pct" binding:"omitempty,min=0,max=300"`
	PasokanPct    *float64 `json:"pasokan_pct" binding:"omitempty,min=-50,max=100"`
	SubsidiPct    *float64 `json:"subsidi_pct" binding:"omitempty,min=0,max=100"`
}
