package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var waktuUji = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)

// fakeRepo menyimpan brief, job, dan versi di memori; id yang sudah ada tidak ditimpa.
type fakeRepo struct {
	rows       map[string]domain.PolicyBrief
	jobs       map[string]domain.BriefJob
	versi      []domain.HasilDraf
	createErr  error
	kumpulErr  error
	data       domain.DataPeriode
	diumpulkan []time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: map[string]domain.PolicyBrief{}, jobs: map[string]domain.BriefJob{}, data: dataLengkap()}
}

func (r *fakeRepo) List(context.Context) ([]domain.BriefView, error) { return nil, nil }

func (r *fakeRepo) Get(_ context.Context, id string) (*domain.BriefView, error) {
	pb, ada := r.rows[id]
	if !ada {
		return nil, gorm.ErrRecordNotFound
	}
	return &domain.BriefView{ID: pb.ID, Periode: pb.Periode, Judul: pb.Judul, Status: pb.Status, Ringkasan: pb.Ringkasan, DisusunOleh: "Penyusun", SumberNarasi: pb.SumberNarasi}, nil
}

func (r *fakeRepo) GetDetail(_ context.Context, id string) (*domain.DetailView, error) {
	pb, ada := r.rows[id]
	if !ada {
		return nil, gorm.ErrRecordNotFound
	}
	v, _ := r.Get(context.Background(), id)
	return &domain.DetailView{BriefView: *v, Konten: pb.Konten, ModelLLM: pb.ModelLLM, PromptVersi: pb.PromptVersi, PromptHash: pb.PromptHash, Galat: pb.Galat}, nil
}

func (r *fakeRepo) Transisi(_ context.Context, id, dari, ke string) (bool, error) {
	pb, ada := r.rows[id]
	if !ada || pb.Status != dari {
		return false, nil
	}
	pb.Status = ke
	r.rows[id] = pb
	return true, nil
}

func (r *fakeRepo) Create(_ context.Context, pb *domain.PolicyBrief) (bool, error) {
	if r.createErr != nil {
		return false, r.createErr
	}
	if _, ada := r.rows[pb.ID]; ada {
		return false, nil
	}
	r.rows[pb.ID] = *pb
	return true, nil
}

func (r *fakeRepo) MulaiUlang(_ context.Context, id string, basiSebelum time.Time) (bool, error) {
	pb, ada := r.rows[id]
	if !ada || !(pb.Status == domain.StatusGagal || (pb.Status == domain.StatusDiproses && pb.Diperbarui.Before(basiSebelum))) {
		return false, nil
	}
	pb.Status, pb.Galat = domain.StatusDiproses, nil
	r.rows[id] = pb
	return true, nil
}

func (r *fakeRepo) MulaiJob(_ context.Context, job *domain.BriefJob) error {
	r.jobs[job.ID] = *job
	return nil
}

func (r *fakeRepo) SelesaikanDraf(_ context.Context, d domain.HasilDraf) (bool, error) {
	pb := r.rows[d.BriefID]
	if pb.Status != domain.StatusDiproses {
		return false, nil
	}
	pb.Status, pb.Judul, pb.Ringkasan, pb.Rekomendasi, pb.Halaman = domain.StatusDraf, d.Judul, d.Ringkasan, d.Rekomendasi, d.Halaman
	pb.Konten, pb.SnapshotData, pb.SumberNarasi = d.Konten, d.Snapshot, d.Penyusun
	pb.ModelLLM, pb.PromptVersi, pb.PromptHash, pb.Galat = d.ModelLLM, d.PromptVersi, d.PromptHash, d.Catatan
	r.rows[d.BriefID] = pb
	r.versi = append(r.versi, d)
	job := r.jobs[d.JobID]
	job.Status = domain.JobSelesai
	r.jobs[d.JobID] = job
	return true, nil
}

func (r *fakeRepo) GagalkanDraf(_ context.Context, briefID, jobID, galat string, _ time.Time) error {
	pb := r.rows[briefID]
	if pb.Status == domain.StatusDiproses {
		pb.Status, pb.Galat = domain.StatusGagal, &galat
		r.rows[briefID] = pb
	}
	job := r.jobs[jobID]
	job.Status, job.Galat = domain.JobGagal, &galat
	r.jobs[jobID] = job
	return nil
}

func (r *fakeRepo) Kumpulkan(_ context.Context, periode time.Time) (*domain.DataPeriode, error) {
	r.diumpulkan = append(r.diumpulkan, periode)
	if r.kumpulErr != nil {
		return nil, r.kumpulErr
	}
	d := r.data
	d.Periode = periode
	return &d, nil
}

