// Package engine menjalankan ingestion: merancang permintaan lewat konektor,
// mengambilnya lewat Fetcher (retry, pembatasan laju, circuit breaker, robots.txt),
// menyimpan payload mentah (bronze), menormalkannya ke silver, dan mencatat
// hasilnya di eksekusi_sumber.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	sumber "github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

const (
	// BatasWaktu adalah waktu maksimum satu eksekusi.
	BatasWaktu = 10 * time.Minute
	// macetSetelah: eksekusi BERJALAN lebih tua dari ini dianggap terhenti (proses mati).
	macetSetelah = 3 * BatasWaktu
	// tutupTimeout memberi waktu mencatat hasil akhir walau konteks run sudah dibatalkan.
	tutupTimeout = 30 * time.Second
	// maksGalat membatasi pesan galat berbeda yang disimpan di satu eksekusi.
	maksGalat = 5
	// maksTakTerpetakan membatasi alias tak terpetakan yang disimpan di ringkasan.
	maksTakTerpetakan = 50
)

// Runner menjalankan konektor terdaftar.
type Runner struct {
	repo     domain.Repository
	fetcher  domain.Fetcher
	konektor map[string]domain.Konektor
	log      *logger.Logger

	// Sekarang bisa diganti pada tes.
	Sekarang func() time.Time
	// SetelahSilver dipanggil bila sebuah eksekusi menulis baris silver, mis.
	// untuk menyegarkan agregat gold. Boleh nil.
	SetelahSilver func()
}

// New membuat Runner dari repository, fetcher, dan konektor.
func New(repo domain.Repository, fetcher domain.Fetcher, log *logger.Logger, konektor ...domain.Konektor) *Runner {
	r := &Runner{repo: repo, fetcher: fetcher, konektor: map[string]domain.Konektor{}, log: log, Sekarang: time.Now}
	for _, k := range konektor {
		r.konektor[k.SumberID()] = k
	}
	return r
}

// Punya benar bila sumber punya konektor terdaftar.
func (r *Runner) Punya(sumberID string) bool {
	_, ok := r.konektor[sumberID]
	return ok
}

// Run adalah satu eksekusi yang sudah tercatat BERJALAN dan siap dijalankan.
type Run struct {
	r        *Runner
	sumber   *sumber.SumberData
	konektor domain.Konektor
	eksekusi domain.EksekusiSumber
}

// Eksekusi adalah salinan baris eksekusi saat dimulai.
func (run *Run) Eksekusi() domain.EksekusiSumber { return run.eksekusi }

// Mulai mencatat eksekusi BERJALAN untuk sumber. Galat: ErrSumberTidakAda,
// ErrTanpaKonektor, ErrDijeda, ErrSudahBerjalan. Panggil Run.Jalankan sesudahnya.
func (r *Runner) Mulai(ctx context.Context, sumberID, pemicu string, aktorID *string) (*Run, error) {
	k, ok := r.konektor[sumberID]
	if !ok {
		if _, err := r.repo.Sumber(ctx, sumberID); err != nil {
			return nil, err
		}
		return nil, domain.ErrTanpaKonektor
	}
	s, err := r.repo.Sumber(ctx, sumberID)
	if err != nil {
		return nil, err
	}
	e, err := r.repo.MulaiEksekusi(ctx, sumberID, pemicu, aktorID, r.Sekarang().Add(-macetSetelah))
	if err != nil {
		return nil, err
	}
	return &Run{r: r, sumber: s, konektor: k, eksekusi: *e}, nil
}

