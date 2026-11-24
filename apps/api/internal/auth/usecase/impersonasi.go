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
	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Impersonasi peran dan akun (docs/impersonasi.md). Ringkasan jaminan keamanan yang ditegakkan di berkas ini:
//
//   - Pelaku asli selalu dari catatan impersonasi di basis data (impersonasi_sesi), tidak pernah dari body atau header
//     klien; klaim `imp` di token hanya penunjuk yang dicocokkan dengan catatan itu pada setiap request (penjaga).
//   - Superadmin (ADMIN_SISTEM) selalu berhak memulai; pihak lain hanya bila kebijakan mengizinkan pelaku dan sasarannya.
//     ADMIN_SISTEM tidak pernah boleh menjadi sasaran, dan tidak ada impersonasi ke diri sendiri.
//   - Tidak ada impersonasi bersarang: memulai saat sedang impersonasi adalah GANTI, dievaluasi terhadap pelaku asli.
//   - Setiap perubahan keadaan (mulai, ganti, berhenti, dipaksa berakhir, berakhir otomatis, kebijakan diubah) dan setiap
//     mutasi selama impersonasi tercatat di audit hash berantai atas nama pelaku asli.

var (
	ErrImpersonasiNonaktif       = errors.New("impersonasi sedang dinonaktifkan oleh kebijakan")
	ErrImpersonasiDitolak        = errors.New("anda tidak berhak melakukan impersonasi ke akun ini")
	ErrTargetTidakAda            = errors.New("akun sasaran tidak ditemukan atau tidak aktif")
	ErrTargetAmbigu              = errors.New("ada lebih dari satu akun berperan ini, pilih salah satu akun")
	ErrTargetWajib               = errors.New("sasaran impersonasi wajib diisi (target_pengguna_id atau target_peran)")
	ErrAlasanImpersonasiWajib    = errors.New("alasan impersonasi wajib diisi")
	ErrTidakAdaImpersonasi       = errors.New("sesi ini bukan sesi impersonasi")
	ErrImpersonasiTidakDitemukan = errors.New("sesi impersonasi tidak ditemukan atau sudah berakhir")
	ErrKebijakanTidakValid       = errors.New("kebijakan impersonasi tidak valid")
	ErrIzinEksklusif             = errors.New("izin impersonasi.kelola eksklusif milik administrator sistem")
	ErrIzinAdminTerkunci         = errors.New("administrator sistem selalu memegang seluruh izin; kolomnya tidak dapat dikurangi")
	ErrImpersonasiTidakTersedia  = errors.New("penyimpanan impersonasi tidak tersedia")

	ErrVersiKebijakanBerubah = domain.ErrVersiKebijakanBerubah
)

// KonflikKebijakan membawa kebijakan terkini pada ErrVersiKebijakanBerubah supaya klien dapat menampilkannya.
type KonflikKebijakan struct {
	Saat KebijakanInfo
}

func (e *KonflikKebijakan) Error() string { return domain.ErrVersiKebijakanBerubah.Error() }
func (e *KonflikKebijakan) Unwrap() error { return domain.ErrVersiKebijakanBerubah }

// ── Cache validitas ───────────────────────────────────────────────────

// impersonasiValid adalah hasil pemeriksaan catatan impersonasi yang boleh dipakai ulang sebentar.
type impersonasiValid struct {
	pelakuID, sasaranID, familyID string
	hingga                        time.Time
}

// impersonasiCache menyimpan catatan impersonasi yang sudah lolos pemeriksaan penuh (catatan hidup, kebijakan aktif,
// pelaku dan sasaran aktif dan berhak) selama umur cache izin, supaya request beruntun tidak membaca lima tabel setiap
// kali. Hanya keberhasilan yang disimpan; pembuangan eksplisit saat impersonasi berakhir, kebijakan atau akun berubah.
type impersonasiCache struct {
	ttl time.Duration
	now func() time.Time

	mu    sync.Mutex
	entri map[string]impersonasiValid
	gen   uint64
}

func newImpersonasiCache(ttl time.Duration, now func() time.Time) *impersonasiCache {
	if now == nil {
		now = time.Now
	}
	return &impersonasiCache{ttl: ttl, now: now, entri: map[string]impersonasiValid{}}
}

func (c *impersonasiCache) ambil(id string) (impersonasiValid, bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ada := c.entri[id]
	if ada && c.now().Before(e.hingga) {
		return e, true, c.gen
	}
	return impersonasiValid{}, false, c.gen
}

func (c *impersonasiCache) simpan(id string, e impersonasiValid, gen uint64) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen {
		return
	}
	if len(c.entri) >= maxEntriCacheIzin {
		sekarang := c.now()
		for k, lama := range c.entri {
			if !sekarang.Before(lama.hingga) {
				delete(c.entri, k)
			}
		}
		if len(c.entri) >= maxEntriCacheIzin {
			return
		}
	}
	e.hingga = c.now().Add(c.ttl)
	c.entri[id] = e
}

func (c *impersonasiCache) buang(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	delete(c.entri, id)
}

func (c *impersonasiCache) buangSemua() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	clear(c.entri)
}

// bersihkanCacheImpersonasi membuang seluruh cache validitas (kebijakan, peran, atau akun berubah).
func (uc *UseCase) bersihkanCacheImpersonasi() {
	if uc.impCache != nil {
		uc.impCache.buangSemua()
	}
}

// ── Izin eksklusif ────────────────────────────────────────────────────

// terapkanIzinEksklusif menegakkan dua aturan pada izin efektif, apa pun isi role_permissions atau user_permissions,
// sehingga keduanya tidak bergantung pada seeder atau pengeditan matriks:
//   - ADMIN_SISTEM selalu memegang seluruh katalog, termasuk izin yang ditambahkan kelak;
//   - domain.IzinImpersonasi tidak pernah dimiliki peran lain.
//
// Peran yang dinilai adalah peran akun pemilik izin. Selama impersonasi itu akun sasaran (bukan pelaku), dan ADMIN_SISTEM
// tidak pernah boleh menjadi sasaran, jadi aturan pertama tidak menaikkan hak sesi impersonasi.
func terapkanIzinEksklusif(peran string, izin []string) []string {
	if peran == domain.PeranSuperadmin {
		lengkap := append(slices.Clone(izin), semuaIzinKatalog()...)
		slices.Sort(lengkap)
		return slices.Compact(lengkap)
	}
	if slices.Contains(izin, domain.IzinImpersonasi) {
		return slices.DeleteFunc(slices.Clone(izin), func(p string) bool { return p == domain.IzinImpersonasi })
	}
	return izin
}