// fakeGen adalah Generator palsu yang mengembalikan narasi atau galat tetap.
type fakeGen struct {
	narasi    *domain.Narasi
	err       error
	dipanggil int
	konteks   domain.Konteks
}

func (g *fakeGen) Nama() string { return "fake-llm" }

func (g *fakeGen) Susun(_ context.Context, k domain.Konteks) (*domain.Narasi, error) {
	g.dipanggil++
	g.konteks = k
	return g.narasi, g.err
}

// narasiLLM adalah narasi LLM yang valid: hanya placeholder, tanpa angka bebas.
func narasiLLM() *domain.Narasi {
	n := narasiValid()
	n.Meta = domain.MetaNarasi{Penyusun: domain.PenyusunLLM, Model: "claude-sonnet-5-5", PromptVersi: "brief-v1", PromptHash: "abc123"}
	return n
}

// newUseCase membuat UseCase yang menjalankan penyusunan secara sinkron dengan jam tetap.
func newUseCase(repo *fakeRepo, gen domain.Generator) (*UseCase, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	uc := New(repo, gen, rec, logger.NewDefault())
	uc.now = func() time.Time { return waktuUji }
	uc.jalankan = func(f func()) { f() }
	return uc, rec
}

func TestGenerate_TanpaLLMMenyusunDrafDariTemplatDanData(t *testing.T) {
	repo := newFakeRepo()
	uc, _ := newUseCase(repo, nil)

	got, err := uc.Generate(t.Context(), uid("manajer_kebijakan"), "2026-11")

	require.NoError(t, err)
	assert.Equal(t, "PB-2026-11", got.ID)
	assert.Equal(t, domain.StatusDiproses, got.Status, "jawaban Generate adalah keadaan saat dimulai, walau penyusunan sudah selesai")

	stored := repo.rows["PB-2026-11"]
	assert.Equal(t, domain.StatusDraf, stored.Status, "penyusunan sinkron di tes selesai sebelum Generate kembali")
	assert.Equal(t, "November 2026", stored.Periode)
	assert.Equal(t, "Policy brief November 2026", stored.Judul)
	assert.Equal(t, uid("manajer_kebijakan"), stored.DisusunOlehID)
	assert.Equal(t, domain.PenyusunTemplat, stored.SumberNarasi)
	assert.Nil(t, stored.ModelLLM)
	assert.Contains(t, stored.Ringkasan, "Inflasi tahunan Oktober 2026 tercatat 2,84%")
	assert.JSONEq(t, `["Siapkan operasi pasar dan pasokan alternatif untuk Cabai merah keriting.","Perkuat pemantauan harga Cabai merah keriting di pasar sentinel.","Perkuat pemantauan harga Beras medium di pasar sentinel.","Tinjau draf ini bersama data resmi terbaru sebelum menetapkan kebijakan.","Bahas langkah pengendalian harga Cabai merah keriting dalam rapat koordinasi TPID.","Verifikasi angka pada lampiran dengan rilis resmi BPS dan Bank Indonesia sebelum draf diajukan."]`, stored.Rekomendasi)
	assert.Positive(t, stored.Halaman)
	assert.Equal(t, []time.Time{bulanUji(2026, time.November)}, repo.diumpulkan, "data dikumpulkan untuk awal bulan periode")

	var konten domain.Konten
	require.NoError(t, json.Unmarshal([]byte(stored.Konten), &konten))
	assert.Equal(t, domain.LabelDrafTemplat, konten.Label)
	assert.NotEmpty(t, konten.Lampiran)

	var snapshot domain.Konteks
	require.NoError(t, json.Unmarshal([]byte(stored.SnapshotData), &snapshot))
	assert.Equal(t, BangunKonteks(repo.data2(bulanUji(2026, time.November))), snapshot, "snapshot = fakta yang dipakai menyusun")

	require.Len(t, repo.jobs, 1)
	for _, job := range repo.jobs {
		assert.Equal(t, domain.JobSelesai, job.Status)
		assert.Equal(t, "PB-2026-11", job.BriefID)
	}
	require.Len(t, repo.versi, 1)
	assert.Equal(t, uid("manajer_kebijakan"), repo.versi[0].OlehID)
	assert.Equal(t, stored.Konten, repo.versi[0].Konten)
}

func (r *fakeRepo) data2(periode time.Time) domain.DataPeriode {
	d := r.data
	d.Periode = periode
	return d
}

