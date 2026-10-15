package dto

type ListEWSRequest struct {
	Limit int `form:"limit" binding:"min=1,max=1000"`
}
