package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/idx"
)

const sandiAktivasi = "Rahasia#2026Kuat"

func entriAudit(f *undanganFixture, aksi string) []audit.Entry {
	var out []audit.Entry
	for _, e := range f.audit.Entries() {
		if e.Aksi == aksi {
			out = append(out, e)
		}
	}
	return out
}

// ── Mengundang ────────────────────────────────────────────────────────

func TestUndang_MembuatAkunMenungguTanpaKataSandiDanMengirimEmail(t *testing.T) {
	f := newUndanganFixture(t)

	h := f.undang(UndangInput{Nama: "  Budi Santoso ", Email: "  Budi@GARDA.test ", Peran: "ops_mahasiswa", Lembaga: " FMIPA USK ", KabupatenKode: "1171"})

	t.Run("akun MENUNGGU tanpa kata sandi, isian dirapikan", func(t *testing.T) {
		u := f.repo.pengguna(h.User.ID)
		require.NotNil(t, u)
		assert.Equal(t, "MENUNGGU", u.Status)
		assert.Nil(t, u.PasswordHash, "tidak ada kata sandi sebelum aktivasi")
		assert.False(t, u.WajibGantiPassword)
		assert.Equal(t, "Budi Santoso", u.Nama)
		assert.Equal(t, "budi@garda.test", u.Email, "email dirapikan ke huruf kecil")
		assert.Equal(t, "OPS_MAHASISWA", u.Peran)
		assert.Equal(t, "OPERASIONAL", u.Lapis, "lapis diturunkan dari peran")
		assert.Equal(t, "FMIPA USK", u.Lembaga)
		require.NotNil(t, u.KabupatenKode)
		assert.Equal(t, "1171", *u.KabupatenKode)
		assert.True(t, idx.ValidUUID(u.ID), "id akun undangan adalah UUID: %q", u.ID)
	})
	t.Run("hasil: berlaku 72 jam dan email terkirim", func(t *testing.T) {
		assert.True(t, h.EmailTerkirim)
		assert.True(t, h.BerlakuSampai.Equal(awalUji.Add(72*time.Hour)), "TTL bawaan 72 jam, bukan %v", h.BerlakuSampai.Sub(awalUji))
	})
	t.Run("email berisi tautan aktivasi; hanya hash tokennya yang disimpan", func(t *testing.T) {
		pesan := f.smtp.Tunggu(t, "budi@garda.test", 1)
		require.Len(t, pesan, 1)
		assert.Contains(t, pesan[0].Data, baseURLUji+"/aktivasi?token=")
		assert.Contains(t, pesan[0].Data, "Budi Santoso")
		assert.Contains(t, pesan[0].Data, "72 jam")
		raw := pesan[0].Token()
		require.NotEmpty(t, raw)
		assert.GreaterOrEqual(t, len(raw), 43, "token 256-bit")

		aktif := f.repo.undanganAktif(h.User.ID)
		require.Len(t, aktif, 1)
		assert.Equal(t, sha256Hex(raw), aktif[0].TokenHash)
		assert.NotContains(t, aktif[0].TokenHash, raw)
		assert.Equal(t, "budi@garda.test", aktif[0].Email)
		require.NotNil(t, aktif[0].DiundangOleh)
		assert.Equal(t, adminUndangID, *aktif[0].DiundangOleh)
	})
	t.Run("audit mencatat undangan tanpa token", func(t *testing.T) {
		entri := entriAudit(f, "UNDANG_PENGGUNA")
		require.Len(t, entri, 1)
		e := entri[0]
		assert.Equal(t, adminUndangID, e.AktorID)
		assert.Equal(t, "pengguna", e.Entitas)
		assert.Equal(t, h.User.ID, e.EntitasID)
		assert.Equal(t, audit.HasilBerhasil, e.Hasil)
		assert.Equal(t, true, e.Metadata["email_terkirim"])
		assert.Equal(t, "form", e.Metadata["sumber"])
		serial, err := json.Marshal(e)
		require.NoError(t, err)
		raw := f.smtp.Untuk("budi@garda.test")[0].Token()
		assert.NotContains(t, string(serial), raw, "token mentah tidak boleh masuk audit")
		assert.NotContains(t, string(serial), "password")
	})
}

