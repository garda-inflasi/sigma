package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/internal/policy-brief/generator"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPeriodeTidakValid = errors.New("periode harus berformat YYYY-MM")
	ErrPeriodeSudahAda   = errors.New("brief untuk periode ini sudah ada")
	ErrAksiTidakValid    = errors.New("aksi tidak valid")
	ErrBriefTidakAda     = errors.New("brief tidak ditemukan")
)

const (
	// batasProses adalah lama maksimum satu penyusunan; setelahnya draf dianggap gagal.
	batasProses = 3 * time.Minute
	// basiProses: brief yang masih DIPROSES lebih lama dari ini dianggap prosesnya mati
	// (mis. server berhenti) dan boleh disusun ulang.
	basiProses = 5 * time.Minute

	halamanKerangka = 1
	kataPerHalaman  = 300
)

// UseCase menyusun, membaca, dan memajukan status policy brief.
type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
	// llm menyusun narasi bila kunci API dikonfigurasi; nil berarti hanya templat.
	// templat adalah cadangan: dipakai bila llm nil, gagal, atau melanggar aturan anti-halusinasi.
	llm     domain.Generator
	templat domain.Generator

	now func() time.Time
	// jalankan memulai penyusunan di latar belakang; tes menggantinya agar sinkron.
	jalankan func(func())
}

// New membuat UseCase. llm boleh nil (tanpa LLM, brief disusun dari templat deterministik).
func New(repo domain.Repository, llm domain.Generator, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{
		repo: repo, audit: auditWriter, log: log, llm: llm, templat: generator.Templat{},
		now: time.Now, jalankan: func(f func()) { go f() },
	}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.BriefView, error) {
	return uc.repo.List(ctx)
}

// Detail mengembalikan satu brief lengkap dengan isi, label draf, dan jejak penyusunannya.
func (uc *UseCase) Detail(ctx context.Context, id string) (*domain.DetailView, error) {
	d, err := uc.repo.GetDetail(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBriefTidakAda
	}
	return d, err
}

// Generate memulai penyusunan draf brief untuk periode (YYYY-MM) atas nama penyusun dan
// segera mengembalikan brief berstatus DIPROSES; draf disusun di latar belakang dari data
// nyata (inflasi, harga, peringatan dini, proyeksi) dan klien memantau lewat Detail sampai
// status menjadi DRAF (atau GAGAL). Brief GAGAL, atau DIPROSES yang prosesnya mati, dapat diulang.
func (uc *UseCase) Generate(ctx context.Context, penyusunID, periode string) (*domain.BriefView, error) {
	tanggal, err := time.Parse(domain.FormatPeriode, periode)
	if err != nil {
		return nil, ErrPeriodeTidakValid
	}
	label := domain.LabelPeriode(tanggal)
	sekarang := uc.now()

	pb := &domain.PolicyBrief{
		ID:            domain.BriefID(tanggal),
		Periode:       label,
		Judul:         fmt.Sprintf("Policy brief %s (sedang disusun)", label),
		Status:        domain.StatusDiproses,
		Ringkasan:     "Draf sedang disusun dari data terbaru.",
		Rekomendasi:   "[]",
		Halaman:       halamanKerangka,
		DisusunOlehID: penyusunID,
	}
	dibuat, err := uc.repo.Create(ctx, pb)
	if err != nil {
		return nil, err
	}
	if !dibuat {
		diulang, err := uc.repo.MulaiUlang(ctx, pb.ID, sekarang.Add(-basiProses))
		if err != nil {
			return nil, err
		}
		if !diulang {
			return nil, ErrPeriodeSudahAda
		}
	}

	job := &domain.BriefJob{ID: uuid.NewString(), BriefID: pb.ID, Status: domain.JobBerjalan, Mulai: sekarang.UTC()}
	if err := uc.repo.MulaiJob(ctx, job); err != nil {
		return nil, err
	}
	uc.catat(ctx, audit.Entry{
		AktorID: penyusunID, Aksi: "GENERATE_BRIEF", Entitas: "policy_brief", EntitasID: pb.ID, Hasil: audit.HasilBerhasil,
		Sesudah:  map[string]any{"status": domain.StatusDiproses, "periode": label},
		Metadata: map[string]any{"job": job.ID, "penyusun": uc.namaPenyusun()},
	})

	// Jawaban selalu keadaan saat dimulai (DIPROSES), walau penyusunan yang cepat sudah selesai.
	view, err := uc.repo.Get(ctx, pb.ID)
	if err != nil {
		return nil, err
	}
	// Proses berjalan lepas dari permintaan (yang segera selesai) tetapi mewarisi asal permintaan untuk audit.
	lepas := context.WithoutCancel(ctx)
	uc.jalankan(func() { uc.proses(lepas, penyusunID, job, tanggal) })
	return view, nil
}

