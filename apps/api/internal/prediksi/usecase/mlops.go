package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/internal/prediksi/mlclient"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

var (
	// ErrLayananMLMati berarti layanan ML tidak dikonfigurasi atau tidak terjangkau.
	ErrLayananMLMati = errors.New("layanan ML tidak tersedia")
	// ErrMLGagal berarti layanan ML menolak permintaan atau menjawab tidak sesuai kontrak.
	ErrMLGagal = errors.New("layanan ML gagal memproses permintaan")
	// ErrDataTidakCukup berarti entri harga tervalidasi belum cukup untuk dilatih.
	ErrDataTidakCukup = errors.New("data harga tervalidasi belum cukup untuk melatih model")
	// ErrModelTidakAda berarti komoditas tidak punya model produksi.
	ErrModelTidakAda = errors.New("model produksi komoditas tidak ditemukan")
	// ErrLebihBurukDariBaseline: model hasil pelatihan tidak boleh dipromosikan bila MAPE
	// backtest-nya lebih buruk dari baseline naif.
	ErrLebihBurukDariBaseline = errors.New("MAPE model lebih buruk dari baseline")
	// ErrAksiModelTidakValid: aksi massal registri tidak dikenal.
	ErrAksiModelTidakValid = errors.New("aksi tidak valid")
	// ErrBukanHasilLatih berarti model produksi berupa data demo, bukan hasil pelatihan.
	ErrBukanHasilLatih = errors.New("model produksi bukan hasil pelatihan; latih model terlebih dahulu")
)

const (
	// riwayatLatihHari adalah panjang riwayat harga yang diambil untuk melatih dan meramal.
	riwayatLatihHari = 180
	namaModelLatih   = "Seleksi otomatis: naive, seasonal naive, Holt, Holt-Winters"
	statusSeriOK     = "OK"

	// maksKode adalah lebar kolom kode (varchar(64)) dan batas panjang kode di rute dan body.
	maksKode = 64
)

// RujukanModel membaca rujukan model dari masukan klien: bilangan bulat positif adalah ID BIGINT, selain itu
// kode tampilan (mis. MDL-CBM-KRT-v2.3.1, ID lama sebelum migrasi 000114) yang dicari pada kolom kode. Kode tidak
// boleh berupa angka murni. Teks kosong atau lebih panjang dari kolom kode bukan rujukan yang sah.
func RujukanModel(teks string) (idx.RujukanID, bool) {
	if id, err := idx.Parse(teks); err == nil {
		return idx.RujukanID{ID: id}, true
	}
	if teks == "" || len(teks) > maksKode {
		return idx.RujukanID{}, false
	}
	return idx.RujukanID{Kode: teks}, true
}

// kodeModel membentuk kode tampilan model "MDL-<komoditas>-<versi>"; nil bila tidak muat di kolom kode.
func kodeModel(komoditas, versi string) *string {
	kode := "MDL-" + komoditas + "-" + versi
	if len(kode) > maksKode {
		return nil
	}
	return &kode
}

// horizonLatih adalah horizon proyeksi yang dihasilkan, dalam hari.
var horizonLatih = []int{7, 14, 30}

// MLOps menjalankan siklus model: melatih lewat layanan ML, menyegarkan proyeksi
// model produksi, dan mengevaluasi proyeksi yang sudah jatuh tempo.
type MLOps struct {
	repo  domain.MLRepository
	ml    mlclient.Client // nil bila layanan ML tidak dikonfigurasi
	audit audit.Writer
	log   *logger.Logger
	now   func() time.Time
}

func NewMLOps(repo domain.MLRepository, ml mlclient.Client, auditWriter audit.Writer, log *logger.Logger) *MLOps {
	return &MLOps{repo: repo, ml: ml, audit: auditWriter, log: log, now: time.Now}
}

// StatusML adalah keadaan layanan ML yang terlihat dari API.
type StatusML struct {
	Dikonfigurasi bool
	Tersedia      bool
	Versi         string
}

