package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/token"
	"gorm.io/gorm"
)

// RBAC berbasis basis data (02-C, Opsi B). Izin efektif seorang pengguna = izin perannya (role_permissions) ∪
// izin langsung yang masih berlaku (user_permissions: berlaku_sampai NULL atau di masa depan).
//
// Batas waktu penegakan. JWTAuth menegakkan izin efektif dari resolver ini, bukan dari klaim token:
//   - perubahan yang dibuat lewat proses API ini (matriks peran, beri/cabut izin langsung, ganti peran,
//     nonaktif, hapus) membuang cache terkait seketika: berlaku pada request berikutnya;
//   - perubahan dari proses lain (replika API lain, SQL langsung) berlaku paling lambat IzinCacheTTL
//     (bawaan 15 detik) kemudian;
//   - izin langsung yang kedaluwarsa berlaku tepat pada waktunya: entri cache tidak pernah hidup melewati
//     berlaku_sampai terdekat (sisa waktu dihitung jam basis data).
//
// Klaim `permissions` pada token tetap dipakai UI web; token yang klaimnya berbeda dari izin efektif dijawab
// 401 PERMISSIONS_CHANGED (pkg/middleware) dan klien memperbaruinya lewat refresh senyap 01-E.

// DefaultIzinCacheTTL adalah umur maksimum satu entri cache izin efektif.
const DefaultIzinCacheTTL = 15 * time.Second

// maxEntriCacheIzin membatasi ukuran peta; lewat batas ini entri yang sudah lewat waktu disapu.
const maxEntriCacheIzin = 10000

const (
	maxAlasanIzin = 500
)

var (
	ErrPemegangTerakhir     = domain.ErrPemegangTerakhir
	ErrVersiPeranBerubah    = domain.ErrVersiPeranBerubah
	ErrIzinTidakDikenal     = domain.ErrIzinTidakDikenal
	ErrIzinDariPeran        = domain.ErrIzinDariPeran
	ErrIzinLangsungAda      = domain.ErrIzinLangsungAda
	ErrIzinLangsungTidakAda = domain.ErrIzinLangsungTidakAda

	ErrAlasanWajib          = errors.New("alasan wajib diisi")
	ErrAlasanTerlaluPanjang = fmt.Errorf("alasan maksimal %d karakter", maxAlasanIzin)
	ErrBerlakuSampaiLampau  = errors.New("masa berlaku harus di masa depan")
	ErrRBACTidakTersedia    = errors.New("penyimpanan izin berbasis basis data tidak tersedia")
)

// KonflikVersiPeran membawa keadaan peran saat ini pada ErrVersiPeranBerubah supaya klien dapat menampilkannya.
type KonflikVersiPeran struct {
	Saat RoleInfo
}

func (e *KonflikVersiPeran) Error() string { return ErrVersiPeranBerubah.Error() }
func (e *KonflikVersiPeran) Unwrap() error { return ErrVersiPeranBerubah }

// ── Cache izin efektif ────────────────────────────────────────────────

type entriIzin struct {
	izin   []string
	hingga time.Time
}

// izinCache menyimpan izin efektif per pengguna dengan umur pendek dan pembuangan eksplisit. Mengimplementasikan
// token.PermissionResolver.
type izinCache struct {
	repo domain.IzinRepository
	ttl  time.Duration
	now  func() time.Time

	mu    sync.Mutex
	entri map[string]entriIzin
	// gen naik pada setiap pembuangan: pembacaan yang dimulai sebelum pembuangan tidak boleh menyimpan hasilnya
	// (bisa saja sudah basi), sehingga tidak ada entri usang yang bertahan sampai ttl habis.
	gen uint64
}

var _ token.PermissionResolver = (*izinCache)(nil)

func newIzinCache(repo domain.IzinRepository, ttl time.Duration, now func() time.Time) *izinCache {
	if now == nil {
		now = time.Now
	}
	return &izinCache{repo: repo, ttl: ttl, now: now, entri: map[string]entriIzin{}}
}

