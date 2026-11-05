package usecase

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panggilanHapus mencatat satu pemanggilan metode repositori palsu.
type panggilanHapus struct {
	metode  string
	sebelum time.Time
	batas   int
}

// repoPembersihanPalsu menjawab tiap metode dari antrean hasil per metode; antrean habis = 0 baris.
type repoPembersihanPalsu struct {
	panggilan []panggilanHapus
	hasil     map[string][]int64
	galat     map[string]error
	// sebelumPanggil dipanggil di awal tiap metode (mis. membatalkan ctx).
	sebelumPanggil func(metode string)
	// sekarangKunci adalah argumen sekarang terakhir pada HapusKunciLoginUsang.
	sekarangKunci time.Time
}

func (r *repoPembersihanPalsu) jawab(metode string, sebelum time.Time, batas int) (int64, error) {
	r.panggilan = append(r.panggilan, panggilanHapus{metode, sebelum, batas})
	if r.sebelumPanggil != nil {
		r.sebelumPanggil(metode)
	}
	if err := r.galat[metode]; err != nil {
		return 0, err
	}
	antrean := r.hasil[metode]
	if len(antrean) == 0 {
		return 0, nil
	}
	r.hasil[metode] = antrean[1:]
	return antrean[0], nil
}

func (r *repoPembersihanPalsu) HapusBlacklistKedaluwarsa(_ context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.jawab("jwt_blacklist", sebelum, batas)
}

func (r *repoPembersihanPalsu) HapusRefreshTokenKedaluwarsa(_ context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.jawab("refresh_tokens", sebelum, batas)
}

func (r *repoPembersihanPalsu) HapusSSOStateKedaluwarsa(_ context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.jawab("sso_login_state", sebelum, batas)
}

func (r *repoPembersihanPalsu) HapusKunciLoginUsang(_ context.Context, sebelum, sekarang time.Time, batas int) (int64, error) {
	r.sekarangKunci = sekarang
	return r.jawab("kunci_login", sebelum, batas)
}

func (r *repoPembersihanPalsu) HapusResetTokenMati(_ context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.jawab("password_reset_tokens", sebelum, batas)
}

func (r *repoPembersihanPalsu) HapusLoginAttemptLama(_ context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.jawab("login_attempts", sebelum, batas)
}

func (r *repoPembersihanPalsu) untuk(metode string) []panggilanHapus {
	var out []panggilanHapus
	for _, p := range r.panggilan {
		if p.metode == metode {
			out = append(out, p)
		}
	}
	return out
}

var sekarangUji = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func langkahHasil(t *testing.T, h HasilPembersihan, nama string) HasilLangkah {
	t.Helper()
	for _, l := range h.Langkah {
		if l.Nama == nama {
			return l
		}
	}
	require.Failf(t, "langkah tidak ada", "%s tidak ada di hasil %+v", nama, h.Langkah)
	return HasilLangkah{}
}

