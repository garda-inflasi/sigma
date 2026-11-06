package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
)

func bacaCSVEkspor(t *testing.T, berkas []byte) [][]string {
	t.Helper()
	require.True(t, bytes.HasPrefix(berkas, []byte(bomUTF8)), "berkas diawali BOM UTF-8")
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(berkas, []byte(bomUTF8))))
	r.Comma = ';'
	rekaman, err := r.ReadAll()
	require.NoError(t, err)
	return rekaman
}

func penggunaEkspor(id, nama, mail string) domain.Pengguna {
	kab := "1171"
	masuk := time.Date(2026, 11, 5, 8, 30, 0, 0, time.FixedZone("WIB", 7*3600))
	return domain.Pengguna{
		ID: id, Nama: nama, Email: mail, Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA USK",
		KabupatenKode: &kab, Status: "AKTIF", MfaAktif: true, TerakhirMasuk: &masuk,
		DibuatPada: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestBuatCSVPengguna_FormatDanKolom(t *testing.T) {
	u := penggunaEkspor("u-1", "Budi Santoso", "budi@garda.test")
	tanpaKab := penggunaEkspor("u-2", "Siti", "siti@garda.test")
	tanpaKab.KabupatenKode, tanpaKab.TerakhirMasuk, tanpaKab.MfaAktif, tanpaKab.Status = nil, nil, false, "MENUNGGU"

	berkas, err := BuatCSVPengguna([]domain.Pengguna{u, tanpaKab})

	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(berkas, []byte(bomUTF8)))
	rekaman := bacaCSVEkspor(t, berkas)
	require.Len(t, rekaman, 3)
	assert.Equal(t, KolomEkspor, rekaman[0])
	assert.Equal(t, []string{"u-1", "Budi Santoso", "budi@garda.test", "OPS_MAHASISWA", "OPERASIONAL", "FMIPA USK", "1171", "AKTIF", "true", "2026-11-05T01:30:00Z", "2026-01-02T03:04:05Z"}, rekaman[1],
		"waktu dalam UTC RFC 3339")
	assert.Equal(t, []string{"u-2", "Siti", "siti@garda.test", "OPS_MAHASISWA", "OPERASIONAL", "FMIPA USK", "", "MENUNGGU", "false", "", "2026-01-02T03:04:05Z"}, rekaman[2])
}

func TestBuatCSVPengguna_TanpaRahasia(t *testing.T) {
	u := penggunaEkspor("u-1", "Budi", "budi@garda.test")
	hash, rahasia := "$2a$12$hashrahasiabcrypt", "v1:RAHASIA-MFA-TERSEGEL"
	u.PasswordHash, u.MfaSecret = &hash, &rahasia
	sso, subjek := "google", "sub-12345"
	u.SSOProvider, u.SSOSubject = &sso, &subjek

	berkas, err := BuatCSVPengguna([]domain.Pengguna{u})

	require.NoError(t, err)
	teks := string(berkas)
	for _, terlarang := range []string{hash, rahasia, subjek, "password", "mfa_secret", "sso"} {
		assert.NotContains(t, teks, terlarang)
	}
}

func TestBuatCSVPengguna_InjeksiRumusDinetralkan(t *testing.T) {
	kasus := map[string]string{
		"=HYPERLINK(\"http://x\",\"klik\")": "'=HYPERLINK(\"http://x\",\"klik\")",
		"+62811":                            "'+62811",
		"-2+3":                              "'-2+3",
		"@SUM(A1:A9)":                       "'@SUM(A1:A9)",
		"\tTab":                             "'\tTab",
		"\rCR":                              "'\rCR",
		"Aman=1":                            "Aman=1",
		"":                                  "",
	}
	for masukan, harapan := range kasus {
		u := penggunaEkspor("u-1", masukan, "budi@garda.test")
		u.Lembaga = masukan

		berkas, err := BuatCSVPengguna([]domain.Pengguna{u})

		require.NoError(t, err)
		rekaman := bacaCSVEkspor(t, berkas)
		assert.Equal(t, harapan, rekaman[1][1], "nama %q", masukan)
		assert.Equal(t, harapan, rekaman[1][5], "lembaga %q", masukan)
	}
	// id, email, dan peran juga melewati penetral yang sama.
	u := penggunaEkspor("=id", "A", "-x@garda.test")
	berkas, err := BuatCSVPengguna([]domain.Pengguna{u})
	require.NoError(t, err)
	rekaman := bacaCSVEkspor(t, berkas)
	assert.Equal(t, "'=id", rekaman[1][0])
	assert.Equal(t, "'-x@garda.test", rekaman[1][2])
}

func TestBuatCSVPengguna_SelDenganPemisahKutipDanBarisBaruDikutip(t *testing.T) {
	u := penggunaEkspor("u-1", "Santoso; \"Budi\"\nKedua", "budi@garda.test")

	berkas, err := BuatCSVPengguna([]domain.Pengguna{u})

	require.NoError(t, err)
	assert.Equal(t, "Santoso; \"Budi\"\nKedua", bacaCSVEkspor(t, berkas)[1][1], "isi sel utuh setelah dibaca balik")
	assert.Contains(t, string(berkas), "\"Santoso; \"\"Budi\"\"\nKedua\"", "sel dikutip dengan benar")
}

func TestCSVAman(t *testing.T) {
	for _, s := range []string{"", "a", "a=b", "1+1", " =a", "'=a"} {
		assert.Equal(t, s, csvAman(s), "%q aman apa adanya", s)
	}
	for _, s := range []string{"=a", "+a", "-a", "@a", "\ta", "\ra"} {
		assert.Equal(t, "'"+s, csvAman(s), "%q dinetralkan", s)
	}
}

func TestEksporPengguna_MenyerahkanBerkasDanMencatatAudit(t *testing.T) {
	f := newUndanganFixture(t)
	f.undang(masukanUndang())

	hasil, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{}, adminUndangID)

	require.NoError(t, err)
	assert.Equal(t, 2, hasil.Baris, "admin dan akun yang diundang (MENUNGGU)")
	assert.False(t, hasil.Terpotong)
	rekaman := bacaCSVEkspor(t, hasil.CSV)
	statusIdx := 7
	var status []string
	for _, r := range rekaman[1:] {
		status = append(status, r[statusIdx])
	}
	assert.ElementsMatch(t, []string{"AKTIF", "MENUNGGU"}, status)

	entri := entriAudit(f, "EKSPOR_DATA")
	require.Len(t, entri, 1)
	assert.Equal(t, adminUndangID, entri[0].AktorID)
	assert.Equal(t, "pengguna", entri[0].Entitas)
	assert.Equal(t, "ekspor", entri[0].EntitasID)
	assert.Equal(t, audit.HasilBerhasil, entri[0].Hasil)
	assert.Equal(t, 2, entri[0].Metadata["baris"])
	assert.Equal(t, false, entri[0].Metadata["terpotong"])
}

