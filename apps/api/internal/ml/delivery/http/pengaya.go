package http

import (
	"context"
	"sort"
	"strings"
	"time"

	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/internal/ml/dto"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// anggaranPengayaan membatasi seluruh bacaan Mongo untuk lampiran satu respons endpoint lama: Mongo yang lambat
// hanya menunda respons sebanyak ini, lalu lampiran dikirim tersedia=false dan data lama tetap utuh.
const anggaranPengayaan = 1500 * time.Millisecond

// kabupatenPrediksiBawaan sama dengan bawaan GET /v1/prediksi (internal/prediksi/usecase.DefaultKabupaten), supaya
// blok forecast menerangkan wilayah yang sama dengan `data`.
const kabupatenPrediksiBawaan = "1171"

// Batas jumlah sinyal EWS dalam satu blok.
const (
	maksSinyalRingkas = 5
	maksSinyalPenuh   = 200
)

// Jenis blok lampiran.
type blok int

const (
	blokInflasi blok = iota
	blokForecastRingkas
	blokForecast
	blokModel
	blokEws
	blokEwsRingkas
	blokWilayah
	blokKesegaran
	blokMakro
)

// aturanRute memetakan pola rute gin (FullPath) ke blok yang dilampirkan. Awalan=true mencocokkan semua rute
// di bawah pola.
type aturanRute struct {
	pola   string
	awalan bool
	blok   []blok
}

// Rute GET yang diperkaya. Hanya respons sukses JSON (response.Success) yang membawa `ml`; ekspor CSV dan respons
// galat tidak berubah. Daftar ini sumber kebenaran docs/PIPELINE-DATA-ML.md §5.2.
var petaRute = []aturanRute{
	{pola: "/v1/siklus/aktif", blok: []blok{blokInflasi}},
	{pola: "/v1/dashboard/eksekutif", blok: []blok{blokInflasi, blokModel, blokEwsRingkas, blokKesegaran, blokMakro}},
	{pola: "/v1/dashboard/operasional/", awalan: true, blok: []blok{blokForecastRingkas, blokEwsRingkas}},
	{pola: "/v1/prediksi", blok: []blok{blokForecastRingkas, blokModel}},
	{pola: "/v1/prediksi/registri", blok: []blok{blokModel}},
	{pola: "/v1/prediksi/ml", blok: []blok{blokModel}},
	{pola: "/v1/prediksi/:kode", blok: []blok{blokForecast}},
	{pola: "/v1/ews", blok: []blok{blokEws}},
	{pola: "/v1/ews/:id", blok: []blok{blokEws}},
	{pola: "/v1/spasial/wilayah", blok: []blok{blokWilayah}},
	{pola: "/v1/spasial/neraca", blok: []blok{blokWilayah}},
	{pola: "/v1/spasial/geometri", blok: []blok{blokWilayah}},
	{pola: "/v1/sumber-data", blok: []blok{blokKesegaran}},
	{pola: "/v1/sumber-data/pipeline", blok: []blok{blokKesegaran}},
	{pola: "/v1/sumber-data/:id", blok: []blok{blokKesegaran}},
	{pola: "/v1/kpi", blok: []blok{blokModel}},
	{pola: "/v1/kpi/:kode/riwayat", blok: []blok{blokModel}},
	{pola: "/v1/policy-brief", blok: []blok{blokInflasi, blokMakro}},
	{pola: "/v1/policy-brief/:id", blok: []blok{blokInflasi, blokMakro}},
}

// BlokUntuk mengembalikan blok yang dilampirkan untuk pola rute; nil bila rute tidak diperkaya.
func BlokUntuk(fullPath string) []blok {
	for _, a := range petaRute {
		if fullPath == a.pola || (a.awalan && strings.HasPrefix(fullPath, a.pola)) {
			return a.blok
		}
	}
	return nil
}

// PolaDiperkaya mengembalikan pola rute yang diperkaya; awalan diakhiri "/" (dipakai tes keberadaan rute).
func PolaDiperkaya() []string {
	out := make([]string, len(petaRute))
	for i, a := range petaRute {
		out[i] = a.pola
	}
	return out
}

// Pengaya memasang lampiran `ml` pada endpoint lama. Isi `data` tidak pernah diubah.
type Pengaya struct {
	svc *usecase.Service
	pg  domain.PostgresRepository
}

func NewPengaya(svc *usecase.Service, pg domain.PostgresRepository) *Pengaya {
	return &Pengaya{svc: svc, pg: pg}
}

// Middleware dipasang pada grup /v1 sebelum rute modul lain didaftarkan. Lampiran disusun malas saat respons sukses
// ditulis, sesudah JWTAuth dan pemeriksaan izin rute, sehingga izin dan cakupan pemanggil sudah diketahui.
func (p *Pengaya) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		env := middleware.DataEnv(c)
		if env != "" {
			response.PasangDataEnv(c, env)
		}
		if c.Request.Method == "GET" {
			if env == middleware.DataEnvProd {
				response.PasangPenyaringData(c, SaringDemoData)
			}
			if bloks := BlokUntuk(c.FullPath()); len(bloks) > 0 {
				response.LampirkanML(c, func() interface{} { return p.Susun(c, bloks) })
			}
		}
		c.Next()
	}
}

