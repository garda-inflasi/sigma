package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Pemicu evaluasi: apa yang menjalankan satu putaran mesin. Dicatat di metadata audit.
const (
	PemicuJadwal = "jadwal"
	PemicuAlert  = "alert"
	PemicuManual = "manual"
)

// Sebab evaluasi dilewati tanpa mengubah apa pun (HasilEvaluasi.SebabDilewati).
const (
	// SebabTanpaAturan: tidak ada aturan aktif, jadi mesin tidak berhak menyimpulkan apa pun. Tanpa penjagaan ini,
	// "tidak ada pemicu" akan salah dibaca sebagai "semuanya normal" dan melepas siklus yang sedang berjalan.
	SebabTanpaAturan = "tanpa_aturan"
	// SebabWilayahTakDikenal: wilayah belum ada di master provinsi (mis. basis data baru dimigrasi tetapi belum di-seed).
	SebabWilayahTakDikenal = "wilayah_tak_dikenal"
)

var (
	// ErrEvaluasiSibuk: kunci evaluasi wilayah tidak didapat dalam batas waktu karena evaluasi lain belum selesai.
	ErrEvaluasiSibuk = errors.New("evaluasi siklus lain sedang berjalan")
	// ErrBentrokSiklus: ada penulis lain yang membuka siklus aktif lebih dulu (indeks unik siklus aktif per wilayah).
	ErrBentrokSiklus = errors.New("siklus aktif sudah diubah penulis lain")
)

// Pengukuran adalah hasil mengukur satu aturan terhadap data nyata.
type Pengukuran struct {
	// Terukur false bila datanya tidak ada (mis. tidak ada fasilitas cold storage operasional): aturan tidak memicu.
	Terukur bool
	// Nilai: ALERT_EWS = jumlah alert yang cocok; UTILISASI_STOK = persen terisi.
	Nilai float64
	// Rujukan memuat kode alert contoh (paling parah dan terbaru lebih dulu, paling banyak lima) untuk ALERT_EWS.
	Rujukan []string
	// Rincian adalah keterangan angka untuk manusia, mis. "813 dari 950 ton".
	Rincian string
}

// HasilAturan adalah satu aturan beserta hasil pengukurannya pada satu putaran evaluasi.
type HasilAturan struct {
	Aturan     AturanSiklus
	Pengukuran Pengukuran
	// Memicu adalah Aturan.Memicu(Pengukuran).
	Memicu bool
}

// PemicuAlasan menjelaskan satu aturan di dalam alasan keputusan.
type PemicuAlasan struct {
	Aturan    string `json:"aturan"`
	Siklus    string `json:"siklus"`
	Indikator string `json:"indikator"`
	// Nilai adalah hasil ukur; Ambang adalah jumlah minimum (ALERT_EWS) atau persen (UTILISASI_STOK); Satuan keduanya.
	Nilai     float64  `json:"nilai"`
	Ambang    float64  `json:"ambang"`
	Satuan    string   `json:"satuan"`
	Prioritas int      `json:"prioritas"`
	Rujukan   []string `json:"rujukan,omitempty"`
	Rincian   string   `json:"rincian,omitempty"`
}

// Alasan adalah foto keputusan mesin yang disimpan di siklus_ekonomi.alasan: kalimat ringkas, aturan yang memicu
// siklus terpilih, dan aturan siklus lain yang ikut memicu tetapi kalah prioritas.
type Alasan struct {
	Ringkasan string         `json:"ringkasan"`
	Pemicu    []PemicuAlasan `json:"pemicu"`
	Ditekan   []PemicuAlasan `json:"ditekan"`
}

// Keputusan adalah hasil murni mesin dari satu keadaan: siklus yang seharusnya aktif.
type Keputusan struct {
	// Siklus yang harus aktif sesudah evaluasi.
	Siklus string
	// Ganti true bila siklus aktif harus ditutup lalu dibuka yang baru (atau dibuka bila belum ada siklus aktif).
	Ganti bool
	// PulihSejak adalah nilai baru penanda masa pulih untuk siklus aktif yang dipertahankan (Ganti false);
	// nil berarti penanda dihapus (siklus masih dipicu atau NORMAL).
	PulihSejak *time.Time
	// Alasan penuh hanya untuk keputusan Ganti; untuk keputusan tetap hanya Ringkasan yang terisi.
	Alasan Alasan
}

