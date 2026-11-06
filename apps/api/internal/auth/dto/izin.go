package dto

import "time"

// Kode galat mesin untuk RBAC berbasis basis data (02-C). Kode ini ikut sampai ke UI lewat BFF (kodeStabil).
const (
	// KodePemegangTerakhir (409): perubahan akan menghapus pemegang terakhir izin pengguna.kelola.
	KodePemegangTerakhir = "LAST_PERMISSION_HOLDER"
	// KodeVersiPeranBerubah (409): matriks peran sudah diubah admin lain; data berisi peran saat ini.
	KodeVersiPeranBerubah = "ROLE_VERSION_CONFLICT"
	// KodeIzinTidakDikenal (400): nama izin tidak ada di katalog.
	KodeIzinTidakDikenal = "UNKNOWN_PERMISSION"
	// KodeIzinDariPeran (409): pengguna sudah memiliki izin itu lewat perannya.
	KodeIzinDariPeran = "PERMISSION_ALREADY_VIA_ROLE"
	// KodeIzinLangsungAda (409): izin langsung yang sama masih berlaku.
	KodeIzinLangsungAda = "DIRECT_PERMISSION_EXISTS"
	// KodeIzinLangsungTidakAda (404): tidak ada izin langsung untuk dicabut.
	KodeIzinLangsungTidakAda = "DIRECT_PERMISSION_NOT_FOUND"
)

// UpdateRolePermissionsRequest mengganti seluruh izin satu peran. Versi adalah versi peran yang dibaca klien
// (GET /v1/roles); versi yang sudah tertinggal dijawab 409 ROLE_VERSION_CONFLICT.
type UpdateRolePermissionsRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
	Versi       int64    `json:"versi" binding:"required,min=1"`
}

// GrantUserPermissionRequest memberi izin langsung. BerlakuSampai (RFC 3339) kosong berarti tanpa batas.
type GrantUserPermissionRequest struct {
	Izin          string     `json:"izin" binding:"required,max=64"`
	BerlakuSampai *time.Time `json:"berlaku_sampai"`
	Alasan        string     `json:"alasan" binding:"required"`
}

// IzinLangsungResponse adalah satu izin langsung milik pengguna.
type IzinLangsungResponse struct {
	Izin              string     `json:"izin"`
	Deskripsi         string     `json:"deskripsi"`
	Kelompok          string     `json:"kelompok"`
	BerlakuSampai     *time.Time `json:"berlaku_sampai"`
	Alasan            string     `json:"alasan"`
	DiberikanOleh     *string    `json:"diberikan_oleh"`
	DiberikanOlehNama *string    `json:"diberikan_oleh_nama"`
	DiberikanPada     time.Time  `json:"diberikan_pada"`
	// Aktif false bila masa berlakunya sudah lewat; barisnya tersimpan sebagai riwayat sampai dicabut.
	Aktif bool `json:"aktif"`
}

// IzinPenggunaResponse adalah ringkasan izin seorang pengguna: izin dari peran, izin langsung, dan yang berlaku.
type IzinPenggunaResponse struct {
	Peran     string                 `json:"peran"`
	IzinPeran []string               `json:"izin_peran"`
	Langsung  []IzinLangsungResponse `json:"langsung"`
	Efektif   []string               `json:"efektif"`
}

// CabutIzinResponse adalah jawaban pencabutan izin langsung.
type CabutIzinResponse struct {
	ID   string `json:"id"`
	Izin string `json:"izin"`
}
