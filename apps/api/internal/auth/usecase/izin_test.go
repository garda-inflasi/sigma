package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// ── Perangkat tes ─────────────────────────────────────────────────────

type izinSuite struct {
	uc    *UseCase
	izin  *izinFake
	repo  *repoDenganIzin
	audit *audittest.Recorder
	jam   *jamTes
}

func newIzinSuite(t *testing.T, ttl time.Duration) *izinSuite {
	t.Helper()
	jam := &jamTes{t: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)}
	fake := newIzinFake(jam.Now)
	fake.tambahAkun(uid("admin-1"), "ADMIN_SISTEM", "AKTIF")
	fake.tambahAkun("mhs-1", "OPS_MAHASISWA", "AKTIF")
	fake.tambahAkun("nonaktif-1", "OPS_MAHASISWA", "NONAKTIF")
	fake.namaPemberi[uid("admin-1")] = "Admin Satu"

	repo := &repoDenganIzin{mockRepository: new(mockRepository), izinFake: fake}
	cfg := DefaultConfig()
	cfg.IzinCacheTTL = ttl
	rec := &audittest.Recorder{}
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	uc := New(repo, tokenMgr, cfg, logger.NewDefault(), email.New(email.Config{}), rec)
	require.NotNil(t, uc.izin, "New harus mendeteksi domain.IzinRepository")
	require.NotNil(t, uc.izinCache)
	uc.izinCache.now = jam.Now
	uc.sekarang = jam.Now
	return &izinSuite{uc: uc, izin: fake, repo: repo, audit: rec, jam: jam}
}

func (s *izinSuite) efektif(t *testing.T, userID string) []string {
	t.Helper()
	izin, err := s.uc.izinCache.Permissions(context.Background(), userID)
	require.NoError(t, err)
	return izin
}

func (s *izinSuite) panggilan() int {
	s.izin.mu.Lock()
	defer s.izin.mu.Unlock()
	return s.izin.panggilanEfektif
}

// ── Cache izin efektif ────────────────────────────────────────────────

func TestIzinCache_MelayaniDariCacheSelamaTTLLaluMemuatUlang(t *testing.T) {
	s := newIzinSuite(t, 15*time.Second)

	s.efektif(t, "mhs-1")
	s.efektif(t, "mhs-1")
	assert.Equal(t, 1, s.panggilan(), "pembacaan kedua dari cache")

	s.jam.Maju(14 * time.Second)
	s.efektif(t, "mhs-1")
	assert.Equal(t, 1, s.panggilan(), "masih dalam TTL")

	s.jam.Maju(2 * time.Second)
	s.efektif(t, "mhs-1")
	assert.Equal(t, 2, s.panggilan(), "TTL habis: baca ulang dari basis data")
}

func TestIzinCache_PerubahanDariLuarProsesBerlakuPalingLambatSetelahTTL(t *testing.T) {
	s := newIzinSuite(t, 15*time.Second)
	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat")

	// Perubahan "dari proses lain": langsung ke penyimpanan tanpa melewati usecase (tanpa pembuangan cache).
	s.izin.peran["OPS_MAHASISWA"].izin["kpi.lihat"] = true

	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "dalam TTL cache belum berubah (batas yang didokumentasikan)")
	s.jam.Maju(15 * time.Second)
	assert.Contains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "paling lambat setelah TTL")
}

func TestIzinCache_EntriTidakHidupMelewatiIzinLangsungYangKedaluwarsa(t *testing.T) {
	s := newIzinSuite(t, 15*time.Second)
	sampai := s.jam.Now().Add(5 * time.Second)
	_, err := s.uc.GrantUserPermission(context.Background(), GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", BerlakuSampai: &sampai, Alasan: "uji"}, uid("admin-1"))
	require.NoError(t, err)

	assert.Contains(t, s.efektif(t, "mhs-1"), "kpi.lihat")
	s.jam.Maju(4 * time.Second)
	assert.Contains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "belum kedaluwarsa")

	s.jam.Maju(2 * time.Second) // 6 detik < TTL 15 detik, tetapi izin langsung sudah lewat
	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "izin langsung kedaluwarsa berhenti berlaku tepat waktu, tanpa menunggu TTL")
}

