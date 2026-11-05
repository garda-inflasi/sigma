package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/google/uuid"
)

// Parameter bawaan biaya dan jarak (sama dengan seeder rute demo).
const (
	TarifBawaanTonKm  = 4200.0
	FaktorJalanBawaan = 1.35
	KecepatanKm       = 45.0

	maxTarifTonKm     = 1_000_000
	maxFaktorJalan    = 3.0
	maxJamTempuh      = 240
	maxPenyesuaian    = 50
	maxPenyesuaianTon = 1_000_000
	// DefaultOptimasiLimit membatasi daftar rencana tersimpan.
	DefaultOptimasiLimit = 20
)

// Galat bisnis optimasi.
var (
	ErrNeracaKosong        = errors.New("belum ada neraca untuk komoditas dan periode ini")
	ErrKomoditasKosong     = errors.New("komoditas wajib diisi")
	ErrSkenarioTidakSimpan = errors.New("skenario dengan penyesuaian hanya simulasi dan tidak dapat disimpan")
)

// InputError adalah galat validasi masukan; pesannya aman ditampilkan.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &InputError{Msg: fmt.Sprintf(format, args...)}
}

// Optimasi menghitung dan menyimpan rencana distribusi (12-C).
type Optimasi struct {
	repo  domain.OptimasiRepository
	log   *logger.Logger
	audit audit.Writer
	now   func() time.Time
}

func NewOptimasi(repo domain.OptimasiRepository, log *logger.Logger, auditWriter audit.Writer) *Optimasi {
	return &Optimasi{repo: repo, log: log, audit: auditWriter, now: time.Now}
}

// WithClock mengganti sumber waktu; dipakai pengujian.
func (o *Optimasi) WithClock(now func() time.Time) *Optimasi {
	o.now = now
	return o
}

// Penyesuaian mengubah neraca satu wilayah pada simulasi: Ton positif menambah
// surplus (atau mengurangi defisit), negatif sebaliknya.
type Penyesuaian struct {
	KabupatenKode string
	Ton           float64
}

// OptimasiInput adalah permintaan satu perhitungan. Nilai nol pada parameter memakai bawaan.
type OptimasiInput struct {
	KomoditasKode string
	// Periode kosong memakai periode neraca terbaru.
	Periode       time.Time
	Penyesuaian   []Penyesuaian
	TarifTonKm    float64
	FaktorJalan   float64
	MaksJamTempuh float64
	// Simpan menulis rencana sebagai usulan rute; tidak boleh bersama Penyesuaian.
	Simpan  bool
	ActorID string
}

// Rute adalah satu rute dalam rencana beserta nama wilayahnya.
type Rute struct {
	domain.Alokasi
	AsalNama, TujuanNama string
}

// Sisa adalah volume tersisa pada satu wilayah beserta namanya.
type Sisa struct {
	Kode, Nama string
	Ton        float64
}

// OptimasiOutput adalah rencana distribusi.
type OptimasiOutput struct {
	KomoditasKode, KomoditasNama string
	Periode                      time.Time
	Rute                         []Rute
	TotalVolumeTon               float64
	TotalBiaya                   int64
	BelumTerpenuhi, SurplusSisa  []Sisa
	// OptimasiID terisi bila rencana disimpan.
	OptimasiID string
}

// parameterTersimpan adalah dokumen JSON pada kolom optimasi_rute.parameter.
type parameterTersimpan struct {
	TarifTonKm    float64 `json:"tarif_ton_km"`
	FaktorJalan   float64 `json:"faktor_jalan"`
	KecepatanKm   float64 `json:"kecepatan_km"`
	MaksJamTempuh float64 `json:"maks_jam_tempuh"`
}

