package engine

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo adalah EngineRepository dan Sesi dalam memori: satu "transaksi" hanyalah pemanggilan fn.
type fakeRepo struct {
	mu         sync.Mutex
	wilayahAda bool
	sekarang   time.Time
	aturan     []domain.AturanSiklus
	nilai      map[string]float64 // nilai ukur per kode aturan (Terukur true)
	errUkur    error
	aktif      *domain.SiklusEkonomi
	bentrok    int // jumlah pemanggilan Ganti berikutnya yang gagal dengan ErrBentrokSiklus
	nextID     idx.ID

	diganti  int
	pulih    []*time.Time
	dalam    int
	riwayat  []domain.SiklusEkonomi // baris yang pernah dibuka (urutan)
	gantiLog []string
}

func baruFakeRepo() *fakeRepo {
	return &fakeRepo{
		wilayahAda: true, sekarang: t0, nextID: 100, nilai: map[string]float64{},
		aturan: []domain.AturanSiklus{lonjakan, kelangkaan, cuaca, overSupply, utilisasi},
	}
}

func (f *fakeRepo) Dalam(_ context.Context, _ string, fn func(domain.Sesi) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dalam++
	return fn(f)
}

func (f *fakeRepo) WilayahAda(context.Context) (bool, error) { return f.wilayahAda, nil }
func (f *fakeRepo) Sekarang(context.Context) (time.Time, error) {
	return f.sekarang, nil
}
func (f *fakeRepo) AturanAktif(context.Context) ([]domain.AturanSiklus, error) {
	var out []domain.AturanSiklus
	for _, a := range f.aturan {
		if a.Aktif {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeRepo) Ukur(_ context.Context, a domain.AturanSiklus, _ time.Time) (domain.Pengukuran, error) {
	if f.errUkur != nil {
		return domain.Pengukuran{}, f.errUkur
	}
	return domain.Pengukuran{Terukur: true, Nilai: f.nilai[a.Kode]}, nil
}
func (f *fakeRepo) SiklusAktif(context.Context) (*domain.SiklusEkonomi, error) {
	if f.aktif == nil {
		return nil, nil
	}
	salinan := *f.aktif
	return &salinan, nil
}
func (f *fakeRepo) CatatPulih(_ context.Context, id idx.ID, sejak *time.Time) error {
	if f.aktif == nil || f.aktif.ID != id {
		return errors.New("CatatPulih pada siklus yang bukan aktif")
	}
	f.aktif.PulihSejak = sejak
	f.pulih = append(f.pulih, sejak)
	return nil
}
func (f *fakeRepo) Ganti(_ context.Context, lama *domain.SiklusEkonomi, baru domain.SiklusBaru, sekarang time.Time) (*domain.SiklusEkonomi, error) {
	f.diganti++
	if f.bentrok > 0 {
		f.bentrok--
		return nil, domain.ErrBentrokSiklus
	}
	if lama != nil {
		if f.aktif == nil || f.aktif.ID != lama.ID {
			return nil, domain.ErrBentrokSiklus
		}
		akhir := sekarang
		f.aktif.AkhirPada, f.aktif.PulihSejak = &akhir, nil
	}
	f.nextID++
	alasan, _ := json.Marshal(baru.Alasan)
	teks := string(alasan)
	f.aktif = &domain.SiklusEkonomi{ID: f.nextID, WilayahKode: baru.WilayahKode, Siklus: baru.Siklus, MulaiPada: sekarang, Otomatis: true, Alasan: &teks}
	f.riwayat = append(f.riwayat, *f.aktif)
	f.gantiLog = append(f.gantiLog, baru.Siklus)
	salinan := *f.aktif
	return &salinan, nil
}

func baruEngine(repo *fakeRepo) (*Engine, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return New(repo, rec, logger.New(logger.ERROR, nil)), rec
}

var opsiJadwal = domain.OpsiEvaluasi{Pemicu: domain.PemicuJadwal}

func TestEvaluasi_TanpaAturanAktifDilewatiTanpaMengubahApaPun(t *testing.T) {
	repo := baruFakeRepo()
	repo.aturan = nil
	repo.aktif = aktif(domain.SiklusTekanan, t0.Add(-time.Hour), nil)
	repo.aktif.ID = 7
	eng, rec := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.NoError(t, err)
	assert.True(t, hasil.Dilewati)
	assert.Equal(t, domain.SebabTanpaAturan, hasil.SebabDilewati)
	assert.False(t, hasil.Berubah)
	assert.Equal(t, domain.SiklusTekanan, hasil.Aktif.Siklus, "siklus yang ada dilaporkan apa adanya")
	assert.Zero(t, repo.diganti)
	assert.Empty(t, repo.pulih)
	assert.Empty(t, rec.Entries())
}

func TestEvaluasi_SemuaAturanNonaktifSamaDenganTanpaAturan(t *testing.T) {
	repo := baruFakeRepo()
	for i := range repo.aturan {
		repo.aturan[i].Aktif = false
	}
	repo.nilai["LONJAKAN"] = 9
	eng, _ := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.NoError(t, err)
	assert.True(t, hasil.Dilewati)
	assert.Nil(t, repo.aktif, "tanpa aturan aktif mesin tidak berhak membuat siklus, bahkan NORMAL")
}

func TestEvaluasi_WilayahTakDikenalDilewati(t *testing.T) {
	repo := baruFakeRepo()
	repo.wilayahAda = false
	eng, rec := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "99", opsiJadwal)

	require.NoError(t, err, "basis data yang baru dimigrasi tetapi belum di-seed bukan galat")
	assert.True(t, hasil.Dilewati)
	assert.Equal(t, domain.SebabWilayahTakDikenal, hasil.SebabDilewati)
	assert.Nil(t, repo.aktif)
	assert.Empty(t, rec.Entries())
}

func TestEvaluasi_BasisDataKosongMembukaSiklusAwalDanMengauditnya(t *testing.T) {
	repo := baruFakeRepo()
	eng, rec := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "11", domain.OpsiEvaluasi{AktorID: "u-1", Pemicu: domain.PemicuManual})

	require.NoError(t, err)
	assert.True(t, hasil.Berubah)
	assert.Empty(t, hasil.Sebelumnya)
	require.NotNil(t, hasil.Aktif)
	assert.Equal(t, domain.SiklusNormal, hasil.Aktif.Siklus)
	assert.True(t, hasil.Aktif.Otomatis)
	require.NotNil(t, hasil.Aktif.Alasan)
	assert.Contains(t, *hasil.Aktif.Alasan, "Siklus awal NORMAL")
	assert.Len(t, hasil.Aturan, 5, "hasil tiap aturan aktif ikut dilaporkan")

	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, AksiAudit, entry.Aksi)
	assert.Equal(t, "u-1", entry.AktorID, "pergantian karena evaluasi manual dicatat atas nama pemintanya")
	assert.Equal(t, hasil.Aktif.ID.String(), entry.EntitasID)
	assert.Nil(t, entry.Sebelum, "tidak ada siklus sebelumnya")
	assert.Equal(t, domain.PemicuManual, entry.Metadata["pemicu"])
	assert.Equal(t, Sumber, entry.Metadata["sumber"])
}