func TestUndang_TTLDapatDiatur(t *testing.T) {
	f := newUndanganFixture(t)
	f.uc.cfg.UndanganTTL = 24 * time.Hour

	h := f.undang(masukanUndang())

	assert.True(t, h.BerlakuSampai.Equal(awalUji.Add(24*time.Hour)))
	assert.Contains(t, f.smtp.Tunggu(t, "budi@garda.test", 1)[0].Data, "24 jam")
}

func TestUndang_Validasi(t *testing.T) {
	kasus := map[string]struct {
		ubah  func(*UndangInput)
		pesan string
	}{
		"nama kosong":            {func(in *UndangInput) { in.Nama = "  " }, "Nama wajib diisi"},
		"nama terlalu panjang":   {func(in *UndangInput) { in.Nama = strings.Repeat("a", 256) }, "Nama maksimal 255"},
		"nama berisi baris baru": {func(in *UndangInput) { in.Nama = "Budi\nSantoso" }, "karakter yang tidak diizinkan"},
		"nama diawali rumus":     {func(in *UndangInput) { in.Nama = "=HYPERLINK(\"x\")" }, "rumus"},
		"nama diawali plus":      {func(in *UndangInput) { in.Nama = "+62" }, "rumus"},
		"email kosong":           {func(in *UndangInput) { in.Email = "" }, "Email wajib diisi"},
		"email tanpa @":          {func(in *UndangInput) { in.Email = "budi.garda.test" }, "Email tidak valid"},
		"email tanpa domain":     {func(in *UndangInput) { in.Email = "budi@garda" }, "Email tidak valid"},
		"email berspasi":         {func(in *UndangInput) { in.Email = "bu di@garda.test" }, "Email tidak valid"},
		"email dengan nama":      {func(in *UndangInput) { in.Email = "Budi <budi@garda.test>" }, "Email tidak valid"},
		"email dua alamat":       {func(in *UndangInput) { in.Email = "a@garda.test,b@garda.test" }, "Email tidak valid"},
		"email injeksi header":   {func(in *UndangInput) { in.Email = "a@garda.test\r\nBcc: x@y.id" }, "Email tidak valid"},
		"email diawali rumus":    {func(in *UndangInput) { in.Email = "=a@garda.test" }, "Email"},
		"peran kosong":           {func(in *UndangInput) { in.Peran = "" }, "Peran wajib diisi"},
		"peran tak dikenal":      {func(in *UndangInput) { in.Peran = "DEWA" }, "Peran tidak dikenal: DEWA"},
		"lembaga kosong":         {func(in *UndangInput) { in.Lembaga = "" }, "Lembaga wajib diisi"},
		"lembaga diawali rumus":  {func(in *UndangInput) { in.Lembaga = "@kantor" }, "rumus"},
		"kabupaten bukan angka":  {func(in *UndangInput) { in.KabupatenKode = "11a1" }, "4 digit"},
		"kabupaten tak dikenal":  {func(in *UndangInput) { in.KabupatenKode = "9999" }, "Kode kabupaten tidak dikenal: 9999"},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			f := newUndanganFixture(t)
			in := masukanUndang()
			k.ubah(&in)

			_, err := f.uc.UndangPengguna(context.Background(), in, adminUndangID)

			var validasi *GalatValidasi
			require.ErrorAs(t, err, &validasi)
			assert.Contains(t, validasi.Pesan, k.pesan)
			assert.Zero(t, f.repo.nBuat, "tidak ada yang ditulis")
			assert.Empty(t, f.smtp.Messages(), "tidak ada email")
			assert.Empty(t, f.audit.Entries())
		})
	}
}

func TestUndang_BeberapaGalatDigabung(t *testing.T) {
	f := newUndanganFixture(t)

	_, err := f.uc.UndangPengguna(context.Background(), UndangInput{Email: "salah"}, adminUndangID)

	var validasi *GalatValidasi
	require.ErrorAs(t, err, &validasi)
	for _, bagian := range []string{"Nama wajib diisi", "Email tidak valid", "Peran wajib diisi", "Lembaga wajib diisi"} {
		assert.Contains(t, validasi.Pesan, bagian)
	}
}