// ── Kebijakan ─────────────────────────────────────────────────────────

// KebijakanInfo adalah kebijakan impersonasi lengkap: pengaturan global dan daftar pelaku beserta sasarannya.
type KebijakanInfo struct {
	domain.Pengaturan
	Pelaku []domain.PelakuKebijakan
}

// KebijakanInput adalah isi PUT kebijakan (mengganti seluruhnya).
type KebijakanInput struct {
	Versi           int64
	Aktif           bool
	WajibAlasan     bool
	DurasiMaksMenit *int
	Pelaku          []domain.PelakuKebijakan
}

func (uc *UseCase) impTersedia() error {
	if uc.imp == nil {
		return ErrImpersonasiTidakTersedia
	}
	return nil
}

// pastikanSuperadmin memuat ulang akun dan memastikan peran ADMIN_SISTEM yang aktif: lapisan pertahanan ketiga di
// belakang izin di rute dan peran di middleware.
func (uc *UseCase) pastikanSuperadmin(ctx context.Context, userID string) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImpersonasiDitolak
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != "AKTIF" || user.Peran != domain.PeranSuperadmin {
		return nil, ErrImpersonasiDitolak
	}
	return user, nil
}

// pengaturan membaca pengaturan global; kebijakan bawaan bila belum pernah disimpan.
func (uc *UseCase) pengaturan(ctx context.Context) (domain.Pengaturan, error) {
	p, err := uc.imp.GetKebijakan(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.BawaanPengaturan(), nil
		}
		return domain.Pengaturan{}, err
	}
	return *p, nil
}

// GetKebijakanImpersonasi mengembalikan kebijakan; hanya superadmin.
func (uc *UseCase) GetKebijakanImpersonasi(ctx context.Context, actorID string) (*KebijakanInfo, error) {
	if err := uc.impTersedia(); err != nil {
		return nil, err
	}
	if _, err := uc.pastikanSuperadmin(ctx, actorID); err != nil {
		return nil, err
	}
	return uc.bacaKebijakan(ctx)
}

func (uc *UseCase) bacaKebijakan(ctx context.Context) (*KebijakanInfo, error) {
	p, err := uc.pengaturan(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca kebijakan impersonasi: %w", err)
	}
	pelaku, err := uc.imp.ListPelaku(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca pelaku impersonasi: %w", err)
	}
	return &KebijakanInfo{Pengaturan: p, Pelaku: pelaku}, nil
}

// SimpanKebijakanImpersonasi mengganti seluruh kebijakan; hanya superadmin. Versi usang menghasilkan *KonflikKebijakan.
func (uc *UseCase) SimpanKebijakanImpersonasi(ctx context.Context, actorID string, in KebijakanInput) (*KebijakanInfo, error) {
	if err := uc.impTersedia(); err != nil {
		return nil, err
	}
	if _, err := uc.pastikanSuperadmin(ctx, actorID); err != nil {
		return nil, err
	}
	pelaku, err := uc.validasiKebijakan(ctx, in)
	if err != nil {
		return nil, err
	}

	sebelum, err := uc.bacaKebijakan(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := uc.imp.SimpanKebijakan(ctx, domain.SimpanKebijakan{
		Versi: in.Versi, Aktif: in.Aktif, WajibAlasan: in.WajibAlasan, DurasiMaksMenit: in.DurasiMaksMenit,
		Pelaku: pelaku, Oleh: actorID,
	}); err != nil {
		if errors.Is(err, domain.ErrVersiKebijakanBerubah) {
			konflik := &KonflikKebijakan{}
			if saat, lerr := uc.bacaKebijakan(ctx); lerr == nil {
				konflik.Saat = *saat
			}
			return nil, konflik
		}
		return nil, fmt.Errorf("simpan kebijakan impersonasi: %w", err)
	}
	// Kebijakan baru berlaku pada request berikutnya: sesi yang pelaku atau sasarannya tidak lagi diizinkan berakhir.
	uc.bersihkanCacheImpersonasi()

	sesudah, err := uc.bacaKebijakan(ctx)
	if err != nil {
		return nil, err
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "IMPERSONASI_KEBIJAKAN_UBAH", Entitas: "impersonasi_kebijakan", EntitasID: "1",
		Hasil: audit.HasilBerhasil, Sebelum: snapshotKebijakan(sebelum), Sesudah: snapshotKebijakan(sesudah),
	})
	return sesudah, nil
}