func TestEvaluasi_PergantianMencatatSebelumSesudahDanAlasan(t *testing.T) {
	repo := baruFakeRepo()
	repo.aktif = aktif(domain.SiklusNormal, t0.Add(-5*time.Hour), nil)
	repo.nilai["LONJAKAN"] = 2
	repo.nilai["UTILISASI"] = 95
	eng, rec := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.NoError(t, err)
	require.True(t, hasil.Berubah)
	assert.Equal(t, domain.SiklusNormal, hasil.Sebelumnya)
	assert.Equal(t, domain.SiklusTekanan, hasil.Aktif.Siklus)
	require.NotNil(t, repo.aktif)
	assert.Equal(t, domain.SiklusTekanan, repo.aktif.Siklus)

	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Empty(t, entry.AktorID, "evaluasi terjadwal dilakukan sistem")
	assert.Equal(t, domain.PemicuJadwal, entry.Metadata["pemicu"])
	sebelum, ok := entry.Sebelum.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, domain.SiklusNormal, sebelum["siklus"])
	assert.Equal(t, repo.aktif.MulaiPada, sebelum["akhir_pada"], "siklus lama ditutup pada instan siklus baru dibuka")
	sesudah, ok := entry.Sesudah.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, domain.SiklusTekanan, sesudah["siklus"])
	assert.Equal(t, true, sesudah["otomatis"])
	assert.NotNil(t, sesudah["alasan"], "alasan keputusan ikut di jejak audit")

	// Audit dapat di-marshal (dipakai penulis audit sungguhan) dan memuat alasan sebagai objek.
	raw, err := json.Marshal(sesudah)
	require.NoError(t, err)
	var dekode struct {
		Alasan domain.Alasan `json:"alasan"`
	}
	require.NoError(t, json.Unmarshal(raw, &dekode))
	assert.Equal(t, []string{"LONJAKAN"}, kodePemicu(dekode.Alasan.Pemicu))
	assert.Equal(t, []string{"UTILISASI"}, kodePemicu(dekode.Alasan.Ditekan))
}

