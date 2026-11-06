package usecase

import (
	"context"
	"slices"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// izinFake adalah domain.IzinRepository dalam memori yang meniru semantik repositori PostgreSQL (versi
// optimistis, penjaga pemegang terakhir, penolakan izin yang sudah dimiliki lewat peran) untuk tes usecase.
type izinFake struct {
	mu       sync.Mutex
	now      func() time.Time
	peran    map[string]*fakePeran
	katalog  map[string]bool
	pengguna map[string]*fakeAkun
	langsung map[string]map[string]*domain.IzinLangsung // pengguna -> izin -> baris

	// Pencacah dan penyela untuk tes cache.
	panggilanEfektif int
	errEfektif       error
	saatEfektif      func() // dipanggil di tengah pembacaan, sebelum hasil dikembalikan
	namaPemberi      map[string]string
}

type fakePeran struct {
	domain.Peran
	izin map[string]bool
}

type fakeAkun struct {
	peran  string
	status string
}

func newIzinFake(now func() time.Time) *izinFake {
	f := &izinFake{
		now: now, peran: map[string]*fakePeran{}, katalog: map[string]bool{},
		pengguna: map[string]*fakeAkun{}, langsung: map[string]map[string]*domain.IzinLangsung{},
		namaPemberi: map[string]string{},
	}
	for _, d := range DaftarPeran() {
		izin := map[string]bool{}
		for _, p := range IzinBawaan(d.Kunci) {
			izin[p] = true
		}
		f.peran[d.Kunci] = &fakePeran{Peran: domain.Peran{Kunci: d.Kunci, Label: d.Label, Lapis: d.Lapis, Urutan: d.Urutan, Versi: 1, DiperbaruiPada: now()}, izin: izin}
	}
	for _, d := range KatalogIzin() {
		f.katalog[d.Nama] = true
	}
	return f
}

func (f *izinFake) tambahAkun(id, peran, status string) {
	f.pengguna[id] = &fakeAkun{peran: peran, status: status}
}

func (f *izinFake) izinPeran(peran string) []string {
	var out []string
	for p := range f.peran[peran].izin {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

func (f *izinFake) pemegang() int {
	n := 0
	for _, a := range f.pengguna {
		if a.status == "AKTIF" && f.peran[a.peran].izin[domain.IzinPengelolaan] {
			n++
		}
	}
	return n
}

func (f *izinFake) IzinEfektifPengguna(_ context.Context, userID string) (*domain.IzinEfektif, error) {
	f.mu.Lock()
	f.panggilanEfektif++
	err, hook := f.errEfektif, f.saatEfektif
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	akun, ok := f.pengguna[userID]
	if !ok || akun.status != "AKTIF" {
		f.mu.Unlock()
		return nil, gorm.ErrRecordNotFound
	}
	set := map[string]bool{}
	for p := range f.peran[akun.peran].izin {
		set[p] = true
	}
	var terdekat time.Duration
	sekarang := f.now()
	for izin, g := range f.langsung[userID] {
		if !g.Aktif(sekarang) {
			continue
		}
		set[izin] = true
		if g.BerlakuSampai != nil {
			sisa := g.BerlakuSampai.Sub(sekarang)
			if terdekat == 0 || sisa < terdekat {
				terdekat = sisa
			}
		}
	}
	hasil := &domain.IzinEfektif{Peran: akun.peran, Izin: []string{}, KedaluwarsaDalam: terdekat}
	for p := range set {
		hasil.Izin = append(hasil.Izin, p)
	}
	slices.Sort(hasil.Izin)
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	return hasil, nil
}

func (f *izinFake) ListPeran(context.Context) ([]domain.PeranDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.PeranDetail
	for _, d := range DaftarPeran() {
		p := f.peran[d.Kunci]
		out = append(out, domain.PeranDetail{Peran: p.Peran, Izin: f.izinPeran(d.Kunci)})
	}
	return out, nil
}

func (f *izinFake) ListIzin(context.Context) ([]domain.Izin, error) {
	var out []domain.Izin
	for _, d := range KatalogIzin() {
		out = append(out, domain.Izin{Nama: d.Nama, Deskripsi: d.Deskripsi, Kelompok: d.Kelompok})
	}
	// Urutan sengaja diacak dari katalog supaya usecase terbukti mengurutkannya.
	slices.Reverse(out)
	return out, nil
}

func (f *izinFake) SetIzinPeran(_ context.Context, p domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	peran, ok := f.peran[p.Peran]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	if peran.Versi != p.Versi {
		return nil, domain.ErrVersiPeranBerubah
	}
	baru := map[string]bool{}
	for _, nama := range p.Izin {
		if !f.katalog[nama] {
			return nil, domain.ErrIzinTidakDikenal
		}
		baru[nama] = true
	}
	sebelum := f.izinPeran(p.Peran)
	lama := peran.izin
	peran.izin = baru
	if f.pemegang() == 0 && slices.Contains(sebelum, domain.IzinPengelolaan) && !baru[domain.IzinPengelolaan] {
		peran.izin = lama
		return nil, domain.ErrPemegangTerakhir
	}
	sesudah := f.izinPeran(p.Peran)
	if slices.Equal(sebelum, sesudah) {
		return &domain.HasilSetIzinPeran{Peran: peran.Peran, Sebelum: sebelum, Sesudah: sesudah}, nil
	}
	peran.Versi++
	peran.DiperbaruiPada = f.now()
	oleh := p.Oleh
	peran.DiperbaruiOleh = &oleh
	return &domain.HasilSetIzinPeran{Peran: peran.Peran, Sebelum: sebelum, Sesudah: sesudah, Berubah: true, DiperbaruiPada: peran.DiperbaruiPada}, nil
}

func (f *izinFake) detail(g *domain.IzinLangsung) domain.IzinLangsungDetail {
	d := domain.IzinLangsungDetail{IzinLangsung: *g, Kelompok: "tes", Deskripsi: "deskripsi " + g.PermissionNama}
	if g.DiberikanOleh != nil {
		if nama, ok := f.namaPemberi[*g.DiberikanOleh]; ok {
			d.DiberikanOlehNama = &nama
		}
	}
	return d
}

func (f *izinFake) ListIzinLangsung(_ context.Context, userID string) (*domain.IzinPengguna, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	akun, ok := f.pengguna[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	hasil := &domain.IzinPengguna{Peran: akun.peran, IzinPeran: f.izinPeran(akun.peran), Langsung: []domain.IzinLangsungDetail{}}
	for _, g := range f.langsung[userID] {
		hasil.Langsung = append(hasil.Langsung, f.detail(g))
	}
	slices.SortFunc(hasil.Langsung, func(a, b domain.IzinLangsungDetail) int {
		if c := b.DiberikanPada.Compare(a.DiberikanPada); c != 0 {
			return c
		}
		return compareString(a.PermissionNama, b.PermissionNama)
	})
	return hasil, nil
}

func compareString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (f *izinFake) BerikanIzinLangsung(_ context.Context, g domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	akun, ok := f.pengguna[g.PenggunaID]
	if !ok {
		return nil, nil, gorm.ErrRecordNotFound
	}
	if !f.katalog[g.Izin] {
		return nil, nil, domain.ErrIzinTidakDikenal
	}
	if f.peran[akun.peran].izin[g.Izin] {
		return nil, nil, domain.ErrIzinDariPeran
	}
	var sebelumnya *domain.IzinLangsung
	if lama, ada := f.langsung[g.PenggunaID][g.Izin]; ada {
		if lama.Aktif(f.now()) {
			return nil, nil, domain.ErrIzinLangsungAda
		}
		salinan := *lama
		sebelumnya = &salinan
	}
	oleh := g.Oleh
	baris := &domain.IzinLangsung{
		PenggunaID: g.PenggunaID, PermissionNama: g.Izin, BerlakuSampai: g.BerlakuSampai, Alasan: g.Alasan,
		DiberikanOleh: &oleh, DiberikanPada: f.now(),
	}
	if f.langsung[g.PenggunaID] == nil {
		f.langsung[g.PenggunaID] = map[string]*domain.IzinLangsung{}
	}
	f.langsung[g.PenggunaID][g.Izin] = baris
	d := f.detail(baris)
	return &d, sebelumnya, nil
}

func (f *izinFake) CabutIzinLangsung(_ context.Context, userID, izin string) (*domain.IzinLangsung, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.langsung[userID][izin]
	if !ok {
		return nil, domain.ErrIzinLangsungTidakAda
	}
	delete(f.langsung[userID], izin)
	return g, nil
}

func (f *izinFake) HitungPemegangPengelolaan(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(f.pemegang()), nil
}

// repoDenganIzin menggabungkan repositori tiruan pengguna dan penyimpanan izin dalam memori, seperti
// *postgres.Repository: New mendeteksi domain.IzinRepository lewat type assertion.
type repoDenganIzin struct {
	*mockRepository
	*izinFake
}

// jamTes adalah jam yang dapat dimajukan untuk tes cache dan masa berlaku.
type jamTes struct {
	mu sync.Mutex
	t  time.Time
}

func (j *jamTes) Now() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.t
}

func (j *jamTes) Maju(d time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.t = j.t.Add(d)
}
