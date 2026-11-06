package integration

import (
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Daftar pengguna sisi server (GET /v1/users): filter, urutan, tiebreaker, paginasi, dan validasi.
//
// Dataset dasar (seedDaftar, 13 akun): 4 akun seedData (dibuat_pada dipatok 2025-12-01..04) ditambah 9 akun dp-*
// (2026-01-*). Admin yang masuk adalah test-user-001 (ADMIN_SISTEM); login itu mengisi terakhir_masuk-nya
// dengan "sekarang". Nama, email, dan lembaga dataset dasar sengaja bebas tanda baca yang urutannya bergantung
// collation basis data; karakter khusus pencarian ada di seedDaftarKarakter.

type barisDaftar struct {
	ID             string     `json:"id"`
	Nama           string     `json:"nama"`
	Email          string     `json:"email"`
	Peran          string     `json:"peran"`
	Lapis          string     `json:"lapis"`
	Lembaga        string     `json:"lembaga"`
	MfaAktif       bool       `json:"mfa_aktif"`
	Status         string     `json:"status"`
	TerakhirMasuk  *time.Time `json:"terakhir_masuk"`
	DibuatPada     time.Time  `json:"dibuat_pada"`
	TerkunciSampai *time.Time `json:"terkunci_sampai"`
}

type halamanDaftar struct {
	Items []barisDaftar `json:"items"`
	Total int64         `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

func (h halamanDaftar) ids() []string {
	out := make([]string, len(h.Items))
	for i, b := range h.Items {
		out[i] = b.ID
	}
	return out
}

type akunDaftar struct {
	id, nama, email, peran, lapis, lembaga, status string
	mfa                                            bool
	masuk                                          string // kosong = belum pernah masuk
	dibuat                                         string
}

func sisipAkunDaftar(t *testing.T, db *gorm.DB, akun []akunDaftar) {
	t.Helper()
	for _, a := range akun {
		var masuk, secret any
		if a.masuk != "" {
			masuk = a.masuk
		}
		if a.mfa {
			secret = "v1:rahasia-uji"
		}
		require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_secret, terakhir_masuk, dibuat_pada)
			SELECT ?, ?, ?, password_hash, ?::peran, ?::lapis, ?, ?::status_akun, ?, ?, ?::timestamptz, ?::timestamptz FROM pengguna WHERE id = 'test-user-001'`,
			a.id, a.nama, a.email, a.peran, a.lapis, a.lembaga, a.status, a.mfa, secret, masuk, a.dibuat).Error)
	}
}

// seedDaftar menambah akun uji bervariasi di atas seedData. Tiga pasang akun sengaja kembar pada kolom urut
// untuk menguji tiebreaker: dibuat_pada (dp-03/dp-04), terakhir_masuk (dp-02/dp-07), dan nama (dp-14/dp-15).
func seedDaftar(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedData(t, db)

	for id, tgl := range map[string]string{
		"test-user-001": "2025-12-01", "test-user-002": "2025-12-02", "test-user-biasa": "2025-12-03", "test-user-mfa": "2025-12-04",
	} {
		require.NoError(t, db.Exec("UPDATE pengguna SET dibuat_pada = ?::timestamptz WHERE id = ?", tgl+" 08:00:00+00", id).Error)
	}
	sisipAkunDaftar(t, db, []akunDaftar{
		{"dp-01", "Ani Aceh", "ani@garda.test", "OPS_MAHASISWA", "OPERASIONAL", "Lembaga B", "AKTIF", false, "", "2026-01-01 08:00:00+00"},
		{"dp-02", "budi santoso", "budi@garda.test", "OPS_MAHASISWA", "OPERASIONAL", "lembaga a", "AKTIF", false, "2025-10-01 08:00:00+00", "2026-01-02 08:00:00+00"},
		{"dp-03", "Citra Dewi", "citra@garda.test", "MANAJER_DATA", "MANAJERIAL", "BPS Aceh", "AKTIF", true, "2025-10-05 08:00:00+00", "2026-01-03 08:00:00+00"},
		{"dp-04", "Dewi Lestari", "dewi@garda.test", "MANAJER_DATA", "MANAJERIAL", "BPS Aceh", "NONAKTIF", false, "", "2026-01-03 08:00:00+00"},
		{"dp-05", "Eka Putra", "eka@garda.test", "OPS_KOORDINATOR", "OPERASIONAL", "BI Aceh", "MENUNGGU", false, "", "2026-01-05 08:00:00+00"},
		{"dp-06", "Fajar Nugraha", "fajar@garda.test", "STRATEGIS_BI", "STRATEGIS", "BI Aceh", "AKTIF", true, "2025-10-02 08:00:00+00", "2026-01-06 08:00:00+00"},
		{"dp-07", "Gita Ayu", "gita@garda.test", "OPS_MAHASISWA", "OPERASIONAL", "Lembaga A", "AKTIF", false, "2025-10-01 08:00:00+00", "2026-01-07 08:00:00+00"},
		{"dp-14", "Kembar Nama", "kembar2@garda.test", "MANAJER_PROGRAM", "MANAJERIAL", "Kemenkeu", "AKTIF", true, "2025-09-01 08:00:00+00", "2026-01-11 08:00:00+00"},
		{"dp-15", "Kembar Nama", "kembar1@garda.test", "MANAJER_PROGRAM", "MANAJERIAL", "Kemenkeu", "AKTIF", false, "", "2026-01-12 08:00:00+00"},
	})
}