// Status memeriksa layanan ML; tidak pernah galat (layanan mati = Tersedia false).
func (uc *MLOps) Status(ctx context.Context) StatusML {
	if uc.ml == nil {
		return StatusML{}
	}
	h, err := uc.ml.Health(ctx)
	if err != nil {
		return StatusML{Dikonfigurasi: true}
	}
	return StatusML{Dikonfigurasi: true, Tersedia: true, Versi: h.Versi}
}

// Latih melatih model baru (status KANDIDAT) untuk satu komoditas dari harga konsumen
// tervalidasi, menyimpan metrik backtest ke registri dan proyeksinya ke prediksi_harga.
// Model baru tidak dipakai sebelum dipromosikan manusia.
func (uc *MLOps) Latih(ctx context.Context, aktorID, kode string) (*domain.ModelView, error) {
	if uc.ml == nil {
		return nil, ErrLayananMLMati
	}
	sekarang := uc.now()
	seri, err := uc.repo.SeriKonsumen(ctx, kode, sekarang.AddDate(0, 0, -riwayatLatihHari))
	if err != nil {
		return nil, err
	}
	if len(seri) == 0 {
		return nil, ErrDataTidakCukup
	}

	res, err := uc.ml.Train(ctx, mlclient.TrainRequest{KomoditasKode: kode, Horizon: slices.Clone(horizonLatih), Seri: keSeriML(seri)})
	if err != nil {
		return nil, petakanGalatML(err)
	}
	if res.Ringkasan.NSeriOK == 0 {
		return nil, fmt.Errorf("%w: %s", ErrDataTidakCukup, alasanPertama(res.Seri))
	}

	diminta := make(map[string]bool, len(seri))
	for _, s := range seri {
		diminta[s.KabupatenKode] = true
	}
	model, proyeksi, err := bangunModel(kode, sekarang, res, diminta)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.SimpanModel(ctx, model, proyeksi); err != nil {
		return nil, err
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "LATIH_MODEL", Entitas: "model_registry", EntitasID: model.ID.String(), Hasil: audit.HasilBerhasil,
		Sesudah: map[string]any{
			"versi": model.Versi, "status": model.Status, "mape": model.Mape,
			"baseline_mape": res.Ringkasan.BaselineMape, "n_seri": res.Ringkasan.NSeriOK,
		},
		Metadata: map[string]any{"kode": model.Kode},
	})
	return uc.repo.GetModelView(ctx, model.ID)
}

// Segarkan meramal ulang proyeksi model produksi hasil pelatihan dari harga terbaru,
// memakai parameter yang sudah dilatih (tanpa melatih ulang). Mengembalikan jumlah
// proyeksi yang ditulis.
func (uc *MLOps) Segarkan(ctx context.Context, aktorID, kode string) (int, error) {
	if uc.ml == nil {
		return 0, ErrLayananMLMati
	}
	model, err := uc.repo.ModelProduksi(ctx, kode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrModelTidakAda
	}
	if err != nil {
		return 0, err
	}
	if model.Sumber != domain.SumberModel {
		return 0, ErrBukanHasilLatih
	}
	metrik, err := domain.BacaMetrik(model.Metrics)
	if err != nil {
		return 0, err
	}

	sekarang := uc.now()
	seri, err := uc.repo.SeriKonsumen(ctx, kode, sekarang.AddDate(0, 0, -riwayatLatihHari))
	if err != nil {
		return 0, err
	}
	req := mlclient.PredictRequest{KomoditasKode: kode, Horizon: metrik.Horizon}
	diminta := map[string]bool{}
	for _, s := range keSeriML(seri) {
		terlatih, ok := metrik.Seri[s.ID]
		if !ok {
			continue // kabupaten yang tak ikut dilatih tidak diramal
		}
		diminta[s.ID] = true
		req.Seri = append(req.Seri, mlclient.SeriPrediksi{Seri: s, Algoritma: terlatih.Algoritma, Params: terlatih.Params, GalatRMSE: terlatih.GalatRMSE})
	}
	if len(req.Seri) == 0 {
		return 0, ErrDataTidakCukup
	}

	res, err := uc.ml.Predict(ctx, req)
	if err != nil {
		return 0, petakanGalatML(err)
	}
	proyeksi, err := proyeksiDariHasil(kode, model.ID, model.Versi, sekarang, res.Seri, diminta)
	if err != nil {
		return 0, err
	}
	if len(proyeksi) == 0 {
		return 0, ErrDataTidakCukup
	}
	if err := uc.repo.SimpanProyeksi(ctx, proyeksi); err != nil {
		return 0, err
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "SEGARKAN_PROYEKSI", Entitas: "model_registry", EntitasID: model.ID.String(), Hasil: audit.HasilBerhasil,
		Sesudah: map[string]any{"versi": model.Versi, "proyeksi": len(proyeksi)},
	})
	return len(proyeksi), nil
}

