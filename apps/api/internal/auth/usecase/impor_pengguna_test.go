package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/email"
)

const (
	headerImpor = "nama,email,peran,lembaga,kabupaten_kode"
	// bomUTF8 adalah penanda urutan byte UTF-8 yang ditambahkan Excel pada "CSV UTF-8".
	bomUTF8 = "\xef\xbb\xbf"
)

func (f *undanganFixture) impor(csv string, dryRun bool) (*HasilImpor, error) {
	f.t.Helper()
	return f.uc.ImporPengguna(context.Background(), csv, dryRun, adminUndangID)
}

func (f *undanganFixture) imporSah(csv string, dryRun bool) *HasilImpor {
	f.t.Helper()
	h, err := f.impor(csv, dryRun)
	require.NoError(f.t, err)
	return h
}

func barisPesan(h *HasilImpor, baris int) []string {
	for _, b := range h.Baris {
		if b.Baris == baris {
			return b.Galat
		}
	}
	return []string{"(baris tidak ada)"}
}

// ── Penguraian berkas ─────────────────────────────────────────────────

func TestUraiCSVImpor_FormatYangDiterima(t *testing.T) {
	baris1 := []string{"Budi Santoso", "budi@garda.test", "OPS_MAHASISWA", "FMIPA USK", "1171"}

	kasus := map[string]string{
		"koma":                          headerImpor + "\nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n",
		"titik koma":                    "nama;email;peran;lembaga;kabupaten_kode\nBudi Santoso;budi@garda.test;OPS_MAHASISWA;FMIPA USK;1171\n",
		"BOM dan koma":                  bomUTF8 + headerImpor + "\nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n",
		"BOM dan titik koma":            bomUTF8 + "nama;email;peran;lembaga;kabupaten_kode\r\nBudi Santoso;budi@garda.test;OPS_MAHASISWA;FMIPA USK;1171\r\n",
		"CRLF":                          strings.ReplaceAll(headerImpor+"\nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n", "\n", "\r\n"),
		"tanpa baris akhir":             headerImpor + "\nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171",
		"header huruf besar dan spasi":  " Nama , EMAIL ,Peran,LEMBAGA, Kabupaten_Kode \nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n",
		"urutan kolom bebas":            "email,lembaga,nama,kabupaten_kode,peran\nbudi@garda.test,FMIPA USK,Budi Santoso,1171,OPS_MAHASISWA\n",
		"sel berkutip":                  headerImpor + "\n\"Budi Santoso\",\"budi@garda.test\",\"OPS_MAHASISWA\",\"FMIPA USK\",\"1171\"\n",
		"spasi di sekitar sel":          headerImpor + "\n  Budi Santoso  ,  budi@garda.test ,OPS_MAHASISWA, FMIPA USK ,  1171\n",
		"baris kosong dan tanpa isi":    headerImpor + "\n\n,,,,\n   ,  ,  ,,\nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n\n",
		"sel header kosong di belakang": headerImpor + ",,\nBudi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171,,\n",
	}
	for nama, csv := range kasus {
		t.Run(nama, func(t *testing.T) {
			baris, err := uraiCSVImpor(csv)
			require.NoError(t, err)
			require.Len(t, baris, 1)
			sel := baris[0].sel
			assert.Equal(t, baris1, []string{sel["nama"], sel["email"], sel["peran"], sel["lembaga"], sel["kabupaten_kode"]})
			assert.False(t, baris[0].kelebihan)
		})
	}
}

func TestUraiCSVImpor_KolomKabupatenBolehTidakAda(t *testing.T) {
	baris, err := uraiCSVImpor("nama,email,peran,lembaga\nBudi,budi@garda.test,OPS_MAHASISWA,FMIPA\n")

	require.NoError(t, err)
	require.Len(t, baris, 1)
	assert.Equal(t, "", baris[0].sel["kabupaten_kode"])
}