const totalDaftarUji = 13

// seedDaftarKarakter menambah tiga akun dengan karakter yang bermakna bagi LIKE (% dan _) di atas seedDaftar.
func seedDaftarKarakter(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDaftar(t, db)
	sisipAkunDaftar(t, db, []akunDaftar{
		{"dp-08", "Hana 100%", "hana@garda.test", "OPS_AGREGATOR", "OPERASIONAL", "FMIPA USK", "AKTIF", false, "", "2026-01-08 08:00:00+00"},
		{"dp-09", "A_b Literal", "alit@garda.test", "OPS_AGREGATOR", "OPERASIONAL", "FMIPA USK", "AKTIF", false, "", "2026-01-09 08:00:00+00"},
		{"dp-10", "Aab Bukan Literal", "aab@garda.test", "OPS_AGREGATOR", "OPERASIONAL", "FMIPA USK", "AKTIF", false, "", "2026-01-10 08:00:00+00"},
	})
}

// daftar memanggil GET /v1/users?query dan mendekode halamannya; status harus 200.
func (s *Suite) daftar(t *testing.T, token, query string) halamanDaftar {
	t.Helper()
	w := s.panggil(t, http.MethodGet, "/v1/users?"+query, nil, token)
	require.Equal(t, http.StatusOK, w.Code, "%s: %s", query, w.Body.String())
	var h halamanDaftar
	s.decodeData(t, w.Body.Bytes(), &h)
	return h
}

func (s *Suite) daftarSemua(t *testing.T, token, query string) halamanDaftar {
	t.Helper()
	if query != "" {
		query += "&"
	}
	return s.daftar(t, token, query+"limit=100")
}

// urutanPeran adalah urutan deklarasi enum peran di basis data (hierarki lapis), sama dengan katalog peran.
var urutanPeran = []string{
	"STRATEGIS_FMIPA", "STRATEGIS_BI", "STRATEGIS_BPS", "STRATEGIS_DJPB",
	"MANAJER_PROGRAM", "MANAJER_DATA", "MANAJER_KEBIJAKAN", "MANAJER_PEMBIAYAAN", "MANAJER_RISIKO", "MANAJER_MUTU",
	"OPS_KOORDINATOR", "OPS_AGREGATOR", "OPS_MAHASISWA", "ADMIN_SISTEM",
}

var urutanStatus = []string{"AKTIF", "NONAKTIF", "MENUNGGU"}

func TestDaftarPengguna_UrutanBawaanTerbaruDuluDenganTiebreakerID(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	h := s.daftar(t, admin, "")

	assert.EqualValues(t, totalDaftarUji, h.Total)
	assert.Equal(t, 1, h.Page)
	assert.Equal(t, 20, h.Limit, "limit bawaan")
	// dibuat_pada menurun; dp-03 dan dp-04 kembar waktunya: id menurun memutuskan (dp-04 lebih dulu).
	assert.Equal(t, []string{
		"dp-15", "dp-14", "dp-07", "dp-06", "dp-05", "dp-04", "dp-03", "dp-02", "dp-01",
		"test-user-mfa", "test-user-biasa", "test-user-002", "test-user-001",
	}, h.ids())
}