func TestIzinCache_PembuanganEksplisit(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	s.efektif(t, "mhs-1")
	s.efektif(t, uid("admin-1"))
	require.Equal(t, 2, s.panggilan())

	s.uc.bersihkanCacheIzin("mhs-1")
	s.efektif(t, "mhs-1")
	s.efektif(t, uid("admin-1"))
	assert.Equal(t, 3, s.panggilan(), "hanya pengguna yang dibuang yang dimuat ulang")

	s.uc.bersihkanSemuaCacheIzin()
	s.efektif(t, "mhs-1")
	s.efektif(t, uid("admin-1"))
	assert.Equal(t, 5, s.panggilan(), "pembuangan semua memuat ulang semuanya")
}

func TestIzinCache_PembacaanYangMulaiSebelumPembuanganTidakMenyimpanHasilBasi(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	var sekali sync.Once
	s.izin.saatEfektif = func() {
		sekali.Do(func() { s.uc.bersihkanCacheIzin("mhs-1") }) // izin berubah di tengah pembacaan
	}

	s.efektif(t, "mhs-1")
	assert.Equal(t, 1, s.panggilan())

	s.efektif(t, "mhs-1")
	assert.Equal(t, 2, s.panggilan(), "hasil pembacaan yang tertimpa pembuangan tidak disimpan, jadi dibaca lagi")
	s.efektif(t, "mhs-1")
	assert.Equal(t, 2, s.panggilan(), "pembacaan berikutnya tersimpan normal")
}

func TestIzinCache_AkunTidakAdaAtauNonaktifAdalahErrPrincipalNotFoundDanTidakDisimpan(t *testing.T) {
	s := newIzinSuite(t, time.Hour)

	for _, id := range []string{"tidak-ada", "nonaktif-1"} {
		_, err := s.uc.izinCache.Permissions(context.Background(), id)
		assert.ErrorIs(t, err, token.ErrPrincipalNotFound, id)
	}
	before := s.panggilan()
	_, _ = s.uc.izinCache.Permissions(context.Background(), "nonaktif-1")
	assert.Equal(t, before+1, s.panggilan(), "jawaban 'tidak ada' tidak di-cache")
}

func TestIzinCache_GalatBasisDataGagalTutupTanpaMenyajikanEntriBasi(t *testing.T) {
	s := newIzinSuite(t, 15*time.Second)
	s.efektif(t, uid("admin-1"))

	s.izin.mu.Lock()
	s.izin.errEfektif = errors.New("connection refused")
	s.izin.mu.Unlock()
	s.jam.Maju(16 * time.Second)

	_, err := s.uc.izinCache.Permissions(context.Background(), uid("admin-1"))
	require.Error(t, err)
	assert.NotErrorIs(t, err, token.ErrPrincipalNotFound, "galat infrastruktur bukan 'akun tidak ada'")
}

func TestIzinCache_TTLNegatifMematikanCache(t *testing.T) {
	s := newIzinSuite(t, -1)
	s.efektif(t, "mhs-1")
	s.efektif(t, "mhs-1")
	assert.Equal(t, 2, s.panggilan())
}

func TestIzinCache_TTLNolMemakaiBawaan(t *testing.T) {
	s := newIzinSuite(t, 0)
	assert.Equal(t, DefaultIzinCacheTTL, s.uc.izinCache.ttl)
}

func TestIzinCache_HasilAdalahSalinan(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	pertama := s.efektif(t, "mhs-1")
	require.NotEmpty(t, pertama)
	pertama[0] = "diubah.pemanggil"

	assert.NotContains(t, s.efektif(t, "mhs-1"), "diubah.pemanggil")
}

func TestIzinCache_SapuEntriKedaluwarsaSaatPenuh(t *testing.T) {
	jam := &jamTes{t: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)}
	c := newIzinCache(newIzinFake(jam.Now), time.Second, jam.Now)
	for i := 0; i < maxEntriCacheIzin; i++ {
		c.entri[string(rune('a'+i%26))+strings.Repeat("x", i)] = entriIzin{hingga: jam.Now().Add(-time.Second)}
	}

	c.simpan("baru", entriIzin{hingga: jam.Now().Add(time.Minute)}, c.gen)

	assert.Len(t, c.entri, 1, "entri kedaluwarsa disapu, entri baru tersimpan")
	assert.Contains(t, c.entri, "baru")
}

