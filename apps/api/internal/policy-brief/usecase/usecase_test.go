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
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var waktuUji = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)

// uuidBrief meniru ID yang diberikan basis data untuk brief berkode tertentu. Deterministik (idx.UUIDStabil) supaya
// tes yang membandingkan dua jalan yang sama tetap bermakna; basis data sungguhan memakai gen_random_uuid().
func uuidBrief(kode string) string { return idx.UUIDStabil("policy_brief", kode) }

// brief membuat baris brief tes berstatus tertentu dengan ID dan kode yang konsisten.
func brief(kode, status string) domain.PolicyBrief {
	return domain.PolicyBrief{ID: uuidBrief(kode), Kode: kode, Status: status}
}

// fakeRepo menyimpan brief (berkunci kode), job, dan versi di memori; kode yang sudah ada tidak ditimpa,
// seperti ON CONFLICT (kode) DO NOTHING. ID brief UUID dan ID job BIGINT "diberikan basis data".
type fakeRepo struct {
	rows       map[string]domain.PolicyBrief
	jobs       map[idx.ID]domain.BriefJob
	versi      []domain.HasilDraf
	createErr  error
	kumpulErr  error
	data       domain.DataPeriode
	diumpulkan []time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: map[string]domain.PolicyBrief{}, jobs: map[idx.ID]domain.BriefJob{}, data: dataLengkap()}
}

// cari mencari brief menurut ID (UUID) atau kode dan mengembalikan kodenya.
func (r *fakeRepo) cari(ref idx.RujukanUUID) (string, bool) {
	for kode, pb := range r.rows {
		if (ref.ID != "" && pb.ID == ref.ID) || (ref.Kode != "" && kode == ref.Kode) {
			return kode, true
		}
	}
	return "", false
}

func (r *fakeRepo) kodeDariID(id string) (string, bool) { return r.cari(idx.RujukanUUID{ID: id}) }

func (r *fakeRepo) List(context.Context) ([]domain.BriefView, error) { return nil, nil }

func (r *fakeRepo) Get(_ context.Context, id string) (*domain.BriefView, error) {
	kode, ada := r.kodeDariID(id)
	if !ada {
		return nil, gorm.ErrRecordNotFound
	}
	pb := r.rows[kode]
	return &domain.BriefView{ID: pb.ID, Kode: pb.Kode, Periode: pb.Periode, Judul: pb.Judul, Status: pb.Status, Ringkasan: pb.Ringkasan, DisusunOleh: "Penyusun", SumberNarasi: pb.SumberNarasi}, nil
}

func (r *fakeRepo) GetDetail(ctx context.Context, ref idx.RujukanUUID) (*domain.DetailView, error) {
	kode, ada := r.cari(ref)
	if !ada {
		return nil, gorm.ErrRecordNotFound
	}
	pb := r.rows[kode]
	v, _ := r.Get(ctx, pb.ID)
	return &domain.DetailView{BriefView: *v, Konten: pb.Konten, ModelLLM: pb.ModelLLM, PromptVersi: pb.PromptVersi, PromptHash: pb.PromptHash, Galat: pb.Galat}, nil
}

// ListVersi menyusun riwayat versi dari draf yang tersimpan: nomor urut per brief, ID BIGINT berurutan, terbaru dulu.
func (r *fakeRepo) ListVersi(_ context.Context, briefID string) ([]domain.VersiView, error) {
	var out []domain.VersiView
	nomor := 0
	for i, d := range r.versi {
		if d.BriefID != briefID {
			continue
		}
		nomor++
		out = append([]domain.VersiView{{ID: idx.ID(i + 1), Versi: nomor, Oleh: "Penyusun", Waktu: d.Waktu}}, out...)
	}
	return out, nil
}

