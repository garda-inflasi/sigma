package dto

// ListAuditRequest adalah filter dan paginasi cursor untuk GET /v1/audit.
// Dari dan Sampai berformat RFC 3339 atau tanggal (YYYY-MM-DD); Sampai bertanggal
// mencakup seluruh hari itu.
type ListAuditRequest struct {
	Dari    string `form:"dari"`
	Sampai  string `form:"sampai"`
	Aktor   string `form:"aktor"`
	Aksi    string `form:"aksi"`
	Entitas string `form:"entitas"`
	Hasil   string `form:"hasil" binding:"omitempty,oneof=BERHASIL GAGAL DITOLAK"`
	Limit   int    `form:"limit" binding:"omitempty,min=1"`
	Cursor  string `form:"cursor"`
}