func TestUndang_KabupatenOpsional(t *testing.T) {
	f := newUndanganFixture(t)
	in := masukanUndang()
	in.KabupatenKode = ""

	h := f.undang(in)

	assert.Nil(t, f.repo.pengguna(h.User.ID).KabupatenKode, "tanpa kabupaten = seluruh Aceh")
}

func TestUndang_EmailKembarDitolakTanpaMemedulikanHurufBesar(t *testing.T) {
	ada := &domain.Pengguna{ID: uid("u-1"), Nama: "Ada", Email: "Budi@Garda.Test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "X", Status: "AKTIF"}
	f := newUndanganFixture(t, ada)

	_, err := f.uc.UndangPengguna(context.Background(), masukanUndang(), adminUndangID)

	require.ErrorIs(t, err, ErrEmailExists)
	assert.Empty(t, f.smtp.Messages())
	assert.Empty(t, entriAudit(f, "UNDANG_PENGGUNA"))
}

func TestUndang_GalatBasisDataDilaporkanTanpaEmail(t *testing.T) {
	f := newUndanganFixture(t)
	f.repo.errBuat = errors.New("koneksi putus")

	_, err := f.uc.UndangPengguna(context.Background(), masukanUndang(), adminUndangID)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrEmailExists)
	assert.Empty(t, f.smtp.Messages())
}

func TestUndang_EmailGagalAkunDanUndanganTetapAda(t *testing.T) {
	f := newUndanganFixture(t)
	f.smtp.TolakPenerima(true)

	h := f.undang(masukanUndang())

	assert.False(t, h.EmailTerkirim)
	require.NotNil(t, f.repo.pengguna(h.User.ID), "akun tetap dibuat")
	assert.Len(t, f.repo.undanganAktif(h.User.ID), 1)
	entri := entriAudit(f, "UNDANG_PENGGUNA")
	require.Len(t, entri, 1)
	assert.Equal(t, false, entri[0].Metadata["email_terkirim"])

	t.Run("kirim ulang sesudah SMTP pulih", func(t *testing.T) {
		f.smtp.TolakPenerima(false)
		ulang, err := f.uc.KirimUlangUndangan(context.Background(), h.User.ID, adminUndangID)
		require.NoError(t, err)
		assert.True(t, ulang.EmailTerkirim)
		assert.Len(t, f.smtp.Tunggu(t, "budi@garda.test", 1), 1)
	})
}

func TestUndang_SMTPTidakDikonfigurasiMelaporkanEmailTidakTerkirim(t *testing.T) {
	f := newUndanganFixture(t)
	f.uc.emailSender = email.New(email.Config{})

	h := f.undang(masukanUndang())

	assert.False(t, h.EmailTerkirim)
	assert.Len(t, f.repo.undanganAktif(h.User.ID), 1)
}

func TestUndang_SMTPMacetDibatasiWaktu(t *testing.T) {
	f := newUndanganFixture(t)
	f.smtp.Diam(true)
	lama := batasWaktuEmail
	batasWaktuEmail = 150 * time.Millisecond
	t.Cleanup(func() { batasWaktuEmail = lama })

	mulai := time.Now()
	h := f.undang(masukanUndang())

	assert.False(t, h.EmailTerkirim)
	assert.Less(t, time.Since(mulai), 3*time.Second, "permintaan admin tidak menggantung menunggu SMTP")
	require.NotNil(t, f.repo.pengguna(h.User.ID))
}

func TestUndang_PermintaanDibatalkanSaatMenungguEmail(t *testing.T) {
	f := newUndanganFixture(t)
	f.smtp.Diam(true)
	ctx, batal := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer batal()

	h, err := f.uc.UndangPengguna(ctx, masukanUndang(), adminUndangID)

	require.NoError(t, err)
	assert.False(t, h.EmailTerkirim)
}

// ── Kirim ulang ───────────────────────────────────────────────────────