// Hitung membentuk rencana distribusi dari neraca (wilayah surplus ke wilayah defisit)
// dan, bila in.Simpan, menyimpannya sebagai usulan rute serta mencatatnya di audit trail.
func (o *Optimasi) Hitung(ctx context.Context, in OptimasiInput) (OptimasiOutput, error) {
	in.KomoditasKode = strings.TrimSpace(in.KomoditasKode)
	p, err := validasiInput(&in)
	if err != nil {
		return OptimasiOutput{}, err
	}

	neraca, err := o.repo.ListNeraca(ctx, domain.NeracaQuery{KomoditasKode: in.KomoditasKode, Periode: in.Periode})
	if err != nil {
		return OptimasiOutput{}, err
	}
	if len(neraca) == 0 {
		return OptimasiOutput{}, ErrNeracaKosong
	}
	periode := neraca[0].Periode

	nama := make(map[string]string, len(neraca))
	posisi := make(map[string]float64, len(neraca))
	for _, n := range neraca {
		nama[n.KabupatenKode] = n.KabupatenNama
		posisi[n.KabupatenKode] = n.SurplusDefisitTon
	}
	for _, a := range in.Penyesuaian {
		if _, ok := posisi[a.KabupatenKode]; !ok {
			return OptimasiOutput{}, invalid("wilayah %s tidak punya neraca untuk komoditas ini", a.KabupatenKode)
		}
		posisi[a.KabupatenKode] += a.Ton
	}
	pasokan, kebutuhan := pisahPosisi(posisi)

	pusat, err := o.repo.ListMarker(ctx)
	if err != nil {
		return OptimasiOutput{}, err
	}
	cache, err := o.repo.ListJarak(ctx)
	if err != nil {
		return OptimasiOutput{}, err
	}
	peta := newPetaJarak(pusat, cache, p)

	hasil, err := domain.Optimalkan(pasokan, kebutuhan, peta.jalur, domain.ParamOptimasi{TarifTonKm: p.TarifTonKm, MaksJamTempuh: p.MaksJamTempuh})
	if err != nil {
		return OptimasiOutput{}, err
	}

	out := OptimasiOutput{
		KomoditasKode: in.KomoditasKode, KomoditasNama: neraca[0].KomoditasNama, Periode: periode,
		TotalVolumeTon: hasil.TotalVolumeTon, TotalBiaya: hasil.TotalBiaya,
		BelumTerpenuhi: bernama(hasil.BelumTerpenuhi, nama), SurplusSisa: bernama(hasil.SurplusSisa, nama),
	}
	for _, a := range hasil.Alokasi {
		out.Rute = append(out.Rute, Rute{Alokasi: a, AsalNama: nama[a.AsalKode], TujuanNama: nama[a.TujuanKode]})
	}

	if in.Simpan {
		id, err := o.simpan(ctx, in, p, out, hasil, peta)
		if err != nil {
			return OptimasiOutput{}, err
		}
		out.OptimasiID = id
	}
	return out, nil
}

// validasiInput memeriksa masukan dan mengisi parameter bawaan.
func validasiInput(in *OptimasiInput) (parameterTersimpan, error) {
	if in.KomoditasKode == "" {
		return parameterTersimpan{}, ErrKomoditasKosong
	}
	p := parameterTersimpan{TarifTonKm: in.TarifTonKm, FaktorJalan: in.FaktorJalan, KecepatanKm: KecepatanKm, MaksJamTempuh: in.MaksJamTempuh}
	if p.TarifTonKm == 0 {
		p.TarifTonKm = TarifBawaanTonKm
	}
	if p.FaktorJalan == 0 {
		p.FaktorJalan = FaktorJalanBawaan
	}
	switch {
	case p.TarifTonKm < 0 || p.TarifTonKm > maxTarifTonKm || math.IsNaN(p.TarifTonKm):
		return p, invalid("tarif_per_ton_km harus antara 0 dan %d", maxTarifTonKm)
	case p.FaktorJalan < 1 || p.FaktorJalan > maxFaktorJalan || math.IsNaN(p.FaktorJalan):
		return p, invalid("faktor_jalan harus antara 1 dan %g", maxFaktorJalan)
	case p.MaksJamTempuh < 0 || p.MaksJamTempuh > maxJamTempuh || math.IsNaN(p.MaksJamTempuh):
		return p, invalid("maks_jam_tempuh harus antara 0 dan %d", maxJamTempuh)
	case len(in.Penyesuaian) > maxPenyesuaian:
		return p, invalid("maksimal %d penyesuaian", maxPenyesuaian)
	case in.Simpan && len(in.Penyesuaian) > 0:
		return p, ErrSkenarioTidakSimpan
	}
	seen := map[string]bool{}
	for _, a := range in.Penyesuaian {
		switch {
		case a.KabupatenKode == "":
			return p, invalid("penyesuaian: kabupaten_kode wajib diisi")
		case seen[a.KabupatenKode]:
			return p, invalid("penyesuaian: wilayah %s ganda", a.KabupatenKode)
		case math.IsNaN(a.Ton) || math.Abs(a.Ton) > maxPenyesuaianTon:
			return p, invalid("penyesuaian %s: ton tidak valid", a.KabupatenKode)
		}
		seen[a.KabupatenKode] = true
	}
	return p, nil
}

// pisahPosisi memisahkan posisi bersih: positif = pasokan, negatif = kebutuhan.
func pisahPosisi(posisi map[string]float64) (pasokan, kebutuhan []domain.SisaWilayah) {
	for kode, ton := range posisi {
		switch {
		case ton > 0:
			pasokan = append(pasokan, domain.SisaWilayah{Kode: kode, Ton: ton})
		case ton < 0:
			kebutuhan = append(kebutuhan, domain.SisaWilayah{Kode: kode, Ton: -ton})
		}
	}
	return pasokan, kebutuhan
}

func bernama(in []domain.SisaWilayah, nama map[string]string) []Sisa {
	out := make([]Sisa, len(in))
	for i, s := range in {
		out[i] = Sisa{Kode: s.Kode, Nama: nama[s.Kode], Ton: s.Ton}
	}
	return out
}

// petaJarak memberi jarak antar wilayah: cache (mis. hasil OSRM) bila ada, selain itu
// jarak garis lurus x faktor jalan. Pasangan hasil estimasi dicatat untuk mengisi cache.
type petaJarak struct {
	cache     map[[2]string]domain.Jalur
	koordinat map[string]domain.Wilayah
	p         parameterTersimpan
	estimasi  map[[2]string]domain.Jalur
}