// validasiKebijakan memeriksa dan menormalkan masukan: id akun kanonik, tanpa duplikat, tanpa ADMIN_SISTEM, akun ada.
func (uc *UseCase) validasiKebijakan(ctx context.Context, in KebijakanInput) ([]domain.PelakuKebijakan, error) {
	salah := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrKebijakanTidakValid, fmt.Sprintf(format, args...))
	}
	if in.DurasiMaksMenit != nil && (*in.DurasiMaksMenit < domain.MinDurasiImpersonasiMenit || *in.DurasiMaksMenit > domain.MaxDurasiImpersonasiMenit) {
		return nil, salah("durasi maksimal harus %d sampai %d menit, atau kosong untuk tanpa batas",
			domain.MinDurasiImpersonasiMenit, domain.MaxDurasiImpersonasiMenit)
	}
	if len(in.Pelaku) > domain.MaxPelakuKebijakan {
		return nil, salah("maksimal %d pelaku", domain.MaxPelakuKebijakan)
	}

	var semuaID []string
	normal := func(p domain.PihakKebijakan, peran string) (domain.PihakKebijakan, error) {
		switch p.Tipe {
		case domain.TipePeran:
			if !PeranDikenal(p.Peran) {
				return p, salah("peran %q tidak dikenal", p.Peran)
			}
			if p.Peran == domain.PeranSuperadmin {
				return p, salah("%s tidak boleh dipakai sebagai %s", domain.PeranSuperadmin, peran)
			}
			return domain.PihakKebijakan{Tipe: p.Tipe, Peran: p.Peran}, nil
		case domain.TipePengguna:
			id, ok := idx.NormalUUID(p.PenggunaID)
			if !ok {
				return p, salah("id akun %q bukan UUID", p.PenggunaID)
			}
			semuaID = append(semuaID, id)
			return domain.PihakKebijakan{Tipe: p.Tipe, PenggunaID: id}, nil
		}
		return p, salah("jenis %q tidak dikenal (PERAN atau PENGGUNA)", p.Tipe)
	}
	kunci := func(p domain.PihakKebijakan) string { return p.Tipe + ":" + p.Peran + p.PenggunaID }

	out := make([]domain.PelakuKebijakan, 0, len(in.Pelaku))
	seenPelaku := map[string]bool{}
	for _, p := range in.Pelaku {
		pihak, err := normal(p.PihakKebijakan, "pelaku")
		if err != nil {
			return nil, err
		}
		if seenPelaku[kunci(pihak)] {
			return nil, salah("pelaku duplikat: %s", kunci(pihak))
		}
		seenPelaku[kunci(pihak)] = true
		if len(p.Target) > domain.MaxTargetPerPelaku {
			return nil, salah("maksimal %d sasaran per pelaku", domain.MaxTargetPerPelaku)
		}
		pelaku := domain.PelakuKebijakan{PihakKebijakan: pihak, Aktif: p.Aktif, Target: make([]domain.PihakKebijakan, 0, len(p.Target))}
		seenTarget := map[string]bool{}
		for _, t := range p.Target {
			sasaran, err := normal(t, "sasaran")
			if err != nil {
				return nil, err
			}
			if seenTarget[kunci(sasaran)] {
				return nil, salah("sasaran duplikat untuk satu pelaku: %s", kunci(sasaran))
			}
			seenTarget[kunci(sasaran)] = true
			pelaku.Target = append(pelaku.Target, sasaran)
		}
		out = append(out, pelaku)
	}

	// Akun yang disebut harus ada, dan akun ADMIN_SISTEM tidak boleh pelaku (selalu berhak) maupun sasaran.
	if len(semuaID) > 0 {
		ringkas, err := uc.imp.RingkasPengguna(ctx, semuaID)
		if err != nil {
			return nil, fmt.Errorf("periksa akun kebijakan: %w", err)
		}
		peranAkun := make(map[string]string, len(ringkas))
		for _, r := range ringkas {
			peranAkun[r.ID] = r.Peran
		}
		for _, id := range semuaID {
			peran, ada := peranAkun[id]
			if !ada {
				return nil, salah("akun %s tidak ditemukan", id)
			}
			if peran == domain.PeranSuperadmin {
				return nil, salah("akun %s berperan %s: selalu berhak sebagai pelaku dan tidak boleh menjadi sasaran", id, domain.PeranSuperadmin)
			}
		}
	}
	return out, nil
}

func snapshotKebijakan(k *KebijakanInfo) map[string]any {
	ref := func(p domain.PihakKebijakan) string {
		if p.Tipe == domain.TipePeran {
			return "PERAN:" + p.Peran
		}
		return "PENGGUNA:" + p.PenggunaID
	}
	pelaku := make([]map[string]any, len(k.Pelaku))
	for i, p := range k.Pelaku {
		target := make([]string, len(p.Target))
		for j, t := range p.Target {
			target[j] = ref(t)
		}
		pelaku[i] = map[string]any{"pelaku": ref(p.PihakKebijakan), "aktif": p.Aktif, "target": target}
	}
	snap := map[string]any{"aktif": k.Aktif, "wajib_alasan": k.WajibAlasan, "versi": k.Versi, "pelaku": pelaku}
	if k.DurasiMaksMenit != nil {
		snap["durasi_maks_menit"] = *k.DurasiMaksMenit
	}
	return snap
}

// ── Hak ───────────────────────────────────────────────────────────────

// aturanSasaran adalah himpunan sasaran yang diizinkan bagi satu pelaku non-superadmin.
type aturanSasaran struct {
	peran      []string
	penggunaID []string
}

func (a aturanSasaran) mengizinkan(sasaran *domain.Pengguna) bool {
	return slices.Contains(a.peran, sasaran.Peran) || slices.Contains(a.penggunaID, sasaran.ID)
}

// aturanUntuk menggabungkan seluruh aturan AKTIF yang berlaku bagi pelaku (yang menunjuk akunnya dan yang menunjuk
// perannya). Superadmin tidak memakai aturan: berhak atas semua akun selain ADMIN_SISTEM.
func (uc *UseCase) aturanUntuk(ctx context.Context, pelaku *domain.Pengguna) (aturanSasaran, bool, error) {
	if pelaku.Peran == domain.PeranSuperadmin {
		return aturanSasaran{}, true, nil
	}
	rules, err := uc.imp.AturanPelaku(ctx, pelaku.ID, pelaku.Peran)
	if err != nil {
		return aturanSasaran{}, false, fmt.Errorf("aturan impersonasi: %w", err)
	}
	var a aturanSasaran
	for _, r := range rules {
		for _, t := range r.Target {
			switch t.Tipe {
			case domain.TipePeran:
				if !slices.Contains(a.peran, t.Peran) {
					a.peran = append(a.peran, t.Peran)
				}
			case domain.TipePengguna:
				if !slices.Contains(a.penggunaID, t.PenggunaID) {
					a.penggunaID = append(a.penggunaID, t.PenggunaID)
				}
			}
		}
	}
	return a, len(rules) > 0, nil
}

