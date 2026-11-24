// Package usecase membaca hasil pipeline ML dengan aman untuk dipakai di jalur request: siklus final terbaru atau
// yang diminta (?cycle_id=), snapshot per siklus yang di-cache (isi siklus append-only, tidak pernah berubah), dan
// pemutus sementara saat Mongo gagal supaya halaman tidak menunggu timeout berulang kali.
package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"golang.org/x/sync/singleflight"
)

// Alasan ketidaktersediaan yang dikirim ke klien (field `alasan`).
const (
	AlasanTidakDikonfigurasi   = "TIDAK_DIKONFIGURASI"
	AlasanKonfigurasiInvalid   = "KONFIGURASI_TIDAK_VALID"
	AlasanMongoTidakTersedia   = "MONGO_TIDAK_TERSEDIA"
	AlasanMongoLambat          = "MONGO_LAMBAT"
	AlasanBelumAdaSiklus       = "BELUM_ADA_SIKLUS"
	AlasanCycleIDTidakValid    = "CYCLE_ID_TIDAK_VALID"
	AlasanSiklusTidakDitemukan = "SIKLUS_TIDAK_DITEMUKAN"
	AlasanSiklusBelumFinal     = "SIKLUS_BELUM_FINAL"
	AlasanTanpaIzin            = "TANPA_IZIN"
	AlasanCakupanGagal         = "CAKUPAN_TIDAK_DAPAT_DIPERIKSA"
	AlasanTidakAdaData         = "TIDAK_ADA_DATA"
	AlasanWilayahTanpaData     = "WILAYAH_TANPA_DATA"
	AlasanLuarCakupan          = "LUAR_CAKUPAN"
)

// Keadaan melaporkan apakah hasil ML tersedia untuk sebuah bacaan, beserta alasannya bila tidak.
type Keadaan struct {
	Tersedia bool
	Alasan   string
}

// tersedia adalah Keadaan sukses.
var tersedia = Keadaan{Tersedia: true}

func mati(alasan string) Keadaan { return Keadaan{Alasan: alasan} }

// Konfigurasi waktu bawaan.
const (
	ttlTerbaru  = 20 * time.Second
	ttlRun      = 30 * time.Second
	jedaPutus   = 20 * time.Second
	batasMuat   = 5 * time.Second
	maksSnap    = 3
	maksDetail  = 64
	ttlKosong   = 10 * time.Second
	kunciTerbar = "terbaru"
)

// Service adalah pembaca hasil pipeline ML. Aman dipakai bersamaan. Repo nil berarti Mongo tidak dikonfigurasi:
// semua bacaan melaporkan alasanMati tanpa menyentuh jaringan.
type Service struct {
	repo       domain.Repository
	alasanMati string
	log        *logger.Logger
	now        func() time.Time

	sf singleflight.Group

	mu          sync.Mutex
	putusSampai time.Time
	terbaru     *domain.Siklus
	terbaruPada time.Time
	snap        map[string]*domain.Snapshot
	urutanSnap  []string
	detail      map[string]*domain.Forecast
	urutanDet   []string
	runs        []domain.RunSumber
	runsPada    time.Time
}

// New membuat Service. repo nil: alasan menjelaskan mengapa (AlasanTidakDikonfigurasi atau AlasanKonfigurasiInvalid).
func New(repo domain.Repository, alasan string, log *logger.Logger) *Service {
	if repo != nil {
		alasan = ""
	} else if alasan == "" {
		alasan = AlasanTidakDikonfigurasi
	}
	return &Service{
		repo: repo, alasanMati: alasan, log: log, now: time.Now,
		snap: map[string]*domain.Snapshot{}, detail: map[string]*domain.Forecast{},
	}
}

// Dikonfigurasi melaporkan apakah Mongo dikonfigurasi.
func (s *Service) Dikonfigurasi() bool { return s.repo != nil }

// AlasanMati adalah alasan bila Mongo tidak dikonfigurasi, kosong bila dikonfigurasi.
func (s *Service) AlasanMati() string { return s.alasanMati }

// Sekarang adalah jam Service (bisa diganti tes).
func (s *Service) Sekarang() time.Time { return s.now() }

// SetJam mengganti sumber waktu (untuk tes).
func (s *Service) SetJam(now func() time.Time) { s.now = now }

// terputus melaporkan apakah pemutus sedang terbuka.
func (s *Service) terputus() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().Before(s.putusSampai)
}

// bukaPemutus membuka pemutus untuk galat Mongo sungguhan (bukan "tidak ditemukan" dan bukan pembatalan oleh klien).
func (s *Service) bukaPemutus(err error) {
	if err == nil || errors.Is(err, domain.ErrTidakDitemukan) || errors.Is(err, context.Canceled) {
		return
	}
	s.mu.Lock()
	s.putusSampai = s.now().Add(jedaPutus)
	s.mu.Unlock()
	if s.log != nil {
		s.log.Warn("ml: bacaan Mongo gagal, hasil ML disembunyikan sementara", map[string]interface{}{"error": err.Error()})
	}
}

