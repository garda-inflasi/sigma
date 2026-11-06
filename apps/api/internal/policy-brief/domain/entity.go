package domain

import "time"

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

// PolicyBrief memetakan tabel policy_brief. ID berbentuk PB-YYYY-MM sehingga satu
// periode hanya bisa punya satu brief.
type PolicyBrief struct {
	ID            string `gorm:"column:id;primaryKey"`
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

// BriefJob memetakan tabel brief_job: satu percobaan penyusunan brief.
type BriefJob struct {
	ID      string     `gorm:"column:id;primaryKey"`
	BriefID string     `gorm:"column:brief_id"`
	Status  string     `gorm:"column:status"`
	Mulai   time.Time  `gorm:"column:mulai;default:now()"`
	Selesai *time.Time `gorm:"column:selesai"`
	Galat   *string    `gorm:"column:galat"`
}

func (BriefJob) TableName() string {
	return "brief_job"
}

// BriefVersi memetakan tabel policy_brief_versi: isi brief pada satu versi.
type BriefVersi struct {
	BriefID string    `gorm:"column:brief_id;primaryKey"`
	Versi   int       `gorm:"column:versi;primaryKey"`
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

// DetailView adalah BriefView lengkap dengan isi dan jejak penyusunannya.
type DetailView struct {
	BriefView   `gorm:"embedded"`
	Konten      string  `gorm:"column:konten"`
	ModelLLM    *string `gorm:"column:model_llm"`
	PromptVersi *string `gorm:"column:prompt_versi"`
	PromptHash  *string `gorm:"column:prompt_hash"`
	Galat       *string `gorm:"column:galat"`
}
