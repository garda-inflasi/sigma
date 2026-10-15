package dto

type ListAuditRequest struct {
	Limit int `form:"limit" binding:"min=1,max=500"`
}
