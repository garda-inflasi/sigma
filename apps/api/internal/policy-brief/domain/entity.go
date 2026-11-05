package domain

import "time"

// Status siklus hidup brief (enum status_brief).
const (
	StatusDraf            = "DRAF"
	StatusTelaah          = "TELAAH"
	StatusDisetujui       = "DISETUJUI"
	StatusDidistribusikan = "DIDISTRIBUSIKAN"
)

// Aksi massal atas brief.
const (
	AksiTelaah  = "telaah"
	AksiSetujui = "setujui"
)

// PolicyBrief memetakan tabel policy_brief. ID berbentuk PB-YYYY-MM sehingga satu
// periode hanya bisa punya satu brief.
type PolicyBrief struct {
	ID            string    `gorm:"column:id;primaryKey"`
	Periode       string    `gorm:"column:periode"`
	Judul         string    `gorm:"column:judul"`
	Status        string    `gorm:"column:status;default:DRAF"`
	Ringkasan     string    `gorm:"column:ringkasan;type:text"`
	Rekomendasi   string    `gorm:"column:rekomendasi;type:jsonb"`
	Halaman       int       `gorm:"column:halaman"`
	DisusunOlehID string    `gorm:"column:disusun_oleh_id"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada;default:now()"`
	Diperbarui    time.Time `gorm:"column:diperbarui;default:now()"`
}

func (PolicyBrief) TableName() string {
	return "policy_brief"
}

// BriefView adalah brief beserta turunan yang dihitung saat dibaca: nama
// penyusun (join pengguna) dan jumlah butir rekomendasi.
type BriefView struct {
	ID                string    `gorm:"column:id"`
	Periode           string    `gorm:"column:periode"`
	Judul             string    `gorm:"column:judul"`
	Status            string    `gorm:"column:status"`
	Ringkasan         string    `gorm:"column:ringkasan"`
	Halaman           int       `gorm:"column:halaman"`
	DisusunOleh       string    `gorm:"column:disusun_oleh"`
	JumlahRekomendasi int       `gorm:"column:jumlah_rekomendasi"`
	DibuatPada        time.Time `gorm:"column:dibuat_pada"`
	Diperbarui        time.Time `gorm:"column:diperbarui"`
}