func TestKirimUlang_MencabutUndanganLamaDanMengirimYangBaru(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	lama := f.tokenEmail("budi@garda.test", 1)
	*f.now = f.now.Add(time.Hour)

	ulang, err := f.uc.KirimUlangUndangan(context.Background(), h.User.ID, adminUndangID)

	require.NoError(t, err)
	assert.True(t, ulang.EmailTerkirim)
	assert.True(t, ulang.BerlakuSampai.Equal(awalUji.Add(time.Hour+72*time.Hour)), "masa berlaku dihitung dari pengiriman ulang")
	baru := f.tokenEmail("budi@garda.test", 2)
	assert.NotEqual(t, lama, baru)

	t.Run("hanya satu undangan hidup, yang baru", func(t *testing.T) {
		aktif := f.repo.undanganAktif(h.User.ID)
		require.Len(t, aktif, 1)
		assert.Equal(t, sha256Hex(baru), aktif[0].TokenHash)
	})
	t.Run("tautan lama tidak lagi berlaku", func(t *testing.T) {
		err := f.uc.AktifkanAkun(context.Background(), lama, sandiAktivasi)
		require.ErrorIs(t, err, ErrUndanganInvalid)
		assert.Equal(t, "MENUNGGU", f.repo.pengguna(h.User.ID).Status)
	})
	t.Run("tautan baru berlaku", func(t *testing.T) {
		require.NoError(t, f.uc.AktifkanAkun(context.Background(), baru, sandiAktivasi))
	})
	t.Run("tercatat di audit", func(t *testing.T) {
		entri := entriAudit(f, "UNDANG_ULANG_PENGGUNA")
		require.Len(t, entri, 1)
		assert.Equal(t, adminUndangID, entri[0].AktorID)
		assert.Equal(t, h.User.ID, entri[0].EntitasID)
	})
}

func TestKirimUlang_HanyaUntukAkunMenunggu(t *testing.T) {
	aktif := &domain.Pengguna{ID: uid("u-aktif"), Nama: "Aktif", Email: "aktif@garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "X", Status: "AKTIF"}
	nonaktif := &domain.Pengguna{ID: uid("u-non"), Nama: "Non", Email: "non@garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "X", Status: "NONAKTIF"}
	f := newUndanganFixture(t, aktif, nonaktif)

	for _, id := range []string{uid("u-aktif"), uid("u-non")} {
		_, err := f.uc.KirimUlangUndangan(context.Background(), id, adminUndangID)
		require.ErrorIs(t, err, ErrBukanMenunggu, id)
	}
	_, err := f.uc.KirimUlangUndangan(context.Background(), "tidak-ada", adminUndangID)
	require.ErrorIs(t, err, ErrUserNotFound)
	assert.Empty(t, f.smtp.Messages())
	assert.Empty(t, entriAudit(f, "UNDANG_ULANG_PENGGUNA"))
}

func TestKirimUlang_GalatBasisDataBukanTidakDitemukan(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	f.repo.errGanti = errors.New("deadlock")

	_, err := f.uc.KirimUlangUndangan(context.Background(), h.User.ID, adminUndangID)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound)
	assert.NotErrorIs(t, err, ErrBukanMenunggu)
}

func TestKirimUlang_UndanganKedaluwarsaDigantiDenganYangBaru(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	lama := f.tokenEmail("budi@garda.test", 1)
	*f.now = f.now.Add(73 * time.Hour)
	require.ErrorIs(t, f.uc.AktifkanAkun(context.Background(), lama, sandiAktivasi), ErrUndanganKedaluwarsa)

	_, err := f.uc.KirimUlangUndangan(context.Background(), h.User.ID, adminUndangID)
	require.NoError(t, err)

	baru := f.tokenEmail("budi@garda.test", 2)
	require.NoError(t, f.uc.AktifkanAkun(context.Background(), baru, sandiAktivasi))
}

// ── Aktivasi ──────────────────────────────────────────────────────────

func TestAktifkan_MenetapkanKataSandiDanMengaktifkanAkun(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)

	require.NoError(t, f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi))

	u := f.repo.pengguna(h.User.ID)
	assert.Equal(t, "AKTIF", u.Status)
	require.NotNil(t, u.PasswordHash)
	assert.True(t, crypto.VerifyPassword(sandiAktivasi, *u.PasswordHash))
	assert.False(t, u.WajibGantiPassword, "pemilik memilih sendiri sandinya: tidak perlu diganti lagi")
	assert.NotNil(t, u.PasswordDiubahPada)
	assert.Empty(t, f.repo.undanganAktif(h.User.ID))

	t.Run("audit dengan aktor pemilik akun", func(t *testing.T) {
		entri := entriAudit(f, "AKTIVASI_AKUN")
		require.Len(t, entri, 1)
		assert.Equal(t, h.User.ID, entri[0].AktorID)
		assert.Equal(t, h.User.ID, entri[0].EntitasID)
		assert.Equal(t, map[string]any{"status": "MENUNGGU"}, entri[0].Sebelum)
		assert.Equal(t, map[string]any{"status": "AKTIF"}, entri[0].Sesudah)
		serial, err := json.Marshal(entri[0])
		require.NoError(t, err)
		assert.NotContains(t, string(serial), sandiAktivasi)
		assert.NotContains(t, string(serial), raw)
	})
	t.Run("sesudahnya masuk dengan kata sandi itu", func(t *testing.T) {
		res, err := f.uc.Login(context.Background(), "budi@garda.test", sandiAktivasi, "10.0.0.1")
		require.NoError(t, err)
		assert.NotEmpty(t, res.AccessToken)
		assert.False(t, res.User.WajibGantiPassword)
	})
}