func TestPembersihan_BatasWaktuPerTabel(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	p := NewPembersihan(repo, DefaultKonfigPembersihan(), nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	assert.Equal(t, int64(0), hasil.Total())
	// Kedaluwarsa murni: tanpa jeda, tidak ada baris hidup yang boleh ikut terhapus.
	require.Len(t, repo.untuk("jwt_blacklist"), 1)
	assert.True(t, repo.untuk("jwt_blacklist")[0].sebelum.Equal(sekarangUji))
	require.Len(t, repo.untuk("refresh_tokens"), 1)
	assert.True(t, repo.untuk("refresh_tokens")[0].sebelum.Equal(sekarangUji))
	// Token reset: 7 hari sesudah mati; login_attempts: 90 hari.
	require.Len(t, repo.untuk("password_reset_tokens"), 1)
	assert.True(t, repo.untuk("password_reset_tokens")[0].sebelum.Equal(sekarangUji.Add(-7*24*time.Hour)))
	require.Len(t, repo.untuk("login_attempts"), 1)
	assert.True(t, repo.untuk("login_attempts")[0].sebelum.Equal(sekarangUji.Add(-90*24*time.Hour)))
	require.Len(t, repo.untuk("sso_login_state"), 1)
	assert.True(t, repo.untuk("sso_login_state")[0].sebelum.Equal(sekarangUji), "state SSO yang masih berlaku dipertahankan")
	for _, c := range repo.panggilan {
		assert.Equal(t, UkuranBatchBawaan, c.batas, c.metode)
	}
}

func TestPembersihan_UrutanLangkahTetap(t *testing.T) {
	p := NewPembersihan(&repoPembersihanPalsu{}, DefaultKonfigPembersihan(), nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	var nama []string
	for _, l := range hasil.Langkah {
		nama = append(nama, l.Nama)
	}
	assert.Equal(t, []string{"jwt_blacklist", "refresh_tokens", "password_reset_tokens", "sso_login_state", "kunci_login", "login_attempts"}, nama)
}

func TestPembersihan_KunciLoginHanyaYangUsangDanTidakTerkunci(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	cfg := DefaultKonfigPembersihan()
	cfg.UmurKunciLogin = time.Minute
	p := NewPembersihan(repo, cfg, nil)

	_, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	k := repo.untuk("kunci_login")
	require.Len(t, k, 1)
	assert.True(t, k[0].sebelum.Equal(sekarangUji.Add(-UmurKunciLoginMin)),
		"umur di bawah dua kali durasi kunci dinaikkan ke lantai: batas %s", k[0].sebelum)
	assert.True(t, repo.sekarangKunci.Equal(sekarangUji), "kunci yang masih berlaku pada waktu acuan dipertahankan")
	assert.Equal(t, 30*time.Minute, UmurKunciLoginMin)
}

func TestPembersihan_DiulangSampaiBatchTidakPenuh(t *testing.T) {
	repo := &repoPembersihanPalsu{hasil: map[string][]int64{"refresh_tokens": {10, 10, 4}}}
	cfg := DefaultKonfigPembersihan()
	cfg.UkuranBatch = 10
	p := NewPembersihan(repo, cfg, nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	l := langkahHasil(t, hasil, "refresh_tokens")
	assert.EqualValues(t, 24, l.Dihapus)
	assert.Equal(t, 3, l.Batch)
	assert.False(t, l.Terpotong)
	assert.Len(t, repo.untuk("refresh_tokens"), 3)
	assert.Len(t, repo.untuk("jwt_blacklist"), 1, "tabel lain tidak ikut berulang")
	assert.EqualValues(t, 24, hasil.Total())
}

func TestPembersihan_KelipatanBatchTepatButuhSatuBatchKosong(t *testing.T) {
	repo := &repoPembersihanPalsu{hasil: map[string][]int64{"jwt_blacklist": {10, 10, 0}}}
	cfg := DefaultKonfigPembersihan()
	cfg.UkuranBatch = 10
	p := NewPembersihan(repo, cfg, nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	l := langkahHasil(t, hasil, "jwt_blacklist")
	assert.EqualValues(t, 20, l.Dihapus)
	assert.Equal(t, 3, l.Batch)
	assert.False(t, l.Terpotong)
}

func TestPembersihan_MaksBatchMemotongPutaran(t *testing.T) {
	penuh := make([]int64, 50)
	for i := range penuh {
		penuh[i] = 10
	}
	repo := &repoPembersihanPalsu{hasil: map[string][]int64{"login_attempts": penuh}}
	cfg := DefaultKonfigPembersihan()
	cfg.UkuranBatch = 10
	cfg.MaksBatch = 5
	p := NewPembersihan(repo, cfg, nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	l := langkahHasil(t, hasil, "login_attempts")
	assert.Equal(t, 5, l.Batch)
	assert.EqualValues(t, 50, l.Dihapus)
	assert.True(t, l.Terpotong, "sisa baris dilanjutkan putaran berikutnya")
	assert.Len(t, repo.untuk("login_attempts"), 5)
	assert.False(t, langkahHasil(t, hasil, "jwt_blacklist").Terpotong)
}

func TestPembersihan_LantaiRetensiLoginAttempt(t *testing.T) {
	for nama, retensi := range map[string]time.Duration{
		"1 jam":         time.Hour,
		"1 detik":       time.Second,
		"23j59m":        23*time.Hour + 59*time.Minute,
		"negatif":       -48 * time.Hour,
		"tepat lantai":  MinRetensiLoginAttempt,
		"nilai positif": 1,
	} {
		t.Run(nama, func(t *testing.T) {
			repo := &repoPembersihanPalsu{}
			cfg := DefaultKonfigPembersihan()
			cfg.RetensiLoginAttempt = retensi
			p := NewPembersihan(repo, cfg, nil)

			_, err := p.JalankanPada(context.Background(), sekarangUji)

			require.NoError(t, err)
			la := repo.untuk("login_attempts")
			require.Len(t, la, 1)
			assert.True(t, la[0].sebelum.Equal(sekarangUji.Add(-MinRetensiLoginAttempt)),
				"baris yang lebih muda dari 1 hari tidak boleh pernah dihapus: batas %s", la[0].sebelum)
		})
	}
}

func TestPembersihan_RetensiLebihPanjangDariLantaiDihormati(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	cfg := DefaultKonfigPembersihan()
	cfg.RetensiLoginAttempt = 30 * 24 * time.Hour
	p := NewPembersihan(repo, cfg, nil)

	_, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	assert.True(t, repo.untuk("login_attempts")[0].sebelum.Equal(sekarangUji.Add(-30*24*time.Hour)))
}

func TestPembersihan_RetensiLoginNolMenonaktifkanLangkah(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	cfg := DefaultKonfigPembersihan()
	cfg.RetensiLoginAttempt = 0
	p := NewPembersihan(repo, cfg, nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	assert.Empty(t, repo.untuk("login_attempts"), "retensi 0 = login_attempts tidak pernah dihapus")
	assert.Len(t, hasil.Langkah, 5)
}

func TestPembersihan_RetensiResetNolMenghapusLangsung(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	cfg := DefaultKonfigPembersihan()
	cfg.RetensiResetToken = 0
	p := NewPembersihan(repo, cfg, nil)

	_, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	assert.True(t, repo.untuk("password_reset_tokens")[0].sebelum.Equal(sekarangUji))

	cfg.RetensiResetToken = -time.Hour
	repo2 := &repoPembersihanPalsu{}
	_, err = NewPembersihan(repo2, cfg, nil).JalankanPada(context.Background(), sekarangUji)
	require.NoError(t, err)
	assert.True(t, repo2.untuk("password_reset_tokens")[0].sebelum.Equal(sekarangUji), "retensi negatif tidak boleh memajukan batas ke masa depan")
}

func TestKonfigPembersihan_NormalisasiMengisiBawaan(t *testing.T) {
	got := KonfigPembersihan{}.Normalisasi()
	assert.Equal(t, UkuranBatchBawaan, got.UkuranBatch)
	assert.Equal(t, MaksBatchBawaan, got.MaksBatch)
	assert.Zero(t, got.RetensiLoginAttempt, "nol tetap berarti tidak dihapus")
	assert.Zero(t, got.RetensiResetToken)
	assert.Equal(t, UmurKunciLoginMin, got.UmurKunciLogin)

	d := DefaultKonfigPembersihan()
	assert.Equal(t, d, d.Normalisasi(), "konfigurasi bawaan sudah normal")
	assert.Equal(t, 90*24*time.Hour, d.RetensiLoginAttempt)
	assert.Equal(t, 7*24*time.Hour, d.RetensiResetToken)
}

func TestPembersihan_GalatSatuLangkahTidakMenghentikanYangLain(t *testing.T) {
	boom := errors.New("koneksi putus")
	repo := &repoPembersihanPalsu{
		galat: map[string]error{"refresh_tokens": boom},
		hasil: map[string][]int64{"jwt_blacklist": {3}, "login_attempts": {7}},
	}
	p := NewPembersihan(repo, DefaultKonfigPembersihan(), nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "refresh_tokens")
	assert.Equal(t, boom, langkahHasil(t, hasil, "refresh_tokens").Galat)
	assert.EqualValues(t, 3, langkahHasil(t, hasil, "jwt_blacklist").Dihapus)
	assert.EqualValues(t, 7, langkahHasil(t, hasil, "login_attempts").Dihapus, "langkah sesudah yang gagal tetap berjalan")
	assert.EqualValues(t, 10, hasil.Total())
}

func TestPembersihan_GalatDiTengahBatchMempertahankanYangSudahTerhapus(t *testing.T) {
	boom := errors.New("timeout")
	repo := &repoPembersihanPalsu{hasil: map[string][]int64{"jwt_blacklist": {10, 10}}}
	repo.sebelumPanggil = func(metode string) {
		if metode == "jwt_blacklist" && len(repo.untuk(metode)) == 3 {
			repo.galat = map[string]error{"jwt_blacklist": boom}
		}
	}
	cfg := DefaultKonfigPembersihan()
	cfg.UkuranBatch = 10
	p := NewPembersihan(repo, cfg, nil)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.ErrorIs(t, err, boom)
	l := langkahHasil(t, hasil, "jwt_blacklist")
	assert.EqualValues(t, 20, l.Dihapus, "dua batch pertama sudah tuntas dan tetap dihitung")
	assert.Equal(t, 2, l.Batch)
}

func TestPembersihan_KonteksSelesaiMenghentikanTanpaMemanggilRepo(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	p := NewPembersihan(repo, DefaultKonfigPembersihan(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hasil, err := p.JalankanPada(ctx, sekarangUji)

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, repo.panggilan)
	assert.Len(t, hasil.Langkah, 6)
}

func TestPembersihan_KonteksDibatalkanDiTengahBerhentiSebelumBatchBerikut(t *testing.T) {
	penuh := make([]int64, 100)
	for i := range penuh {
		penuh[i] = 10
	}
	repo := &repoPembersihanPalsu{hasil: map[string][]int64{"jwt_blacklist": penuh}}
	ctx, cancel := context.WithCancel(context.Background())
	repo.sebelumPanggil = func(string) {
		if len(repo.panggilan) == 3 {
			cancel()
		}
	}
	cfg := DefaultKonfigPembersihan()
	cfg.UkuranBatch = 10
	p := NewPembersihan(repo, cfg, nil)

	hasil, err := p.JalankanPada(ctx, sekarangUji)

	require.ErrorIs(t, err, context.Canceled)
	l := langkahHasil(t, hasil, "jwt_blacklist")
	assert.Equal(t, 3, l.Batch)
	assert.Len(t, repo.panggilan, 3)
}

func TestPembersihan_LangkahTambahanDijalankanSesudahBawaan(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	var dipanggil []int
	var waktu time.Time
	tambahan := Langkah{
		Nama: "sesi_dicabut",
		HapusBatch: func(_ context.Context, now time.Time, batas int) (int64, error) {
			dipanggil = append(dipanggil, batas)
			waktu = now
			return 2, nil
		},
	}
	p := NewPembersihan(repo, DefaultKonfigPembersihan(), nil, tambahan)

	hasil, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	require.Len(t, hasil.Langkah, 7)
	assert.Equal(t, "sesi_dicabut", hasil.Langkah[6].Nama)
	assert.EqualValues(t, 2, hasil.Langkah[6].Dihapus)
	assert.Equal(t, []int{UkuranBatchBawaan}, dipanggil)
	assert.True(t, waktu.Equal(sekarangUji), "semua langkah memakai waktu acuan yang sama")
}

func TestPembersihan_JalankanMemakaiWaktuSekarang(t *testing.T) {
	repo := &repoPembersihanPalsu{}
	p := NewPembersihan(repo, DefaultKonfigPembersihan(), nil)

	before := time.Now()
	_, err := p.Jalankan(context.Background())
	after := time.Now()

	require.NoError(t, err)
	got := repo.untuk("jwt_blacklist")[0].sebelum
	assert.False(t, got.Before(before) || got.After(after))
}

func TestPembersihan_LogMemuatJumlahPerTabel(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.DEBUG, &buf)
	repo := &repoPembersihanPalsu{hasil: map[string][]int64{"jwt_blacklist": {3}, "login_attempts": {5}}}
	p := NewPembersihan(repo, DefaultKonfigPembersihan(), log)

	_, err := p.JalankanPada(context.Background(), sekarangUji)

	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "pembersihan selesai")
	assert.Contains(t, out, `"jwt_blacklist":3`)
	assert.Contains(t, out, `"login_attempts":5`)
	assert.Contains(t, out, `"total":8`)
	assert.NotContains(t, out, "terpotong")
}

func TestPembersihan_LogMenandaiLangkahTerpotongDanGagal(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.DEBUG, &buf)
	repo := &repoPembersihanPalsu{
		hasil: map[string][]int64{"jwt_blacklist": {5, 5}},
		galat: map[string]error{"login_attempts": errors.New("boom")},
	}
	cfg := DefaultKonfigPembersihan()
	cfg.UkuranBatch = 5
	cfg.MaksBatch = 2
	p := NewPembersihan(repo, cfg, log)

	_, err := p.JalankanPada(context.Background(), sekarangUji)

	require.Error(t, err)
	out := buf.String()
	assert.Contains(t, out, "selesai sebagian")
	assert.Contains(t, out, `"terpotong":["jwt_blacklist"]`)
	assert.Contains(t, out, `"gagal":["login_attempts"]`)
}