func newPetaJarak(pusat []domain.Marker, cache []domain.JarakAntarWilayah, p parameterTersimpan) *petaJarak {
	m := &petaJarak{
		cache:     make(map[[2]string]domain.Jalur, len(cache)),
		koordinat: make(map[string]domain.Wilayah, len(pusat)),
		p:         p,
		estimasi:  map[[2]string]domain.Jalur{},
	}
	for _, c := range cache {
		m.cache[[2]string{c.AsalKode, c.TujuanKode}] = domain.Jalur{JarakKm: c.JarakKm, JamTempuh: c.JamTempuh}
	}
	for _, w := range pusat {
		m.koordinat[w.Kode] = w.Wilayah
	}
	return m
}

func (m *petaJarak) jalur(asal, tujuan string) domain.Jalur {
	key := [2]string{asal, tujuan}
	if j, ok := m.cache[key]; ok {
		return j
	}
	if j, ok := m.estimasi[key]; ok {
		return j
	}
	a, b := m.koordinat[asal], m.koordinat[tujuan]
	km := math.Round(domain.JarakKm(a.Lat, a.Lng, b.Lat, b.Lng)*m.p.FaktorJalan*10) / 10
	j := domain.Jalur{JarakKm: km, JamTempuh: math.Round(km/m.p.KecepatanKm*10) / 10}
	m.estimasi[key] = j
	return j
}

func (o *Optimasi) simpan(ctx context.Context, in OptimasiInput, p parameterTersimpan, out OptimasiOutput, hasil domain.HasilOptimasi, peta *petaJarak) (string, error) {
	now := o.now().UTC()
	id := "OPT-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
	parameter, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("encode parameter optimasi: %w", err)
	}
	var belum float64
	for _, b := range hasil.BelumTerpenuhi {
		belum += b.Ton
	}
	rencana := domain.OptimasiRute{
		ID: id, KomoditasKode: in.KomoditasKode, Periode: out.Periode, Parameter: string(parameter),
		TotalVolumeTon: hasil.TotalVolumeTon, TotalBiaya: hasil.TotalBiaya, BelumTerpenuhiTon: math.Round(belum*10) / 10, DibuatPada: now,
	}
	if in.ActorID != "" {
		rencana.DibuatOleh = &in.ActorID
	}

	rute := make([]domain.RuteDistribusi, len(hasil.Alokasi))
	var jarak []domain.JarakAntarWilayah
	for i, a := range hasil.Alokasi {
		rute[i] = domain.RuteDistribusi{
			ID: fmt.Sprintf("RT-%s-%02d", id, i+1), AsalKode: a.AsalKode, TujuanKode: a.TujuanKode, KomoditasKode: in.KomoditasKode,
			VolumeTon: a.VolumeTon, JarakKm: int(math.Round(a.JarakKm)), JamTempuh: a.JamTempuh, Biaya: a.Biaya,
			Status: domain.StatusRuteUsulan, Sumber: domain.SumberRuteOptimasi, DibuatPada: now, OptimasiID: &id,
		}
		if j, ok := peta.estimasi[[2]string{a.AsalKode, a.TujuanKode}]; ok {
			jarak = append(jarak, domain.JarakAntarWilayah{
				AsalKode: a.AsalKode, TujuanKode: a.TujuanKode, JarakKm: j.JarakKm, JamTempuh: j.JamTempuh,
				Sumber: domain.SumberJarakEstimasi, DiperbaruiPada: now,
			})
		}
	}
	sort.Slice(jarak, func(i, j int) bool {
		return jarak[i].AsalKode+jarak[i].TujuanKode < jarak[j].AsalKode+jarak[j].TujuanKode
	})

	if err := o.repo.SimpanOptimasi(ctx, rencana, rute, jarak); err != nil {
		o.log.Error("spasial: simpan optimasi gagal", map[string]interface{}{"error": err.Error()})
		return "", err
	}
	if err := o.audit.Write(ctx, audit.Entry{
		AktorID: in.ActorID, Aksi: "OPTIMASI_RUTE", Entitas: "optimasi_rute", EntitasID: id, Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{
			"komoditas": in.KomoditasKode, "periode": out.Periode.Format(time.DateOnly), "rute": len(rute),
			"total_volume_ton": hasil.TotalVolumeTon, "total_biaya": hasil.TotalBiaya,
		},
	}); err != nil {
		o.log.Warn("spasial: gagal mencatat audit optimasi", map[string]interface{}{"error": err.Error()})
	}
	return id, nil
}

// Riwayat mengembalikan rencana yang pernah disimpan, terbaru lebih dulu.
func (o *Optimasi) Riwayat(ctx context.Context) ([]domain.RencanaOptimasi, error) {
	items, err := o.repo.ListOptimasi(ctx, DefaultOptimasiLimit)
	if err != nil {
		o.log.Error("spasial: riwayat optimasi gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}