// perakit menyusun satu lampiran.
type perakit struct {
	p    *Pengaya
	c    *gin.Context
	ctx  context.Context
	snap *domain.Snapshot
	k    usecase.Keadaan

	cakupanSiap bool
	cakupan     hargadomain.Cakupan
	cakupanErr  error
}

// Susun membangun lampiran untuk blok yang diminta.
func (p *Pengaya) Susun(c *gin.Context, bloks []blok) dto.Lampiran {
	if middleware.DataEnv(c) == middleware.DataEnvStaging {
		return dto.Lampiran{Tersedia: false, Alasan: "MODE_STAGING", DataUtama: dto.DataUtamaPostgres}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), anggaranPengayaan)
	defer cancel()
	snap, k := p.svc.Snapshot(ctx, strings.TrimSpace(c.Query("cycle_id")))
	dataUtama := dto.DataUtamaPostgres
	if middleware.DataEnv(c) == middleware.DataEnvProd && k.Tersedia {
		dataUtama = dto.SumberDatasafe
	}
	l := dto.Lampiran{Tersedia: k.Tersedia, Alasan: k.Alasan, DataUtama: dataUtama}
	if snap != nil {
		l.Siklus = dto.ToSiklusRingkas(snap.Siklus)
	}
	r := &perakit{p: p, c: c, ctx: ctx, snap: snap, k: k}
	for _, b := range bloks {
		switch b {
		case blokInflasi:
			l.Inflasi = r.inflasi()
		case blokForecastRingkas:
			l.ForecastRingkas = r.forecastRingkas()
		case blokForecast:
			l.Forecast = r.forecast()
		case blokModel:
			l.Model = r.model()
		case blokEws:
			l.Ews = r.ews(maksSinyalPenuh, true)
		case blokEwsRingkas:
			l.Ews = r.ews(maksSinyalRingkas, false)
		case blokWilayah:
			l.Wilayah = r.wilayah()
		case blokKesegaran:
			l.Kesegaran = r.kesegaran()
		case blokMakro:
			l.Makro = r.makro()
		}
	}
	return l
}

// gerbang memeriksa siklus, izin, dan (bila perluCakupan) cakupan baris; ok=false berarti blok mati dengan meta.
func (r *perakit) gerbang(izin string, perluCakupan bool) (dto.Meta, bool) {
	if !middleware.HasPermission(r.c, izin) {
		return dto.MetaMati(usecase.AlasanTanpaIzin), false
	}
	if !r.k.Tersedia {
		return dto.MetaMati(r.k.Alasan), false
	}
	if perluCakupan {
		if _, err := r.cakupanPemanggil(); err != nil {
			return dto.MetaSiklusMati(r.snap.Siklus, usecase.AlasanCakupanGagal), false
		}
	}
	return dto.MetaSiklus(r.snap.Siklus), true
}

func (r *perakit) cakupanPemanggil() (hargadomain.Cakupan, error) {
	if !r.cakupanSiap {
		r.cakupanSiap = true
		if r.p.pg == nil {
			r.cakupan = hargadomain.Cakupan{}
		} else {
			r.cakupan, r.cakupanErr = cakupanDari(r.ctx, r.p.pg, r.c.GetString("user_id"))
		}
	}
	return r.cakupan, r.cakupanErr
}

// boleh melaporkan apakah wilayah masuk cakupan. Kode kosong (tingkat sistem) dan kode provinsi hanya untuk peran
// tingkat provinsi; peran berkabupaten hanya melihat kabupatennya (gagal-tutup bila kabupaten tugasnya kosong).
func (r *perakit) boleh(kode string) bool {
	c, err := r.cakupanPemanggil()
	if err != nil {
		return false
	}
	if c.Semua {
		return true
	}
	return c.KabupatenKode != "" && kode == c.KabupatenKode
}

