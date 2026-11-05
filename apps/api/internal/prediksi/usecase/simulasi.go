package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sumber koefisien simulasi. Saat ini semuanya ASUMSI: riwayat pasokan, cuaca, dan
// subsidi belum tersedia di basis data (ingestion menyusul di 05-B/05-C), sehingga
// sensitivitas harga belum dapat diestimasi dari data.
const (
	SumberKoefisienAsumsi = "ASUMSI"

	// CurahHujanNormal adalah nilai parameter curah hujan yang berarti "sama dengan normal" (persen).
	CurahHujanNormal = 100.0

	catatanTerbatas = "Terbatas: koefisien sensitivitas belum diestimasi dari data (riwayat pasokan, cuaca, dan subsidi " +
		"belum tersedia) dan memakai asumsi generik yang sama untuk semua komoditas. Efek dijumlahkan linear di atas proyeksi " +
		"model. Gunakan sebagai bahan diskusi, bukan keputusan final."
)

// koefisien adalah perubahan harga (poin persen) per satu satuan parameter.
type koefisien struct {
	Faktor string
	Nilai  float64
	Sumber string
}

var (
	koefHujan   = koefisien{"Curah hujan sentra (vs normal)", 0.06, SumberKoefisienAsumsi}
	koefPasokan = koefisien{"Tambahan pasokan antardaerah", -0.35, SumberKoefisienAsumsi}
	koefSubsidi = koefisien{"Subsidi ongkos angkut", -0.18, SumberKoefisienAsumsi}
)

var ErrProyeksiTidakAda = errors.New("proyeksi model produksi tidak ditemukan untuk skenario ini")

// ParameterSimulasi adalah masukan skenario what-if.
type ParameterSimulasi struct {
	KomoditasKode string  `json:"komoditas_kode"`
	KabupatenKode string  `json:"kabupaten_kode"`
	HorizonHari   int     `json:"horizon_hari"`
	HujanPct      float64 `json:"hujan_pct"`
	PasokanPct    float64 `json:"pasokan_pct"`
	SubsidiPct    float64 `json:"subsidi_pct"`
}

// EfekSimulasi adalah dampak satu parameter terhadap perubahan harga proyeksi.
type EfekSimulasi struct {
	Faktor    string  `json:"faktor"`
	Parameter float64 `json:"parameter"`
	Koefisien float64 `json:"koefisien"`
	EfekPct   float64 `json:"efek_pct"`
	Sumber    string  `json:"sumber"`
}

// HasilSimulasi adalah proyeksi dasar model ditambah efek skenario.
type HasilSimulasi struct {
	ID            string         `json:"id"`
	KomoditasKode string         `json:"komoditas_kode"`
	KomoditasNama string         `json:"komoditas_nama"`
	KabupatenKode string         `json:"kabupaten_kode"`
	HorizonHari   int            `json:"horizon_hari"`
	HargaKini     int            `json:"harga_kini"`
	Proyeksi      int            `json:"proyeksi"`
	DasarPct      float64        `json:"dasar_pct"`
	HasilPct      float64        `json:"hasil_pct"`
	BawahPct      float64        `json:"bawah_pct"`
	AtasPct       float64        `json:"atas_pct"`
	Efek          []EfekSimulasi `json:"efek"`
	ModelVersi    string         `json:"model_versi"`
	SumberModel   string         `json:"sumber_model"`
	// Terbatas benar bila ada koefisien yang berupa asumsi, bukan estimasi dari data.
	Terbatas   bool      `json:"terbatas"`
	Catatan    string    `json:"catatan"`
	DibuatPada time.Time `json:"dibuat_pada"`
}

// Simulasi menghitung skenario what-if di server dari proyeksi model produksi.
type Simulasi struct {
	repo  domain.SimulasiRepository
	audit audit.Writer
	log   *logger.Logger
	now   func() time.Time
}

func NewSimulasi(repo domain.SimulasiRepository, auditWriter audit.Writer, log *logger.Logger) *Simulasi {
	return &Simulasi{repo: repo, audit: auditWriter, log: log, now: time.Now}
}