func TestAktifkan_TautanHanyaSekaliPakai(t *testing.T) {
	f := newUndanganFixture(t)
	f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)
	require.NoError(t, f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi))

	err := f.uc.AktifkanAkun(context.Background(), raw, "LainLagi#2026Kuat")

	require.ErrorIs(t, err, ErrUndanganInvalid)
	res, errMasuk := f.uc.Login(context.Background(), "budi@garda.test", sandiAktivasi, "10.0.0.1")
	require.NoError(t, errMasuk, "kata sandi pertama tidak tertimpa")
	assert.NotEmpty(t, res.AccessToken)
}

func TestAktifkan_SerentakHanyaSatuYangBerhasil(t *testing.T) {
	f := newUndanganFixture(t)
	f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)

	const jumlah = 8
	hasil := make([]error, jumlah)
	var wg sync.WaitGroup
	for i := range jumlah {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i] = f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi)
		}()
	}
	wg.Wait()

	var berhasil int
	for _, err := range hasil {
		if err == nil {
			berhasil++
		} else {
			assert.ErrorIs(t, err, ErrUndanganInvalid)
		}
	}
	assert.Equal(t, 1, berhasil)
	assert.Len(t, entriAudit(f, "AKTIVASI_AKUN"), 1)
}

func TestAktifkan_KataSandiLemahTidakMenghabiskanTautan(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)

	kasus := map[string]string{
		"terlalu pendek":        "Ab1!",
		"tanpa kapital":         "rahasia#2026kuat",
		"tanpa angka":           "Rahasia#Kuatsekali",
		"tanpa karakter khusus": "Rahasia2026Kuat",
		"terlalu umum":          "password",
		"melebihi batas bcrypt": "Aa1!" + strings.Repeat("x", 80),
	}
	for nama, sandi := range kasus {
		t.Run(nama, func(t *testing.T) {
			err := f.uc.AktifkanAkun(context.Background(), raw, sandi)
			require.ErrorIs(t, err, ErrWeakPassword)
		})
	}

	assert.Equal(t, "MENUNGGU", f.repo.pengguna(h.User.ID).Status)
	require.NoError(t, f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi), "tautan masih berlaku")
}

func TestAktifkan_Kedaluwarsa(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)

	t.Run("tepat sebelum batas masih berlaku", func(t *testing.T) {
		*f.now = awalUji.Add(72*time.Hour - time.Second)
		// Diperiksa tanpa mengonsumsi: kata sandi lemah ditolak setelah lolos pemeriksaan kedaluwarsa.
		require.ErrorIs(t, f.uc.AktifkanAkun(context.Background(), raw, "lemah"), ErrWeakPassword)
	})
	t.Run("pada batas sudah kedaluwarsa", func(t *testing.T) {
		*f.now = awalUji.Add(72 * time.Hour)
		require.ErrorIs(t, f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi), ErrUndanganKedaluwarsa)
	})
	t.Run("lama sesudahnya", func(t *testing.T) {
		*f.now = awalUji.Add(30 * 24 * time.Hour)
		require.ErrorIs(t, f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi), ErrUndanganKedaluwarsa)
	})
	assert.Equal(t, "MENUNGGU", f.repo.pengguna(h.User.ID).Status)
	assert.Nil(t, f.repo.pengguna(h.User.ID).PasswordHash)
}