// Evaluasi membandingkan proyeksi model yang sudah jatuh tempo dengan harga
// realisasi (dasar MAPE realisasi). Mengembalikan jumlah proyeksi yang dievaluasi.
func (uc *MLOps) Evaluasi(ctx context.Context, aktorID string) (int64, error) {
	n, err := uc.repo.EvaluasiJatuhTempo(ctx, uc.now())
	if err != nil {
		return 0, err
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "EVALUASI_PREDIKSI", Entitas: "prediksi_evaluasi", EntitasID: "-", Hasil: audit.HasilBerhasil,
		Sesudah: map[string]any{"dievaluasi": n},
	})
	return n, nil
}

func (uc *MLOps) catat(ctx context.Context, e audit.Entry) {
	if err := uc.audit.Write(ctx, e); err != nil {
		uc.log.Warn("prediksi: gagal mencatat audit", map[string]interface{}{"aksi": e.Aksi, "id": e.EntitasID, "error": err.Error()})
	}
}

func petakanGalatML(err error) error {
	switch {
	case errors.Is(err, mlclient.ErrTidakTersedia):
		return fmt.Errorf("%w: %w", ErrLayananMLMati, err)
	case errors.Is(err, mlclient.ErrDitolak):
		return fmt.Errorf("%w: %w", ErrMLGagal, err)
	}
	return err
}

func alasanPertama(seri []mlclient.HasilSeri) string {
	for _, s := range seri {
		if s.Alasan != "" {
			return s.Alasan
		}
	}
	return "tidak ada seri yang dapat dilatih"
}

func keSeriML(seri []domain.SeriHarian) []mlclient.Seri {
	out := make([]mlclient.Seri, len(seri))
	for i, s := range seri {
		titik := make([]mlclient.Titik, len(s.Titik))
		for j, t := range s.Titik {
			titik[j] = mlclient.Titik{Tanggal: t.Tanggal.Format(time.DateOnly), Nilai: t.Harga}
		}
		out[i] = mlclient.Seri{ID: s.KabupatenKode, Titik: titik}
	}
	return out
}

func round(v float64, desimal int) float64 {
	p := math.Pow10(desimal)
	return math.Round(v*p) / p
}