func TestDaftarPengguna_KolomUrutDuaArahKebalikanDanTiebreaker(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	// Kunci urut sisi tes per kolom, dibandingkan pada baris yang dikembalikan server. terakhir_masuk punya
	// aturan NULL tersendiri dan diuji di TestDaftarPengguna_TerakhirMasuk.
	kunci := map[string]func(b barisDaftar) string{
		"nama":        func(b barisDaftar) string { return strings.ToLower(b.Nama) },
		"email":       func(b barisDaftar) string { return strings.ToLower(b.Email) },
		"lembaga":     func(b barisDaftar) string { return strings.ToLower(b.Lembaga) },
		"peran":       func(b barisDaftar) string { return string(rune('a' + slices.Index(urutanPeran, b.Peran))) },
		"status":      func(b barisDaftar) string { return string(rune('a' + slices.Index(urutanStatus, b.Status))) },
		"dibuat_pada": func(b barisDaftar) string { return b.DibuatPada.UTC().Format(time.RFC3339Nano) },
	}

	for kolom, kunciBaris := range kunci {
		t.Run(kolom, func(t *testing.T) {
			naik := s.daftarSemua(t, admin, "sort="+kolom+"&arah=asc")
			turun := s.daftarSemua(t, admin, "sort="+kolom+"&arah=desc")
			require.Len(t, naik.Items, totalDaftarUji)
			require.Len(t, turun.Items, totalDaftarUji)

			// Tiebreaker id pada arah yang sama membuat urutan total: menurun = kebalikan persis dari menaik.
			balik := slices.Clone(naik.ids())
			slices.Reverse(balik)
			assert.Equal(t, balik, turun.ids(), "desc harus kebalikan asc")

			for i := 1; i < totalDaftarUji; i++ {
				// asc
				a, b := naik.Items[i-1], naik.Items[i]
				ka, kb := kunciBaris(a), kunciBaris(b)
				assert.LessOrEqual(t, ka, kb, "%s asc: %s lalu %s", kolom, a.ID, b.ID)
				if ka == kb {
					assert.Less(t, a.ID, b.ID, "%s asc: nilai kembar diputus id menaik", kolom)
				}
				// desc
				a, b = turun.Items[i-1], turun.Items[i]
				ka, kb = kunciBaris(a), kunciBaris(b)
				assert.GreaterOrEqual(t, ka, kb, "%s desc: %s lalu %s", kolom, a.ID, b.ID)
				if ka == kb {
					assert.Greater(t, a.ID, b.ID, "%s desc: nilai kembar diputus id menurun", kolom)
				}
			}
		})
	}
}

func TestDaftarPengguna_UrutanEksplisit(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	t.Run("nama tak peka huruf besar-kecil, nama kembar diputus id", func(t *testing.T) {
		ids := s.daftarSemua(t, admin, "sort=nama&arah=asc").ids()
		// Ani, budi (huruf kecil di tengah), Citra, Dewi, Eka, Fajar, Gita, Inactive, Kembar x2, MFA, Pengguna, Test.
		assert.Equal(t, []string{
			"dp-01", "dp-02", "dp-03", "dp-04", "dp-05", "dp-06", "dp-07",
			"test-user-002", "dp-14", "dp-15", "test-user-mfa", "test-user-biasa", "test-user-001",
		}, ids)
	})

	t.Run("peran mengikuti enum, bukan abjad", func(t *testing.T) {
		naik := s.daftarSemua(t, admin, "sort=peran&arah=asc")
		assert.Equal(t, "dp-06", naik.Items[0].ID, "STRATEGIS_BI paling awal")
		assert.Equal(t, "test-user-001", naik.Items[totalDaftarUji-1].ID, "ADMIN_SISTEM paling akhir")
		turun := s.daftarSemua(t, admin, "sort=peran&arah=desc")
		assert.Equal(t, "test-user-001", turun.Items[0].ID)
	})

	t.Run("status mengikuti enum AKTIF, NONAKTIF, MENUNGGU", func(t *testing.T) {
		naik := s.daftarSemua(t, admin, "sort=status&arah=asc")
		assert.Equal(t, "AKTIF", naik.Items[0].Status)
		assert.Equal(t, "MENUNGGU", naik.Items[totalDaftarUji-1].Status)
		assert.Equal(t, "NONAKTIF", naik.Items[totalDaftarUji-2].Status, "NONAKTIF tepat sebelum MENUNGGU; dp-05 satu-satunya MENUNGGU")
	})
}

