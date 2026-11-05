package dto

// GenerateRequest meminta draf brief untuk satu periode bulanan berformat YYYY-MM.
type GenerateRequest struct {
	Periode string `json:"periode" binding:"required"`
}

// BulkRequest adalah aksi massal pada brief menurut id.
type BulkRequest struct {
	Action string   `json:"action" binding:"required,oneof=telaah setujui"`
	IDs    []string `json:"ids" binding:"required,min=1,max=200,dive,required"`
}