// bangunModel menyusun baris registri (KANDIDAT) dan proyeksi dari jawaban pelatihan. ID model belum ada
// (diberikan basis data saat disimpan), jadi proyeksi baru ditautkan ke model oleh SimpanModel.
func bangunModel(kode string, sekarang time.Time, res *mlclient.TrainResponse, diminta map[string]bool) (*domain.ModelRegistry, []domain.PrediksiHarga, error) {
	versi := "ml-" + sekarang.UTC().Format("20060102-150405")
	proyeksi, err := proyeksiDariHasil(kode, 0, versi, sekarang, res.Seri, diminta)
	if err != nil {
		return nil, nil, err
	}

	r := res.Ringkasan
	metrik := domain.MetrikModel{
		Mape: r.Mape, Rmse: r.Rmse, BaselineMape: r.BaselineMape, BaselineRmse: r.BaselineRmse, Optimistis: r.Optimistis,
		Horizon: slices.Clone(horizonLatih), RiwayatHari: riwayatLatihHari, NSeriOK: r.NSeriOK, NSeriDilewati: r.NSeriDilewati,
		VersiLayanan: res.VersiLayanan, Algoritma: r.Algoritma,
		Seri: map[string]domain.MetrikSeri{}, Dilewati: map[string]string{},
	}
	for _, s := range res.Seri {
		if s.Status == statusSeriOK {
			metrik.Seri[s.ID] = domain.MetrikSeri{Algoritma: s.Algoritma, Params: s.Params, GalatRMSE: s.GalatRMSE, Mape: s.Metrik.Mape}
		} else {
			metrik.Dilewati[s.ID] = s.Alasan
		}
	}
	rawMetrik, err := json.Marshal(metrik)
	if err != nil {
		return nil, nil, fmt.Errorf("encode metrics: %w", err)
	}
	rawImportansi, err := json.Marshal(importansiFaktor(res.Seri))
	if err != nil {
		return nil, nil, fmt.Errorf("encode importansi: %w", err)
	}

	algoritma := ringkasAlgoritma(r.Algoritma)
	artefak := "ml://garda-ml/" + res.VersiLayanan
	return &domain.ModelRegistry{
		Kode: kodeModel(kode, versi), KomoditasKode: kode, Nama: namaModelLatih, Versi: versi,
		Mape: round(r.Mape, 2), Status: domain.StatusKandidat, DilatihPada: sekarang.UTC(),
		Algorithm: &algoritma, Metrics: string(rawMetrik), FeatureImportance: string(rawImportansi),
		Sumber: domain.SumberModel, ArtefakURI: &artefak, Catatan: catatanLatih(r),
	}, proyeksi, nil
}

func ringkasAlgoritma(hitung map[string]int) string {
	nama := make([]string, 0, len(hitung))
	for n := range hitung {
		nama = append(nama, n)
	}
	sort.Strings(nama)
	bagian := make([]string, len(nama))
	for i, n := range nama {
		bagian[i] = fmt.Sprintf("%s×%d", n, hitung[n])
	}
	return strings.Join(bagian, ", ")
}

func catatanLatih(r mlclient.Ringkasan) string {
	teks := fmt.Sprintf("Dilatih otomatis dari entri harga konsumen tervalidasi (rata-rata harian per kabupaten, %d hari terakhir). "+
		"MAPE dan RMSE dari backtest walk-forward pada lipatan uji yang tidak ikut memilih model; baseline = harga terakhir dipertahankan. "+
		"Bukan jaminan akurasi ke depan.", riwayatLatihHari)
	if r.Optimistis {
		teks += " Sebagian seri terlalu pendek untuk dipisah, sehingga metriknya optimistis."
	}
	if r.NSeriDilewati > 0 {
		teks += fmt.Sprintf(" %d kabupaten dilewati karena riwayat tidak memadai.", r.NSeriDilewati)
	}
	return teks
}