// Permissions mengembalikan izin efektif terkini userID dari cache atau, bila tidak ada/kedaluwarsa, dari basis
// data. Tidak pernah menyajikan entri basi saat basis data gagal (gagal-tutup).
func (c *izinCache) Permissions(ctx context.Context, userID string) ([]string, error) {
	sekarang := c.now()
	c.mu.Lock()
	e, ada := c.entri[userID]
	gen := c.gen
	c.mu.Unlock()
	if ada && sekarang.Before(e.hingga) {
		return slices.Clone(e.izin), nil
	}

	hasil, err := c.repo.IzinEfektifPengguna(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.hapus(userID)
			return nil, token.ErrPrincipalNotFound
		}
		return nil, fmt.Errorf("izin efektif: %w", err)
	}

	if c.ttl > 0 {
		umur := c.ttl
		if hasil.KedaluwarsaDalam > 0 && hasil.KedaluwarsaDalam < umur {
			umur = hasil.KedaluwarsaDalam
		}
		c.simpan(userID, entriIzin{izin: slices.Clone(hasil.Izin), hingga: sekarang.Add(umur)}, gen)
	}
	return slices.Clone(hasil.Izin), nil
}

func (c *izinCache) simpan(userID string, e entriIzin, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen {
		return
	}
	if len(c.entri) >= maxEntriCacheIzin {
		sekarang := c.now()
		for id, lama := range c.entri {
			if !sekarang.Before(lama.hingga) {
				delete(c.entri, id)
			}
		}
		if len(c.entri) >= maxEntriCacheIzin {
			return
		}
	}
	c.entri[userID] = e
}

func (c *izinCache) hapus(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entri, userID)
}

// buangPengguna membuang entri milik pengguna tertentu.
func (c *izinCache) buangPengguna(ids ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	for _, id := range ids {
		delete(c.entri, id)
	}
}

// buangSemua membuang seluruh entri (matriks peran berubah: semua pengguna peran itu terdampak).
func (c *izinCache) buangSemua() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	clear(c.entri)
}

// PermissionResolver mengembalikan resolver izin efektif untuk dipasang pada token.Manager
// (SetPermissionResolver); nil bila UseCase dibuat tanpa penyimpanan izin berbasis basis data (hanya tes unit).
func (uc *UseCase) PermissionResolver() token.PermissionResolver {
	if uc.izinCache == nil {
		return nil
	}
	return uc.izinCache
}

// bersihkanCacheIzin membuang cache izin pengguna yang perannya, statusnya, atau izin langsungnya berubah.
func (uc *UseCase) bersihkanCacheIzin(ids ...string) {
	if uc.izinCache != nil {
		uc.izinCache.buangPengguna(ids...)
	}
}

// bersihkanSemuaCacheIzin membuang seluruh cache izin (matriks peran berubah).
func (uc *UseCase) bersihkanSemuaCacheIzin() {
	if uc.izinCache != nil {
		uc.izinCache.buangSemua()
	}
}

// izinPengguna adalah izin efektif yang dibawa token baru. Memakai cache yang sama dengan JWTAuth supaya token
// yang baru terbit tidak langsung ditolak sebagai basi. Tanpa penyimpanan basis data (tes unit) memakai
// matriks bawaan.
func (uc *UseCase) izinPengguna(ctx context.Context, user *domain.Pengguna) ([]string, error) {
	if uc.izinCache == nil {
		return uc.getPermissionsForRole(user.Peran), nil
	}
	izin, err := uc.izinCache.Permissions(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("izin pengguna: %w", err)
	}
	return izin, nil
}

// ── Matriks peran x izin ──────────────────────────────────────────────

// PermissionInfo adalah satu izin di katalog.
type PermissionInfo struct {
	ID        string
	Deskripsi string
	Kelompok  string
}