func TestPermissionResolver_NilTanpaPenyimpananBasisData(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))
	assert.Nil(t, uc.PermissionResolver(), "repositori tiruan tanpa izin: tidak ada resolver (hanya tes unit)")

	s := newIzinSuite(t, time.Second)
	assert.NotNil(t, s.uc.PermissionResolver())
}

// ── Token membawa gabungan izin peran + izin langsung ─────────────────

func TestMintAccess_TokenMembawaGabunganIzinPerandanIzinLangsungAktif(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	sampai := s.jam.Now().Add(time.Hour)
	_, err := s.uc.GrantUserPermission(context.Background(), GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", BerlakuSampai: &sampai, Alasan: "uji"}, uid("admin-1"))
	require.NoError(t, err)

	tk, err := s.uc.mintAccess(context.Background(), &domain.Pengguna{ID: "mhs-1", Peran: "OPS_MAHASISWA"}, "sid-1")
	require.NoError(t, err)
	claims, err := s.uc.token.VerifyToken(tk)
	require.NoError(t, err)

	assert.ElementsMatch(t, append(IzinBawaan("OPS_MAHASISWA"), "kpi.lihat"), claims.Permissions)
}

func TestMintAccess_GalatIzinMenggagalkanPenerbitan(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	s.izin.mu.Lock()
	s.izin.errEfektif = errors.New("db mati")
	s.izin.mu.Unlock()

	_, err := s.uc.mintAccess(context.Background(), &domain.Pengguna{ID: "mhs-1", Peran: "OPS_MAHASISWA"}, "sid-1")
	assert.Error(t, err, "token tidak boleh terbit dengan izin yang tidak diketahui")
}

// ── Matriks peran ─────────────────────────────────────────────────────

func TestListRoles_DariBasisDataMembawaVersiDanBawaan(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	_, err := s.uc.UpdateRolePermissions(context.Background(), "OPS_MAHASISWA", 1, []string{"harga.input", "kpi.lihat"}, uid("admin-1"))
	require.NoError(t, err)

	roles, err := s.uc.ListRoles(context.Background())

	require.NoError(t, err)
	require.Len(t, roles, 14)
	var mhs RoleInfo
	for i, r := range roles {
		assert.Equal(t, roleOrder[i], r.ID)
		if r.ID == "OPS_MAHASISWA" {
			mhs = r
		}
	}
	assert.Equal(t, []string{"harga.input", "kpi.lihat"}, mhs.Permissions)
	assert.Equal(t, rolePermissions["OPS_MAHASISWA"], mhs.Bawaan, "bawaan tetap matriks di kode")
	assert.EqualValues(t, 2, mhs.Versi)
	assert.NotNil(t, mhs.DiperbaruiPada)
	assert.Equal(t, "Agen Lapangan Mahasiswa", mhs.Label)
}

func TestListPermissions_UrutanKatalogDiKode(t *testing.T) {
	s := newIzinSuite(t, time.Hour)

	perms, err := s.uc.ListPermissions(context.Background())

	require.NoError(t, err)
	require.Len(t, perms, 35)
	for i, p := range perms {
		assert.Equal(t, katalogIzin[i].Nama, p.ID)
		assert.Equal(t, katalogIzin[i].Kelompok, p.Kelompok)
	}
}

func TestUpdateRolePermissions_BerhasilMenulisAuditSebelumSesudahDanMembuangCache(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat")

	info, err := s.uc.UpdateRolePermissions(context.Background(), "OPS_MAHASISWA", 1, []string{"kpi.lihat", "harga.input", "harga.input"}, uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, []string{"harga.input", "kpi.lihat"}, info.Permissions)
	assert.EqualValues(t, 2, info.Versi)
	assert.Contains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "perubahan dari proses ini berlaku seketika, tanpa menunggu TTL")
	assert.NotContains(t, s.efektif(t, "mhs-1"), "tugas.lihat_sendiri", "izin yang dicabut dari peran ikut hilang")

	entry, ok := s.audit.Last()
	require.True(t, ok)
	assert.Equal(t, "UPDATE_ROLE_PERMISSIONS", entry.Aksi)
	assert.Equal(t, uid("admin-1"), entry.AktorID)
	assert.Equal(t, "OPS_MAHASISWA", entry.EntitasID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
	assert.Equal(t, []string{"harga.input", "tugas.lihat_sendiri"}, entry.Sebelum.(map[string]any)["izin"])
	assert.Equal(t, []string{"harga.input", "kpi.lihat"}, entry.Sesudah.(map[string]any)["izin"])
	assert.Equal(t, []string{"kpi.lihat"}, entry.Metadata["ditambah"])
	assert.Equal(t, []string{"tugas.lihat_sendiri"}, entry.Metadata["dicabut"])
}

func TestUpdateRolePermissions_TanpaPerubahanTidakMenulisAuditAtauMenaikkanVersi(t *testing.T) {
	s := newIzinSuite(t, time.Hour)

	info, err := s.uc.UpdateRolePermissions(context.Background(), "OPS_MAHASISWA", 1, IzinBawaan("OPS_MAHASISWA"), uid("admin-1"))

	require.NoError(t, err)
	assert.EqualValues(t, 1, info.Versi)
	assert.Empty(t, s.audit.Entries())
}

func TestUpdateRolePermissions_VersiBasiDitolakDenganKeadaanSaatIni(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	_, err := s.uc.UpdateRolePermissions(context.Background(), "OPS_MAHASISWA", 1, []string{"harga.input"}, uid("admin-1"))
	require.NoError(t, err)

	// Admin kedua masih memegang versi 1.
	_, err = s.uc.UpdateRolePermissions(context.Background(), "OPS_MAHASISWA", 1, []string{"kpi.lihat"}, uid("admin-2"))

	require.ErrorIs(t, err, ErrVersiPeranBerubah)
	var konflik *KonflikVersiPeran
	require.ErrorAs(t, err, &konflik)
	assert.EqualValues(t, 2, konflik.Saat.Versi)
	assert.Equal(t, []string{"harga.input"}, konflik.Saat.Permissions, "tidak ada yang ditimpa")
	assert.Len(t, s.audit.Entries(), 1, "perubahan yang ditolak tidak dicatat sebagai berhasil")
}

func TestUpdateRolePermissions_Penolakan(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	ctx := context.Background()

	_, err := s.uc.UpdateRolePermissions(ctx, "SUPERADMIN", 1, nil, uid("admin-1"))
	assert.ErrorIs(t, err, ErrRoleNotFound)

	_, err = s.uc.UpdateRolePermissions(ctx, "OPS_MAHASISWA", 1, []string{"harga.input", "izin.palsu", "izin.palsu"}, uid("admin-1"))
	require.ErrorIs(t, err, ErrIzinTidakDikenal)
	assert.Contains(t, err.Error(), "izin.palsu", "nama izin asing disebut sekali")
	assert.Equal(t, 1, strings.Count(err.Error(), "izin.palsu"))

	// Menghapus pengguna.kelola dari satu-satunya peran yang memegangnya.
	_, err = s.uc.UpdateRolePermissions(ctx, "ADMIN_SISTEM", 1, []string{"audit.lihat"}, uid("admin-1"))
	assert.ErrorIs(t, err, ErrPemegangTerakhir)

	assert.Empty(t, s.audit.Entries())
	roles, _ := s.uc.ListRoles(ctx)
	for _, r := range roles {
		assert.ElementsMatch(t, IzinBawaan(r.ID), r.Permissions, "tidak ada perubahan yang bocor: %s", r.ID)
	}
}

func TestUpdateRolePermissions_PeranBukanPemegangBolehKehilanganIzinLain(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	_, err := s.uc.UpdateRolePermissions(context.Background(), "ADMIN_SISTEM", 1, []string{"pengguna.kelola"}, uid("admin-1"))
	require.NoError(t, err, "selama pengguna.kelola tetap dipegang, izin lain boleh dicabut")
}

func TestUpdateRolePermissions_TanpaPenyimpananBasisData(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))
	_, err := uc.UpdateRolePermissions(context.Background(), "OPS_MAHASISWA", 1, nil, "x")
	assert.ErrorIs(t, err, ErrRBACTidakTersedia)
}