// bolehMenjadi memutuskan apakah pelaku boleh meng-impersonate sasaran menurut kebijakan terkini. Tidak memeriksa
// saklar global (itu urusan pemanggil).
func (uc *UseCase) bolehMenjadi(ctx context.Context, pelaku, sasaran *domain.Pengguna) error {
	if sasaran.ID == pelaku.ID || sasaran.Peran == domain.PeranSuperadmin {
		return ErrImpersonasiDitolak
	}
	aturan, berhak, err := uc.aturanUntuk(ctx, pelaku)
	if err != nil {
		return err
	}
	if !berhak {
		return ErrImpersonasiDitolak
	}
	if pelaku.Peran == domain.PeranSuperadmin || aturan.mengizinkan(sasaran) {
		return nil
	}
	return ErrImpersonasiDitolak
}

// pelakuAsli menentukan pelaku sebenarnya: bila token pemanggil adalah token impersonasi, pelaku dari catatan di basis
// data (bukan dari klaim), selain itu pemanggil sendiri. rec terisi hanya pada kasus pertama.
func (uc *UseCase) pelakuAsli(ctx context.Context, callerID string) (pelaku *domain.Pengguna, rec *domain.ImpersonasiSesi, err error) {
	userID := callerID
	if ic, ok := token.ImpersonationFrom(ctx); ok {
		rec, err = uc.imp.GetImpersonasi(ctx, ic.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil, token.ErrImpersonationEnded
			}
			return nil, nil, fmt.Errorf("baca impersonasi: %w", err)
		}
		if rec.BerakhirPada != nil || rec.PelakuID == nil || rec.FamilyID != strings.ToLower(currentSession(ctx)) {
			return nil, nil, token.ErrImpersonationEnded
		}
		userID = *rec.PelakuID
	}
	pelaku, err = uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrImpersonasiDitolak
		}
		return nil, nil, fmt.Errorf("get user: %w", err)
	}
	if pelaku.Status != "AKTIF" {
		return nil, nil, ErrImpersonasiDitolak
	}
	return pelaku, rec, nil
}

// ── Pemilih sasaran ───────────────────────────────────────────────────

// PeranSasaran adalah satu peran beserta akun AKTIF yang dapat diimpersonasi.
type PeranSasaran struct {
	Peran string
	Label string
	Lapis string
	Akun  []domain.AkunSasaran
}

// DaftarSasaran adalah jawaban pemilih sasaran bagi pemanggil.
type DaftarSasaran struct {
	// Boleh true bila pemanggil (atau pelaku asli, bila sedang impersonasi) berhak memulai dan kebijakan aktif.
	Boleh bool
	// Aktif adalah saklar global kebijakan.
	Aktif           bool
	WajibAlasan     bool
	DurasiMaksMenit *int
	Peran           []PeranSasaran
}