// ListRoles mengembalikan 14 peran beserta izinnya yang berlaku (basis data), versi untuk kunci optimistis, dan
// izin bawaan sebagai pembanding.
func (uc *UseCase) ListRoles(ctx context.Context) ([]RoleInfo, error) {
	if uc.izin == nil {
		roles := make([]RoleInfo, 0, len(roleOrder))
		for _, d := range DaftarPeran() {
			roles = append(roles, RoleInfo{ID: d.Kunci, Label: d.Label, Lapis: d.Lapis, Permissions: IzinBawaan(d.Kunci), Bawaan: IzinBawaan(d.Kunci)})
		}
		return roles, nil
	}
	peran, err := uc.izin.ListPeran(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	roles := make([]RoleInfo, len(peran))
	for i, p := range peran {
		roles[i] = roleInfoDari(p)
	}
	return roles, nil
}

func roleInfoDari(p domain.PeranDetail) RoleInfo {
	pembaruan := p.DiperbaruiPada
	return RoleInfo{
		ID: p.Kunci, Label: p.Label, Lapis: p.Lapis, Permissions: p.Izin, Bawaan: IzinBawaan(p.Kunci),
		Versi: p.Versi, DiperbaruiPada: &pembaruan, DiperbaruiOleh: p.DiperbaruiOleh,
	}
}

// ListPermissions mengembalikan katalog izin dalam urutan tampil di kode.
func (uc *UseCase) ListPermissions(ctx context.Context) ([]PermissionInfo, error) {
	if uc.izin == nil {
		out := make([]PermissionInfo, len(katalogIzin))
		for i, d := range katalogIzin {
			out[i] = PermissionInfo{ID: d.Nama, Deskripsi: d.Deskripsi, Kelompok: d.Kelompok}
		}
		return out, nil
	}
	izin, err := uc.izin.ListIzin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	urutan := func(nama string) int {
		for i, d := range katalogIzin {
			if d.Nama == nama {
				return i
			}
		}
		return len(katalogIzin)
	}
	slices.SortStableFunc(izin, func(a, b domain.Izin) int { return urutan(a.Nama) - urutan(b.Nama) })
	out := make([]PermissionInfo, len(izin))
	for i, p := range izin {
		out[i] = PermissionInfo{ID: p.Nama, Deskripsi: p.Deskripsi, Kelompok: p.Kelompok}
	}
	return out, nil
}

// UpdateRolePermissions mengganti seluruh izin satu peran. versi adalah versi yang dibaca klien (kunci
// optimistis): bila matriks sudah diubah admin lain hasilnya *KonflikVersiPeran (membungkus ErrVersiPeranBerubah)
// dan tidak ada yang ditimpa. Menolak izin di luar katalog dan perubahan yang membuat sistem kehilangan pemegang
// terakhir pengguna.kelola (ErrPemegangTerakhir). Mengganti dengan himpunan yang sama tidak menulis apa pun.
func (uc *UseCase) UpdateRolePermissions(ctx context.Context, peran string, versi int64, izin []string, actorID string) (*RoleInfo, error) {
	if uc.izin == nil {
		return nil, ErrRBACTidakTersedia
	}
	if !PeranDikenal(peran) {
		return nil, ErrRoleNotFound
	}
	if err := validasiNamaIzin(izin); err != nil {
		return nil, err
	}

	hasil, err := uc.izin.SetIzinPeran(ctx, domain.SetIzinPeran{Peran: peran, Versi: versi, Izin: izin, Oleh: actorID})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, ErrRoleNotFound
		case errors.Is(err, domain.ErrVersiPeranBerubah):
			konflik := &KonflikVersiPeran{}
			if roles, lerr := uc.ListRoles(ctx); lerr == nil {
				for _, r := range roles {
					if r.ID == peran {
						konflik.Saat = r
					}
				}
			}
			return nil, konflik
		case errors.Is(err, domain.ErrPemegangTerakhir):
			return nil, ErrPemegangTerakhir
		case errors.Is(err, domain.ErrIzinTidakDikenal):
			return nil, ErrIzinTidakDikenal
		}
		return nil, fmt.Errorf("update role permissions: %w", err)
	}

	info := roleInfoDari(domain.PeranDetail{Peran: hasil.Peran, Izin: hasil.Sesudah})
	if !hasil.Berubah {
		return &info, nil
	}

	// Semua pengguna peran ini terdampak: buang seluruh cache supaya perubahan berlaku pada request berikutnya.
	uc.bersihkanSemuaCacheIzin()

	tambah, cabut := selisih(hasil.Sebelum, hasil.Sesudah)
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "UPDATE_ROLE_PERMISSIONS", Entitas: "peran", EntitasID: peran, Hasil: audit.HasilBerhasil,
		Sebelum:  map[string]any{"izin": hasil.Sebelum, "versi": versi},
		Sesudah:  map[string]any{"izin": hasil.Sesudah, "versi": hasil.Peran.Versi},
		Metadata: map[string]any{"ditambah": tambah, "dicabut": cabut},
	})
	return &info, nil
}