func (r *perakit) xai(target string, komoditas *string, wilayah string) *domain.Xai {
	for i := range r.snap.Xai {
		x := &r.snap.Xai[i]
		if x.Target != target || x.WilayahKode != wilayah {
			continue
		}
		if (komoditas == nil) != (x.KomoditasKode == nil) {
			continue
		}
		if komoditas != nil && *komoditas != *x.KomoditasKode {
			continue
		}
		return x
	}
	return nil
}

func (r *perakit) inflasi() *dto.BlokInflasi {
	meta, ok := r.gerbang(IzinPrediksi, true)
	out := &dto.BlokInflasi{Meta: meta, Wilayah: []dto.InflasiWilayahResponse{}}
	if !ok {
		return out
	}
	s := r.snap.Siklus
	out.Target = s.TargetInflasi
	if s.TargetInflasi != nil {
		label := domain.LabelTarget(*s.TargetInflasi)
		out.LabelTarget = &label
	}
	out.BukanInflasiResmi = s.TargetInflasi == nil || *s.TargetInflasi != domain.TargetResmiBPS
	out.Kelayakan = domain.KelayakanTidakDiketahui
	if s.ModelMetrics != nil && s.ModelMetrics.Klasifikasi != nil {
		out.Kelayakan, out.AlasanKelayakan = domain.Kelayakan(s.ModelMetrics.Klasifikasi.Layak), s.ModelMetrics.Klasifikasi.Alasan
	}
	filter := strings.TrimSpace(r.c.Query("kabupaten_kode"))
	for _, sg := range r.snap.Inflasi {
		if !r.boleh(sg.WilayahKode) || (filter != "" && sg.WilayahKode != filter) {
			continue
		}
		out.Wilayah = append(out.Wilayah, dto.ToInflasiWilayah(sg, r.xai(domain.XaiInflasi, nil, sg.WilayahKode)))
		if sg.Target == domain.TargetResmiBPS {
			continue
		}
		out.BukanInflasiResmi = true
	}
	if len(out.Wilayah) == 0 {
		out.Tersedia, out.Alasan = false, usecase.AlasanTidakAdaData
		if filter != "" {
			out.Alasan = usecase.AlasanWilayahTanpaData
		}
	}
	return out
}