// alasanGalat memetakan galat bacaan ke alasan untuk klien.
func alasanGalat(err error) Keadaan {
	switch {
	case errors.Is(err, domain.ErrTidakDitemukan):
		return mati(AlasanTidakAdaData)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return mati(AlasanMongoLambat)
	}
	return mati(AlasanMongoTidakTersedia)
}

// langsung memetakan galat bacaan langsung (memakai context request) dan membuka pemutus bila perlu. Batas waktu
// request yang habis tetap dianggap Mongo lambat: bacaan langsung tidak punya pemuatan latar yang bisa menyusul.
func (s *Service) langsung(err error) Keadaan {
	s.bukaPemutus(err)
	return alasanGalat(err)
}

// awal memeriksa konfigurasi dan pemutus sebelum bacaan apa pun.
func (s *Service) awal() (Keadaan, bool) {
	if s.repo == nil {
		return mati(s.alasanMati), false
	}
	if s.terputus() {
		return mati(AlasanMongoTidakTersedia), false
	}
	return tersedia, true
}

// tunggu menjalankan fn sekali per kunci (singleflight) dengan batas waktunya sendiri, dan menunggu hasilnya selama
// ctx masih hidup. Bila ctx habis lebih dulu, pemuatan tetap selesai di latar dan hasilnya di-cache untuk request
// berikutnya.
func (s *Service) tunggu(ctx context.Context, kunci string, fn func(context.Context) (any, error)) (any, error) {
	ch := s.sf.DoChan(kunci, func() (any, error) {
		muatCtx, cancel := context.WithTimeout(context.Background(), batasMuat)
		defer cancel()
		v, err := fn(muatCtx)
		s.bukaPemutus(err)
		return v, err
	})
	select {
	case r := <-ch:
		return r.Val, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SiklusTerbaru mengembalikan siklus final terbaru (di-cache ttlTerbaru).
func (s *Service) SiklusTerbaru(ctx context.Context) (*domain.Siklus, Keadaan) {
	if k, ok := s.awal(); !ok {
		return nil, k
	}
	s.mu.Lock()
	if s.terbaruPada.After(s.now().Add(-ttlTerbaru)) {
		t := s.terbaru
		s.mu.Unlock()
		if t == nil {
			return nil, mati(AlasanBelumAdaSiklus)
		}
		return t, tersedia
	}
	s.mu.Unlock()

	v, err := s.tunggu(ctx, kunciTerbar, func(c context.Context) (any, error) {
		siklus, err := s.repo.SiklusTerbaru(c)
		if errors.Is(err, domain.ErrTidakDitemukan) {
			s.mu.Lock()
			s.terbaru, s.terbaruPada = nil, s.now().Add(ttlKosong-ttlTerbaru)
			s.mu.Unlock()
			return (*domain.Siklus)(nil), nil
		}
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.terbaru, s.terbaruPada = siklus, s.now()
		s.mu.Unlock()
		return siklus, nil
	})
	if err != nil {
		return nil, alasanGalat(err)
	}
	siklus, _ := v.(*domain.Siklus)
	if siklus == nil {
		return nil, mati(AlasanBelumAdaSiklus)
	}
	return siklus, tersedia
}

// Snapshot mengembalikan hasil satu siklus final: cycleID kosong berarti terbaru. Siklus yang diminta harus final;
// siklus BERJALAN atau GAGAL tidak pernah dibaca isinya.
func (s *Service) Snapshot(ctx context.Context, cycleID string) (*domain.Snapshot, Keadaan) {
	if k, ok := s.awal(); !ok {
		return nil, k
	}
	if cycleID != "" && !domain.CycleIDValid(cycleID) {
		return nil, mati(AlasanCycleIDTidakValid)
	}
	if cycleID != "" {
		if snap := s.snapCache(cycleID); snap != nil {
			return snap, tersedia
		}
	}

	var siklus *domain.Siklus
	if cycleID == "" {
		var k Keadaan
		if siklus, k = s.SiklusTerbaru(ctx); !k.Tersedia {
			return nil, k
		}
		if snap := s.snapCache(siklus.CycleID); snap != nil {
			return snap, tersedia
		}
	}

	v, err := s.tunggu(ctx, "snap:"+cycleIDAtau(cycleID, siklus), func(c context.Context) (any, error) {
		target := siklus
		if target == nil {
			got, err := s.repo.Siklus(c, cycleID)
			if err != nil {
				return nil, err
			}
			if !domain.Final(got.Status) {
				return got, nil // dilaporkan sebagai belum final
			}
			target = got
		}
		snap, err := s.repo.Snapshot(c, *target)
		if err != nil {
			return nil, err
		}
		s.simpanSnap(snap)
		return snap, nil
	})
	if errors.Is(err, domain.ErrTidakDitemukan) {
		return nil, mati(AlasanSiklusTidakDitemukan)
	}
	if err != nil {
		return nil, alasanGalat(err)
	}
	switch hasil := v.(type) {
	case *domain.Snapshot:
		return hasil, tersedia
	case *domain.Siklus:
		return nil, mati(AlasanSiklusBelumFinal)
	}
	return nil, mati(AlasanTidakAdaData)
}

func cycleIDAtau(id string, s *domain.Siklus) string {
	if id != "" || s == nil {
		return id
	}
	return s.CycleID
}

func (s *Service) snapCache(id string) *domain.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap[id]
}

func (s *Service) simpanSnap(snap *domain.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := snap.Siklus.CycleID
	if _, ada := s.snap[id]; !ada {
		s.urutanSnap = append(s.urutanSnap, id)
	}
	s.snap[id] = snap
	for len(s.urutanSnap) > maksSnap {
		delete(s.snap, s.urutanSnap[0])
		s.urutanSnap = s.urutanSnap[1:]
	}
}

// Forecast mengembalikan satu forecast lengkap dengan riwayat untuk siklus final (di-cache per kunci).
func (s *Service) Forecast(ctx context.Context, cycleID, komoditas, kabupaten string) (*domain.Forecast, Keadaan) {
	if k, ok := s.awal(); !ok {
		return nil, k
	}
	kunci := cycleID + "|" + komoditas + "|" + kabupaten
	s.mu.Lock()
	if f, ok := s.detail[kunci]; ok {
		s.mu.Unlock()
		return f, tersedia
	}
	s.mu.Unlock()
	v, err := s.tunggu(ctx, "fc:"+kunci, func(c context.Context) (any, error) {
		return s.repo.Forecast(c, cycleID, komoditas, kabupaten)
	})
	if errors.Is(err, domain.ErrTidakDitemukan) {
		return nil, mati(AlasanWilayahTanpaData)
	}
	if err != nil {
		return nil, alasanGalat(err)
	}
	f := v.(*domain.Forecast)
	s.mu.Lock()
	if _, ada := s.detail[kunci]; !ada {
		s.urutanDet = append(s.urutanDet, kunci)
	}
	s.detail[kunci] = f
	for len(s.urutanDet) > maksDetail {
		delete(s.detail, s.urutanDet[0])
		s.urutanDet = s.urutanDet[1:]
	}
	s.mu.Unlock()
	return f, tersedia
}

// ListSiklus mengembalikan satu halaman riwayat siklus.
func (s *Service) ListSiklus(ctx context.Context, q domain.ListSiklusQuery) ([]domain.Siklus, int64, Keadaan) {
	if k, ok := s.awal(); !ok {
		return nil, 0, k
	}
	items, total, err := s.repo.ListSiklus(ctx, q)
	if err != nil {
		return nil, 0, s.langsung(err)
	}
	return items, total, tersedia
}

// Siklus mengembalikan satu siklus apa pun statusnya.
func (s *Service) Siklus(ctx context.Context, cycleID string) (*domain.Siklus, Keadaan) {
	if k, ok := s.awal(); !ok {
		return nil, k
	}
	if !domain.CycleIDValid(cycleID) {
		return nil, mati(AlasanCycleIDTidakValid)
	}
	siklus, err := s.repo.Siklus(ctx, cycleID)
	if errors.Is(err, domain.ErrTidakDitemukan) {
		return nil, mati(AlasanSiklusTidakDitemukan)
	}
	if err != nil {
		return nil, s.langsung(err)
	}
	return siklus, tersedia
}

// RunSumber mengembalikan run crawl terakhir per sumber (di-cache ttlRun).
func (s *Service) RunSumber(ctx context.Context) ([]domain.RunSumber, Keadaan) {
	if k, ok := s.awal(); !ok {
		return nil, k
	}
	s.mu.Lock()
	if s.runs != nil && s.runsPada.After(s.now().Add(-ttlRun)) {
		runs := s.runs
		s.mu.Unlock()
		return runs, tersedia
	}
	s.mu.Unlock()
	v, err := s.tunggu(ctx, "runs", func(c context.Context) (any, error) {
		runs, err := s.repo.RunSumber(c)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.runs, s.runsPada = runs, s.now()
		s.mu.Unlock()
		return runs, nil
	})
	if err != nil {
		return nil, alasanGalat(err)
	}
	return v.([]domain.RunSumber), tersedia
}