// selisih menghitung izin yang ditambah dan dicabut antara dua himpunan.
func selisih(sebelum, sesudah []string) (tambah, cabut []string) {
	tambah, cabut = []string{}, []string{}
	for _, p := range sesudah {
		if !slices.Contains(sebelum, p) {
			tambah = append(tambah, p)
		}
	}
	for _, p := range sebelum {
		if !slices.Contains(sesudah, p) {
			cabut = append(cabut, p)
		}
	}
	return tambah, cabut
}

// validasiNamaIzin menolak nama di luar katalog (katalog didefinisikan di kode; UI tidak bisa menciptakan izin).
func validasiNamaIzin(izin []string) error {
	var asing []string
	for _, nama := range izin {
		if !IzinDikenal(nama) && !slices.Contains(asing, nama) {
			asing = append(asing, nama)
		}
	}
	if len(asing) > 0 {
		return fmt.Errorf("%w: %s", ErrIzinTidakDikenal, strings.Join(asing, ", "))
	}
	return nil
}

// ── Izin langsung per pengguna ────────────────────────────────────────

// IzinLangsungInfo adalah satu izin langsung milik pengguna.
type IzinLangsungInfo struct {
	Izin              string
	Deskripsi         string
	Kelompok          string
	BerlakuSampai     *time.Time
	Alasan            string
	DiberikanOleh     *string
	DiberikanOlehNama *string
	DiberikanPada     time.Time
	// Aktif false bila masa berlakunya sudah lewat (baris tersimpan sebagai riwayat sampai dicabut).
	Aktif bool
}

// IzinPenggunaInfo adalah ringkasan izin seorang pengguna: dari mana asalnya dan apa yang berlaku sekarang.
type IzinPenggunaInfo struct {
	Peran     string
	IzinPeran []string
	Langsung  []IzinLangsungInfo
	// Efektif adalah gabungan izin peran dan izin langsung yang masih berlaku, terurut.
	Efektif []string
}

func infoLangsung(d domain.IzinLangsungDetail, sekarang time.Time) IzinLangsungInfo {
	return IzinLangsungInfo{
		Izin: d.PermissionNama, Deskripsi: d.Deskripsi, Kelompok: d.Kelompok, BerlakuSampai: d.BerlakuSampai,
		Alasan: d.Alasan, DiberikanOleh: d.DiberikanOleh, DiberikanOlehNama: d.DiberikanOlehNama,
		DiberikanPada: d.DiberikanPada, Aktif: d.Aktif(sekarang),
	}
}

// ListUserPermissions mengembalikan izin peran dan izin langsung (aktif dan kedaluwarsa) seorang pengguna.
func (uc *UseCase) ListUserPermissions(ctx context.Context, userID string) (*IzinPenggunaInfo, error) {
	if uc.izin == nil {
		return nil, ErrRBACTidakTersedia
	}
	hasil, err := uc.izin.ListIzinLangsung(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("list user permissions: %w", err)
	}
	sekarang := uc.sekarang()
	info := &IzinPenggunaInfo{Peran: hasil.Peran, IzinPeran: hasil.IzinPeran, Langsung: make([]IzinLangsungInfo, len(hasil.Langsung))}
	efektif := slices.Clone(hasil.IzinPeran)
	for i, d := range hasil.Langsung {
		info.Langsung[i] = infoLangsung(d, sekarang)
		if info.Langsung[i].Aktif && !slices.Contains(efektif, d.PermissionNama) {
			efektif = append(efektif, d.PermissionNama)
		}
	}
	slices.Sort(efektif)
	if efektif == nil {
		efektif = []string{}
	}
	info.Efektif = efektif
	return info, nil
}

// GrantInput adalah permintaan memberi izin langsung. BerlakuSampai nil berarti tanpa batas.
type GrantInput struct {
	UserID        string
	Izin          string
	BerlakuSampai *time.Time
	Alasan        string
}

