package dto

type PolicyBriefResponse struct {
	ID           int64  `json:"id"`
	Judul        string `json:"judul"`
	Periode      string `json:"periode"`
	Ringkasan    string `json:"ringkasan"`
	Status       string `json:"status"`
	DibuatPada   string `json:"dibuat_pada"`
	DiupdatePada string `json:"diupdate_pada"`
}

type GenerateResponse struct {
	ID int64 `json:"id"`
}