func TestUraiCSVImpor_SelBerkutipMemuatPemisahDanBarisBaru(t *testing.T) {
	csv := headerImpor + "\n" +
		"\"Santoso, Budi\",budi@garda.test,OPS_MAHASISWA,\"FMIPA \"\"USK\"\"\n\nBanda Aceh\",1171\n" +
		"Siti;Rahma,siti@garda.test,OPS_AGREGATOR,BPS,\n"

	baris, err := uraiCSVImpor(csv)

	require.NoError(t, err)
	require.Len(t, baris, 2)
	assert.Equal(t, "Santoso, Budi", baris[0].sel["nama"])
	assert.Equal(t, "FMIPA \"USK\"\n\nBanda Aceh", baris[0].sel["lembaga"])
	assert.Equal(t, "Siti;Rahma", baris[1].sel["nama"], "titik koma di berkas berkoma adalah isi sel")
	assert.Equal(t, 2, baris[0].nomor)
	assert.Equal(t, 5, baris[1].nomor, "nomor baris mengikuti baris fisik di berkas, termasuk sel berbaris-baru")
}

func TestUraiCSVImpor_PemisahDariHeader(t *testing.T) {
	assert.Equal(t, ',', deteksiPemisah(headerImpor+"\n"))
	assert.Equal(t, ';', deteksiPemisah("nama;email;peran;lembaga;kabupaten_kode\n"))
	assert.Equal(t, ',', deteksiPemisah("nama\n"), "satu kolom: koma")
	assert.Equal(t, ',', deteksiPemisah(""))
	assert.Equal(t, ';', deteksiPemisah("nama;email;peran;lembaga\r\nA,B;C"), "yang menentukan hanya header")
}

func TestUraiCSVImpor_GalatBerkas(t *testing.T) {
	kasus := map[string]struct {
		csv   string
		pesan string
	}{
		"kosong":                  {"", "Berkas kosong"},
		"hanya spasi":             {"  \n \n", "Berkas kosong"},
		"hanya BOM":               {bomUTF8, "Berkas kosong"},
		"hanya header":            {headerImpor + "\n", "tidak ada baris data"},
		"hanya header dan kosong": {headerImpor + "\n,,,,\n\n", "tidak ada baris data"},
		"kolom tak dikenal":       {"nama,email,peran,lembaga,telepon\nA,a@b.id,OPS_MAHASISWA,X,08\n", "Kolom tidak dikenal di header: telepon"},
		"salah ketik kolom":       {"nama,email,peran,lembaga,kabupaten\nA,a@b.id,OPS_MAHASISWA,X,1171\n", "Kolom tidak dikenal di header: kabupaten"},
		"kolom wajib hilang":      {"nama,email,lembaga\nA,a@b.id,X\n", "Kolom wajib tidak ada di header: peran"},
		"header baris data":       {"Budi,budi@garda.test,OPS_MAHASISWA,FMIPA,1171\n", "Kolom tidak dikenal"},
		"kolom ganda":             {"nama,email,peran,lembaga,email\nA,a@b.id,OPS_MAHASISWA,X,c@d.id\n", "muncul lebih dari sekali"},
		"bukan UTF-8":             {headerImpor + "\nBudi \xff\xfe,b@c.id,OPS_MAHASISWA,X,\n", "UTF-8"},
		"karakter pengganti":      {headerImpor + "\nBudi �,b@c.id,OPS_MAHASISWA,X,\n", "UTF-8"},
		"byte NUL":                {headerImpor + "\nBudi\x00,b@c.id,OPS_MAHASISWA,X,\n", "UTF-8"},
		"kutip tak berpasangan":   {headerImpor + "\n\"Budi,b@c.id,OPS_MAHASISWA,X,\n", "kutip"},
		"kutip di tengah sel":     {headerImpor + "\nBu\"di,b@c.id,OPS_MAHASISWA,X,\n", "baris 2"},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			_, err := uraiCSVImpor(k.csv)
			var validasi *GalatValidasi
			require.ErrorAs(t, err, &validasi, "galat berkas adalah masukan yang salah")
			assert.Contains(t, validasi.Pesan, k.pesan)
		})
	}
}