// GrantUserPermission memberi izin langsung. Aturan: izin harus ada di katalog; alasan wajib (maks. 500
// karakter); masa berlaku harus di masa depan. Izin yang sudah dimiliki lewat peran ditolak (ErrIzinDariPeran,
// bukan idempoten: pemberian itu tidak berarti apa pun dan akan tersisa sebagai izin "hantu" bila peran
// kelak berubah) dan izin langsung yang masih berlaku ditolak (ErrIzinLangsungAda: cabut dulu bila ingin mengubah
// masa berlaku atau alasan). Baris yang sudah kedaluwarsa diganti.
func (uc *UseCase) GrantUserPermission(ctx context.Context, in GrantInput, actorID string) (*IzinLangsungInfo, error) {
	if uc.izin == nil {
		return nil, ErrRBACTidakTersedia
	}
	if !IzinDikenal(in.Izin) {
		return nil, fmt.Errorf("%w: %s", ErrIzinTidakDikenal, in.Izin)
	}
	alasan := strings.TrimSpace(in.Alasan)
	switch {
	case alasan == "":
		return nil, ErrAlasanWajib
	case utf8.RuneCountInString(alasan) > maxAlasanIzin:
		return nil, ErrAlasanTerlaluPanjang
	}
	if in.BerlakuSampai != nil && !in.BerlakuSampai.After(uc.sekarang()) {
		return nil, ErrBerlakuSampaiLampau
	}

	sesudah, sebelumnya, err := uc.izin.BerikanIzinLangsung(ctx, domain.BerikanIzin{
		PenggunaID: in.UserID, Izin: in.Izin, BerlakuSampai: in.BerlakuSampai, Alasan: alasan, Oleh: actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, ErrUserNotFound
		case errors.Is(err, domain.ErrIzinTidakDikenal):
			return nil, ErrIzinTidakDikenal
		case errors.Is(err, domain.ErrIzinDariPeran):
			return nil, ErrIzinDariPeran
		case errors.Is(err, domain.ErrIzinLangsungAda):
			return nil, ErrIzinLangsungAda
		}
		return nil, fmt.Errorf("grant user permission: %w", err)
	}

	uc.bersihkanCacheIzin(in.UserID)

	info := infoLangsung(*sesudah, uc.sekarang())
	entry := audit.Entry{
		AktorID: actorID, Aksi: "GRANT_USER_PERMISSION", Entitas: "pengguna", EntitasID: in.UserID, Hasil: audit.HasilBerhasil,
		Sesudah:  snapshotLangsung(sesudah.IzinLangsung),
		Metadata: map[string]any{"izin": in.Izin},
	}
	if sebelumnya != nil {
		entry.Sebelum = snapshotLangsung(*sebelumnya)
	}
	uc.writeAudit(ctx, entry)
	return &info, nil
}

// RevokeUserPermission mencabut (menghapus) izin langsung, aktif maupun kedaluwarsa.
func (uc *UseCase) RevokeUserPermission(ctx context.Context, userID, izin, actorID string) error {
	if uc.izin == nil {
		return ErrRBACTidakTersedia
	}
	baris, err := uc.izin.CabutIzinLangsung(ctx, userID, izin)
	if err != nil {
		if errors.Is(err, domain.ErrIzinLangsungTidakAda) {
			return ErrIzinLangsungTidakAda
		}
		return fmt.Errorf("revoke user permission: %w", err)
	}
	uc.bersihkanCacheIzin(userID)
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "REVOKE_USER_PERMISSION", Entitas: "pengguna", EntitasID: userID, Hasil: audit.HasilBerhasil,
		Sebelum:  snapshotLangsung(*baris),
		Metadata: map[string]any{"izin": izin},
	})
	return nil
}

// snapshotLangsung adalah keadaan izin langsung yang disimpan di audit.
func snapshotLangsung(g domain.IzinLangsung) map[string]any {
	snap := map[string]any{"izin": g.PermissionNama, "alasan": g.Alasan, "diberikan_pada": g.DiberikanPada}
	if g.BerlakuSampai != nil {
		snap["berlaku_sampai"] = *g.BerlakuSampai
	}
	if g.DiberikanOleh != nil {
		snap["diberikan_oleh"] = *g.DiberikanOleh
	}
	return snap
}