// importansiFaktor merata-ratakan |kontribusi| tiap faktor lintas proyeksi, dinormalkan
// menjadi bobot relatif (jumlah 1); kosong bila semua kontribusi nol.
func importansiFaktor(seri []mlclient.HasilSeri) []domain.ImportansiFaktor {
	jumlah := map[string]float64{}
	var total float64
	for _, s := range seri {
		for _, r := range s.Ramalan {
			for _, f := range r.Faktor {
				jumlah[f.Nama] += math.Abs(f.Kontribusi)
				total += math.Abs(f.Kontribusi)
			}
		}
	}
	out := []domain.ImportansiFaktor{}
	if total == 0 {
		return out
	}
	for nama, v := range jumlah {
		out = append(out, domain.ImportansiFaktor{Nama: nama, Bobot: round(v/total, 3)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bobot != out[j].Bobot {
			return out[i].Bobot > out[j].Bobot
		}
		return out[i].Nama < out[j].Nama
	})
	return out
}

// proyeksiDariHasil mengubah jawaban ML menjadi baris prediksi_harga bersumber MODEL, tanpa ID dan kode: ID
// diberikan basis data dan baris di-upsert menurut kunci alaminya. modelID boleh 0 bila model belum disimpan.
// Seri yang bukan OK dilewati; id seri (kode kabupaten) di luar `diminta` atau interval yang tidak
// berurutan dianggap pelanggaran kontrak.
func proyeksiDariHasil(kode string, modelID idx.ID, versi string, sekarang time.Time, seri []mlclient.HasilSeri, diminta map[string]bool) ([]domain.PrediksiHarga, error) {
	var rows []domain.PrediksiHarga
	for _, s := range seri {
		if s.Status != statusSeriOK {
			continue
		}
		if !diminta[s.ID] {
			return nil, fmt.Errorf("%w: seri %q tidak diminta", ErrMLGagal, s.ID)
		}
		if s.HargaDasar <= 0 {
			return nil, fmt.Errorf("%w: harga dasar seri %q tidak valid", ErrMLGagal, s.ID)
		}
		for _, r := range s.Ramalan {
			target, err := time.Parse(time.DateOnly, r.TanggalTarget)
			if err != nil || r.HorizonHari <= 0 || r.Bawah > r.Nilai || r.Nilai > r.Atas || r.Bawah <= 0 {
				return nil, fmt.Errorf("%w: proyeksi seri %q tidak valid", ErrMLGagal, s.ID)
			}
			faktor := make([]domain.Faktor, len(r.Faktor))
			for i, f := range r.Faktor {
				faktor[i] = domain.Faktor{Nama: f.Nama, Kontribusi: f.Kontribusi, Detail: f.Detail}
			}
			raw, err := json.Marshal(faktor)
			if err != nil {
				return nil, fmt.Errorf("encode faktor: %w", err)
			}
			rows = append(rows, domain.PrediksiHarga{
				KomoditasKode: kode, KabupatenKode: s.ID, TanggalTarget: target, HorizonHari: r.HorizonHari,
				Nilai: r.Nilai, Bawah: r.Bawah, Atas: r.Atas, HargaDasar: s.HargaDasar, ModelID: modelID, ModelVersi: versi,
				Faktor: string(raw), Sumber: domain.SumberModel, DibuatPada: sekarang.UTC(),
			})
		}
	}
	return rows, nil
}

// Aksi massal registri model (nilai field JSON `action`).
const (
	AksiPromosi = "promosi"
	AksiArsip   = "arsip"
)

// GagalModel adalah satu model yang tidak diproses beserta alasannya.
type GagalModel struct {
	ID     string
	Alasan string
}

// BulkModelResult adalah hasil aksi massal registri.
type BulkModelResult struct {
	Berhasil int
	Gagal    []GagalModel
}

// AksiMassal mempromosikan atau mengarsipkan model menurut ID atau kode (RujukanModel). Model yang tidak ada,
// berstatus tidak sesuai, atau (untuk promosi) lebih buruk dari baseline dilewati dan dilaporkan dengan
// alasannya; GagalModel.ID memuat rujukan persis seperti dikirim klien. Setiap perubahan dicatat di riwayat
// model_promosi dan audit (dengan ID BIGINT model).
func (uc *MLOps) AksiMassal(ctx context.Context, aksi string, ids []string, aktorID string) (BulkModelResult, error) {
	if aksi != AksiPromosi && aksi != AksiArsip {
		return BulkModelResult{}, ErrAksiModelTidakValid
	}
	var hasil BulkModelResult
	for _, id := range ids {
		ref, sah := RujukanModel(id)
		var err error
		switch {
		case !sah:
			err = domain.ErrModelTidakDitemukan
		case aksi == AksiPromosi:
			err = uc.promosikan(ctx, ref, aktorID)
		default:
			err = uc.arsipkan(ctx, ref, aktorID)
		}
		alasan, dilewati := alasanDilewati(err)
		switch {
		case dilewati:
			hasil.Gagal = append(hasil.Gagal, GagalModel{ID: id, Alasan: alasan})
		case err != nil:
			return hasil, err
		default:
			hasil.Berhasil++
		}
	}
	return hasil, nil
}

// alasanDilewati menerjemahkan galat aturan bisnis menjadi alasan yang bisa ditampilkan;
// galat lain (basis data) bukan alasan melewati.
func alasanDilewati(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrModelTidakDitemukan):
		return "model tidak ditemukan", true
	case errors.Is(err, domain.ErrStatusModelTidakValid):
		return "status model tidak memungkinkan aksi ini", true
	case errors.Is(err, domain.ErrModelTanpaProyeksi):
		return "model belum punya proyeksi; latih atau segarkan dulu agar komoditas tidak kehilangan proyeksi", true
	case errors.Is(err, ErrLebihBurukDariBaseline):
		return err.Error(), true
	}
	return "", false
}

