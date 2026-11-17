package dto

// ListAuditRequest adalah filter dan paginasi cursor untuk GET /v1/audit; ekspor
// (GET /v1/audit/export) memakai filter yang sama dan mengabaikan Limit/Cursor.
// Dari dan Sampai berformat RFC 3339 atau tanggal (YYYY-MM-DD); Sampai bertanggal
// mencakup seluruh hari itu.
type ListAuditRequest struct {
	Dari    string `form:"dari"`
	Sampai  string `form:"sampai"`
	Aktor   string `form:"aktor"`
	Aksi    string `form:"aksi"`
	Entitas string `form:"entitas"`
	Hasil   string `form:"hasil" binding:"omitempty,oneof=BERHASIL GAGAL DITOLAK"`
	// EntitasID, Peran, dan CorrelationID mencocokkan persis; Q mencari bebas.
	EntitasID     string `form:"entitas_id"`
	Peran         string `form:"peran"`
	CorrelationID string `form:"correlation_id"`
	Q             string `form:"q"`
	Limit         int    `form:"limit" binding:"omitempty,min=1"`
	Cursor        string `form:"cursor"`
}

// ListAktivitasSayaRequest adalah filter dan paginasi cursor untuk GET /v1/auth/me/aktivitas. Tidak ada
// parameter yang memilih akun: riwayat selalu milik pemanggil. Dari dan Sampai berformat RFC 3339 atau
// tanggal (YYYY-MM-DD), seperti GET /v1/audit.
type ListAktivitasSayaRequest struct {
	Dari   string `form:"dari"`
	Sampai string `form:"sampai"`
	Aksi   string `form:"aksi"`
	Hasil  string `form:"hasil" binding:"omitempty,oneof=BERHASIL GAGAL DITOLAK"`
	// Sebagai: pelaku (aksi akun ini), sasaran (aksi orang lain terhadap akun ini), atau semua (bawaan).
	Sebagai string `form:"sebagai" binding:"omitempty,oneof=pelaku sasaran semua"`
	Limit   int    `form:"limit" binding:"omitempty,min=1"`
	Cursor  string `form:"cursor"`
}