// Idempoten: keadaan yang sama tidak membuat baris, tidak menutup apa pun, dan tidak menambah audit.
func TestEvaluasi_SiklusSamaTidakMengubahApaPun(t *testing.T) {
	repo := baruFakeRepo()
	repo.aktif = aktif(domain.SiklusTekanan, t0.Add(-30*time.Hour), nil)
	repo.nilai["LONJAKAN"] = 3
	eng, rec := baruEngine(repo)

	for range 3 {
		hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)
		require.NoError(t, err)
		assert.False(t, hasil.Berubah)
		assert.Equal(t, domain.SiklusTekanan, hasil.Aktif.Siklus)
	}

	assert.Zero(t, repo.diganti)
	assert.Empty(t, repo.pulih, "masih dipicu: tidak ada penanda pulih yang ditulis")
	assert.Empty(t, rec.Entries())
	assert.True(t, repo.aktif.MulaiPada.Equal(t0.Add(-30*time.Hour)))
}

// Penanda masa pulih ditulis sekali saat pemicu hilang, tidak ditulis ulang tiap evaluasi, dan dihapus bila pemicu kembali.
func TestEvaluasi_PenandaPulihDitulisSekaliDanDihapusBilaPemicuKembali(t *testing.T) {
	repo := baruFakeRepo()
	repo.aktif = aktif(domain.SiklusTekanan, t0.Add(-30*time.Hour), nil)
	eng, rec := baruEngine(repo)

	_, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)
	require.NoError(t, err)
	require.Len(t, repo.pulih, 1)
	require.NotNil(t, repo.pulih[0])
	assert.True(t, repo.pulih[0].Equal(t0), "masa pulih dimulai pada jam basis data saat pemicu pertama kali hilang")

	repo.sekarang = t0.Add(10 * time.Minute)
	_, err = eng.Evaluasi(t.Context(), "11", opsiJadwal)
	require.NoError(t, err)
	assert.Len(t, repo.pulih, 1, "penanda yang sama tidak ditulis ulang")
	assert.True(t, repo.aktif.PulihSejak.Equal(t0), "penanda tidak bergeser")

	repo.nilai["CUACA"] = 1
	repo.sekarang = t0.Add(20 * time.Minute)
	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)
	require.NoError(t, err)
	assert.False(t, hasil.Berubah)
	require.Len(t, repo.pulih, 2)
	assert.Nil(t, repo.pulih[1], "pemicu kembali: penanda dihapus")
	assert.Nil(t, repo.aktif.PulihSejak)
	assert.Zero(t, repo.diganti)
	assert.Empty(t, rec.Entries(), "bookkeeping masa pulih bukan perubahan siklus: tidak diaudit")
}

func TestEvaluasi_PulihPenuhMenutupTekananDanMembukaNormal(t *testing.T) {
	repo := baruFakeRepo()
	repo.aktif = aktif(domain.SiklusTekanan, t0.Add(-100*time.Hour), waktu(t0.Add(-49*time.Hour)))
	eng, rec := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.NoError(t, err)
	require.True(t, hasil.Berubah)
	assert.Equal(t, domain.SiklusTekanan, hasil.Sebelumnya)
	assert.Equal(t, domain.SiklusNormal, hasil.Aktif.Siklus)
	assert.Contains(t, hasil.Ringkasan, "Kembali ke NORMAL")
	assert.Len(t, rec.Entries(), 1)
	assert.Equal(t, domain.SiklusNormal, repo.aktif.Siklus)
}

// "Tidak tahu" tidak boleh dibaca "tidak ada pemicu": satu aturan yang gagal diukur membatalkan seluruh putaran.
func TestEvaluasi_GalatUkurMembatalkanPutaranTanpaMengubahApaPun(t *testing.T) {
	repo := baruFakeRepo()
	repo.aktif = aktif(domain.SiklusTekanan, t0.Add(-100*time.Hour), waktu(t0.Add(-49*time.Hour)))
	repo.errUkur = errors.New("db putus")
	eng, rec := baruEngine(repo)

	_, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.Error(t, err)
	assert.ErrorContains(t, err, "db putus")
	assert.Zero(t, repo.diganti, "TEKANAN tidak boleh dilepas hanya karena pengukuran gagal")
	assert.Empty(t, repo.pulih)
	assert.Empty(t, rec.Entries())
	assert.Equal(t, domain.SiklusTekanan, repo.aktif.Siklus)
}