func TestGenerate_AsinkronDrafBaruAdaSetelahProsesBerjalan(t *testing.T) {
	repo := newFakeRepo()
	uc, _ := newUseCase(repo, nil)
	var tertunda func()
	uc.jalankan = func(f func()) { tertunda = f }

	got, err := uc.Generate(t.Context(), uid("u1"), "2026-11")

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDiproses, got.Status)
	assert.Equal(t, domain.StatusDiproses, repo.rows["PB-2026-11"].Status, "belum disusun saat Generate kembali")
	assert.Empty(t, repo.versi)
	require.NotNil(t, tertunda)
	for _, job := range repo.jobs {
		assert.Equal(t, domain.JobBerjalan, job.Status)
	}

	tertunda()

	assert.Equal(t, domain.StatusDraf, repo.rows["PB-2026-11"].Status)
	assert.Len(t, repo.versi, 1)
}

func TestGenerate_LLMDipakaiBilaValid(t *testing.T) {
	repo := newFakeRepo()
	gen := &fakeGen{narasi: narasiLLM()}
	uc, rec := newUseCase(repo, gen)

	_, err := uc.Generate(t.Context(), uid("u1"), "2026-11")

	require.NoError(t, err)
	stored := repo.rows["PB-2026-11"]
	assert.Equal(t, domain.PenyusunLLM, stored.SumberNarasi)
	require.NotNil(t, stored.ModelLLM)
	assert.Equal(t, "claude-sonnet-5-5", *stored.ModelLLM)
	assert.Equal(t, "brief-v1", *stored.PromptVersi)
	assert.Equal(t, "abc123", *stored.PromptHash)
	assert.Nil(t, stored.Galat)
	assert.Equal(t, "Inflasi tahunan Oktober 2026 tercatat 2,84%.", stored.Ringkasan, "placeholder sudah diganti nilai fakta")

	var konten domain.Konten
	require.NoError(t, json.Unmarshal([]byte(stored.Konten), &konten))
	assert.Equal(t, domain.LabelDrafAI, konten.Label)
	assert.Equal(t, gen.konteks, BangunKonteks(repo.data2(bulanUji(2026, time.November))), "LLM hanya menerima konteks hasil rangkuman deterministik")

	selesai, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "BRIEF_DRAF_SELESAI", selesai.Aksi)
	assert.Equal(t, "claude-sonnet-5-5", selesai.Metadata["model_llm"])
	assert.Equal(t, "abc123", selesai.Metadata["prompt_hash"])
}

func TestGenerate_LLMGagalAtauMelanggarAturanDigantikanTemplat(t *testing.T) {
	salahAngka := narasiLLM()
	salahAngka.Ringkasan = "Inflasi naik 5 persen."
	salahPlaceholder := narasiLLM()
	salahPlaceholder.Temuan = []string{"Nilai {{TIDAK_ADA}}."}

	kasus := map[string]*fakeGen{
		"llm galat":           {err: errors.New("429: rate limit")},
		"angka tak bersumber": {narasi: salahAngka},
		"placeholder palsu":   {narasi: salahPlaceholder},
	}
	for nama, gen := range kasus {
		t.Run(nama, func(t *testing.T) {
			repo := newFakeRepo()
			uc, _ := newUseCase(repo, gen)

			_, err := uc.Generate(t.Context(), uid("u1"), "2026-11")

			require.NoError(t, err)
			stored := repo.rows["PB-2026-11"]
			assert.Equal(t, domain.StatusDraf, stored.Status, "pengguna tetap mendapat draf")
			assert.Equal(t, domain.PenyusunTemplat, stored.SumberNarasi, "narasi LLM yang ditolak tidak pernah tersimpan")
			assert.Nil(t, stored.ModelLLM)
			require.NotNil(t, stored.Galat)
			assert.Contains(t, *stored.Galat, "fake-llm")
			assert.Contains(t, *stored.Galat, "draf memakai templat")
			assert.NotContains(t, *stored.Galat, "persen", "teks narasi yang ditolak tidak disimpan")
			assert.NotContains(t, stored.Konten, "naik 5 persen")
			assert.Equal(t, 1, gen.dipanggil)

			var konten domain.Konten
			require.NoError(t, json.Unmarshal([]byte(stored.Konten), &konten))
			assert.Equal(t, domain.LabelDrafTemplat, konten.Label)
			assert.Contains(t, konten.Catatan, "draf memakai templat", "alasan terlihat di isi brief")
		})
	}
}