// ── Izin langsung ─────────────────────────────────────────────────────

func TestGrantUserPermission_BerhasilBerlakuLalu_KedaluwarsaMenurutJam(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	ctx := context.Background()
	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat")
	sampai := s.jam.Now().Add(10 * time.Minute)

	info, err := s.uc.GrantUserPermission(ctx, GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", BerlakuSampai: &sampai, Alasan: "  Proyek KPI triwulan  "}, uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, "kpi.lihat", info.Izin)
	assert.Equal(t, "Proyek KPI triwulan", info.Alasan, "alasan dirapikan")
	assert.True(t, info.Aktif)
	require.NotNil(t, info.DiberikanOlehNama)
	assert.Equal(t, "Admin Satu", *info.DiberikanOlehNama)
	assert.Contains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "cache pengguna dibuang: berlaku seketika")

	s.jam.Maju(11 * time.Minute)
	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "kedaluwarsa")

	daftar, err := s.uc.ListUserPermissions(ctx, "mhs-1")
	require.NoError(t, err)
	require.Len(t, daftar.Langsung, 1)
	assert.False(t, daftar.Langsung[0].Aktif, "baris kedaluwarsa tetap terlihat sebagai riwayat")
	assert.NotContains(t, daftar.Efektif, "kpi.lihat")
}