// Hitung menyimpan dan mengembalikan hasil skenario. ErrProyeksiTidakAda bila komoditas
// tidak punya proyeksi model produksi untuk kabupaten dan horizon itu.
func (uc *Simulasi) Hitung(ctx context.Context, aktorID string, p ParameterSimulasi) (*HasilSimulasi, error) {
	dasar, err := uc.repo.GetTerbaru(ctx, p.KomoditasKode, p.KabupatenKode, p.HorizonHari)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProyeksiTidakAda
	}
	if err != nil {
		return nil, err
	}

	sekarang := uc.now()
	hasil := hitungSkenario(*dasar, p)
	hasil.ID, hasil.DibuatPada = uuid.NewString(), sekarang.UTC()

	parameter, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("encode parameter: %w", err)
	}
	rincian, err := json.Marshal(hasil)
	if err != nil {
		return nil, fmt.Errorf("encode hasil: %w", err)
	}
	err = uc.repo.SimpanSkenario(ctx, &domain.SkenarioSimulasi{
		ID: hasil.ID, KomoditasKode: p.KomoditasKode, KabupatenKode: p.KabupatenKode, HorizonHari: p.HorizonHari,
		Parameter: string(parameter), Hasil: string(rincian), DibuatOleh: aktorID, DibuatPada: hasil.DibuatPada,
	})
	if err != nil {
		return nil, err
	}

	if err := uc.audit.Write(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "SIMULASI_SKENARIO", Entitas: "skenario_simulasi", EntitasID: hasil.ID, Hasil: audit.HasilBerhasil,
		Sesudah:  map[string]any{"hasil_pct": hasil.HasilPct, "dasar_pct": hasil.DasarPct},
		Metadata: map[string]any{"komoditas": p.KomoditasKode, "kabupaten": p.KabupatenKode, "horizon_hari": p.HorizonHari},
	}); err != nil {
		uc.log.Warn("prediksi: gagal mencatat audit", map[string]interface{}{"aksi": "SIMULASI_SKENARIO", "id": hasil.ID, "error": err.Error()})
	}
	return hasil, nil
}

// hitungSkenario adalah inti perhitungan: perubahan proyeksi dasar ditambah efek tiap
// parameter (koefisien × parameter), satu desimal. Rentang keyakinan model bergeser
// sebesar total efek. Murni; tidak menyentuh basis data.
func hitungSkenario(dasar domain.ProyeksiView, p ParameterSimulasi) *HasilSimulasi {
	efek := []EfekSimulasi{
		buatEfek(koefHujan, p.HujanPct-CurahHujanNormal, p.HujanPct),
		buatEfek(koefPasokan, p.PasokanPct, p.PasokanPct),
		buatEfek(koefSubsidi, p.SubsidiPct, p.SubsidiPct),
	}
	var total float64
	terbatas := false
	for _, e := range efek {
		total += e.EfekPct
		terbatas = terbatas || e.Sumber == SumberKoefisienAsumsi
	}

	pct := persenTerhadapDasar(dasar.Nilai, dasar.HargaDasar)
	bawah := persenTerhadapDasar(dasar.Bawah, dasar.HargaDasar)
	atas := persenTerhadapDasar(dasar.Atas, dasar.HargaDasar)
	hasilPct := round(pct+total, 1)
	return &HasilSimulasi{
		KomoditasKode: dasar.KomoditasKode, KomoditasNama: dasar.KomoditasNama, KabupatenKode: dasar.KabupatenKode,
		HorizonHari: dasar.HorizonHari, HargaKini: dasar.HargaDasar,
		Proyeksi: int(math.Round(float64(dasar.HargaDasar) * (1 + hasilPct/100))),
		DasarPct: pct, HasilPct: hasilPct, BawahPct: round(bawah+total, 1), AtasPct: round(atas+total, 1),
		Efek: efek, ModelVersi: dasar.ModelVersi, SumberModel: dasar.Sumber,
		Terbatas: terbatas, Catatan: catatanTerbatas,
	}
}

// buatEfek menghitung efek satu parameter; `deviasi` adalah selisih dari keadaan netral,
// `parameter` nilai yang tampil.
func buatEfek(k koefisien, deviasi, parameter float64) EfekSimulasi {
	return EfekSimulasi{Faktor: k.Faktor, Parameter: parameter, Koefisien: k.Nilai, EfekPct: round(k.Nilai*deviasi, 1), Sumber: k.Sumber}
}