// DaftarSasaranImpersonasi menjawab siapa yang dapat di-impersonate oleh pemanggil. Tidak berhak bukan galat: Boleh false
// dan daftar kosong (klien memakainya untuk menyembunyikan menu).
func (uc *UseCase) DaftarSasaranImpersonasi(ctx context.Context, callerID string) (*DaftarSasaran, error) {
	if err := uc.impTersedia(); err != nil {
		return nil, err
	}
	p, err := uc.pengaturan(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca kebijakan impersonasi: %w", err)
	}
	hasil := &DaftarSasaran{Aktif: p.Aktif, WajibAlasan: p.WajibAlasan, DurasiMaksMenit: p.DurasiMaksMenit, Peran: []PeranSasaran{}}

	pelaku, _, err := uc.pelakuAsli(ctx, callerID)
	if err != nil {
		if errors.Is(err, ErrImpersonasiDitolak) || errors.Is(err, token.ErrImpersonationEnded) {
			return hasil, nil
		}
		return nil, err
	}
	aturan, berhak, err := uc.aturanUntuk(ctx, pelaku)
	if err != nil {
		return nil, err
	}
	if !berhak || !p.Aktif {
		return hasil, nil
	}
	hasil.Boleh = true

	akun, err := uc.imp.ListAkunSasaran(ctx, domain.FilterAkunSasaran{
		Semua: pelaku.Peran == domain.PeranSuperadmin, Peran: aturan.peran, PenggunaID: aturan.penggunaID, KecualiID: pelaku.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("daftar sasaran: %w", err)
	}
	perPeran := map[string][]domain.AkunSasaran{}
	for _, a := range akun {
		perPeran[a.Peran] = append(perPeran[a.Peran], a)
	}
	for _, d := range DaftarPeran() {
		if len(perPeran[d.Kunci]) == 0 {
			continue
		}
		hasil.Peran = append(hasil.Peran, PeranSasaran{Peran: d.Kunci, Label: d.Label, Lapis: d.Lapis, Akun: perPeran[d.Kunci]})
	}
	return hasil, nil
}

// ── Mulai, ganti ──────────────────────────────────────────────────────

// MulaiImpersonasiInput adalah permintaan memulai atau mengganti impersonasi. TargetPenggunaID didahulukan.
type MulaiImpersonasiInput struct {
	TargetPenggunaID string
	TargetPeran      string
	Alasan           string
}

// ImpersonasiHasil adalah hasil mulai atau berhenti: sesi baru dan akun yang kini dipakai.
type ImpersonasiHasil struct {
	AccessToken  string
	RefreshToken string
	// User adalah akun sasaran (mulai) atau pelaku (berhenti).
	User *domain.Pengguna
	// Meta terisi hanya saat mulai.
	Meta *ImpersonasiMeta
}

// MulaiImpersonasi memulai impersonasi (atau mengganti sasarannya bila pemanggil sedang impersonasi). callerID adalah
// pemilik token; pelaku asli ditentukan server (pelakuAsli).
func (uc *UseCase) MulaiImpersonasi(ctx context.Context, callerID string, in MulaiImpersonasiInput, ip string) (*ImpersonasiHasil, error) {
	if err := uc.impTersedia(); err != nil {
		return nil, err
	}
	pelaku, ganti, err := uc.pelakuAsli(ctx, callerID)
	if err != nil {
		return nil, err
	}

	// Setiap jalur penolakan setelah ini dicatat sebagai DITOLAK atas nama pelaku asli. entitas_id adalah akun yang
	// dituju, atau perannya bila hanya peran yang diminta (tidak pernah kosong).
	tolak := func(err error, in MulaiImpersonasiInput) (*ImpersonasiHasil, error) {
		sasaranRef := strings.TrimSpace(in.TargetPenggunaID)
		if sasaranRef == "" {
			sasaranRef = strings.TrimSpace(in.TargetPeran)
		}
		if sasaranRef == "" {
			sasaranRef = pelaku.ID
		}
		uc.writeAudit(ctx, audit.Entry{
			AktorID: pelaku.ID, Aksi: "IMPERSONASI_MULAI", Entitas: "pengguna", EntitasID: sasaranRef, Hasil: audit.HasilDitolak,
			Metadata: map[string]any{"alasan_ditolak": err.Error()},
		})
		return nil, err
	}

	p, err := uc.pengaturan(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca kebijakan impersonasi: %w", err)
	}
	if !p.Aktif {
		return tolak(ErrImpersonasiNonaktif, in)
	}
	alasan := strings.TrimSpace(in.Alasan)
	switch {
	case p.WajibAlasan && alasan == "":
		return nil, ErrAlasanImpersonasiWajib
	case utf8.RuneCountInString(alasan) > domain.MaxAlasanImpersonasi:
		return nil, ErrAlasanTerlaluPanjang
	}

	sasaran, err := uc.pilihSasaran(ctx, pelaku, in)
	if err != nil {
		return tolak(err, in)
	}

	now := uc.sekarang()
	familyID := uuid.New().String()
	rec := &domain.ImpersonasiSesi{
		ID:       uuid.New().String(),
		PelakuID: &pelaku.ID, PelakuNama: pelaku.Nama, PelakuEmail: pelaku.Email, PelakuPeran: pelaku.Peran,
		SasaranID: &sasaran.ID, SasaranNama: sasaran.Nama, SasaranEmail: sasaran.Email, SasaranPeran: sasaran.Peran,
		FamilyID: familyID, SesiAsalMulai: now, MulaiPada: now,
	}
	if alasan != "" {
		rec.Alasan = &alasan
	}
	if p.DurasiMaksMenit != nil {
		habis := now.Add(time.Duration(*p.DurasiMaksMenit) * time.Minute)
		rec.KedaluwarsaPada = &habis
	}
	if ua := cleanUserAgent(auditdomain.OriginFrom(ctx).UserAgent); ua != nil {
		rec.UserAgent = ua
	}
	if ip != "" {
		rec.IP = &ip
	}

	mulai := domain.MulaiImpersonasi{Sesi: rec, PelakuID: pelaku.ID, Waktu: now}
	switch {
	case ganti != nil:
		rec.SesiAsalMulai = ganti.SesiAsalMulai
		mulai.Ganti = ganti
	default:
		if sid := currentSession(ctx); sid != "" {
			mulai.CabutSesiAsal = sid
			rec.SesiAsalMulai = uc.awalSesi(ctx, pelaku.ID, sid, now)
		}
	}

	minted, err := uc.mintSessionDengan(ctx, sasaran, familyID, ip, now, now, rec)
	if err != nil {
		return nil, err
	}
	mulai.Token = minted.row
	if err := uc.imp.MulaiImpersonasi(ctx, mulai); err != nil {
		return nil, fmt.Errorf("mulai impersonasi: %w", err)
	}
	if ganti != nil && uc.impCache != nil {
		uc.impCache.buang(ganti.ID)
	}

	aksi := "IMPERSONASI_MULAI"
	meta := map[string]any{
		"sesi_id": rec.ID, "sasaran_peran": sasaran.Peran, "sasaran_nama": sasaran.Nama,
	}
	if alasan != "" {
		meta["alasan"] = alasan
	}
	if rec.KedaluwarsaPada != nil {
		meta["kedaluwarsa_pada"] = *rec.KedaluwarsaPada
	}
	if ganti != nil {
		aksi = "IMPERSONASI_GANTI"
		meta["sesi_sebelumnya"] = ganti.ID
		meta["sasaran_sebelumnya_peran"] = ganti.SasaranPeran
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: pelaku.ID, Aksi: aksi, Entitas: "pengguna", EntitasID: sasaran.ID, Hasil: audit.HasilBerhasil, Metadata: meta,
	})
	uc.log.Info("impersonasi dimulai", map[string]interface{}{"pelaku_id": pelaku.ID, "sasaran_id": sasaran.ID, "sesi_id": rec.ID})

	return &ImpersonasiHasil{
		AccessToken: minted.accessToken, RefreshToken: minted.refreshToken, User: sasaran, Meta: metaDari(rec),
	}, nil
}

// awalSesi adalah kapan sesi biasa pelaku dimulai (dipulihkan saat berhenti); sekarang bila tidak dikenal.
func (uc *UseCase) awalSesi(ctx context.Context, userID, sid string, sekarang time.Time) time.Time {
	rows, err := uc.repo.ListActiveRefreshTokens(ctx, userID, sekarang)
	if err != nil {
		return sekarang
	}
	for _, r := range rows {
		if r.FamilyID == sid {
			return r.SessionStartedAt
		}
	}
	return sekarang
}