func TestAktifkan_TokenTakDikenal(t *testing.T) {
	f := newUndanganFixture(t)
	f.undang(masukanUndang())

	for nama, token := range map[string]string{
		"kosong":           "",
		"acak":             "bukan-token-yang-pernah-ada",
		"hash disajikan":   sha256Hex("x"),
		"terlalu panjang":  strings.Repeat("a", maksTokenKar+1),
		"token orang lain": "tkn-lain",
	} {
		t.Run(nama, func(t *testing.T) {
			require.ErrorIs(t, f.uc.AktifkanAkun(context.Background(), token, sandiAktivasi), ErrUndanganInvalid)
		})
	}
}

func TestAktifkan_HashTokenYangTersimpanBukanTokenAktivasi(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	f.tokenEmail("budi@garda.test", 1)
	hash := f.repo.undanganAktif(h.User.ID)[0].TokenHash

	err := f.uc.AktifkanAkun(context.Background(), hash, sandiAktivasi)

	require.ErrorIs(t, err, ErrUndanganInvalid, "pembocor basis data tidak bisa mengaktifkan akun dengan hash")
}

func TestAktifkan_AkunBukanLagiMenunggu(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)
	// Admin menonaktifkan akun (mis. lewat basis data) sebelum tautan dipakai.
	f.repo.mu.Lock()
	f.repo.users[h.User.ID].Status = "NONAKTIF"
	f.repo.mu.Unlock()

	err := f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi)

	require.ErrorIs(t, err, ErrUndanganInvalid)
	assert.Equal(t, "NONAKTIF", f.repo.pengguna(h.User.ID).Status)
	assert.Nil(t, f.repo.pengguna(h.User.ID).PasswordHash)
}

func TestAktifkan_EmailAkunBerubahMenghanguskanTautanLama(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	lama := f.tokenEmail("budi@garda.test", 1)
	baru := "budi.baru@garda.test"
	_, err := f.uc.UpdateUser(context.Background(), h.User.ID, UserPatch{Email: &baru}, adminUndangID)
	require.NoError(t, err)

	err = f.uc.AktifkanAkun(context.Background(), lama, sandiAktivasi)

	require.ErrorIs(t, err, ErrUndanganInvalid, "pemilik alamat lama tidak boleh menguasai akun milik alamat baru")
	assert.Equal(t, "MENUNGGU", f.repo.pengguna(h.User.ID).Status)

	t.Run("undangan baru ke alamat baru berlaku", func(t *testing.T) {
		_, err := f.uc.KirimUlangUndangan(context.Background(), h.User.ID, adminUndangID)
		require.NoError(t, err)
		token := f.tokenEmail(baru, 1)
		require.NoError(t, f.uc.AktifkanAkun(context.Background(), token, sandiAktivasi))
	})
}

func TestAktifkan_GalatPenyimpananBukanTautanTidakSah(t *testing.T) {
	f := newUndanganFixture(t)
	f.undang(masukanUndang())
	raw := f.tokenEmail("budi@garda.test", 1)
	f.repo.errAktifkan = errors.New("koneksi putus")

	err := f.uc.AktifkanAkun(context.Background(), raw, sandiAktivasi)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUndanganInvalid, "galat infrastruktur tidak boleh menyamar sebagai tautan tidak sah")
	assert.Empty(t, entriAudit(f, "AKTIVASI_AKUN"))
}

func TestAktifkan_GalatMembacaUndanganDilaporkan(t *testing.T) {
	f := newUndanganFixture(t)
	f.uc.repo = &galatBacaUndangan{memUndangan: f.repo}

	err := f.uc.AktifkanAkun(context.Background(), "token", sandiAktivasi)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUndanganInvalid)
}

type galatBacaUndangan struct{ *memUndangan }