func (uc *UseCase) namaPenyusun() string {
	if uc.llm != nil {
		return uc.llm.Nama()
	}
	return uc.templat.Nama()
}

// proses menyusun draf dan menyimpannya; kegagalan apa pun menandai brief GAGAL.
func (uc *UseCase) proses(ctx context.Context, penyusunID string, job *domain.BriefJob, tanggal time.Time) {
	ctx, batal := context.WithTimeout(ctx, batasProses)
	defer batal()

	draf, err := uc.susun(ctx, tanggal)
	if err != nil {
		uc.gagal(ctx, penyusunID, job, err)
		return
	}
	draf.BriefID, draf.JobID, draf.OlehID, draf.Waktu = job.BriefID, job.ID, penyusunID, uc.now().UTC()

	tersimpan, err := uc.repo.SelesaikanDraf(ctx, *draf)
	if err != nil {
		uc.gagal(ctx, penyusunID, job, err)
		return
	}
	if !tersimpan {
		uc.gagal(ctx, penyusunID, job, errors.New("status brief berubah selama penyusunan"))
		return
	}
	meta := map[string]any{"job": job.ID, "penyusun": draf.Penyusun}
	if draf.ModelLLM != nil {
		meta["model_llm"], meta["prompt_versi"], meta["prompt_hash"] = *draf.ModelLLM, *draf.PromptVersi, *draf.PromptHash
	}
	if draf.Catatan != nil {
		meta["catatan"] = *draf.Catatan
	}
	uc.catat(ctx, audit.Entry{
		AktorID: penyusunID, Aksi: "BRIEF_DRAF_SELESAI", Entitas: "policy_brief", EntitasID: job.BriefID, Hasil: audit.HasilBerhasil,
		Sesudah: map[string]any{"status": domain.StatusDraf}, Metadata: meta,
	})
}

func (uc *UseCase) gagal(ctx context.Context, penyusunID string, job *domain.BriefJob, sebab error) {
	uc.log.Error("policy-brief: penyusunan draf gagal", map[string]interface{}{"id": job.BriefID, "job": job.ID, "error": sebab.Error()})
	// Konteks proses mungkin sudah habis waktu; pencatatan kegagalan tetap harus jalan.
	catatan, batal := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer batal()
	if err := uc.repo.GagalkanDraf(catatan, job.BriefID, job.ID, sebab.Error(), uc.now().UTC()); err != nil {
		uc.log.Error("policy-brief: gagal menandai brief GAGAL", map[string]interface{}{"id": job.BriefID, "error": err.Error()})
	}
	uc.catat(catatan, audit.Entry{
		AktorID: penyusunID, Aksi: "BRIEF_DRAF_GAGAL", Entitas: "policy_brief", EntitasID: job.BriefID, Hasil: audit.HasilGagal,
		Metadata: map[string]any{"job": job.ID, "galat": sebab.Error()},
	})
}