func TestGenerate_GagalMengumpulkanDataMenandaiBriefGagal(t *testing.T) {
	repo := newFakeRepo()
	repo.kumpulErr = errors.New("db down")
	uc, rec := newUseCase(repo, nil)

	got, err := uc.Generate(t.Context(), uid("u1"), "2026-11")

	require.NoError(t, err, "Generate sendiri berhasil memulai; kegagalan tampil di status")
	assert.Equal(t, "PB-2026-11", got.ID)
	stored := repo.rows["PB-2026-11"]
	assert.Equal(t, domain.StatusGagal, stored.Status)
	require.NotNil(t, stored.Galat)
	assert.Contains(t, *stored.Galat, "db down")
	for _, job := range repo.jobs {
		assert.Equal(t, domain.JobGagal, job.Status)
	}
	assert.Empty(t, repo.versi)
	last, _ := rec.Last()
	assert.Equal(t, "BRIEF_DRAF_GAGAL", last.Aksi)
	assert.Equal(t, audit.HasilGagal, last.Hasil)
}

func TestGenerate_BriefGagalDapatDiulang(t *testing.T) {
	repo := newFakeRepo()
	repo.kumpulErr = errors.New("db down")
	uc, _ := newUseCase(repo, nil)
	_, err := uc.Generate(t.Context(), uid("u1"), "2026-11")
	require.NoError(t, err)
	require.Equal(t, domain.StatusGagal, repo.rows["PB-2026-11"].Status)

	repo.kumpulErr = nil
	_, err = uc.Generate(t.Context(), uid("u1"), "2026-11")

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDraf, repo.rows["PB-2026-11"].Status)
	assert.Nil(t, repo.rows["PB-2026-11"].Galat, "galat lama dibersihkan")
	assert.Len(t, repo.jobs, 2, "tiap percobaan punya job sendiri")
}

func TestGenerate_DiprosesBasiDapatDiulangYangBaruTidak(t *testing.T) {
	repo := newFakeRepo()
	uc, _ := newUseCase(repo, nil)

	repo.rows["PB-2026-11"] = domain.PolicyBrief{ID: "PB-2026-11", Status: domain.StatusDiproses, Diperbarui: waktuUji.Add(-time.Minute)}
	_, err := uc.Generate(t.Context(), uid("u1"), "2026-11")
	assert.ErrorIs(t, err, ErrPeriodeSudahAda, "masih berjalan: jangan disusun ganda")

	repo.rows["PB-2026-11"] = domain.PolicyBrief{ID: "PB-2026-11", Status: domain.StatusDiproses, Diperbarui: waktuUji.Add(-basiProses - time.Minute)}
	_, err = uc.Generate(t.Context(), uid("u1"), "2026-11")
	require.NoError(t, err, "prosesnya mati: boleh diulang")
	assert.Equal(t, domain.StatusDraf, repo.rows["PB-2026-11"].Status)
}

func TestGenerate_IsDeterministic(t *testing.T) {
	a, b := newFakeRepo(), newFakeRepo()
	ucA, _ := newUseCase(a, nil)
	ucB, _ := newUseCase(b, nil)

	_, errA := ucA.Generate(t.Context(), uid("u1"), "2026-03")
	_, errB := ucB.Generate(t.Context(), uid("u1"), "2026-03")

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, a.rows, b.rows, "data dan jam yang sama menghasilkan brief yang sama")
}

func TestGenerate_RejectsDuplicatePeriodWithoutOverwriting(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-10"] = domain.PolicyBrief{ID: "PB-2026-10", Judul: "Asli", Status: domain.StatusDisetujui}
	uc, rec := newUseCase(repo, nil)

	_, err := uc.Generate(t.Context(), uid("u1"), "2026-10")

	assert.ErrorIs(t, err, ErrPeriodeSudahAda)
	assert.Equal(t, "Asli", repo.rows["PB-2026-10"].Judul)
	assert.Empty(t, rec.Entries(), "tidak ada perubahan data, tidak ada audit")
	assert.Empty(t, repo.jobs)
}

func TestGenerate_RejectsInvalidPeriod(t *testing.T) {
	for _, periode := range []string{"", "Oktober 2026", "2026-13", "2026-00", "2026-1", "2026/10"} {
		repo := newFakeRepo()
		uc, _ := newUseCase(repo, nil)

		_, err := uc.Generate(t.Context(), uid("u1"), periode)

		assert.ErrorIs(t, err, ErrPeriodeTidakValid, periode)
		assert.Empty(t, repo.rows, periode)
	}
}