func TestUraiCSVImpor_Batas(t *testing.T) {
	bangun := func(n int) string {
		var sb strings.Builder
		sb.WriteString(headerImpor + "\n")
		for i := range n {
			fmt.Fprintf(&sb, "Nama %d,u%d@garda.test,OPS_MAHASISWA,FMIPA,\n", i, i)
		}
		return sb.String()
	}

	t.Run("tepat 500 baris diterima", func(t *testing.T) {
		baris, err := uraiCSVImpor(bangun(ImporMaksBaris))
		require.NoError(t, err)
		assert.Len(t, baris, ImporMaksBaris)
	})
	t.Run("501 baris ditolak seluruhnya", func(t *testing.T) {
		_, err := uraiCSVImpor(bangun(ImporMaksBaris + 1))
		var validasi *GalatValidasi
		require.ErrorAs(t, err, &validasi)
		assert.Contains(t, validasi.Pesan, "lebih dari 500 baris")
	})
	t.Run("baris kosong tidak dihitung", func(t *testing.T) {
		csv := bangun(ImporMaksBaris) + strings.Repeat(",,,,\n\n", 50)
		baris, err := uraiCSVImpor(csv)
		require.NoError(t, err)
		assert.Len(t, baris, ImporMaksBaris)
	})
	t.Run("melebihi ukuran berkas", func(t *testing.T) {
		_, err := uraiCSVImpor(headerImpor + "\n" + strings.Repeat("x", ImporMaksByte))
		require.ErrorIs(t, err, ErrImporTerlaluBesar)
	})
	t.Run("tepat di ukuran maksimum tidak ditolak karena ukuran", func(t *testing.T) {
		isi := headerImpor + "\nA,a@b.id,OPS_MAHASISWA,"
		csv := isi + strings.Repeat("x", ImporMaksByte-len(isi)-1) + "\n"
		_, err := uraiCSVImpor(csv)
		assert.NotErrorIs(t, err, ErrImporTerlaluBesar)
	})
	t.Run("BOM tidak dihitung ke batas ukuran", func(t *testing.T) {
		isi := headerImpor + "\nA,a@b.id,OPS_MAHASISWA,"
		csv := bomUTF8 + isi + strings.Repeat("x", ImporMaksByte-len(isi)-1) + "\n"
		_, err := uraiCSVImpor(csv)
		assert.NotErrorIs(t, err, ErrImporTerlaluBesar)
	})
}

func TestUraiCSVImpor_SelBerlebihanDitandai(t *testing.T) {
	baris, err := uraiCSVImpor(headerImpor + "\nBudi,b@c.id,OPS_MAHASISWA,X,1171,sisa\nSiti,s@c.id,OPS_MAHASISWA,X,1171,\n")

	require.NoError(t, err)
	require.Len(t, baris, 2)
	assert.True(t, baris[0].kelebihan)
	assert.False(t, baris[1].kelebihan, "sel kosong di luar header tidak dihitung")
}

// ── Pemeriksaan isi ───────────────────────────────────────────────────

