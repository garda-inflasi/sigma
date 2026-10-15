package dto

type ListEWSRequest struct {
	Limit int `form:"limit" binding:"min=1,max=1000"`
}

type EWSDetailRequest struct {
	IncludeHistory bool `form:"include_history"`
}

type BulkEWSRequest struct {
	Action  string  `json:"action" binding:"required,oneof=akui eskalasi selesai"`
	IDs     []int64 `json:"ids" binding:"required,min=1"`
	Catatan string  `json:"catatan"`
}