// wilayahForecast mengembalikan kabupaten ber-forecast dalam cakupan (untuk komoditas tertentu bila diisi).
func (r *perakit) wilayahForecast(komoditas string) []string {
	ada := map[string]bool{}
	for _, f := range r.snap.Forecast {
		if (komoditas == "" || f.KomoditasKode == komoditas) && r.boleh(f.KabupatenKode) {
			ada[f.KabupatenKode] = true
		}
	}
	out := make([]string, 0, len(ada))
	for k := range ada {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *perakit) forecastRingkas() *dto.BlokForecastRingkas {
	meta, ok := r.gerbang(IzinPrediksi, true)
	out := &dto.BlokForecastRingkas{Meta: meta, WilayahTersedia: []string{}, Items: []dto.ForecastResponse{}}
	if !ok {
		return out
	}
	kab := strings.TrimSpace(r.c.Query("kabupaten_kode"))
	if strings.HasPrefix(r.c.FullPath(), "/v1/prediksi") {
		kab = strings.TrimSpace(r.c.Query("kabupaten"))
		if kab == "" {
			kab = kabupatenPrediksiBawaan
		}
	}
	kom := strings.TrimSpace(r.c.Query("komoditas_kode"))
	if kab != "" {
		out.KabupatenKode = &kab
	}
	out.WilayahTersedia = r.wilayahForecast("")
	if kab != "" && !r.boleh(kab) {
		out.Tersedia, out.Alasan = false, usecase.AlasanLuarCakupan
		return out
	}
	for _, f := range r.snap.Forecast {
		if (kab != "" && f.KabupatenKode != kab) || (kom != "" && f.KomoditasKode != kom) || !r.boleh(f.KabupatenKode) {
			continue
		}
		out.Items = append(out.Items, dto.ToForecast(f))
	}
	if len(out.Items) == 0 {
		out.Tersedia, out.Alasan = false, alasanKosong(len(r.snap.Forecast))
	}
	return out
}

// alasanKosong membedakan siklus yang sama sekali tidak punya forecast (TIDAK_ADA_DATA) dari wilayah atau komoditas
// yang tidak tercakup (WILAYAH_TANPA_DATA).
func alasanKosong(jumlahForecast int) string {
	if jumlahForecast == 0 {
		return usecase.AlasanTidakAdaData
	}
	return usecase.AlasanWilayahTanpaData
}

func (r *perakit) forecast() *dto.BlokForecast {
	kom := strings.TrimSpace(r.c.Param("kode"))
	kab := strings.TrimSpace(r.c.Query("kabupaten"))
	if kab == "" {
		kab = kabupatenPrediksiBawaan
	}
	meta, ok := r.gerbang(IzinPrediksi, true)
	out := &dto.BlokForecast{Meta: meta, KomoditasKode: kom, KabupatenKode: kab, WilayahTersedia: []string{}}
	if !ok {
		return out
	}
	out.WilayahTersedia = r.wilayahForecast(kom)
	if !r.boleh(kab) {
		out.Tersedia, out.Alasan = false, usecase.AlasanLuarCakupan
		return out
	}
	ada := false
	for _, f := range r.snap.Forecast {
		if f.KomoditasKode == kom && f.KabupatenKode == kab {
			ada = true
			break
		}
	}
	if !ada {
		out.Tersedia, out.Alasan = false, alasanKosong(len(r.snap.Forecast))
		return out
	}
	f, k := r.p.svc.Forecast(r.ctx, r.snap.Siklus.CycleID, kom, kab)
	if !k.Tersedia {
		out.Tersedia, out.Alasan = false, k.Alasan
		return out
	}
	out.Item = dto.ToForecastDetail(*f, r.xai(domain.XaiForecast, &kom, kab))
	return out
}

func (r *perakit) model() *dto.BlokModel {
	meta, ok := r.gerbang(IzinPrediksi, false)
	out := &dto.BlokModel{Meta: meta}
	if !ok {
		return out
	}
	s := r.snap.Siklus
	out.Status, out.TargetInflasi = s.Status, s.TargetInflasi
	if s.TargetInflasi != nil {
		label := domain.LabelTarget(*s.TargetInflasi)
		out.LabelTarget = &label
	}
	out.MetrikModelResponse = dto.ToMetrikModel(s.ModelMetrics)
	if s.ModelMetrics == nil {
		out.Tersedia, out.Alasan = false, usecase.AlasanTidakAdaData
	}
	return out
}

// ews menyusun sinyal aktif dalam cakupan, paling parah lebih dulu. Pada /v1/ews/:id hanya sinyal yang
// dijembatani ke alert itu yang dicantumkan.
func (r *perakit) ews(maks int, penuh bool) *dto.BlokEws {
	meta, ok := r.gerbang(IzinEws, true)
	out := &dto.BlokEws{Meta: meta, Sinyal: []dto.SinyalEwsResponse{}}
	if !ok {
		return out
	}
	filterKab := strings.TrimSpace(r.c.Query("kabupaten_kode"))
	var dipilih []domain.SinyalEws
	for _, s := range r.snap.Ews {
		if !s.Aktif {
			continue
		}
		kab := ""
		if s.KabupatenKode != nil {
			kab = *s.KabupatenKode
		}
		if !r.boleh(kab) || (filterKab != "" && kab != filterKab) {
			continue
		}
		dipilih = append(dipilih, s)
	}
	sort.SliceStable(dipilih, func(i, j int) bool {
		a, b := ewsdomain.SeverityRank(dipilih[i].Severity), ewsdomain.SeverityRank(dipilih[j].Severity)
		if a != b {
			return a > b
		}
		return dipilih[i].SinyalKunci < dipilih[j].SinyalKunci
	})
	tautan := map[string]domain.TautanAlert{}
	if r.p.pg != nil && penuh {
		kunci := make([]string, len(dipilih))
		for i, s := range dipilih {
			kunci[i] = s.SinyalKunci
		}
		if t, err := r.p.pg.TautanAlert(r.ctx, kunci); err == nil {
			tautan = t
		}
	}
	alertID := strings.TrimSpace(r.c.Param("id"))
	for _, s := range dipilih {
		if alertID != "" {
			t, ada := tautan[s.SinyalKunci]
			if !ada || (t.AlertID.String() != alertID && (t.AlertKode == nil || *t.AlertKode != alertID)) {
				continue
			}
		}
		out.Ringkasan.Aktif++
		switch s.Severity {
		case ewsdomain.SeverityKritis:
			out.Ringkasan.Kritis++
		case ewsdomain.SeveritySiaga:
			out.Ringkasan.Siaga++
		case ewsdomain.SeverityWaspada:
			out.Ringkasan.Waspada++
		default:
			out.Ringkasan.Info++
		}
		if len(out.Sinyal) >= maks {
			out.Terpotong = true
			continue
		}
		out.Sinyal = append(out.Sinyal, dto.ToSinyalEws(s, tautan))
	}
	return out
}

func (r *perakit) wilayah() *dto.BlokWilayah {
	meta, ok := r.gerbang(IzinPrediksi, true)
	out := &dto.BlokWilayah{Meta: meta, Items: []dto.WilayahMLResponse{}}
	if !ok {
		return out
	}
	per := map[string]*dto.WilayahMLResponse{}
	ambil := func(kode string) *dto.WilayahMLResponse {
		w, ada := per[kode]
		if !ada {
			w = &dto.WilayahMLResponse{WilayahKode: kode, Tingkat: dto.Tingkat(kode), KomoditasForecast: []string{}}
			per[kode] = w
		}
		return w
	}
	for _, f := range r.snap.Forecast {
		if r.boleh(f.KabupatenKode) {
			w := ambil(f.KabupatenKode)
			w.KomoditasForecast = append(w.KomoditasForecast, f.KomoditasKode)
		}
	}
	for _, s := range r.snap.Inflasi {
		if r.boleh(s.WilayahKode) {
			ambil(s.WilayahKode).Inflasi = &dto.InflasiRingkasResponse{
				Kelas: s.Kelas, Kelayakan: domain.Kelayakan(s.Layak), Probabilitas: s.Probabilitas, SkorTekanan: s.SkorTekanan, Siklus: s.Siklus,
				Target: s.Target, LabelTarget: domain.LabelTarget(s.Target),
			}
		}
	}
	for _, s := range r.snap.Ews {
		if !s.Aktif || s.KabupatenKode == nil || !r.boleh(*s.KabupatenKode) {
			continue
		}
		w := ambil(*s.KabupatenKode)
		w.SinyalAktif.Aktif++
		switch s.Severity {
		case ewsdomain.SeverityKritis:
			w.SinyalAktif.Kritis++
		case ewsdomain.SeveritySiaga:
			w.SinyalAktif.Siaga++
		case ewsdomain.SeverityWaspada:
			w.SinyalAktif.Waspada++
		default:
			w.SinyalAktif.Info++
		}
	}
	for _, w := range per {
		sort.Strings(w.KomoditasForecast)
		out.Items = append(out.Items, *w)
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].WilayahKode < out.Items[j].WilayahKode })
	if len(out.Items) == 0 {
		out.Tersedia, out.Alasan = false, usecase.AlasanTidakAdaData
	}
	return out
}