func TestEvaluasi_BentrokDenganPenulisLainDiulangSekali(t *testing.T) {
	repo := baruFakeRepo()
	repo.aktif = aktif(domain.SiklusNormal, t0.Add(-5*time.Hour), nil)
	repo.nilai["LONJAKAN"] = 1
	repo.bentrok = 1
	eng, rec := baruEngine(repo)

	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.NoError(t, err)
	assert.True(t, hasil.Berubah)
	assert.Equal(t, 2, repo.dalam, "satu ulangan setelah bentrok")
	assert.Len(t, rec.Entries(), 1, "hanya pergantian yang berhasil yang diaudit")

	repo2 := baruFakeRepo()
	repo2.aktif = aktif(domain.SiklusNormal, t0.Add(-5*time.Hour), nil)
	repo2.nilai["LONJAKAN"] = 1
	repo2.bentrok = 5
	eng2, rec2 := baruEngine(repo2)
	_, err = eng2.Evaluasi(t.Context(), "11", opsiJadwal)
	require.ErrorIs(t, err, domain.ErrBentrokSiklus, "bentrok terus-menerus dilaporkan, tidak berulang tanpa batas")
	assert.Equal(t, 2, repo2.dalam)
	assert.Empty(t, rec2.Entries())
}

func TestEvaluasi_KegagalanAuditHanyaDilogDanPergantianTetapBerlaku(t *testing.T) {
	repo := baruFakeRepo()
	repo.nilai["LONJAKAN"] = 1
	eng, rec := baruEngine(repo)
	rec.Err = errors.New("audit mati")

	hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)

	require.NoError(t, err)
	assert.True(t, hasil.Berubah)
	assert.Equal(t, domain.SiklusTekanan, repo.aktif.Siklus)
}

func TestEvaluasi_PemicuKosongDianggapJadwal(t *testing.T) {
	repo := baruFakeRepo()
	eng, rec := baruEngine(repo)

	_, err := eng.Evaluasi(t.Context(), "11", domain.OpsiEvaluasi{})

	require.NoError(t, err)
	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, domain.PemicuJadwal, entry.Metadata["pemicu"])
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
}

// Skenario penuh di atas fake: TEKANAN dari alert, pemulihan bertahap, lalu SURPLUS langsung dari TEKANAN.
func TestEvaluasi_SkenarioTekananPulihLaluSurplus(t *testing.T) {
	repo := baruFakeRepo()
	eng, rec := baruEngine(repo)
	jalan := func(maju time.Duration) domain.HasilEvaluasi {
		repo.sekarang = repo.sekarang.Add(maju)
		hasil, err := eng.Evaluasi(t.Context(), "11", opsiJadwal)
		require.NoError(t, err)
		return hasil
	}

	jalan(0) // NORMAL awal
	repo.nilai["LONJAKAN"] = 2
	assert.Equal(t, domain.SiklusTekanan, jalan(time.Hour).Aktif.Siklus)
	repo.nilai["LONJAKAN"] = 0
	repo.nilai["UTILISASI"] = 95 // surplus muncul selagi tekanan mereda
	assert.Equal(t, domain.SiklusTekanan, jalan(time.Hour).Aktif.Siklus, "SURPLUS menunggu masa pulih TEKANAN")
	assert.Equal(t, domain.SiklusTekanan, jalan(47*time.Hour).Aktif.Siklus)
	assert.Equal(t, domain.SiklusSurplus, jalan(2*time.Hour).Aktif.Siklus, "masa pulih habis: langsung SURPLUS")
	repo.nilai["LONJAKAN"] = 1
	assert.Equal(t, domain.SiklusTekanan, jalan(time.Hour).Aktif.Siklus, "TEKANAN merebut kembali segera")

	assert.Equal(t, []string{domain.SiklusNormal, domain.SiklusTekanan, domain.SiklusSurplus, domain.SiklusTekanan}, repo.gantiLog)
	assert.Len(t, rec.Entries(), 4)
}

func TestRun_IntervalNolMatiDanIntervalPositifBerjalanSampaiBerhenti(t *testing.T) {
	repo := baruFakeRepo()
	eng, _ := baruEngine(repo)

	eng.Run(t.Context(), "11", 0) // kembali segera
	assert.Zero(t, repo.dalam)

	ctx, cancel := context.WithCancel(t.Context())
	selesai := make(chan struct{})
	go func() { eng.Run(ctx, "11", 10*time.Millisecond); close(selesai) }()
	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.dalam >= 3
	}, 5*time.Second, 5*time.Millisecond, "dievaluasi segera lalu tiap interval")
	cancel()
	select {
	case <-selesai:
	case <-time.After(5 * time.Second):
		t.Fatal("Run tidak berhenti setelah ctx dibatalkan")
	}
}