// susun mengumpulkan data, membangun konteks, lalu menyusun dan memverifikasi narasi. LLM yang gagal
// atau melanggar aturan anti-halusinasi digantikan templat, dengan alasannya dicatat di brief.
func (uc *UseCase) susun(ctx context.Context, tanggal time.Time) (*domain.HasilDraf, error) {
	data, err := uc.repo.Kumpulkan(ctx, tanggal)
	if err != nil {
		return nil, fmt.Errorf("kumpulkan data: %w", err)
	}
	konteks := BangunKonteks(*data)

	var (
		narasi  *domain.Narasi
		konten  *domain.Konten
		catatan string
	)
	if uc.llm != nil {
		narasi, konten, err = uc.susunDengan(ctx, uc.llm, konteks, "")
		if err != nil {
			// Catatan yang tersimpan tidak memuat teks narasi yang ditolak (bisa berisi angka tak bersumber);
			// rinciannya hanya masuk log.
			catatan = fmt.Sprintf("Penyusunan oleh %s gagal (%s); draf memakai templat.", uc.llm.Nama(), alasanPenolakan(err))
			uc.log.Warn("policy-brief: LLM gagal, memakai templat", map[string]interface{}{"error": err.Error()})
		}
	}
	if narasi == nil {
		narasi, konten, err = uc.susunDengan(ctx, uc.templat, konteks, catatan)
		if err != nil {
			return nil, fmt.Errorf("templat: %w", err)
		}
	}

	rawKonten, err := json.Marshal(konten)
	if err != nil {
		return nil, fmt.Errorf("encode konten: %w", err)
	}
	rawSnapshot, err := json.Marshal(konteks)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	rekomendasi := append(append([]string{}, konten.Butir(domain.BagianRekomendasiPemda)...), konten.Butir(domain.BagianRekomendasiTPID)...)
	rawRekomendasi, err := json.Marshal(rekomendasi)
	if err != nil {
		return nil, fmt.Errorf("encode rekomendasi: %w", err)
	}

	draf := &domain.HasilDraf{
		Judul:       "Policy brief " + domain.LabelPeriode(tanggal),
		Ringkasan:   strings.Join(konten.Butir(domain.BagianRingkasan), " "),
		Rekomendasi: string(rawRekomendasi),
		Halaman:     hitungHalaman(konten),
		Konten:      string(rawKonten),
		Snapshot:    string(rawSnapshot),
		Penyusun:    narasi.Meta.Penyusun,
	}
	if catatan != "" {
		draf.Catatan = &catatan
	}
	if narasi.Meta.Penyusun == domain.PenyusunLLM {
		model, versi, hash := narasi.Meta.Model, narasi.Meta.PromptVersi, narasi.Meta.PromptHash
		draf.ModelLLM, draf.PromptVersi, draf.PromptHash = &model, &versi, &hash
	}
	return draf, nil
}

// alasanPenolakan meringkas mengapa narasi LLM tidak dipakai, tanpa mengutip isinya.
func alasanPenolakan(err error) string {
	if errors.Is(err, ErrNarasiTidakValid) {
		return "narasi melanggar aturan anti-halusinasi"
	}
	return "pemanggilan LLM gagal"
}

// susunDengan menyusun narasi dengan satu Generator lalu memverifikasinya lewat Render.
func (uc *UseCase) susunDengan(ctx context.Context, gen domain.Generator, k domain.Konteks, catatan string) (*domain.Narasi, *domain.Konten, error) {
	narasi, err := gen.Susun(ctx, k)
	if err != nil {
		return nil, nil, err
	}
	konten, err := Render(narasi, k, catatan)
	if err != nil {
		return nil, nil, err
	}
	return narasi, konten, nil
}

// hitungHalaman memperkirakan panjang dokumen dari jumlah kata (kataPerHalaman), minimal satu halaman.
func hitungHalaman(k *domain.Konten) int {
	kata := 0
	for _, b := range k.Bagian {
		for _, butir := range b.Butir {
			kata += len(strings.Fields(butir))
		}
	}
	return max(halamanKerangka, (kata+kataPerHalaman-1)/kataPerHalaman)
}

func (uc *UseCase) catat(ctx context.Context, e audit.Entry) {
	if err := uc.audit.Write(ctx, e); err != nil {
		uc.log.Warn("policy-brief: gagal mencatat audit", map[string]interface{}{"id": e.EntitasID, "aksi": e.Aksi, "error": err.Error()})
	}
}

// BulkResult adalah hasil aksi massal: yang berpindah status dan yang dilewati.
type BulkResult struct {
	Berhasil int
	Gagal    int
}

// transisiAksi memetakan aksi ke status asal dan tujuannya.
var transisiAksi = map[string][2]string{
	domain.AksiTelaah:  {domain.StatusDraf, domain.StatusTelaah},
	domain.AksiSetujui: {domain.StatusTelaah, domain.StatusDisetujui},
}

// BulkAction memajukan status brief. Brief yang tidak ada atau bukan di status asal
// dihitung gagal; setiap perubahan dicatat di audit.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, aksi, aktorID string) (BulkResult, error) {
	rute, ok := transisiAksi[aksi]
	if !ok {
		return BulkResult{}, ErrAksiTidakValid
	}

	var hasil BulkResult
	for _, id := range ids {
		berubah, err := uc.repo.Transisi(ctx, id, rute[0], rute[1])
		if err != nil {
			return hasil, err
		}
		if !berubah {
			hasil.Gagal++
			continue
		}
		hasil.Berhasil++
		uc.catat(ctx, audit.Entry{
			AktorID: aktorID, Aksi: "BRIEF_" + strings.ToUpper(aksi), Entitas: "policy_brief", EntitasID: id,
			Hasil:   audit.HasilBerhasil,
			Sebelum: map[string]any{"status": rute[0]},
			Sesudah: map[string]any{"status": rute[1]},
		})
	}
	return hasil, nil
}