func (r *perakit) kesegaran() *dto.BlokKesegaran {
	if !middleware.HasPermission(r.c, IzinSumber) {
		return &dto.BlokKesegaran{Meta: dto.MetaMati(usecase.AlasanTanpaIzin), Sumber: []dto.SumberKesegaranResponse{},
			SinyalSumber: []dto.SinyalEwsResponse{}}
	}
	return susunKesegaran(r.ctx, r.p.svc, r.p.pg)
}

func (r *perakit) makro() *dto.BlokMakro {
	meta, ok := r.gerbang(IzinPrediksi, false)
	out := &dto.BlokMakro{Meta: meta, Items: []dto.MakroResponse{}}
	if !ok {
		return out
	}
	for _, m := range r.snap.Makro {
		// Indikator wilayah dibatasi cakupan; indikator nasional/provinsi (tanpa kabupaten) untuk peran provinsi.
		kode := ""
		if m.WilayahKode != nil {
			kode = *m.WilayahKode
		}
		if kode != "" && len(kode) > 2 && !r.boleh(kode) {
			continue
		}
		if (kode == "" || len(kode) <= 2) && !r.boleh("") {
			continue
		}
		out.Items = append(out.Items, dto.ToMakro(m))
	}
	if len(out.Items) == 0 {
		out.Tersedia, out.Alasan = false, usecase.AlasanTidakAdaData
	}
	return out
}