func TestImpor_DryRun_MelaporkanSetiapKelasGalatTanpaMenulis(t *testing.T) {
	ada := &domain.Pengguna{ID: uid("u-1"), Nama: "Ada", Email: "Sudah@Garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "X", Status: "AKTIF"}
	f := newUndanganFixture(t, ada)
	csv := headerImpor + "\n" +
		"Budi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n" + // 2 sah
		",tanpa.nama@garda.test,OPS_MAHASISWA,FMIPA,\n" + // 3 nama kosong
		"Siti,bukan-email,OPS_MAHASISWA,FMIPA,\n" + // 4 email tidak valid
		"Agus,agus@garda.test,DEWA,FMIPA,\n" + // 5 peran tak dikenal
		"Rina,rina@garda.test,OPS_MAHASISWA,,\n" + // 6 lembaga kosong
		"Dedi,dedi@garda.test,OPS_MAHASISWA,FMIPA,9999\n" + // 7 kabupaten tak dikenal
		"Eko,eko@garda.test,OPS_MAHASISWA,FMIPA,11\n" + // 8 kabupaten salah format
		"Budi Dua,BUDI@garda.test,OPS_MAHASISWA,FMIPA,\n" + // 9 duplikat dalam berkas (beda huruf)
		"Sudah,sudah@garda.test,OPS_MAHASISWA,FMIPA,\n" + // 10 sudah terdaftar
		"Rumus,=cmd@garda.test,OPS_MAHASISWA,FMIPA,\n" + // 11 email diawali rumus
		"=SUM(A1),rumus@garda.test,OPS_MAHASISWA,FMIPA,\n" + // 12 nama diawali rumus
		",,,,x\n" + // 13 banyak galat sekaligus
		"Fani,fani@garda.test,OPS_KOORDINATOR,BPS,1108\n" // 14 sah

	h := f.imporSah(csv, true)

	assert.True(t, h.DryRun)
	assert.Equal(t, 13, h.Total)
	assert.Equal(t, 2, h.Valid)
	assert.Equal(t, 11, h.Galat)
	assert.Zero(t, h.Dibuat)
	byLine := func(n int) BarisImpor {
		for _, b := range h.Baris {
			if b.Baris == n {
				return b
			}
		}
		t.Fatalf("baris %d tidak ada", n)
		return BarisImpor{}
	}
	assert.Equal(t, ImporValid, byLine(2).Status)
	assert.Empty(t, byLine(2).Galat)
	assert.Equal(t, ImporValid, byLine(14).Status)
	assert.Contains(t, barisPesan(h, 3), "Nama wajib diisi.")
	assert.Contains(t, barisPesan(h, 4), "Email tidak valid.")
	assert.Contains(t, barisPesan(h, 5), "Peran tidak dikenal: DEWA.")
	assert.Contains(t, barisPesan(h, 6), "Lembaga wajib diisi.")
	assert.Contains(t, barisPesan(h, 7), "Kode kabupaten tidak dikenal: 9999.")
	assert.Contains(t, barisPesan(h, 8), "Kode kabupaten harus 4 digit angka.")
	assert.Contains(t, barisPesan(h, 9), "Email sama dengan baris 2.")
	assert.Contains(t, barisPesan(h, 10), "Email sudah terdaftar.")
	assert.Contains(t, strings.Join(barisPesan(h, 11), " "), "Email tidak boleh diawali")
	assert.Contains(t, strings.Join(barisPesan(h, 12), " "), "rumus")
	assert.GreaterOrEqual(t, len(barisPesan(h, 13)), 4, "semua galat satu baris dilaporkan, bukan hanya yang pertama")
	for _, b := range h.Baris {
		if b.Status == ImporGalat {
			assert.NotEmpty(t, b.Galat, "baris %d", b.Baris)
		}
	}

	t.Run("dry run tidak menulis apa pun", func(t *testing.T) {
		assert.Zero(t, f.repo.nBuat)
		assert.Empty(t, f.audit.Entries())
		assert.Empty(t, f.smtp.Messages())
		assert.Len(t, f.repo.users, 2, "hanya admin dan akun yang sudah ada")
		f.uc.TungguPengirimanEmail()
		assert.Empty(t, f.smtp.Messages())
	})
	t.Run("isian dirapikan di hasil", func(t *testing.T) {
		assert.Equal(t, "budi@garda.test", byLine(9).Email, "email baris duplikat dirapikan ke huruf kecil")
		assert.Equal(t, "OPS_MAHASISWA", byLine(2).Peran)
	})
}

func TestImpor_DuplikatDalamBerkas_BarisPertamaTetapSah(t *testing.T) {
	f := newUndanganFixture(t)
	csv := headerImpor + "\nA,sama@garda.test,OPS_MAHASISWA,X,\nB,SAMA@garda.test,OPS_MAHASISWA,X,\nC,sama@garda.test,OPS_MAHASISWA,X,\n"

	h := f.imporSah(csv, true)

	assert.Equal(t, ImporValid, h.Baris[0].Status)
	assert.Equal(t, ImporGalat, h.Baris[1].Status)
	assert.Equal(t, ImporGalat, h.Baris[2].Status)
	assert.Equal(t, []string{"Email sama dengan baris 2."}, h.Baris[1].Galat)
	assert.Equal(t, []string{"Email sama dengan baris 2."}, h.Baris[2].Galat)
}

func TestImpor_EmailTerdaftarHanyaDilaporkanSekali(t *testing.T) {
	ada := &domain.Pengguna{ID: uid("u-1"), Nama: "Ada", Email: "ada@garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "X", Status: "AKTIF"}
	f := newUndanganFixture(t, ada)

	h := f.imporSah(headerImpor+"\nA,ada@garda.test,OPS_MAHASISWA,X,\nB,ada@garda.test,OPS_MAHASISWA,X,\n", true)

	assert.Equal(t, []string{"Email sudah terdaftar."}, h.Baris[0].Galat)
	assert.Equal(t, []string{"Email sama dengan baris 2."}, h.Baris[1].Galat, "salinan ditandai duplikat dalam berkas, bukan dua kali")
}

func TestImpor_PeranHurufKecilDiterima(t *testing.T) {
	f := newUndanganFixture(t)

	h := f.imporSah(headerImpor+"\nA,a@garda.test,ops_mahasiswa,X,\n", true)

	assert.Equal(t, ImporValid, h.Baris[0].Status)
	assert.Equal(t, "OPS_MAHASISWA", h.Baris[0].Peran)
}

func TestImpor_GalatBerkasMenolakTanpaMenulis(t *testing.T) {
	f := newUndanganFixture(t)

	_, err := f.impor("nama,email\nA,a@b.id\n", false)

	var validasi *GalatValidasi
	require.ErrorAs(t, err, &validasi)
	assert.Zero(t, f.repo.nBuat)
	assert.Empty(t, f.audit.Entries())
}

// ── Komit ─────────────────────────────────────────────────────────────

func csvKomit() string {
	return headerImpor + "\n" +
		"Budi Santoso,budi@garda.test,OPS_MAHASISWA,FMIPA USK,1171\n" +
		"Siti Rahma,siti@garda.test,OPS_AGREGATOR,BPS Aceh,\n" +
		"Tanpa Email,bukan-email,OPS_MAHASISWA,FMIPA,\n"
}

func TestImpor_Komit_MembuatBarisSahSebagaiUndangan(t *testing.T) {
	f := newUndanganFixture(t)

	h := f.imporSah(csvKomit(), false)
	f.uc.TungguPengirimanEmail()

	assert.False(t, h.DryRun)
	assert.Equal(t, 3, h.Total)
	assert.Equal(t, 2, h.Valid)
	assert.Equal(t, 2, h.Dibuat)
	assert.Equal(t, 1, h.Galat)
	assert.True(t, h.EmailAktif)
	assert.Equal(t, 1, f.repo.nBuat, "semua baris sah ditulis dalam satu transaksi")

	t.Run("hasil per baris", func(t *testing.T) {
		assert.Equal(t, ImporDibuat, h.Baris[0].Status)
		assert.Equal(t, ImporDibuat, h.Baris[1].Status)
		assert.Equal(t, ImporGalat, h.Baris[2].Status)
		assert.NotEmpty(t, h.Baris[0].ID)
		assert.Empty(t, h.Baris[2].ID)
	})
	t.Run("akun MENUNGGU tanpa kata sandi dengan undangan hidup", func(t *testing.T) {
		for _, b := range h.Baris[:2] {
			u := f.repo.pengguna(b.ID)
			require.NotNil(t, u, b.Email)
			assert.Equal(t, "MENUNGGU", u.Status)
			assert.Nil(t, u.PasswordHash)
			assert.Equal(t, b.Email, u.Email)
			assert.Len(t, f.repo.undanganAktif(b.ID), 1)
		}
		assert.Equal(t, "1171", *f.repo.pengguna(h.Baris[0].ID).KabupatenKode)
		assert.Nil(t, f.repo.pengguna(h.Baris[1].ID).KabupatenKode)
		assert.Equal(t, "OPERASIONAL", f.repo.pengguna(h.Baris[1].ID).Lapis)
	})
	t.Run("email undangan hanya untuk baris yang dibuat", func(t *testing.T) {
		assert.Len(t, f.smtp.Untuk("budi@garda.test"), 1)
		assert.Len(t, f.smtp.Untuk("siti@garda.test"), 1)
		assert.Len(t, f.smtp.Messages(), 2)
		token := f.smtp.Untuk("budi@garda.test")[0].Token()
		require.NoError(t, f.uc.AktifkanAkun(context.Background(), token, sandiAktivasi), "tautan dari impor bisa dipakai")
	})
	t.Run("audit: satu entri per akun dan satu ringkasan", func(t *testing.T) {
		undang := entriAudit(f, "UNDANG_PENGGUNA")
		require.Len(t, undang, 2)
		for _, e := range undang {
			assert.Equal(t, adminUndangID, e.AktorID)
			assert.Equal(t, "impor", e.Metadata["sumber"])
		}
		ringkasan := entriAudit(f, "IMPOR_PENGGUNA")
		require.Len(t, ringkasan, 1)
		assert.Equal(t, 3, ringkasan[0].Metadata["total"])
		assert.Equal(t, 2, ringkasan[0].Metadata["dibuat"])
		assert.Equal(t, 1, ringkasan[0].Metadata["ditolak"])
	})
}

func TestImpor_Komit_TanpaBarisSahTidakMenulis(t *testing.T) {
	f := newUndanganFixture(t)

	h := f.imporSah(headerImpor+"\n,,,,\nA,salah,OPS_MAHASISWA,X,\n", false)

	assert.Zero(t, h.Valid)
	assert.Zero(t, h.Dibuat)
	assert.Zero(t, f.repo.nBuat)
	assert.Empty(t, f.audit.Entries(), "tidak ada perubahan, tidak ada audit")
}

func TestImpor_Komit_EmailDidaftarkanPihakLainSaatBerjalan(t *testing.T) {
	f := newUndanganFixture(t)
	f.repo.errBuat = fmt.Errorf("buat akun undangan: %w", gorm.ErrDuplicatedKey)

	_, err := f.impor(csvKomit(), false)

	require.ErrorIs(t, err, ErrImporBentrok)
	assert.Empty(t, f.audit.Entries(), "tidak ada akun yang dibuat, tidak ada audit")
	f.uc.TungguPengirimanEmail()
	assert.Empty(t, f.smtp.Messages())
}

func TestImpor_Komit_GalatBasisDataDilaporkan(t *testing.T) {
	f := newUndanganFixture(t)
	f.repo.errBuat = errors.New("koneksi putus")

	_, err := f.impor(csvKomit(), false)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrImporBentrok)
	assert.Empty(t, f.audit.Entries())
	assert.Empty(t, f.smtp.Messages())
}

func TestImpor_Komit_EmailSudahTerdaftarSejakPratinjauDilewati(t *testing.T) {
	f := newUndanganFixture(t)
	pratinjau := f.imporSah(csvKomit(), true)
	require.Equal(t, 2, pratinjau.Valid)
	// Sesudah pratinjau, Budi didaftarkan lewat jalur lain.
	f.undang(UndangInput{Nama: "Budi Lain", Email: "budi@garda.test", Peran: "OPS_MAHASISWA", Lembaga: "X"})

	h := f.imporSah(csvKomit(), false)

	assert.Equal(t, 1, h.Dibuat, "komit memeriksa ulang dengan data terkini")
	assert.Equal(t, ImporGalat, h.Baris[0].Status)
	assert.Contains(t, h.Baris[0].Galat, "Email sudah terdaftar.")
	assert.Equal(t, ImporDibuat, h.Baris[1].Status)
}

func TestImpor_Komit_SMTPTidakDikonfigurasi(t *testing.T) {
	f := newUndanganFixture(t)
	f.uc.emailSender = email.New(email.Config{})

	h := f.imporSah(csvKomit(), false)

	assert.False(t, h.EmailAktif)
	assert.Equal(t, 2, h.Dibuat, "akun tetap dibuat; undangan dikirim ulang setelah SMTP aktif")
	f.uc.TungguPengirimanEmail()
	assert.Empty(t, f.smtp.Messages())
}

func TestImpor_Komit_EmailGagalTercatatDiAudit(t *testing.T) {
	f := newUndanganFixture(t)
	f.smtp.TolakPenerima(true)

	h := f.imporSah(csvKomit(), false)
	f.uc.TungguPengirimanEmail()

	assert.Equal(t, 2, h.Dibuat)
	gagal := entriAudit(f, "KIRIM_UNDANGAN")
	require.Len(t, gagal, 2)
	for _, e := range gagal {
		assert.Equal(t, audit.HasilGagal, e.Hasil)
		assert.Equal(t, adminUndangID, e.AktorID)
	}
}

func TestImpor_Komit_PengirimanEmailDibatasiKonkurensinya(t *testing.T) {
	f := newUndanganFixture(t)
	var sb strings.Builder
	sb.WriteString(headerImpor + "\n")
	const jumlah = 60
	for i := range jumlah {
		fmt.Fprintf(&sb, "Nama %d,u%d@garda.test,OPS_MAHASISWA,FMIPA,\n", i, i)
	}

	h := f.imporSah(sb.String(), false)
	f.uc.TungguPengirimanEmail()

	assert.Equal(t, jumlah, h.Dibuat)
	assert.Len(t, f.smtp.Messages(), jumlah, "setiap akun mendapat satu email")
	assert.LessOrEqual(t, f.smtp.MaksKoneksiSerempak(), imporKonkurensiEmail)
}

func TestImpor_Komit_PermintaanSelesaiTidakMembatalkanPengirimanLatar(t *testing.T) {
	f := newUndanganFixture(t)
	ctx, batal := context.WithCancel(context.Background())

	h, err := f.uc.ImporPengguna(ctx, csvKomit(), false, adminUndangID)
	batal() // jawaban sudah terkirim ke klien; konteks permintaan dibatalkan server HTTP
	f.uc.TungguPengirimanEmail()

	require.NoError(t, err)
	assert.Equal(t, 2, h.Dibuat)
	assert.Len(t, f.smtp.Messages(), 2)
}

func TestImpor_Komit_BerkasMaksimum500Baris(t *testing.T) {
	f := newUndanganFixture(t)
	var sb strings.Builder
	sb.WriteString(headerImpor + "\n")
	for i := range ImporMaksBaris {
		fmt.Fprintf(&sb, "Nama %d,u%d@garda.test,OPS_MAHASISWA,FMIPA,\n", i, i)
	}

	h := f.imporSah(sb.String(), false)
	f.uc.TungguPengirimanEmail()

	assert.Equal(t, ImporMaksBaris, h.Dibuat)
	assert.Len(t, entriAudit(f, "UNDANG_PENGGUNA"), ImporMaksBaris)
	assert.Len(t, f.smtp.Messages(), ImporMaksBaris)
}