// Jalankan mengerjakan eksekusi sampai selesai dan mencatat hasilnya. Kegagalan
// sumber bukan galat Jalankan: ia tercatat sebagai status GAGAL/SEBAGIAN. Galat
// dikembalikan hanya bila hasil akhir tak bisa disimpan.
func (run *Run) Jalankan(ctx context.Context) (*domain.EksekusiSumber, error) {
	r := run.r
	e := run.eksekusi
	mulai := r.Sekarang()

	akum := &akumulasi{}
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				akum.fatal = fmt.Errorf("konektor panik: %v", rec)
			}
		}()
		run.proses(ctx, akum)
	}()

	akum.isi(&e, r.Sekarang().Sub(mulai))
	selesai := r.Sekarang()
	e.Selesai = &selesai

	tutup, cancel := context.WithTimeout(context.WithoutCancel(ctx), tutupTimeout)
	defer cancel()
	if err := r.repo.SelesaikanEksekusi(tutup, &e); err != nil {
		return nil, fmt.Errorf("catat hasil eksekusi %d: %w", e.ID, err)
	}

	fields := map[string]interface{}{
		"sumber": e.SumberID, "eksekusi": e.ID, "status": e.Status, "pemicu": e.Pemicu,
		"diambil": e.BarisDiambil, "ditolak": e.BarisTakTerpetakan + e.BarisTidakValid,
	}
	if e.Galat != nil {
		fields["galat"] = *e.Galat
		r.log.Warn("ingestion: eksekusi selesai dengan galat", fields)
	} else {
		r.log.Info("ingestion: eksekusi selesai", fields)
	}
	if e.BarisDiambil > 0 && r.SetelahSilver != nil {
		r.SetelahSilver()
	}
	return &e, nil
}

// akumulasi mengumpulkan hasil satu eksekusi.
type akumulasi struct {
	hasil    domain.Hasil
	dokumen  int // respons yang berhasil diambil
	diproses int // respons yang berhasil dinormalkan dan disimpan
	gagal    int // permintaan yang gagal diambil atau diproses
	galat    []string
	latensi  []time.Duration
	fatal    error
}

func (a *akumulasi) catatGalat(msg string) {
	a.gagal++
	a.galat = append(a.galat, msg)
}

func (run *Run) proses(ctx context.Context, a *akumulasi) {
	r, id := run.r, run.sumber.ID
	pem, err := r.repo.Pemetaan(ctx, id)
	if err != nil {
		a.fatal = fmt.Errorf("muat pemetaan: %w", err)
		return
	}
	in := domain.Masukan{Sekarang: r.Sekarang(), Konfigurasi: json.RawMessage(run.sumber.Konfigurasi), Pemetaan: pem}
	if run.sumber.URLDasar != nil {
		in.URLDasar = *run.sumber.URLDasar
	}
	reqs, err := run.konektor.Permintaan(in)
	if err != nil {
		a.fatal = err
		return
	}
	if len(reqs) == 0 {
		a.fatal = errors.New("konektor tidak merancang permintaan apa pun (periksa pemetaan wilayah)")
		return
	}

	for _, p := range reqs {
		if ctx.Err() != nil {
			a.fatal = fmt.Errorf("dihentikan: %w", ctx.Err())
			return
		}
		p.SumberID, p.PatuhRobots = id, run.sumber.RobotsTxtPatuh

		t0 := time.Now()
		body, err := r.fetcher.Get(ctx, p)
		if err != nil {
			a.catatGalat(fmt.Sprintf("%s: %v", p.Kunci, err))
			continue
		}
		a.latensi = append(a.latensi, time.Since(t0))
		a.dokumen++

		if !json.Valid(body) {
			a.catatGalat(fmt.Sprintf("%s: respons bukan JSON", p.Kunci))
			continue
		}
		mid, err := r.repo.SimpanMentah(ctx, domain.DataMentah{
			SumberID: id, EksekusiID: run.eksekusi.ID, KunciPermintaan: p.Kunci,
			TipeKonten: "application/json", DiambilPada: r.Sekarang(), Payload: string(body),
		})
		if err != nil {
			a.fatal = fmt.Errorf("simpan bronze %s: %w", p.Kunci, err)
			return
		}

		hasil, err := run.konektor.Normalisasi(in, domain.Dokumen{Kunci: p.Kunci, Konten: body, DiambilPada: r.Sekarang()})
		if err != nil {
			a.catatGalat(fmt.Sprintf("%s: normalisasi: %v", p.Kunci, err))
			continue
		}
		lampirkanGaris(&hasil, mid, run.eksekusi.ID)
		if err := r.repo.SimpanSilver(ctx, hasil); err != nil {
			a.fatal = fmt.Errorf("simpan silver %s: %w", p.Kunci, err)
			return
		}
		a.hasil.Gabung(hasil)
		a.diproses++
	}
}

