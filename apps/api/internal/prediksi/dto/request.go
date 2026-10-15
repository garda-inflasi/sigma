package dto

type GetProyeksiRequest struct {
	Kode string `uri:"kode" binding:"required"`
}

type ListRequest struct {
	Page   int    `form:"page" binding:"min=1"`
	Limit  int    `form:"limit" binding:"min=1,max=100"`
	Search string `form:"search"`
}