func (r *fakeRepo) Transisi(_ context.Context, ref idx.RujukanUUID, dari, ke string) (string, bool, error) {
	kode, ada := r.cari(ref)
	if !ada || r.rows[kode].Status != dari {
		return "", false, nil
	}
	pb := r.rows[kode]
	pb.Status = ke
	r.rows[kode] = pb
	return pb.ID, true, nil
}

func (r *fakeRepo) Create(_ context.Context, pb *domain.PolicyBrief) (bool, error) {
	if r.createErr != nil {
		return false, r.createErr
	}
	if ada, ok := r.rows[pb.Kode]; ok {
		pb.ID = ada.ID
		return false, nil
	}
	pb.ID = uuidBrief(pb.Kode)
	r.rows[pb.Kode] = *pb
	return true, nil
}

func (r *fakeRepo) MulaiUlang(_ context.Context, id string, basiSebelum time.Time) (bool, error) {
	kode, ada := r.kodeDariID(id)
	if !ada {
		return false, nil
	}
	pb := r.rows[kode]
	if !(pb.Status == domain.StatusGagal || (pb.Status == domain.StatusDiproses && pb.Diperbarui.Before(basiSebelum))) {
		return false, nil
	}
	pb.Status, pb.Galat = domain.StatusDiproses, nil
	r.rows[kode] = pb
	return true, nil
}

func (r *fakeRepo) MulaiJob(_ context.Context, job *domain.BriefJob) error {
	job.ID = idx.ID(len(r.jobs) + 1)
	r.jobs[job.ID] = *job
	return nil
}

func (r *fakeRepo) SelesaikanDraf(_ context.Context, d domain.HasilDraf) (bool, error) {
	kode, ada := r.kodeDariID(d.BriefID)
	if !ada {
		return false, nil
	}
	pb := r.rows[kode]
	if pb.Status != domain.StatusDiproses {
		return false, nil
	}
	pb.Status, pb.Judul, pb.Ringkasan, pb.Rekomendasi, pb.Halaman = domain.StatusDraf, d.Judul, d.Ringkasan, d.Rekomendasi, d.Halaman
	pb.Konten, pb.SnapshotData, pb.SumberNarasi = d.Konten, d.Snapshot, d.Penyusun
	pb.ModelLLM, pb.PromptVersi, pb.PromptHash, pb.Galat = d.ModelLLM, d.PromptVersi, d.PromptHash, d.Catatan
	r.rows[kode] = pb
	r.versi = append(r.versi, d)
	job := r.jobs[d.JobID]
	job.Status = domain.JobSelesai
	r.jobs[d.JobID] = job
	return true, nil
}

