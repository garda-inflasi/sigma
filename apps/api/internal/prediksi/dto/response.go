package dto

type ProyeksiXaiResponse struct {
	ID                 int64    `json:"id"`
	KomoditasKode      string   `json:"komoditas_kode"`
	ProyeksiHarga      float64  `json:"proyeksi_harga"`
	IntervalAtas       float64  `json:"interval_atas"`
	IntervalBawah      float64  `json:"interval_bawah"`
	FaktorDominan      []string `json:"faktor_dominan"`
	TingkatKepercayaan float64  `json:"tingkat_kepercayaan"`
	Periode            string   `json:"periode"`
}

type ListResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}