func TestGrantUserPermission_AuditSesudahDanMetadata(t *testing.T) {
	s := newIzinSuite(t, time.Hour)

	_, err := s.uc.GrantUserPermission(context.Background(), GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "Tugas khusus"}, uid("admin-1"))

	require.NoError(t, err)
	entry, ok := s.audit.Last()
	require.True(t, ok)
	assert.Equal(t, "GRANT_USER_PERMISSION", entry.Aksi)
	assert.Equal(t, uid("admin-1"), entry.AktorID)
	assert.Equal(t, "mhs-1", entry.EntitasID)
	assert.Equal(t, "kpi.lihat", entry.Metadata["izin"])
	assert.Nil(t, entry.Sebelum)
	sesudah := entry.Sesudah.(map[string]any)
	assert.Equal(t, "Tugas khusus", sesudah["alasan"])
	assert.Equal(t, uid("admin-1"), sesudah["diberikan_oleh"])
	assert.NotContains(t, sesudah, "berlaku_sampai", "tanpa batas")
}

func TestGrantUserPermission_Validasi(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	ctx := context.Background()
	lampau := s.jam.Now().Add(-time.Minute)
	kini := s.jam.Now()

	cases := []struct {
		nama string
		in   GrantInput
		want error
	}{
		{"izin di luar katalog", GrantInput{UserID: "mhs-1", Izin: "izin.palsu", Alasan: "x"}, ErrIzinTidakDikenal},
		{"alasan kosong", GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: ""}, ErrAlasanWajib},
		{"alasan hanya spasi", GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: " \t\n "}, ErrAlasanWajib},
		{"alasan terlalu panjang", GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: strings.Repeat("a", 501)}, ErrAlasanTerlaluPanjang},
		{"berlaku sampai di masa lalu", GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "x", BerlakuSampai: &lampau}, ErrBerlakuSampaiLampau},
		{"berlaku sampai sekarang", GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "x", BerlakuSampai: &kini}, ErrBerlakuSampaiLampau},
		{"pengguna tidak ada", GrantInput{UserID: "hantu", Izin: "kpi.lihat", Alasan: "x"}, ErrUserNotFound},
		{"sudah dimiliki lewat peran", GrantInput{UserID: "mhs-1", Izin: "harga.input", Alasan: "x"}, ErrIzinDariPeran},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			_, err := s.uc.GrantUserPermission(ctx, tc.in, uid("admin-1"))
			assert.ErrorIs(t, err, tc.want)
		})
	}
	assert.Empty(t, s.audit.Entries(), "pemberian yang ditolak tidak dicatat")
	assert.Empty(t, s.izin.langsung["mhs-1"])
}

func TestGrantUserPermission_AlasanPanjangPasTerbatasDalamKarakterBukanByte(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	alasan := strings.Repeat("é", 500) // 500 karakter, 1000 byte

	_, err := s.uc.GrantUserPermission(context.Background(), GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: alasan}, uid("admin-1"))

	assert.NoError(t, err)
}

func TestGrantUserPermission_IzinLangsungAktifDitolakLaluKedaluwarsaDiganti(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	ctx := context.Background()
	sampai := s.jam.Now().Add(time.Minute)
	_, err := s.uc.GrantUserPermission(ctx, GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", BerlakuSampai: &sampai, Alasan: "pertama"}, uid("admin-1"))
	require.NoError(t, err)

	_, err = s.uc.GrantUserPermission(ctx, GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "kedua"}, uid("admin-1"))
	assert.ErrorIs(t, err, ErrIzinLangsungAda, "izin langsung yang masih berlaku tidak ditimpa diam-diam")

	s.jam.Maju(2 * time.Minute)
	_, err = s.uc.GrantUserPermission(ctx, GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "kedua"}, uid("admin-1"))
	require.NoError(t, err, "baris yang sudah kedaluwarsa boleh diganti")

	entry, _ := s.audit.Last()
	require.NotNil(t, entry.Sebelum, "audit membawa baris yang digantikan")
	assert.Equal(t, "pertama", entry.Sebelum.(map[string]any)["alasan"])
	assert.Equal(t, "kedua", entry.Sesudah.(map[string]any)["alasan"])
}