func (r *fakeRepo) GagalkanDraf(_ context.Context, briefID string, jobID idx.ID, galat string, _ time.Time) error {
	if kode, ada := r.kodeDariID(briefID); ada {
		pb := r.rows[kode]
		if pb.Status == domain.StatusDiproses {
			pb.Status, pb.Galat = domain.StatusGagal, &galat
			r.rows[kode] = pb
		}
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
	assert.Equal(t, "PB-2026-11", got.Kode, "kode = PB-YYYY-MM dari periode")
	assert.Equal(t, uuidBrief("PB-2026-11"), got.ID, "ID brief UUID dari basis data, bukan lagi PB-YYYY-MM")
	assert.True(t, idx.ValidUUID(got.ID))
	assert.Equal(t, domain.StatusDiproses, got.Status, "jawaban Generate adalah keadaan saat dimulai, walau penyusunan sudah selesai")

	stored := repo.rows["PB-2026-11"]
	assert.Equal(t, "PB-2026-11", stored.Kode)
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
	for id, job := range repo.jobs {
		assert.Equal(t, domain.JobSelesai, job.Status)
		assert.Equal(t, uuidBrief("PB-2026-11"), job.BriefID)
		assert.Equal(t, id, job.ID, "ID job BIGINT dari basis data")
		assert.Nil(t, job.Kode, "job baru tanpa kode lama")
	}
	require.Len(t, repo.versi, 1)
	assert.Equal(t, uid("manajer_kebijakan"), repo.versi[0].OlehID)
	assert.Equal(t, stored.Konten, repo.versi[0].Konten)
	assert.Equal(t, uuidBrief("PB-2026-11"), repo.versi[0].BriefID)
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
	assert.Equal(t, uuidBrief("PB-2026-11"), got.ID)
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

	baru := brief("PB-2026-11", domain.StatusDiproses)
	baru.Diperbarui = waktuUji.Add(-time.Minute)
	repo.rows["PB-2026-11"] = baru
	_, err := uc.Generate(t.Context(), uid("u1"), "2026-11")
	assert.ErrorIs(t, err, ErrPeriodeSudahAda, "masih berjalan: jangan disusun ganda")

	basi := brief("PB-2026-11", domain.StatusDiproses)
	basi.Diperbarui = waktuUji.Add(-basiProses - time.Minute)
	repo.rows["PB-2026-11"] = basi
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
	asli := brief("PB-2026-10", domain.StatusDisetujui)
	asli.Judul = "Asli"
	repo.rows["PB-2026-10"] = asli
	uc, rec := newUseCase(repo, nil)

	_, err := uc.Generate(t.Context(), uid("u1"), "2026-10")

	assert.ErrorIs(t, err, ErrPeriodeSudahAda, "satu periode hanya punya satu brief (kode unik)")
	assert.Equal(t, "Asli", repo.rows["PB-2026-10"].Judul)
	assert.Len(t, repo.rows, 1, "tidak ada brief kedua untuk periode yang sama")
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
	assert.Equal(t, uuidBrief("PB-2026-10"), entries[0].EntitasID, "audit memakai UUID brief")
	assert.Equal(t, "PB-2026-10", entries[0].Metadata["kode"], "kode tampilan ikut di metadata")
	assert.Equal(t, audit.HasilBerhasil, entries[0].Hasil)
	assert.Equal(t, "fake-llm", entries[0].Metadata["penyusun"])
}

func TestGenerate_AuditFailureDoesNotFailTheAction(t *testing.T) {
	uc, rec := newUseCase(newFakeRepo(), nil)
	rec.Err = errors.New("audit down")

	got, err := uc.Generate(t.Context(), uid("u1"), "2026-10")

	require.NoError(t, err)
	assert.Equal(t, "PB-2026-10", got.Kode)
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

	for nama, ref := range map[string]idx.RujukanUUID{
		"kode": {Kode: "PB-2026-11"},
		"uuid": {ID: uuidBrief("PB-2026-11")},
	} {
		got, err := uc.Detail(t.Context(), ref)
		require.NoError(t, err, nama)
		assert.Equal(t, domain.StatusDraf, got.Status, nama)
		assert.Equal(t, uuidBrief("PB-2026-11"), got.ID, nama)
		assert.Equal(t, "PB-2026-11", got.Kode, nama)
		assert.NotEmpty(t, got.Konten, nama)
		require.Len(t, got.RiwayatVersi, 1, nama+": versi pertama dari penyusunan draf")
		assert.Equal(t, 1, got.RiwayatVersi[0].Versi, nama)
		assert.Positive(t, int64(got.RiwayatVersi[0].ID), nama+": versi_id BIGINT")
	}

	_, err = uc.Detail(t.Context(), idx.RujukanUUID{Kode: "PB-1999-01"})
	assert.ErrorIs(t, err, ErrBriefTidakAda)
	_, err = uc.Detail(t.Context(), idx.RujukanUUID{ID: uuidBrief("PB-1999-01")})
	assert.ErrorIs(t, err, ErrBriefTidakAda)
}

func TestRujukanBrief_UUIDAtauKode(t *testing.T) {
	id := uuidBrief("PB-2026-10")
	kasus := []struct {
		teks string
		want idx.RujukanUUID
		sah  bool
	}{
		{id, idx.RujukanUUID{ID: id}, true},
		{strings.ToUpper(id), idx.RujukanUUID{ID: id}, true}, // UUID huruf besar dinormalkan
		{"PB-2026-10", idx.RujukanUUID{Kode: "PB-2026-10"}, true},
		{"bukan-uuid-dan-bukan-kode", idx.RujukanUUID{Kode: "bukan-uuid-dan-bukan-kode"}, true}, // dicari sebagai kode, 404 bila tak ada
		{strings.Repeat("P", 64), idx.RujukanUUID{Kode: strings.Repeat("P", 64)}, true},
		{strings.Repeat("P", 65), idx.RujukanUUID{}, false},
		{"", idx.RujukanUUID{}, false},
	}
	for _, k := range kasus {
		got, sah := RujukanBrief(k.teks)
		assert.Equal(t, k.sah, sah, k.teks)
		assert.Equal(t, k.want, got, k.teks)
	}
}

func TestHitungHalaman(t *testing.T) {
	butir := func(jumlahKata int) string { return strings.TrimSpace(strings.Repeat("kata ", jumlahKata)) }

	assert.Equal(t, 1, hitungHalaman(&domain.Konten{}), "minimal satu halaman")
	assert.Equal(t, 1, hitungHalaman(&domain.Konten{Bagian: []domain.BagianKonten{{Butir: []string{butir(kataPerHalaman)}}}}))
	assert.Equal(t, 2, hitungHalaman(&domain.Konten{Bagian: []domain.BagianKonten{{Butir: []string{butir(200)}}, {Butir: []string{butir(101)}}}}))
}

func TestBulkAction_MemajukanStatusDanMencatatAudit(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = brief("PB-2026-09", domain.StatusDraf)
	repo.rows["PB-2026-08"] = brief("PB-2026-08", domain.StatusDisetujui)
	uc, rec := newUseCase(repo, nil)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09", "PB-2026-08", "tidak-ada"}, domain.AksiTelaah, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 2}, hasil)
	assert.Equal(t, domain.StatusTelaah, repo.rows["PB-2026-09"].Status)
	assert.Equal(t, domain.StatusDisetujui, repo.rows["PB-2026-08"].Status)
	entries := rec.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, uuidBrief("PB-2026-09"), entries[0].EntitasID, "audit memakai UUID brief walau rujukannya kode")
}