// pilihSasaran memuat akun sasaran dari target_pengguna_id, atau menyelesaikan target_peran menjadi tepat satu akun, lalu
// memeriksa hak pelaku atasnya.
func (uc *UseCase) pilihSasaran(ctx context.Context, pelaku *domain.Pengguna, in MulaiImpersonasiInput) (*domain.Pengguna, error) {
	switch {
	case strings.TrimSpace(in.TargetPenggunaID) != "":
		id, ok := idx.NormalUUID(strings.TrimSpace(in.TargetPenggunaID))
		if !ok {
			return nil, ErrTargetTidakAda
		}
		sasaran, err := uc.repo.GetUserByID(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrTargetTidakAda
			}
			return nil, fmt.Errorf("get user: %w", err)
		}
		if sasaran.Status != "AKTIF" {
			return nil, ErrTargetTidakAda
		}
		if err := uc.bolehMenjadi(ctx, pelaku, sasaran); err != nil {
			return nil, err
		}
		return sasaran, nil

	case strings.TrimSpace(in.TargetPeran) != "":
		peran := strings.TrimSpace(in.TargetPeran)
		if !PeranDikenal(peran) {
			return nil, ErrRoleNotFound
		}
		if peran == domain.PeranSuperadmin {
			return nil, ErrImpersonasiDitolak
		}
		aturan, berhak, err := uc.aturanUntuk(ctx, pelaku)
		if err != nil {
			return nil, err
		}
		if !berhak {
			return nil, ErrImpersonasiDitolak
		}
		kandidat, err := uc.imp.ListAkunSasaran(ctx, domain.FilterAkunSasaran{
			Semua: pelaku.Peran == domain.PeranSuperadmin, Peran: aturan.peran, PenggunaID: aturan.penggunaID,
			KecualiID: pelaku.ID, HanyaPeran: peran,
		})
		if err != nil {
			return nil, fmt.Errorf("cari akun sasaran: %w", err)
		}
		switch len(kandidat) {
		case 0:
			// Peran yang tidak termasuk hak pelaku dijawab sama dengan peran tanpa akun.
			if pelaku.Peran != domain.PeranSuperadmin && !slices.Contains(aturan.peran, peran) && len(aturan.penggunaID) == 0 {
				return nil, ErrImpersonasiDitolak
			}
			return nil, ErrTargetTidakAda
		case 1:
			return uc.repo.GetUserByID(ctx, kandidat[0].ID)
		}
		return nil, ErrTargetAmbigu
	}
	return nil, ErrTargetWajib
}

// ── Berhenti ──────────────────────────────────────────────────────────

// HentikanImpersonasi mengakhiri impersonasi yang sedang dipakai dan menerbitkan sesi biasa bagi pelaku asli, tanpa kata
// sandi atau MFA ulang: buktinya adalah catatan impersonasi di server yang cocok dengan sesi (sid) token ini.
func (uc *UseCase) HentikanImpersonasi(ctx context.Context, ip string) (*ImpersonasiHasil, error) {
	if err := uc.impTersedia(); err != nil {
		return nil, err
	}
	ic, ok := token.ImpersonationFrom(ctx)
	if !ok {
		return nil, ErrTidakAdaImpersonasi
	}
	rec, err := uc.imp.GetImpersonasi(ctx, ic.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTidakAdaImpersonasi
		}
		return nil, fmt.Errorf("baca impersonasi: %w", err)
	}
	// Catatan harus hidup dan milik sesi dan akun pada token ini; klaim saja tidak cukup.
	if rec.BerakhirPada != nil || rec.PelakuID == nil || rec.SasaranID == nil ||
		rec.FamilyID != strings.ToLower(currentSession(ctx)) || *rec.SasaranID != ic.EffectiveUserID {
		return nil, ErrTidakAdaImpersonasi
	}

	pelaku, err := uc.repo.GetUserByID(ctx, *rec.PelakuID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.akhiriOtomatis(ctx, rec, domain.AkhirDicabut)
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if pelaku.Status != "AKTIF" {
		uc.akhiriOtomatis(ctx, rec, domain.AkhirDicabut)
		return nil, ErrInvalidToken
	}

	now := uc.sekarang()
	minted, err := uc.mintSession(ctx, pelaku, uuid.New().String(), ip, rec.SesiAsalMulai, now)
	if err != nil {
		return nil, err
	}
	if _, err := uc.imp.AkhiriImpersonasi(ctx, domain.AkhiriImpersonasi{
		ID: rec.ID, Alasan: domain.AkhirStop, Waktu: now, Pengganti: minted.row,
	}); err != nil {
		if errors.Is(err, domain.ErrImpersonasiTidakAktif) {
			return nil, ErrTidakAdaImpersonasi
		}
		return nil, fmt.Errorf("akhiri impersonasi: %w", err)
	}
	if uc.impCache != nil {
		uc.impCache.buang(rec.ID)
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: pelaku.ID, Aksi: "IMPERSONASI_BERHENTI", Entitas: "pengguna", EntitasID: *rec.SasaranID, Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"sesi_id": rec.ID, "sasaran_peran": rec.SasaranPeran, "lama_detik": int64(now.Sub(rec.MulaiPada).Seconds())},
	})
	uc.log.Info("impersonasi dihentikan", map[string]interface{}{"pelaku_id": pelaku.ID, "sesi_id": rec.ID})
	return &ImpersonasiHasil{AccessToken: minted.accessToken, RefreshToken: minted.refreshToken, User: pelaku}, nil
}

// akhiriBilaLogout menutup catatan impersonasi saat sesinya keluar (logout). Gagal tidak menggagalkan logout: sesinya
// sudah dicabut, dan catatan yang tertinggal tampil berakhir DICABUT di riwayat.
func (uc *UseCase) akhiriBilaLogout(ctx context.Context) {
	ic, ok := token.ImpersonationFrom(ctx)
	if !ok || uc.imp == nil {
		return
	}
	if _, err := uc.imp.AkhiriImpersonasi(ctx, domain.AkhiriImpersonasi{ID: ic.ID, Alasan: domain.AkhirLogout, Waktu: uc.sekarang()}); err != nil &&
		!errors.Is(err, domain.ErrImpersonasiTidakAktif) {
		uc.log.Warn("impersonasi: gagal menutup catatan saat logout", map[string]interface{}{"sesi_id": ic.ID, "error": err.Error()})
	}
	if uc.impCache != nil {
		uc.impCache.buang(ic.ID)
	}
}

// akhiriOtomatis menutup catatan yang sudah tidak sah (kedaluwarsa, kebijakan nonaktif, akun tidak lagi berhak) dan
// mencatatnya. Dipanggil dari penjaga dan refresh; kegagalan menulis hanya dilaporkan di log: sesinya tetap ditolak.
func (uc *UseCase) akhiriOtomatis(ctx context.Context, rec *domain.ImpersonasiSesi, alasan string) {
	ctx = context.WithoutCancel(ctx)
	tutup, err := uc.imp.AkhiriImpersonasi(ctx, domain.AkhiriImpersonasi{ID: rec.ID, Alasan: alasan, Waktu: uc.sekarang()})
	if uc.impCache != nil {
		uc.impCache.buang(rec.ID)
	}
	if err != nil {
		if !errors.Is(err, domain.ErrImpersonasiTidakAktif) {
			uc.log.Warn("impersonasi: gagal menutup catatan", map[string]interface{}{"sesi_id": rec.ID, "alasan": alasan, "error": err.Error()})
		}
		return
	}
	entry := audit.Entry{
		Aksi: "IMPERSONASI_BERAKHIR", Entitas: "impersonasi_sesi", EntitasID: tutup.ID, Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"sebab": alasan, "sesi_id": tutup.ID},
	}
	if tutup.PelakuID != nil {
		entry.AktorID = *tutup.PelakuID
	}
	uc.writeAudit(ctx, entry)
}