func TestRevokeUserPermission_BerhasilMembuangCacheDanMenulisAudit(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	ctx := context.Background()
	_, err := s.uc.GrantUserPermission(ctx, GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "uji"}, uid("admin-1"))
	require.NoError(t, err)
	require.Contains(t, s.efektif(t, "mhs-1"), "kpi.lihat")

	require.NoError(t, s.uc.RevokeUserPermission(ctx, "mhs-1", "kpi.lihat", uid("admin-1")))

	assert.NotContains(t, s.efektif(t, "mhs-1"), "kpi.lihat", "pencabutan berlaku seketika di proses ini")
	entry, _ := s.audit.Last()
	assert.Equal(t, "REVOKE_USER_PERMISSION", entry.Aksi)
	assert.Equal(t, "mhs-1", entry.EntitasID)
	assert.Equal(t, "kpi.lihat", entry.Sebelum.(map[string]any)["izin"])
	assert.Nil(t, entry.Sesudah)

	assert.ErrorIs(t, s.uc.RevokeUserPermission(ctx, "mhs-1", "kpi.lihat", uid("admin-1")), ErrIzinLangsungTidakAda)
}

func TestListUserPermissions(t *testing.T) {
	s := newIzinSuite(t, time.Hour)
	ctx := context.Background()
	_, err := s.uc.GrantUserPermission(ctx, GrantInput{UserID: "mhs-1", Izin: "kpi.lihat", Alasan: "uji"}, uid("admin-1"))
	require.NoError(t, err)

	info, err := s.uc.ListUserPermissions(ctx, "mhs-1")

	require.NoError(t, err)
	assert.Equal(t, "OPS_MAHASISWA", info.Peran)
	assert.Equal(t, []string{"harga.input", "tugas.lihat_sendiri"}, info.IzinPeran)
	assert.Equal(t, []string{"harga.input", "kpi.lihat", "tugas.lihat_sendiri"}, info.Efektif)
	require.Len(t, info.Langsung, 1)
	assert.True(t, info.Langsung[0].Aktif)

	_, err = s.uc.ListUserPermissions(ctx, "hantu")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestIzinLangsung_TanpaPenyimpananBasisData(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))
	ctx := context.Background()

	_, err := uc.GrantUserPermission(ctx, GrantInput{UserID: "a", Izin: "kpi.lihat", Alasan: "x"}, "b")
	assert.ErrorIs(t, err, ErrRBACTidakTersedia)
	assert.ErrorIs(t, uc.RevokeUserPermission(ctx, "a", "kpi.lihat", "b"), ErrRBACTidakTersedia)
	_, err = uc.ListUserPermissions(ctx, "a")
	assert.ErrorIs(t, err, ErrRBACTidakTersedia)
}

// ── Penjaga pemegang terakhir pada perubahan akun ─────────────────────

