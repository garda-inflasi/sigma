package domain

import (
	"context"
	"time"
)

// ProviderGoogle adalah nilai pengguna.sso_provider dan sso_login_state.provider untuk Google.
const ProviderGoogle = "google"

// SSOLoginState memetakan tabel sso_login_state: langkah awal satu alur Authorization Code + PKCE.
// State dan binding hanya tersimpan sebagai hash SHA-256 (hex); nonce dan code_verifier tidak pernah
// meninggalkan server.
type SSOLoginState struct {
	StateHash    string    `gorm:"column:state_hash;primaryKey"`
	BindingHash  string    `gorm:"column:binding_hash"`
	Provider     string    `gorm:"column:provider"`
	Nonce        string    `gorm:"column:nonce"`
	CodeVerifier string    `gorm:"column:code_verifier"`
	ExpiresAt    time.Time `gorm:"column:expires_at"`
	DibuatPada   time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (SSOLoginState) TableName() string {
	return "sso_login_state"
}

// SSORepository adalah penyimpanan khusus SSO. Terpisah dari Repository supaya penambahan SSO tidak
// mengubah kontrak yang dipakai fitur auth lain.
type SSORepository interface {
	// SaveLoginState menyimpan langkah awal alur dan menghapus baris kedaluwarsa (pembersihan oportunistik).
	SaveLoginState(ctx context.Context, state *SSOLoginState) error
	// ConsumeLoginState mengambil sekaligus menghapus baris dengan hash state tersebut (sekali pakai, atomik).
	// Mengembalikan gorm.ErrRecordNotFound bila tidak ada; kedaluwarsa dan binding diperiksa pemanggil.
	ConsumeLoginState(ctx context.Context, stateHash string) (*SSOLoginState, error)
	// GetUserBySSO mencari akun menurut identitas SSO; gorm.ErrRecordNotFound bila belum ada yang menaut.
	GetUserBySSO(ctx context.Context, provider, subject string) (*Pengguna, error)
	// LinkSSO menautkan identitas ke akun yang belum bertaut (sso_subject masih kosong). Mengembalikan false
	// bila akun sudah bertaut (termasuk oleh permintaan serentak); gorm.ErrDuplicatedKey bila identitas itu
	// sudah dipakai akun lain.
	LinkSSO(ctx context.Context, userID, provider, subject string, at time.Time) (bool, error)
}