func TestDaftarPengguna_TerakhirMasuk(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access // test-user-001 kini punya terakhir_masuk = sekarang

	naik := s.daftarSemua(t, admin, "sort=terakhir_masuk&arah=asc").ids()
	turun := s.daftarSemua(t, admin, "sort=terakhir_masuk&arah=desc").ids()

	// Bertanggal: dp-14 (2025-09-01), dp-02 dan dp-07 (kembar 2025-10-01), dp-06 (10-02), dp-03 (10-05), admin (sekarang).
	assert.Equal(t, []string{"dp-14", "dp-02", "dp-07", "dp-06", "dp-03", "test-user-001"}, naik[:6])
	assert.Equal(t, []string{"test-user-001", "dp-03", "dp-06", "dp-07", "dp-02", "dp-14"}, turun[:6],
		"kembar diputus id menurun pada desc")
	// 7 akun belum pernah masuk selalu di akhir, juga pada desc, urut id sesuai arah.
	require.Len(t, naik[6:], 7)
	assert.True(t, slices.IsSorted(naik[6:]), "belum pernah masuk, asc: id menaik")
	assert.True(t, slices.IsSortedFunc(turun[6:], func(a, b string) int { return strings.Compare(b, a) }), "belum pernah masuk, desc: id menurun")
	assert.ElementsMatch(t, naik[6:], turun[6:])
}