func TestPerubahanAkun_PemegangTerakhirDitolakDanCacheTidakDibuangSiaSia(t *testing.T) {
	ctx := context.Background()
	admin := &domain.Pengguna{ID: uid("admin-1"), Peran: "ADMIN_SISTEM", Status: "AKTIF", Email: "a@x.id"}

	t.Run("AssignRole", func(t *testing.T) {
		s := newIzinSuite(t, time.Hour)
		s.repo.On("PatchUser", mock.Anything, uid("admin-1"), mock.Anything).Return(domain.ErrPemegangTerakhir)
		_, err := s.uc.AssignRole(ctx, uid("admin-1"), "OPS_MAHASISWA", uid("admin-1"))
		assert.ErrorIs(t, err, ErrPemegangTerakhir)
		assert.Empty(t, s.audit.Entries())
	})
	t.Run("UpdateUser peran", func(t *testing.T) {
		s := newIzinSuite(t, time.Hour)
		s.repo.On("GetUserByID", mock.Anything, uid("admin-1")).Return(admin, nil)
		s.repo.On("PatchUser", mock.Anything, uid("admin-1"), mock.Anything).Return(domain.ErrPemegangTerakhir)
		peran := "OPS_MAHASISWA"
		_, err := s.uc.UpdateUser(ctx, uid("admin-1"), UserPatch{Peran: &peran}, uid("admin-1"))
		assert.ErrorIs(t, err, ErrPemegangTerakhir)
	})
	t.Run("UpdateUser status", func(t *testing.T) {
		s := newIzinSuite(t, time.Hour)
		s.repo.On("GetUserByID", mock.Anything, uid("admin-1")).Return(admin, nil)
		s.repo.On("PatchUser", mock.Anything, uid("admin-1"), mock.Anything).Return(domain.ErrPemegangTerakhir)
		status := "NONAKTIF"
		_, err := s.uc.UpdateUser(ctx, uid("admin-1"), UserPatch{Status: &status}, uid("admin-2"))
		assert.ErrorIs(t, err, ErrPemegangTerakhir)
	})
	t.Run("DeleteUser", func(t *testing.T) {
		s := newIzinSuite(t, time.Hour)
		s.repo.On("GetUserByID", mock.Anything, uid("admin-1")).Return(admin, nil)
		s.repo.On("DeleteUser", mock.Anything, uid("admin-1")).Return(domain.ErrPemegangTerakhir)
		assert.ErrorIs(t, s.uc.DeleteUser(ctx, uid("admin-1"), uid("admin-2")), ErrPemegangTerakhir)
	})
	t.Run("Bulk nonaktifkan", func(t *testing.T) {
		s := newIzinSuite(t, time.Hour)
		s.repo.On("BulkUpdateUsers", mock.Anything, []string{uid("admin-1")}, domain.BulkNonaktifkan).Return(int64(0), domain.ErrPemegangTerakhir)
		_, err := s.uc.BulkUserAction(ctx, []string{uid("admin-1")}, "nonaktifkan", uid("admin-2"))
		assert.ErrorIs(t, err, ErrPemegangTerakhir)
		assert.Empty(t, s.audit.Entries())
	})
}

func TestPerubahanAkun_Berhasil_MembuangCacheIzinPengguna(t *testing.T) {
	ctx := context.Background()
	s := newIzinSuite(t, time.Hour)
	require.Contains(t, s.efektif(t, "mhs-1"), "harga.input")
	before := s.panggilan()

	s.repo.On("PatchUser", mock.Anything, "mhs-1", mock.Anything).Return(nil)
	s.repo.On("GetUserByID", mock.Anything, "mhs-1").Return(&domain.Pengguna{ID: "mhs-1", Peran: "OPS_MAHASISWA", Status: "AKTIF", Email: "m@x.id"}, nil)
	s.repo.On("RevokeAllUserRefreshTokens", mock.Anything, "mhs-1").Return(nil)
	s.repo.On("DeleteUser", mock.Anything, "mhs-1").Return(nil)
	s.repo.On("BulkUpdateUsers", mock.Anything, []string{"mhs-1"}, domain.BulkAktifkan).Return(int64(1), nil)

	_, err := s.uc.AssignRole(ctx, "mhs-1", "MANAJER_DATA", uid("admin-1"))
	require.NoError(t, err)
	s.efektif(t, "mhs-1")
	assert.Equal(t, before+1, s.panggilan(), "AssignRole membuang cache pengguna")

	nama := "Baru"
	_, err = s.uc.UpdateUser(ctx, "mhs-1", UserPatch{Nama: &nama}, uid("admin-1"))
	require.NoError(t, err)
	s.efektif(t, "mhs-1")
	assert.Equal(t, before+2, s.panggilan(), "UpdateUser membuang cache pengguna")

	_, err = s.uc.BulkUserAction(ctx, []string{"mhs-1"}, "aktifkan", uid("admin-1"))
	require.NoError(t, err)
	s.efektif(t, "mhs-1")
	assert.Equal(t, before+3, s.panggilan(), "aksi massal membuang cache pengguna")

	require.NoError(t, s.uc.DeleteUser(ctx, "mhs-1", uid("admin-1")))
	s.efektif(t, "mhs-1")
	assert.Equal(t, before+4, s.panggilan(), "DeleteUser membuang cache pengguna")
}