// ── Admin: riwayat dan pengakhiran paksa ──────────────────────────────

// RiwayatImpersonasiHalaman adalah satu halaman riwayat; NextCursor kosong berarti halaman terakhir.
type RiwayatImpersonasiHalaman struct {
	Items      []domain.RiwayatImpersonasi
	NextCursor string
}

// ListRiwayatImpersonasi mengembalikan riwayat impersonasi (superadmin). status: "" atau "semua", atau "aktif".
func (uc *UseCase) ListRiwayatImpersonasi(ctx context.Context, actorID, status string, limit int, rawCursor string) (RiwayatImpersonasiHalaman, error) {
	if err := uc.impTersedia(); err != nil {
		return RiwayatImpersonasiHalaman{}, err
	}
	if _, err := uc.pastikanSuperadmin(ctx, actorID); err != nil {
		return RiwayatImpersonasiHalaman{}, err
	}
	f := domain.FilterRiwayatImpersonasi{}
	switch status {
	case "", domain.RiwayatSemua:
	case domain.RiwayatAktif:
		f.HanyaAktif = true
	default:
		return RiwayatImpersonasiHalaman{}, ErrRiwayatSesiTidakValid
	}
	if limit < 1 {
		limit = DefaultLimitRiwayatSesi
	}
	limit = min(limit, MaxLimitRiwayatSesi)
	f.Limit = limit + 1
	if rawCursor != "" {
		cursor, err := decodeCursorRiwayat(rawCursor)
		if err != nil {
			return RiwayatImpersonasiHalaman{}, err
		}
		f.After = &cursor
	}
	rows, err := uc.imp.ListRiwayatImpersonasi(ctx, f, uc.sekarang())
	if err != nil {
		return RiwayatImpersonasiHalaman{}, fmt.Errorf("riwayat impersonasi: %w", err)
	}
	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeCursorRiwayat(domain.RiwayatCursor{MulaiPada: rows[limit-1].MulaiPada, ID: rows[limit-1].ID})
	}
	return RiwayatImpersonasiHalaman{Items: rows, NextCursor: next}, nil
}

// AkhiriImpersonasiPaksa mengakhiri impersonasi yang masih hidup atas perintah superadmin; tokennya mati seketika.
func (uc *UseCase) AkhiriImpersonasiPaksa(ctx context.Context, actorID, impersonasiID string) error {
	if err := uc.impTersedia(); err != nil {
		return err
	}
	if _, err := uc.pastikanSuperadmin(ctx, actorID); err != nil {
		return err
	}
	tutup, err := uc.imp.AkhiriImpersonasi(ctx, domain.AkhiriImpersonasi{ID: impersonasiID, Alasan: domain.AkhirDiakhiri, Waktu: uc.sekarang()})
	if err != nil {
		if errors.Is(err, domain.ErrImpersonasiTidakAktif) {
			return ErrImpersonasiTidakDitemukan
		}
		return fmt.Errorf("akhiri impersonasi: %w", err)
	}
	if uc.impCache != nil {
		uc.impCache.buang(tutup.ID)
	}
	meta := map[string]any{"sesi_id": tutup.ID}
	if tutup.PelakuID != nil {
		meta["pelaku_id"] = *tutup.PelakuID
	}
	if tutup.SasaranID != nil {
		meta["sasaran_id"] = *tutup.SasaranID
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "IMPERSONASI_DIAKHIRI", Entitas: "impersonasi_sesi", EntitasID: tutup.ID, Hasil: audit.HasilBerhasil, Metadata: meta,
	})
	return nil
}

// ── Metadata untuk GET /auth/me ───────────────────────────────────────

// ImpersonasiMeta adalah keadaan impersonasi yang sedang berjalan, untuk klien.
type ImpersonasiMeta struct {
	SesiID          string
	PelakuID        string
	PelakuNama      string
	PelakuEmail     string
	PelakuPeran     string
	SasaranID       string
	SasaranNama     string
	SasaranPeran    string
	MulaiPada       time.Time
	KedaluwarsaPada *time.Time
	Alasan          *string
}

func metaDari(rec *domain.ImpersonasiSesi) *ImpersonasiMeta {
	m := &ImpersonasiMeta{
		SesiID: rec.ID, PelakuNama: rec.PelakuNama, PelakuEmail: rec.PelakuEmail, PelakuPeran: rec.PelakuPeran,
		SasaranNama: rec.SasaranNama, SasaranPeran: rec.SasaranPeran, MulaiPada: rec.MulaiPada,
		KedaluwarsaPada: rec.KedaluwarsaPada, Alasan: rec.Alasan,
	}
	if rec.PelakuID != nil {
		m.PelakuID = *rec.PelakuID
	}
	if rec.SasaranID != nil {
		m.SasaranID = *rec.SasaranID
	}
	return m
}

// ImpersonasiMeta mengembalikan keadaan impersonasi request ini (dari catatan di basis data); nil bila bukan impersonasi.
func (uc *UseCase) ImpersonasiMeta(ctx context.Context) (*ImpersonasiMeta, error) {
	ic, ok := token.ImpersonationFrom(ctx)
	if !ok || uc.imp == nil {
		return nil, nil
	}
	rec, err := uc.imp.GetImpersonasi(ctx, ic.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("baca impersonasi: %w", err)
	}
	return metaDari(rec), nil
}

// ── Penjaga per request (token.ImpersonationGuard) ────────────────────

var _ token.ImpersonationGuard = (*UseCase)(nil)