// lampirkanGaris mengisi lineage (payload mentah dan eksekusi) pada baris silver.
func lampirkanGaris(h *domain.Hasil, dataMentahID, eksekusiID int64) {
	for i := range h.Harga {
		h.Harga[i].DataMentahID, h.Harga[i].EksekusiID = &dataMentahID, &eksekusiID
	}
}

// isi menyalin hasil akumulasi ke baris eksekusi dan menentukan status akhir.
func (a *akumulasi) isi(e *domain.EksekusiSumber, durasi time.Duration) {
	e.Dokumen = a.dokumen
	e.BarisDiambil = a.hasil.Diambil()
	e.BarisTakTerpetakan = a.hasil.JumlahTakTerpetakan()
	e.BarisTidakValid = a.hasil.JumlahTidakValid()
	e.BarisDibaca = e.BarisDiambil + e.BarisTakTerpetakan + e.BarisTidakValid
	ms := int(durasi.Milliseconds())
	e.DurasiMs = &ms
	if len(a.latensi) > 0 {
		var total time.Duration
		for _, l := range a.latensi {
			total += l
		}
		rata := int((total / time.Duration(len(a.latensi))).Milliseconds())
		e.LatensiMs = &rata
	}

	galat := slices.Clone(a.galat)
	switch {
	case a.fatal != nil:
		e.Status = domain.StatusGagal
		galat = append([]string{a.fatal.Error()}, galat...)
	case a.diproses == 0:
		e.Status = domain.StatusGagal
	case a.gagal > 0:
		e.Status = domain.StatusSebagian
	case e.BarisDibaca > 0 && e.BarisDiambil == 0:
		e.Status = domain.StatusGagal
		galat = append(galat, "semua baris ditolak; periksa pemetaan_sumber dan bentuk respons")
	default:
		e.Status = domain.StatusSukses
	}
	if len(galat) > 0 {
		e.Galat = new(ringkasGalat(galat))
	}
	e.Ringkasan = a.ringkasan()
}

// ringkasGalat menggabungkan pesan galat berbeda (maks maksGalat) dan menyebut sisanya.
func ringkasGalat(pesan []string) string {
	unik := slices.Compact(slices.Clone(pesan))
	sisa := len(unik) - maksGalat
	if sisa > 0 {
		unik = append(unik[:maksGalat], fmt.Sprintf("... dan %d galat lain", sisa))
	}
	return strings.Join(unik, "; ")
}

type ringkasanEksekusi struct {
	Permintaan    permintaanRingkas `json:"permintaan"`
	TakTerpetakan map[string]int    `json:"tak_terpetakan,omitempty"`
	TidakValid    map[string]int    `json:"tidak_valid,omitempty"`
}

type permintaanRingkas struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

func (a *akumulasi) ringkasan() string {
	rk := ringkasanEksekusi{
		Permintaan: permintaanRingkas{Berhasil: a.diproses, Gagal: a.gagal},
		TidakValid: a.hasil.TidakValid,
	}
	rk.TakTerpetakan = teratas(a.hasil.TakTerpetakan, maksTakTerpetakan)
	raw, err := json.Marshal(rk)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// teratas menyisakan n entri dengan jumlah terbesar (seri: urut abjad) agar ringkasan tetap kecil.
func teratas(m map[string]int, n int) map[string]int {
	if len(m) <= n {
		return m
	}
	kunci := make([]string, 0, len(m))
	for k := range m {
		kunci = append(kunci, k)
	}
	slices.SortFunc(kunci, func(a, b string) int {
		if m[a] != m[b] {
			return m[b] - m[a]
		}
		return strings.Compare(a, b)
	})
	res := make(map[string]int, n)
	for _, k := range kunci[:n] {
		res[k] = m[k]
	}
	return res
}