func TestGenerate_WritesAuditWithActorAndPenyusun(t *testing.T) {
	uc, rec := newUseCase(newFakeRepo(), &fakeGen{narasi: narasiLLM()})

	_, err := uc.Generate(t.Context(), uid("manajer_kebijakan"), "2026-10")

	require.NoError(t, err)
	entries := rec.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, uid("manajer_kebijakan"), entries[0].AktorID)
	assert.Equal(t, "GENERATE_BRIEF", entries[0].Aksi)
	assert.Equal(t, "policy_brief", entries[0].Entitas)
	assert.Equal(t, "PB-2026-10", entries[0].EntitasID)
	assert.Equal(t, audit.HasilBerhasil, entries[0].Hasil)
	assert.Equal(t, "fake-llm", entries[0].Metadata["penyusun"])
}

func TestGenerate_AuditFailureDoesNotFailTheAction(t *testing.T) {
	uc, rec := newUseCase(newFakeRepo(), nil)
	rec.Err = errors.New("audit down")

	got, err := uc.Generate(t.Context(), uid("u1"), "2026-10")

	require.NoError(t, err)
	assert.Equal(t, "PB-2026-10", got.ID)
}

func TestGenerate_PropagatesRepositoryError(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = errors.New("db down")
	uc, rec := newUseCase(repo, nil)

	_, err := uc.Generate(t.Context(), uid("u1"), "2026-10")

	assert.Error(t, err)
	assert.Empty(t, rec.Entries())
}

func TestDetail(t *testing.T) {
	repo := newFakeRepo()
	uc, _ := newUseCase(repo, nil)
	_, err := uc.Generate(t.Context(), uid("u1"), "2026-11")
	require.NoError(t, err)

	got, err := uc.Detail(t.Context(), "PB-2026-11")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDraf, got.Status)
	assert.NotEmpty(t, got.Konten)

	_, err = uc.Detail(t.Context(), "PB-1999-01")
	assert.ErrorIs(t, err, ErrBriefTidakAda)
}

func TestHitungHalaman(t *testing.T) {
	butir := func(jumlahKata int) string { return strings.TrimSpace(strings.Repeat("kata ", jumlahKata)) }

	assert.Equal(t, 1, hitungHalaman(&domain.Konten{}), "minimal satu halaman")
	assert.Equal(t, 1, hitungHalaman(&domain.Konten{Bagian: []domain.BagianKonten{{Butir: []string{butir(kataPerHalaman)}}}}))
	assert.Equal(t, 2, hitungHalaman(&domain.Konten{Bagian: []domain.BagianKonten{{Butir: []string{butir(200)}}, {Butir: []string{butir(101)}}}}))
}

func TestBulkAction_MemajukanStatusDanMencatatAudit(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = domain.PolicyBrief{ID: "PB-2026-09", Status: domain.StatusDraf}
	repo.rows["PB-2026-08"] = domain.PolicyBrief{ID: "PB-2026-08", Status: domain.StatusDisetujui}
	uc, rec := newUseCase(repo, nil)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09", "PB-2026-08", "tidak-ada"}, domain.AksiTelaah, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 2}, hasil)
	assert.Equal(t, domain.StatusTelaah, repo.rows["PB-2026-09"].Status)
	assert.Equal(t, domain.StatusDisetujui, repo.rows["PB-2026-08"].Status)
	assert.Len(t, rec.Entries(), 1)
}

func TestBulkAction_BriefDiprosesDanGagalTidakBisaDitelaah(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = domain.PolicyBrief{ID: "PB-2026-09", Status: domain.StatusDiproses}
	repo.rows["PB-2026-08"] = domain.PolicyBrief{ID: "PB-2026-08", Status: domain.StatusGagal}
	uc, _ := newUseCase(repo, nil)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09", "PB-2026-08"}, domain.AksiTelaah, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Gagal: 2}, hasil, "draf yang belum jadi tidak boleh diajukan telaah")
}

func TestBulkAction_SetujuiHanyaDariTelaah(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = domain.PolicyBrief{ID: "PB-2026-09", Status: domain.StatusDraf}
	uc, _ := newUseCase(repo, nil)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09"}, domain.AksiSetujui, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Gagal: 1}, hasil)
}

func TestBulkAction_AksiTidakValid(t *testing.T) {
	uc, _ := newUseCase(newFakeRepo(), nil)

	_, err := uc.BulkAction(context.Background(), []string{"x"}, "hapus", uid("u1"))

	assert.ErrorIs(t, err, ErrAksiTidakValid)
}