// ImpersonationGuard mengembalikan penjaga untuk dipasang pada token.Manager; nil bila UseCase dibuat tanpa penyimpanan
// impersonasi (tes unit lama), sehingga token ber-`imp` ditolak (gagal-tutup).
func (uc *UseCase) ImpersonationGuard() token.ImpersonationGuard {
	if uc.imp == nil {
		return nil
	}
	return uc
}

// ValidateImpersonation memeriksa catatan impersonasi pada klaim: harus cocok dengan sesi dan akun token, belum berakhir
// atau kedaluwarsa, kebijakan aktif, pelaku dan sasaran aktif dan masih berhak. Mengembalikan izin efektif sasaran.
func (uc *UseCase) ValidateImpersonation(ctx context.Context, claims *token.Claims) ([]string, error) {
	imp := claims.Imp
	id, ok := idx.NormalUUID(imp.ID)
	if !ok {
		return nil, token.ErrImpersonationEnded
	}

	valid, ada, gen := uc.impCache.ambil(id)
	if !ada {
		rec, err := uc.imp.GetImpersonasi(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, token.ErrImpersonationEnded
			}
			return nil, fmt.Errorf("baca impersonasi: %w", err)
		}
		if err := uc.periksaImpersonasi(ctx, rec); err != nil {
			return nil, err
		}
		valid = impersonasiValid{pelakuID: uuidKecil(rec.PelakuID), sasaranID: uuidKecil(rec.SasaranID), familyID: rec.FamilyID}
		uc.impCache.simpan(id, valid, gen)
	}

	// Token harus milik sesi dan akun yang tercatat; token yang ditempeli klaim `imp` milik sesi lain ditolak.
	userID, _ := idx.NormalUUID(claims.UserID)
	if valid.sasaranID != userID || valid.familyID != strings.ToLower(claims.SessionID) ||
		valid.pelakuID != strings.ToLower(imp.By.UserID) {
		return nil, token.ErrImpersonationEnded
	}
	return uc.izinUntukID(ctx, valid.sasaranID)
}

// periksaImpersonasi memeriksa satu catatan terhadap keadaan terkini dan menutupnya bila sudah tidak sah.
// token.ErrImpersonationEnded bila tidak sah; galat lain bila keadaan tidak dapat dipastikan.
func (uc *UseCase) periksaImpersonasi(ctx context.Context, rec *domain.ImpersonasiSesi) error {
	if rec.BerakhirPada != nil {
		return token.ErrImpersonationEnded
	}
	if rec.KedaluwarsaPada != nil && !uc.sekarang().Before(*rec.KedaluwarsaPada) {
		uc.akhiriOtomatis(ctx, rec, domain.AkhirKedaluwarsa)
		return token.ErrImpersonationEnded
	}
	p, err := uc.pengaturan(ctx)
	if err != nil {
		return fmt.Errorf("baca kebijakan impersonasi: %w", err)
	}
	if !p.Aktif {
		uc.akhiriOtomatis(ctx, rec, domain.AkhirKebijakan)
		return token.ErrImpersonationEnded
	}
	if rec.PelakuID == nil || rec.SasaranID == nil {
		uc.akhiriOtomatis(ctx, rec, domain.AkhirDicabut)
		return token.ErrImpersonationEnded
	}
	pelaku, err := uc.repo.GetUserByID(ctx, *rec.PelakuID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.akhiriOtomatis(ctx, rec, domain.AkhirDicabut)
			return token.ErrImpersonationEnded
		}
		return fmt.Errorf("get user: %w", err)
	}
	sasaran, err := uc.repo.GetUserByID(ctx, *rec.SasaranID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.akhiriOtomatis(ctx, rec, domain.AkhirDicabut)
			return token.ErrImpersonationEnded
		}
		return fmt.Errorf("get user: %w", err)
	}
	if pelaku.Status != "AKTIF" || sasaran.Status != "AKTIF" {
		uc.akhiriOtomatis(ctx, rec, domain.AkhirDicabut)
		return token.ErrImpersonationEnded
	}
	if err := uc.bolehMenjadi(ctx, pelaku, sasaran); err != nil {
		if errors.Is(err, ErrImpersonasiDitolak) {
			uc.akhiriOtomatis(ctx, rec, domain.AkhirKebijakan)
			return token.ErrImpersonationEnded
		}
		return err
	}
	return nil
}

// izinUntukID adalah izin efektif akun menurut id: cache izin yang sama dengan JWTAuth dan penerbitan token.
func (uc *UseCase) izinUntukID(ctx context.Context, userID string) ([]string, error) {
	if uc.izinCache != nil {
		return uc.izinCache.Permissions(ctx, userID)
	}
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, token.ErrPrincipalNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return terapkanIzinEksklusif(user.Peran, uc.getPermissionsForRole(user.Peran)), nil
}

// RecordImpersonationMutation mencatat satu request mutasi selama impersonasi. aktor_id di baris audit adalah pelaku asli
// dan metadata memuat sesi dan peran efektif (atributKeImpersonasi di modul audit).
func (uc *UseCase) RecordImpersonationMutation(ctx context.Context, ic token.ImpersonationContext, info token.MutationInfo) {
	hasil := audit.HasilBerhasil
	switch {
	case info.Status == 401 || info.Status == 403:
		hasil = audit.HasilDitolak
	case info.Status >= 400:
		hasil = audit.HasilGagal
	}
	// Dicatat walau klien memutus sambungan: context dilepas dari pembatalan tetapi tetap membawa impersonasi.
	ctx = auditdomain.WithOrigin(context.WithoutCancel(ctx), auditdomain.Origin{IP: info.IP, UserAgent: info.UserAgent, CorrelationID: info.RequestID})
	uc.writeAudit(ctx, audit.Entry{
		AktorID: ic.EffectiveUserID, Aksi: "IMPERSONASI_AKSI", Entitas: "impersonasi_sesi", EntitasID: ic.ID, Hasil: hasil,
		Metadata: map[string]any{"metode": info.Method, "rute": info.Route, "status": info.Status},
	})
}

func uuidKecil(s *string) string {
	if s == nil {
		return ""
	}
	return strings.ToLower(*s)
}
