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