// OpsiEvaluasi menjelaskan siapa atau apa yang meminta evaluasi (untuk audit).
type OpsiEvaluasi struct {
	// AktorID adalah pengguna yang meminta evaluasi manual; kosong untuk sistem.
	AktorID string
	// Pemicu adalah salah satu PemicuJadwal, PemicuAlert, PemicuManual.
	Pemicu string
}

// HasilEvaluasi melaporkan satu putaran mesin.
type HasilEvaluasi struct {
	WilayahKode string
	// Dilewati true bila evaluasi tidak dijalankan (lihat SebabDilewati); tidak ada yang berubah.
	Dilewati      bool
	SebabDilewati string
	// Berubah true bila siklus aktif diganti (atau dibuka pertama kali) pada putaran ini.
	Berubah bool
	// Sebelumnya adalah siklus aktif sebelum putaran; kosong bila belum ada.
	Sebelumnya string
	// Aktif adalah siklus aktif sesudah putaran; nil bila dilewati dan belum ada siklus aktif.
	Aktif *SiklusEkonomi
	// Ringkasan menjelaskan keputusan putaran ini dengan kalimat.
	Ringkasan string
	// Aturan memuat hasil tiap aturan aktif (kosong bila dilewati).
	Aturan []HasilAturan
}

// Evaluator menjalankan satu putaran mesin penentuan siklus.
type Evaluator interface {
	Evaluasi(ctx context.Context, wilayahKode string, opsi OpsiEvaluasi) (HasilEvaluasi, error)
}

// SiklusBaru adalah siklus yang hendak dibuka mesin.
type SiklusBaru struct {
	WilayahKode string
	Siklus      string
	Alasan      Alasan
}

// EngineRepository adalah akses data yang dibutuhkan mesin. Seluruh putaran berjalan di dalam satu transaksi yang
// memegang kunci evaluasi wilayah, sehingga dua evaluasi serentak (job, evaluasi manual, pemicu alert, antar-instance)
// tidak pernah saling menimpa.
type EngineRepository interface {
	// Dalam menjalankan fn di dalam satu transaksi yang memegang kunci evaluasi wilayah. Galat dari fn membatalkan
	// transaksi. Mengembalikan ErrEvaluasiSibuk bila kunci tidak didapat dalam batas waktu dan ErrBentrokSiklus bila
	// penulis lain membuka siklus aktif lebih dulu.
	Dalam(ctx context.Context, wilayahKode string, fn func(Sesi) error) error
}

// Sesi adalah akses data di dalam satu transaksi evaluasi.
type Sesi interface {
	// WilayahAda memeriksa wilayah di master provinsi.
	WilayahAda(ctx context.Context) (bool, error)
	// Sekarang mengembalikan jam basis data, satu-satunya sumber waktu mesin sehingga antar-instance dan baris lain
	// (mulai_pada, pulih_sejak) memakai jam yang sama.
	Sekarang(ctx context.Context) (time.Time, error)
	// AturanAktif mengembalikan aturan yang aktif, terurut menurut prioritas.
	AturanAktif(ctx context.Context) ([]AturanSiklus, error)
	// Ukur mengukur satu aturan pada waktu sekarang dari data nyata.
	Ukur(ctx context.Context, aturan AturanSiklus, sekarang time.Time) (Pengukuran, error)
	// SiklusAktif mengunci dan mengembalikan siklus aktif wilayah; nil bila belum ada.
	SiklusAktif(ctx context.Context) (*SiklusEkonomi, error)
	// CatatPulih menulis penanda masa pulih siklus aktif (nil = hapus).
	CatatPulih(ctx context.Context, id idx.ID, sejak *time.Time) error
	// Ganti menutup siklus aktif lama (bila ada) lalu membuka yang baru pada saat yang sama, dan mengembalikan baris
	// baru. Waktu tutup/buka tidak pernah mendahului mulai_pada siklus lama (CHECK akhir_pada > mulai_pada).
	Ganti(ctx context.Context, lama *SiklusEkonomi, baru SiklusBaru, sekarang time.Time) (*SiklusEkonomi, error)
}