func (galatBacaUndangan) GetUndangan(context.Context, string) (*domain.Undangan, error) {
	return nil, errors.New("koneksi putus")
}

// ── Akun menunggu di jalur lain ───────────────────────────────────────

func TestLogin_AkunMenunggu_SamaDenganEmailTakTerdaftar(t *testing.T) {
	f := newUndanganFixture(t)
	f.undang(masukanUndang())

	ulang := func(emailUji string) []error {
		var galat []error
		for range 6 {
			_, err := f.uc.Login(context.Background(), emailUji, "Rahasia#2026Kuat", "10.0.0.1")
			galat = append(galat, err)
		}
		return galat
	}
	menunggu := ulang("budi@garda.test")
	takAda := ulang("tidak.ada@garda.test")

	for i := range menunggu {
		require.Error(t, menunggu[i])
		require.Error(t, takAda[i])
		assert.Equal(t, errors.Is(takAda[i], ErrAccountLocked), errors.Is(menunggu[i], ErrAccountLocked), "percobaan ke-%d", i+1)
		assert.Equal(t, errors.Is(takAda[i], ErrInvalidCredentials), errors.Is(menunggu[i], ErrInvalidCredentials), "percobaan ke-%d", i+1)
		assert.False(t, errors.Is(menunggu[i], ErrAccountInactive), "tidak membedakan akun yang menunggu dari yang tidak ada")
	}
	assert.ErrorIs(t, menunggu[0], ErrInvalidCredentials)
	assert.ErrorIs(t, menunggu[5], ErrAccountLocked, "hitungan lockout per email berlaku juga untuk akun menunggu")
}

func TestForgotPassword_AkunMenunggu_TidakMengirimEmailDanTidakMengaktifkan(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	f.smtp.Tunggu(t, "budi@garda.test", 1)

	require.NoError(t, f.uc.ForgotPassword(context.Background(), "budi@garda.test"))
	time.Sleep(100 * time.Millisecond) // pengiriman email reset berjalan di latar belakang

	assert.Len(t, f.smtp.Untuk("budi@garda.test"), 1, "hanya email undangan, tanpa email atur ulang")
	assert.Equal(t, "MENUNGGU", f.repo.pengguna(h.User.ID).Status)
}

func TestUpdateUser_AkunMenunggu(t *testing.T) {
	f := newUndanganFixture(t)
	h := f.undang(masukanUndang())
	ptr := func(s string) *string { return &s }

	t.Run("status tidak bisa diubah dari PATCH", func(t *testing.T) {
		for _, status := range []string{"AKTIF", "NONAKTIF"} {
			_, err := f.uc.UpdateUser(context.Background(), h.User.ID, UserPatch{Status: ptr(status)}, adminUndangID)
			require.ErrorIs(t, err, ErrAkunMenunggu, status)
		}
		assert.Equal(t, "MENUNGGU", f.repo.pengguna(h.User.ID).Status)
	})
	t.Run("MENUNGGU diisi ulang tidak mengubah apa pun", func(t *testing.T) {
		f.repo.On("RevokeAllUserRefreshTokens", mock.Anything, h.User.ID).Return(nil) // status selain AKTIF selalu mengakhiri sesi
		_, err := f.uc.UpdateUser(context.Background(), h.User.ID, UserPatch{Status: ptr("MENUNGGU"), Nama: ptr("Budi S.")}, adminUndangID)
		require.NoError(t, err)
		assert.Equal(t, "Budi S.", f.repo.pengguna(h.User.ID).Nama)
	})
	t.Run("MENUNGGU tidak bisa dipasang ke akun aktif", func(t *testing.T) {
		_, err := f.uc.UpdateUser(context.Background(), adminUndangID, UserPatch{Status: ptr("MENUNGGU")}, adminUndangID)
		var validasi *GalatValidasi
		require.ErrorAs(t, err, &validasi)
		assert.Equal(t, "AKTIF", f.repo.pengguna(adminUndangID).Status)
	})
	t.Run("reset kata sandi oleh admin ditolak", func(t *testing.T) {
		err := f.uc.ResetPasswordAdmin(context.Background(), h.User.ID, sandiAktivasi, adminUndangID)
		require.ErrorIs(t, err, ErrAkunMenunggu)
		assert.Nil(t, f.repo.pengguna(h.User.ID).PasswordHash)
	})
	t.Run("profil dan peran tetap bisa diubah", func(t *testing.T) {
		peran := "OPS_AGREGATOR"
		_, err := f.uc.UpdateUser(context.Background(), h.User.ID, UserPatch{Peran: &peran, Lembaga: ptr("BPS")}, adminUndangID)
		require.NoError(t, err)
		assert.Equal(t, "OPS_AGREGATOR", f.repo.pengguna(h.User.ID).Peran)
	})
}