func TestDaftarPengguna_PaginasiTotalStabilDanTanpaTumpangTindih(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	// Urutan dengan banyak nilai kembar (status AKTIF mendominasi, 7 akun belum pernah masuk) diambil per halaman
	// kecil: gabungan semua halaman harus persis sama dengan satu halaman penuh, tanpa duplikat atau lubang.
	for _, urut := range []string{"status", "lembaga", "peran", "terakhir_masuk", "nama", "dibuat_pada"} {
		for _, arah := range []string{"asc", "desc"} {
			t.Run(urut+"_"+arah, func(t *testing.T) {
				penuh := s.daftarSemua(t, admin, "sort="+urut+"&arah="+arah).ids()
				var gabungan []string
				for page := 1; page <= 5; page++ {
					h := s.daftar(t, admin, "sort="+urut+"&arah="+arah+"&limit=3&page="+strconv.Itoa(page))
					assert.EqualValues(t, totalDaftarUji, h.Total, "total tetap di semua halaman")
					assert.Equal(t, page, h.Page)
					assert.Equal(t, 3, h.Limit)
					assert.Len(t, h.Items, min(3, max(0, totalDaftarUji-(page-1)*3)), "halaman %d", page)
					gabungan = append(gabungan, h.ids()...)
				}
				assert.Equal(t, penuh, gabungan)
			})
		}
	}

	t.Run("halaman di luar jangkauan kosong tetapi total benar", func(t *testing.T) {
		h := s.daftar(t, admin, "limit=10&page=99")
		assert.Empty(t, h.Items)
		assert.NotNil(t, h.Items, "items larik kosong, bukan null")
		assert.EqualValues(t, totalDaftarUji, h.Total)
		assert.Equal(t, 99, h.Page)
	})
	t.Run("limit 100 diterima, 101 ditolak", func(t *testing.T) {
		assert.Equal(t, 100, s.daftar(t, admin, "limit=100").Limit)
		w := s.panggil(t, http.MethodGet, "/v1/users?limit=101", nil, admin)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("page 0 dan limit 0 dianggap bawaan", func(t *testing.T) {
		h := s.daftar(t, admin, "page=0&limit=0")
		assert.Equal(t, 1, h.Page)
		assert.Equal(t, 20, h.Limit)
	})
}

func TestDaftarPengguna_Filter(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	// Akun terkunci lewat jalur sungguhan: lima kata sandi salah, dengan email berbeda huruf besar-kecil dari
	// yang di tabel (kunci lockout memakai email ternormalisasi).
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, s.coba(t, "Budi@GARDA.test", sandiSalahUji).Code)
	}
	require.Equal(t, http.StatusLocked, s.coba(t, "budi@garda.test", "Test1234!").Code, "dp-02 terkunci")
	// Kunci yang sudah lewat tidak dihitung terkunci (dp-05); baris kunci email tak terdaftar tidak mengganggu.
	require.NoError(t, s.db.Exec(`INSERT INTO kunci_login (email_hash, gagal, jendela_mulai, terkunci_sampai)
		VALUES (?, 5, now() - interval '1 hour', now() - interval '1 minute')`, usecase.HashEmailKunci("eka@garda.test")).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO kunci_login (email_hash, gagal, jendela_mulai, terkunci_sampai)
		VALUES (?, 5, now(), now() + interval '10 minutes')`, usecase.HashEmailKunci("hantu@garda.test")).Error)

	semuaKecuali := func(kecuali ...string) []string {
		var out []string
		for _, id := range []string{"dp-01", "dp-02", "dp-03", "dp-04", "dp-05", "dp-06", "dp-07", "dp-14", "dp-15", "test-user-001", "test-user-002", "test-user-biasa", "test-user-mfa"} {
			if !slices.Contains(kecuali, id) {
				out = append(out, id)
			}
		}
		return out
	}

	cases := []struct {
		nama, query string
		want        []string // urutan tidak dipentingkan
	}{
		{"peran", "peran=MANAJER_DATA", []string{"dp-03", "dp-04"}},
		{"peran satu-satunya admin", "peran=ADMIN_SISTEM", []string{"test-user-001"}},
		{"status NONAKTIF", "status=NONAKTIF", []string{"dp-04", "test-user-002"}},
		{"status MENUNGGU", "status=MENUNGGU", []string{"dp-05"}},
		{"lapis STRATEGIS", "lapis=STRATEGIS", []string{"dp-06"}},
		{"lapis SISTEM", "lapis=SISTEM", []string{"test-user-001"}},
		{"mfa aktif", "mfa=true", []string{"dp-03", "dp-06", "dp-14", "test-user-mfa"}},
		{"mfa belum aktif", "mfa=false", semuaKecuali("dp-03", "dp-06", "dp-14", "test-user-mfa")},
		{"terkunci", "terkunci=true", []string{"dp-02"}},
		{"tidak terkunci (kunci yang sudah lewat tidak dihitung)", "terkunci=false", semuaKecuali("dp-02")},
		{"gabungan dengan AND", "peran=OPS_MAHASISWA&status=AKTIF&mfa=false&terkunci=false", []string{"dp-01", "dp-07", "test-user-biasa"}},
		{"gabungan tanpa hasil", "peran=ADMIN_SISTEM&mfa=true", []string{}},
		{"terkunci dan peran", "terkunci=true&peran=OPS_MAHASISWA", []string{"dp-02"}},
		{"terkunci dan peran lain", "terkunci=true&peran=MANAJER_DATA", []string{}},
		{"parameter kosong sama dengan tidak ada", "peran=&status=&lapis=&mfa=&terkunci=", semuaKecuali()},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			h := s.daftarSemua(t, admin, tc.query)
			assert.ElementsMatch(t, tc.want, h.ids())
			assert.EqualValues(t, len(tc.want), h.Total, "total menghitung hanya baris yang cocok")
		})
	}

	t.Run("total mengikuti filter walau halaman lebih kecil", func(t *testing.T) {
		h := s.daftar(t, admin, "mfa=false&limit=2")
		assert.Len(t, h.Items, 2)
		assert.EqualValues(t, 9, h.Total)
	})

	t.Run("akun terkunci membawa terkunci_sampai", func(t *testing.T) {
		h := s.daftarSemua(t, admin, "terkunci=true")
		require.Len(t, h.Items, 1)
		require.NotNil(t, h.Items[0].TerkunciSampai)
		assert.True(t, h.Items[0].TerkunciSampai.After(time.Now()))
	})

	t.Run("dibuka kuncinya tidak lagi terkunci", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.bukaKunci(t, admin, "dp-02").Code)
		assert.Empty(t, s.daftarSemua(t, admin, "terkunci=true").Items)
		assert.Len(t, s.daftarSemua(t, admin, "terkunci=false").Items, totalDaftarUji)
	})
}

func TestDaftarPengguna_Pencarian(t *testing.T) {
	s := setupSuiteWith(t, seedDaftarKarakter)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	cases := []struct {
		nama, query string
		want        []string
	}{
		{"nama tak peka huruf besar-kecil", "search=ANI+ACEH", []string{"dp-01"}},
		{"potongan nama", "search=santoso", []string{"dp-02"}},
		{"email", "search=fajar%40garda", []string{"dp-06"}},
		{"lembaga", "search=bps+aceh", []string{"dp-03", "dp-04"}},
		{"peran sebagai id", "search=manajer_data", []string{"dp-03", "dp-04"}},
		{"peran sebagai label dengan spasi", "search=manajer+data", []string{"dp-03", "dp-04"}},
		{"spasi tepi dibuang", "search=++fajar++", []string{"dp-06"}},
		{"persen dicari apa adanya", "search=100%25", []string{"dp-08"}},
		{"garis bawah bukan wildcard", "search=a_b", []string{"dp-09"}},
		{"garis miring terbalik tidak merusak pola", "search=%5C", []string{}},
		{"tanpa kecocokan", "search=tidak-ada-orang-ini", []string{}},
		{"dengan filter", "search=aceh&status=AKTIF", []string{"dp-01", "dp-03", "dp-06"}},
		{"kata kunci hanya spasi sama dengan tanpa pencarian", "search=+++", nil},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			h := s.daftarSemua(t, admin, tc.query)
			if tc.want == nil {
				assert.EqualValues(t, totalDaftarUji+3, h.Total)
				return
			}
			assert.ElementsMatch(t, tc.want, h.ids())
			assert.EqualValues(t, len(tc.want), h.Total)
		})
	}

	t.Run("total mengikuti kecocokan, bukan seluruh tabel", func(t *testing.T) {
		h := s.daftar(t, admin, "search=aceh&limit=1")
		assert.Len(t, h.Items, 1)
		assert.EqualValues(t, 5, h.Total, "Ani Aceh, Citra dan Dewi (BPS Aceh), Eka dan Fajar (BI Aceh)")
	})
}

func TestDaftarPengguna_ParameterSalahDijawab400(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	for nama, query := range map[string]string{
		"sort tak dikenal":        "sort=password_hash",
		"sort injeksi SQL":        "sort=" + url.QueryEscape("nama; DROP TABLE pengguna; --"),
		"sort huruf besar":        "sort=NAMA",
		"arah tak dikenal":        "sort=nama&arah=ASC",
		"arah saja salah":         "arah=naik",
		"peran tak dikenal":       "peran=SUPERADMIN",
		"peran huruf kecil":       "peran=admin_sistem",
		"peran injeksi":           "peran=" + url.QueryEscape("ADMIN_SISTEM' OR '1'='1"),
		"status tak dikenal":      "status=aktif",
		"lapis tak dikenal":       "lapis=OPS",
		"mfa angka":               "mfa=1",
		"mfa huruf besar":         "mfa=TRUE",
		"terkunci ya":             "terkunci=ya",
		"limit 101":               "limit=101",
		"limit negatif":           "limit=-5",
		"page negatif":            "page=-1",
		"page bukan angka":        "page=satu",
		"search lebih dari 100":   "search=" + strings.Repeat("x", 101),
		"limit tidak bulat":       "limit=2.5",
		"filter benar sort salah": "status=AKTIF&sort=foto",
	} {
		w := s.panggil(t, http.MethodGet, "/v1/users?"+query, nil, admin)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s (%s): %s", nama, query, w.Body.String())
		assert.Equal(t, "400", s.parseResponse(t, w)["code"], nama)
	}

	// Seratus karakter tepat masih sah.
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/users?search="+strings.Repeat("x", 100), nil, admin).Code)
}

func TestDaftarPengguna_Otorisasi(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	biasa := s.mulaiSesi(t, emailBiasa, uaIPhone).access

	assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/users", nil, biasa).Code, "tanpa pengguna.kelola")
	assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/users?sort=nama&status=AKTIF", nil, biasa).Code)
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/users", nil, "").Code, "tanpa token")
	// Parameter yang salah tidak boleh membocorkan apa pun ke peran tanpa izin: 403 lebih dulu dari 400.
	assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/users?sort=salah", nil, biasa).Code)
}

func TestDaftarPengguna_TidakMembocorkanRahasia(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	w := s.panggil(t, http.MethodGet, "/v1/users?limit=100", nil, admin)
	require.Equal(t, http.StatusOK, w.Code)
	for _, terlarang := range []string{"password_hash", "mfa_secret", "rahasia-uji", "mfa_langkah"} {
		assert.NotContains(t, w.Body.String(), terlarang)
	}
}

// Mengurutkan daftar tidak boleh memengaruhi isinya: sort yang berbeda menghasilkan himpunan baris yang sama.
func TestDaftarPengguna_UrutanTidakMengubahHimpunan(t *testing.T) {
	s := setupSuiteWith(t, seedDaftar)
	admin := s.mulaiSesi(t, emailAdmin, uaMac).access

	acuan := s.daftarSemua(t, admin, "").ids()
	for _, kolom := range []string{"nama", "email", "peran", "lembaga", "status", "terakhir_masuk", "dibuat_pada"} {
		sort.Strings(acuan)
		ids := s.daftarSemua(t, admin, "sort="+kolom).ids()
		sort.Strings(ids)
		assert.Equal(t, acuan, ids, kolom)
	}
}