func TestEksporPengguna_FilterCariDanIdTerpilih(t *testing.T) {
	a := &domain.Pengguna{ID: "u-a", Nama: "Ani Rahma", Email: "ani@garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "FMIPA", Status: "AKTIF"}
	b := &domain.Pengguna{ID: "u-b", Nama: "Budi", Email: "budi@garda.test", Peran: "OPS_AGREGATOR", Lapis: "OPERASIONAL", Lembaga: "BPS", Status: "AKTIF"}
	f := newUndanganFixture(t, a, b)
	idKolom := func(h *HasilEkspor) []string {
		var id []string
		for _, r := range bacaCSVEkspor(t, h.CSV)[1:] {
			id = append(id, r[0])
		}
		return id
	}

	t.Run("pencarian", func(t *testing.T) {
		h, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{Search: "ani"}, adminUndangID)
		require.NoError(t, err)
		assert.Equal(t, []string{"u-a"}, idKolom(h))
	})
	t.Run("id terpilih", func(t *testing.T) {
		h, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{IDs: []string{"u-b", "tidak-ada"}}, adminUndangID)
		require.NoError(t, err)
		assert.Equal(t, []string{"u-b"}, idKolom(h), "id yang tidak ada diabaikan, bukan galat")
		entri := entriAudit(f, "EKSPOR_DATA")
		assert.Equal(t, 2, entri[len(entri)-1].Metadata["jumlah_id"])
	})
	t.Run("pencarian dan id digabung (irisan)", func(t *testing.T) {
		h, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{Search: "ani", IDs: []string{"u-b"}}, adminUndangID)
		require.NoError(t, err)
		assert.Empty(t, idKolom(h))
		assert.Equal(t, 0, h.Baris)
	})
}

func TestEksporPengguna_PemilihanTerlaluBanyakDitolak(t *testing.T) {
	f := newUndanganFixture(t)
	ids := make([]string, EksporMaksID+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("u-%d", i)
	}

	_, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{IDs: ids}, adminUndangID)

	var validasi *GalatValidasi
	require.ErrorAs(t, err, &validasi)
	assert.Empty(t, f.audit.Entries())
}

func TestEksporPengguna_TerpotongPadaBatasBaris(t *testing.T) {
	var users []*domain.Pengguna
	for i := range 3 {
		u := penggunaEkspor(fmt.Sprintf("u-%d", i), fmt.Sprintf("Nama %d", i), fmt.Sprintf("u%d@garda.test", i))
		u.DibuatPada = time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC)
		users = append(users, &u)
	}
	f := newUndanganFixture(t, users...)
	lama := eksporMaksBaris
	eksporMaksBaris = 2
	t.Cleanup(func() { eksporMaksBaris = lama })

	h, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{}, adminUndangID)

	require.NoError(t, err)
	assert.Equal(t, 2, h.Baris)
	assert.True(t, h.Terpotong)
	assert.Equal(t, true, entriAudit(f, "EKSPOR_DATA")[0].Metadata["terpotong"])
	assert.Len(t, bacaCSVEkspor(t, h.CSV), 3)
}

func TestEksporPengguna_AuditGagalBerkasTidakDiserahkan(t *testing.T) {
	f := newUndanganFixture(t)
	f.audit.Err = errors.New("audit_log penuh")

	h, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{}, adminUndangID)

	require.Error(t, err)
	assert.Nil(t, h, "data pribadi tidak keluar bila ekspor tidak tercatat")
}

func TestEksporPengguna_GalatBasisDataDilaporkanTanpaAudit(t *testing.T) {
	f := newUndanganFixture(t)
	f.uc.repo = &galatEkspor{memUndangan: f.repo}

	h, err := f.uc.EksporPengguna(context.Background(), domain.FilterPengguna{}, adminUndangID)

	require.Error(t, err)
	assert.Nil(t, h)
	assert.Empty(t, f.audit.Entries(), "ekspor yang gagal tidak dicatat sebagai ekspor")
}

type galatEkspor struct{ *memUndangan }

func (galatEkspor) ListUsersUntukEkspor(context.Context, domain.FilterPengguna, int) ([]domain.Pengguna, bool, error) {
	return nil, false, errors.New("koneksi putus")
}

func TestEksporPengguna_NamaKolomDiGaransi(t *testing.T) {
	assert.Equal(t, "id;nama;email;peran;lapis;lembaga;kabupaten_kode;status;mfa_aktif;terakhir_masuk;dibuat_pada", strings.Join(KolomEkspor, ";"))
}