// ── Pembantu ──────────────────────────────────────────────────────────

func TestEmailValid(t *testing.T) {
	sah := []string{"a@b.id", "budi.santoso@fmipa.usk.ac.id", "a+tag@garda.test", "x_y-z@sub.domain.co.id", "e2e.1730000000000.1@gardainflasi.id"}
	for _, e := range sah {
		assert.True(t, emailValid(e), e)
	}
	tidak := []string{
		"", "a", "a@", "@b.id", "a@b", "a@@b.id", "a b@c.id", "a@b c.id", "a@b..id", "a@.id", "a@b.", "a@-b.id", "a@b-.id",
		"<a@b.id>", "a@b.id>", "\"a\"@b.id", "a,b@c.id", "a;b@c.id", "a\n@b.id", "a@b.id\r\n", "(c)a@b.id", "a@[1.2.3.4]",
		strings.Repeat("a", 65) + "@b.id", "a@" + strings.Repeat("b", 250) + ".id",
	}
	for _, e := range tidak {
		assert.False(t, emailValid(e), "%q", e)
	}
}

func TestTokenUndanganBaru(t *testing.T) {
	terlihat := map[string]bool{}
	for range 200 {
		raw, hash, err := tokenUndanganBaru()
		require.NoError(t, err)
		assert.Len(t, raw, 43, "32 byte base64url tanpa padding")
		assert.Regexp(t, `^[A-Za-z0-9_-]{43}$`, raw, "aman di URL tanpa escape")
		assert.Equal(t, sha256Hex(raw), hash)
		assert.False(t, terlihat[raw], "token tidak boleh berulang")
		terlihat[raw] = true
	}
}

func TestIDPenggunaBaru(t *testing.T) {
	id, err := idPenggunaBaru()
	require.NoError(t, err)
	assert.True(t, idx.ValidUUID(id), "UUID kanonik huruf kecil: %q", id)
	assert.Equal(t, strings.ToLower(id), id)
	lain, _ := idPenggunaBaru()
	assert.NotEqual(t, id, lain)
}

func TestPeriksaPerubahanStatus(t *testing.T) {
	menunggu := &domain.Pengguna{Status: "MENUNGGU"}
	aktif := &domain.Pengguna{Status: "AKTIF"}
	s := func(v string) *string { return &v }

	assert.NoError(t, periksaPerubahanStatus(aktif, nil))
	assert.NoError(t, periksaPerubahanStatus(aktif, s("NONAKTIF")))
	assert.NoError(t, periksaPerubahanStatus(aktif, s("AKTIF")))
	assert.NoError(t, periksaPerubahanStatus(menunggu, s("MENUNGGU")))
	assert.ErrorIs(t, periksaPerubahanStatus(menunggu, s("AKTIF")), ErrAkunMenunggu)
	assert.ErrorIs(t, periksaPerubahanStatus(menunggu, s("NONAKTIF")), ErrAkunMenunggu)
	var validasi *GalatValidasi
	assert.ErrorAs(t, periksaPerubahanStatus(aktif, s("MENUNGGU")), &validasi)
}

func TestDuplikatKunciGORMDikenali(t *testing.T) {
	// UndangPengguna memetakan gorm.ErrDuplicatedKey ke ErrEmailExists, juga bila dibungkus.
	f := newUndanganFixture(t)
	f.repo.errBuat = errors.Join(errors.New("insert"), gorm.ErrDuplicatedKey)

	_, err := f.uc.UndangPengguna(context.Background(), masukanUndang(), adminUndangID)

	require.ErrorIs(t, err, ErrEmailExists)
}
