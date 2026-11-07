package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Status siklus hidup brief (enum status_brief). DIPROSES dan GAGAL adalah status
// penyusunan asinkron: brief baru berstatus DIPROSES sampai draf siap (DRAF) atau gagal (GAGAL).
const (
	StatusDiproses        = "DIPROSES"
	StatusGagal           = "GAGAL"
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

// Asal narasi brief (kolom sumber_narasi).
const (
	PenyusunLLM     = "LLM"
	PenyusunTemplat = "TEMPLAT"
	PenyusunDemo    = "DEMO"
)

// Status brief_job.
const (
	JobBerjalan = "BERJALAN"
	JobSelesai  = "SELESAI"
	JobGagal    = "GAGAL"
)

// PolicyBrief memetakan tabel policy_brief. ID adalah UUID yang diberikan basis data. Kode berbentuk PB-YYYY-MM
// (NOT NULL, UNIQUE, dijaga CHECK) sehingga satu periode hanya bisa punya satu brief; kode juga alias URL.
type PolicyBrief struct {
	ID            string `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Kode          string `gorm:"column:kode" json:"kode"`
	Periode       string `gorm:"column:periode"`
	Judul         string `gorm:"column:judul"`
	Status        string `gorm:"column:status;default:DRAF"`
	Ringkasan     string `gorm:"column:ringkasan;type:text"`
	Rekomendasi   string `gorm:"column:rekomendasi;type:jsonb"`
	Halaman       int    `gorm:"column:halaman"`
	DisusunOlehID string `gorm:"column:disusun_oleh_id;type:uuid"`
	// Konten adalah dokumen JSON Konten (narasi per bagian dan lampiran sitasi);
	// SnapshotData berisi fakta yang dipakai saat penyusunan.
	Konten       string  `gorm:"column:konten;type:jsonb;default:{}"`
	SnapshotData string  `gorm:"column:snapshot_data;type:jsonb;default:{}"`
	SumberNarasi string  `gorm:"column:sumber_narasi;default:TEMPLAT"`
	ModelLLM     *string `gorm:"column:model_llm"`
	PromptVersi  *string `gorm:"column:prompt_versi"`
	PromptHash   *string `gorm:"column:prompt_hash"`
	// Galat mencatat kegagalan penyusunan, atau alasan LLM digantikan templat.
	Galat      *string   `gorm:"column:galat"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
	Diperbarui time.Time `gorm:"column:diperbarui;default:now()"`
}

func (PolicyBrief) TableName() string {
	return "policy_brief"
}

// BriefJob memetakan tabel brief_job: satu percobaan penyusunan brief. ID BIGINT diberikan basis data; Kode
// berisi ID lama (UUID) pada job sebelum migrasi 000114 dan kosong pada job baru.
type BriefJob struct {
	ID      idx.ID     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Kode    *string    `gorm:"column:kode" json:"kode"`
	BriefID string     `gorm:"column:brief_id;type:uuid"`
	Status  string     `gorm:"column:status"`
	Mulai   time.Time  `gorm:"column:mulai;default:now()"`
	Selesai *time.Time `gorm:"column:selesai"`
	Galat   *string    `gorm:"column:galat"`
}

func (BriefJob) TableName() string {
	return "brief_job"
}

// BriefVersi memetakan tabel policy_brief_versi: isi brief pada satu versi. ID BIGINT (versi_id) diberikan
// basis data dan dirujuk oleh telaah dan revisi; (BriefID, Versi) tetap unik dan Versi adalah nomor urut per brief.
type BriefVersi struct {
	ID      idx.ID    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	BriefID string    `gorm:"column:brief_id;type:uuid"`
	Versi   int       `gorm:"column:versi"`
	Konten  string    `gorm:"column:konten;type:jsonb"`
	Oleh    string    `gorm:"column:oleh;type:uuid"`
	Waktu   time.Time `gorm:"column:waktu;default:now()"`
}

func (BriefVersi) TableName() string {
	return "policy_brief_versi"
}

// BriefView adalah brief beserta turunan yang dihitung saat dibaca: nama
// penyusun (join pengguna) dan jumlah butir rekomendasi.
type BriefView struct {
	ID                string    `gorm:"column:id"`
	Kode              string    `gorm:"column:kode"`
	Periode           string    `gorm:"column:periode"`
	Judul             string    `gorm:"column:judul"`
	Status            string    `gorm:"column:status"`
	Ringkasan         string    `gorm:"column:ringkasan"`
	Halaman           int       `gorm:"column:halaman"`
	DisusunOleh       string    `gorm:"column:disusun_oleh"`
	JumlahRekomendasi int       `gorm:"column:jumlah_rekomendasi"`
	SumberNarasi      string    `gorm:"column:sumber_narasi"`
	DibuatPada        time.Time `gorm:"column:dibuat_pada"`
	Diperbarui        time.Time `gorm:"column:diperbarui"`
}

// VersiView adalah satu versi isi brief: ID BIGINT (versi_id), nomor urut, siapa yang menulis (nama), dan kapan.
type VersiView struct {
	ID    idx.ID    `gorm:"column:id"`
	Versi int       `gorm:"column:versi"`
	Oleh  string    `gorm:"column:oleh"`
	Waktu time.Time `gorm:"column:waktu"`
}

// DetailView adalah BriefView lengkap dengan isi dan jejak penyusunannya. RiwayatVersi (versi terbaru lebih dulu)
// diisi oleh use case dari policy_brief_versi.
type DetailView struct {
	BriefView    `gorm:"embedded"`
	Konten       string      `gorm:"column:konten"`
	ModelLLM     *string     `gorm:"column:model_llm"`
	PromptVersi  *string     `gorm:"column:prompt_versi"`
	PromptHash   *string     `gorm:"column:prompt_hash"`
	Galat        *string     `gorm:"column:galat"`
	RiwayatVersi []VersiView `gorm:"-"`
}