// cekBaseline menolak model hasil pelatihan yang MAPE backtest-nya lebih buruk dari baseline naif.
// Model demo tidak punya metrik pelatihan dan tidak diperiksa.
func cekBaseline(m *domain.ModelRegistry) error {
	if m.Sumber != domain.SumberModel {
		return nil
	}
	metrik, err := domain.BacaMetrik(m.Metrics)
	if err != nil {
		return err
	}
	if metrik.Mape > metrik.BaselineMape {
		return fmt.Errorf("%w: MAPE %s%% > baseline naif %s%%", ErrLebihBurukDariBaseline, angka2(metrik.Mape), angka2(metrik.BaselineMape))
	}
	return nil
}

func angka2(v float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1)
}

func (uc *MLOps) promosikan(ctx context.Context, ref idx.RujukanID, aktorID string) error {
	m, err := uc.repo.GetModel(ctx, ref)
	if err != nil {
		return err
	}
	if m.Status == domain.StatusProduksi {
		return domain.ErrStatusModelTidakValid
	}
	if err := cekBaseline(m); err != nil {
		return err
	}
	hasil, err := uc.repo.Promosikan(ctx, idx.RujukanID{ID: m.ID}, aktorID, uc.now())
	if err != nil {
		return err
	}
	sesudah := map[string]any{"status": domain.StatusProduksi, "versi": hasil.Model.Versi}
	if hasil.Digantikan != nil {
		sesudah["menggantikan"] = hasil.Digantikan.ID.String()
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "PROMOSI_MODEL", Entitas: "model_registry", EntitasID: hasil.Model.ID.String(), Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"status": hasil.DariStatus}, Sesudah: sesudah,
		Metadata: map[string]any{"komoditas": hasil.Model.KomoditasKode, "kode": hasil.Model.Kode, "rollback": hasil.DariStatus == domain.StatusArsip},
	})
	return nil
}

func (uc *MLOps) arsipkan(ctx context.Context, ref idx.RujukanID, aktorID string) error {
	m, err := uc.repo.Arsipkan(ctx, ref, aktorID, uc.now())
	if err != nil {
		return err
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "ARSIP_MODEL", Entitas: "model_registry", EntitasID: m.ID.String(), Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"status": domain.StatusKandidat}, Sesudah: map[string]any{"status": m.Status, "versi": m.Versi},
		Metadata: map[string]any{"komoditas": m.KomoditasKode, "kode": m.Kode},
	})
	return nil
}