func TestBulkAction_MenerimaUUIDMaupunKodeDanMenolakRujukanTakSah(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = brief("PB-2026-09", domain.StatusDraf)
	repo.rows["PB-2026-08"] = brief("PB-2026-08", domain.StatusDraf)
	uc, rec := newUseCase(repo, nil)

	hasil, err := uc.BulkAction(context.Background(), []string{uuidBrief("PB-2026-09"), "PB-2026-08", uuidBrief("PB-1999-01"), strings.Repeat("x", 65)}, domain.AksiTelaah, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 2, Gagal: 2}, hasil)
	assert.Equal(t, domain.StatusTelaah, repo.rows["PB-2026-09"].Status)
	assert.Equal(t, domain.StatusTelaah, repo.rows["PB-2026-08"].Status)
	for _, e := range rec.Entries() {
		assert.True(t, idx.ValidUUID(e.EntitasID), e.EntitasID)
	}
}

func TestBulkAction_BriefDiprosesDanGagalTidakBisaDitelaah(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = brief("PB-2026-09", domain.StatusDiproses)
	repo.rows["PB-2026-08"] = brief("PB-2026-08", domain.StatusGagal)
	uc, _ := newUseCase(repo, nil)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09", "PB-2026-08"}, domain.AksiTelaah, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Gagal: 2}, hasil, "draf yang belum jadi tidak boleh diajukan telaah")
}

func TestBulkAction_SetujuiHanyaDariTelaah(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = brief("PB-2026-09", domain.StatusDraf)
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
