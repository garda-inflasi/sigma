package http

import (
	"context"
	"math"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/internal/spasial/routing"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandlerTitik melayani daftar titik spasial desa/perkampungan & kalkulator jarak berbasis jaringan jalan raya.
type HandlerTitik struct {
	db     *gorm.DB
	router *routing.Client
	cache  *routing.Cache
}

func NewTitik(db *gorm.DB) *HandlerTitik {
	if db == nil {
		return &HandlerTitik{}
	}
	return &HandlerTitik{
		db:     db,
		router: routing.NewClientFromEnv(),
		cache:  routing.NewCacheFromEnv(),
	}
}

func NewTitikWithRouting(db *gorm.DB, router *routing.Client, cache *routing.Cache) *HandlerTitik {
	return &HandlerTitik{
		db:     db,
		router: router,
		cache:  cache,
	}
}

type TitikDesa struct {
	Kode             string  `json:"kode"`
	Jenis            string  `json:"jenis"` // PERTANIAN | PERKAMPUNGAN
	Nama             string  `json:"nama"`
	KabupatenKode    string  `json:"kabupaten_kode"`
	KabupatenNama    string  `json:"kabupaten_nama"`
	KecamatanKode    *string `json:"kecamatan_kode,omitempty"`
	KomoditasUtama   string  `json:"komoditas_utama"`
	LuasHa           float64 `json:"luas_ha"`
	EstimasiPanenTon float64 `json:"estimasi_panen_ton"`
	Populasi         int     `json:"populasi"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	Aktif            bool    `json:"aktif"`
}

type FasilitasTerdekat struct {
	Jenis               string   `json:"jenis"` // PASAR | COLD_STORAGE
	Kode                string   `json:"kode"`
	Nama                string   `json:"nama"`
	KabupatenKode       string   `json:"kabupaten_kode"`
	Lat                 float64  `json:"lat"`
	Lng                 float64  `json:"lng"`
	JarakLurusKm        float64  `json:"jarak_lurus_km"`
	JarakJalanKm        float64  `json:"jarak_jalan_km"`
	EstimasiMenitTempuh int      `json:"estimasi_menit_tempuh"`
	KapasitasTon        *float64 `json:"kapasitas_ton,omitempty"`
}

type JarakResponse struct {
	AsalKode            string                  `json:"asal_kode,omitempty"`
	TujuanKode          string                  `json:"tujuan_kode,omitempty"`
	AsalLat             float64                 `json:"asal_lat"`
	AsalLng             float64                 `json:"asal_lng"`
	TujuanLat           *float64                `json:"tujuan_lat,omitempty"`
	TujuanLng           *float64                `json:"tujuan_lng,omitempty"`
	FaktorJalan         float64                 `json:"faktor_jalan"`
	JarakLurusKm        *float64                `json:"jarak_lurus_km,omitempty"`
	JarakJalanKm        *float64                `json:"jarak_jalan_km,omitempty"`
	EstimasiMenitTempuh *int                    `json:"estimasi_menit_tempuh,omitempty"`
	RisikoFoodLoss      string                  `json:"risiko_food_loss,omitempty"`
	SumberRute          string                  `json:"sumber_rute,omitempty"`
	ProfilKendaraan     string                  `json:"profil_kendaraan,omitempty"`
	JalurUtamaDiblokir  bool                    `json:"jalur_utama_diblokir"`
	Rute                []routing.OpsiRuteJalan `json:"rute,omitempty"`
	RuteDiblokir        *routing.OpsiRuteJalan  `json:"rute_diblokir,omitempty"`
	GangguanTerkait     []routing.GangguanRef   `json:"gangguan_terkait,omitempty"`
	PasarTerdekat       []FasilitasTerdekat     `json:"pasar_terdekat"`
	ColdStorageTerdekat []FasilitasTerdekat     `json:"cold_storage_terdekat"`
}

func (h *HandlerTitik) ListTitikDesa(c *gin.Context) {
	if h.db == nil {
		response.Success(c, []TitikDesa{})
		return
	}
	jenis := strings.ToUpper(strings.TrimSpace(c.Query("jenis")))
	kab := strings.TrimSpace(c.Query("kabupaten_kode"))

	q := h.db.WithContext(c.Request.Context()).Table("lokasi_spasial ls").
		Select(`ls.kode, ls.jenis, ls.nama, ls.kabupaten_kode, COALESCE(k.nama, '') AS kabupaten_nama,
		        ls.kecamatan_kode, ls.komoditas_utama, ls.luas_ha, ls.estimasi_panen_ton, ls.populasi,
		        ls.lat, ls.lng, ls.aktif`).
		Joins("LEFT JOIN kabupaten k ON k.kode = ls.kabupaten_kode").
		Where("ls.aktif = ?", true)
	if jenis == "PERTANIAN" || jenis == "PERKAMPUNGAN" {
		q = q.Where("ls.jenis = ?", jenis)
	}
	if kab != "" {
		q = q.Where("ls.kabupaten_kode = ?", kab)
	}

	var rows []TitikDesa
	if err := q.Order("ls.kabupaten_kode ASC, ls.jenis ASC, ls.nama ASC").Scan(&rows).Error; err != nil {
		response.InternalError(c, "Gagal memuat titik spasial")
		return
	}
	if rows == nil {
		rows = []TitikDesa{}
	}
	response.Success(c, rows)
}

func (h *HandlerTitik) HitungJarak(c *gin.Context) {
	ctx := c.Request.Context()
	faktor := 1.30
	if rawF := strings.TrimSpace(c.Query("faktor_jalan")); rawF != "" {
		if parsed, err := strconv.ParseFloat(rawF, 64); err == nil && parsed >= 1.0 && parsed <= 3.0 {
			faktor = parsed
		}
	}
	profil := strings.TrimSpace(c.Query("profil"))

	asalKode := strings.TrimSpace(c.Query("asal_kode"))
	tujuanKode := strings.TrimSpace(c.Query("tujuan_kode"))
	asalLat, asalLng, okAsal := h.resolvePoint(ctx, asalKode, c.Query("asal_lat"), c.Query("asal_lng"))
	if !okAsal {
		response.Error(c, stdhttp.StatusBadRequest, "400", "Koordinat asal (asal_kode atau asal_lat & asal_lng) wajib valid")
		return
	}

	out := JarakResponse{
		AsalKode:            asalKode,
		TujuanKode:          tujuanKode,
		AsalLat:             asalLat,
		AsalLng:             asalLng,
		FaktorJalan:         faktor,
		PasarTerdekat:       []FasilitasTerdekat{},
		ColdStorageTerdekat: []FasilitasTerdekat{},
	}

	tujuanLat, tujuanLng, okTujuan := h.resolvePoint(ctx, tujuanKode, c.Query("tujuan_lat"), c.Query("tujuan_lng"))
	if okTujuan {
		lurus := math.Round(domain.JarakKm(asalLat, asalLng, tujuanLat, tujuanLatToLng(tujuanLng))*10) / 10
		hasil := h.hitungRuteJalan(ctx, asalLat, asalLng, tujuanLat, tujuanLng, faktor, profil)
		out.TujuanLat = &tujuanLat
		out.TujuanLng = &tujuanLng
		out.JarakLurusKm = &lurus
		out.SumberRute = hasil.SumberRute
		out.ProfilKendaraan = hasil.ProfilKendaraan
		out.JalurUtamaDiblokir = hasil.JalurUtamaDiblokir
		out.Rute = hasil.Rute
		out.RuteDiblokir = hasil.RuteDiblokir
		out.GangguanTerkait = hasil.GangguanTerkait
		if len(hasil.Rute) > 0 {
			j := hasil.Rute[0].JarakKm
			m := hasil.Rute[0].DurasiMenit
			out.JarakJalanKm = &j
			out.EstimasiMenitTempuh = &m
			out.RisikoFoodLoss = hasil.Rute[0].RisikoFoodLoss
		}
	}

	if h.db != nil {
		out.PasarTerdekat = h.findNearestFacilities(ctx, "PASAR", asalLat, asalLng, faktor, 3)
		out.ColdStorageTerdekat = h.findNearestFacilities(ctx, "COLD_STORAGE", asalLat, asalLng, faktor, 3)
	}

	response.Success(c, out)
}

func tujuanLatToLng(v float64) float64 { return v }

func (h *HandlerTitik) hitungRuteJalan(
	ctx context.Context,
	asalLat, asalLng, tujuanLat, tujuanLng, faktor float64,
	profilOverride string,
) routing.HasilRute {
	client := h.router
	if client != nil && (profilOverride == "truck" || profilOverride == "auto") {
		client = client.WithProfile(profilOverride)
	}
	profileName := "truck"
	cacheScope := "haversine-truck"
	if client != nil {
		profileName = client.Profile()
		cacheScope = client.CacheScope()
	}

	disruptions := loadActiveDisruptions(ctx, h.db)
	cacheVersion := "0"
	if h.cache != nil {
		cacheVersion = h.cache.Version(ctx)
	}
	cacheKey := routing.RouteCacheKey(asalLat, asalLng, tujuanLat, tujuanLng, cacheScope, cacheVersion)
	if h.cache != nil {
		var cached routing.HasilRute
		if h.cache.GetRoute(ctx, cacheKey, &cached) && len(cached.Rute) > 0 && cached.SumberRute != "ESTIMASI_HAVERSINE" {
			return cached
		}
	}

	if client != nil && client.Enabled() {
		rawRoutes, sumberRute, err := client.FetchRoadRoutesWithSource(ctx, asalLat, asalLng, tujuanLat, tujuanLng, nil)
		if err == nil && len(rawRoutes) > 0 {
			hasil := evaluateDisruptionsAndReroute(ctx, client, sumberRute, asalLat, asalLng, tujuanLat, tujuanLng, rawRoutes, disruptions)
			if h.cache != nil {
				h.cache.SetRoute(ctx, cacheKey, hasil)
			}
			return hasil
		}
	}

	return buildHaversineFallbackRoute(asalLat, asalLng, tujuanLat, tujuanLng, faktor, profileName, disruptions)
}

func evaluateDisruptionsAndReroute(
	ctx context.Context,
	client *routing.Client,
	sumberRute string,
	asalLat, asalLng, tujuanLat, tujuanLng float64,
	rawRoutes []routing.OpsiRuteJalan,
	disruptions []routing.GangguanRef,
) routing.HasilRute {
	if sumberRute == "" {
		sumberRute = "VALHALLA_OSM"
	}
	hasil := routing.HasilRute{
		SumberRute:      sumberRute,
		ProfilKendaraan: client.Profile(),
		Rute:            rawRoutes,
	}
	if len(disruptions) == 0 || len(rawRoutes) == 0 {
		return hasil
	}

	primaryCoords := rawRoutes[0].Geometry.Coordinates
	hitTotal := make([]routing.GangguanRef, 0)
	hitWarn := make([]routing.GangguanRef, 0)
	for _, g := range disruptions {
		distM := routing.PointToPolylineMeters(g.Lat, g.Lng, primaryCoords)
		if distM <= float64(g.RadiusM) {
			if g.Tingkat == "TUTUP_TOTAL" {
				hitTotal = append(hitTotal, g)
			} else {
				hitWarn = append(hitWarn, g)
			}
		}
	}

	if len(hitTotal) > 0 {
		blocked := rawRoutes[0]
		blocked.Tipe = "Jalur Terdampak"
		blocked.TerdampakGangguan = true
		hasil.JalurUtamaDiblokir = true
		hasil.RuteDiblokir = &blocked
		hasil.GangguanTerkait = append(hitTotal, hitWarn...)

		var detours []routing.OpsiRuteJalan
		if client.HasValhalla() {
			if fetched, err := client.FetchRoadRoutes(ctx, asalLat, asalLng, tujuanLat, tujuanLng, hitTotal); err == nil {
				detours = fetched
			}
		} else {
			// OSRM: pakai alternatif bebas gangguan terlebih dahulu, atau cari detour waypoint tegak lurus.
			for _, alt := range rawRoutes[1:] {
				if !routeHitsAnyDisruption(alt.Geometry.Coordinates, hitTotal) {
					detours = append(detours, alt)
				}
			}
			if len(detours) == 0 && client.HasOSRM() {
				if fetched, err := client.FetchOSRMDetour(ctx, asalLat, asalLng, tujuanLat, tujuanLng, hitTotal[0]); err == nil {
					detours = fetched
				}
			}
		}

		if len(detours) > 0 {
			for i := range detours {
				detours[i].SelisihKm = math.Round((detours[i].JarakKm-blocked.JarakKm)*10) / 10
				detours[i].SelisihMenit = detours[i].DurasiMenit - blocked.DurasiMenit
				if i == 0 {
					detours[i].Tipe = "Jalur Pengalihan"
				} else {
					detours[i].Tipe = "Jalur Alternatif"
				}
				detours[i].TerdampakGangguan = routeHitsAnyDisruption(detours[i].Geometry.Coordinates, disruptions)
			}
			hasil.Rute = detours
			return hasil
		}

		// Bila hanya ada satu akses dan detour gagal, tandai semua opsi yang kena gangguan.
		for i := range hasil.Rute {
			hasil.Rute[i].TerdampakGangguan = routeHitsAnyDisruption(hasil.Rute[i].Geometry.Coordinates, disruptions)
		}
		return hasil
	}

	relatedMap := make(map[string]routing.GangguanRef)
	for i := range hasil.Rute {
		for _, g := range disruptions {
			if routing.PointToPolylineMeters(g.Lat, g.Lng, hasil.Rute[i].Geometry.Coordinates) <= float64(g.RadiusM) {
				hasil.Rute[i].TerdampakGangguan = true
				relatedMap[g.ID+":"+g.Kode] = g
			}
		}
	}
	for _, g := range relatedMap {
		hasil.GangguanTerkait = append(hasil.GangguanTerkait, g)
	}
	return hasil
}

func routeHitsAnyDisruption(coords [][]float64, disruptions []routing.GangguanRef) bool {
	for _, g := range disruptions {
		if routing.PointToPolylineMeters(g.Lat, g.Lng, coords) <= float64(g.RadiusM) {
			return true
		}
	}
	return false
}

func buildHaversineFallbackRoute(
	asalLat, asalLng, tujuanLat, tujuanLng, faktor float64,
	profileName string,
	disruptions []routing.GangguanRef,
) routing.HasilRute {
	lurus := domain.JarakKm(asalLat, asalLng, tujuanLat, tujuanLng)
	jalanKm := math.Round(lurus*faktor*10) / 10
	menit := int(math.Round((jalanKm / 45.0) * 60.0))
	if menit < 1 && jalanKm > 0 {
		menit = 1
	}
	coords := [][]float64{
		{asalLng, asalLat},
		{tujuanLng, tujuanLat},
	}

	hit := make([]routing.GangguanRef, 0)
	terblokir := false
	for _, g := range disruptions {
		if routing.PointToPolylineMeters(g.Lat, g.Lng, coords) <= float64(g.RadiusM) {
			hit = append(hit, g)
			if g.Tingkat == "TUTUP_TOTAL" {
				terblokir = true
			}
		}
	}

	rute := routing.OpsiRuteJalan{
		Tipe:              "Jalur Utama",
		RingkasanJalan:    "Estimasi jaringan jalan (Haversine)",
		JarakKm:           jalanKm,
		DurasiMenit:       menit,
		RisikoFoodLoss:    foodLossRiskFromMinutes(menit),
		TerdampakGangguan: len(hit) > 0,
		Geometry: routing.GeoJSONLineString{
			Type:        "LineString",
			Coordinates: coords,
		},
	}

	return routing.HasilRute{
		SumberRute:         "ESTIMASI_HAVERSINE",
		ProfilKendaraan:    profileName,
		JalurUtamaDiblokir: terblokir,
		Rute:               []routing.OpsiRuteJalan{rute},
		GangguanTerkait:    hit,
	}
}

func (h *HandlerTitik) resolvePoint(ctx context.Context, kode, rawLat, rawLng string) (float64, float64, bool) {
	if rawLat != "" && rawLng != "" {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(rawLat), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(rawLng), 64)
		if err1 == nil && err2 == nil && lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 {
			return lat, lng, true
		}
	}
	if kode == "" || h.db == nil {
		return 0, 0, false
	}

	type pt struct {
		Lat float64
		Lng float64
	}
	var res pt

	queries := []string{
		`SELECT lat, lng FROM lokasi_spasial WHERE kode = ? AND aktif = TRUE LIMIT 1`,
		`SELECT lat, lng FROM pasar WHERE kode = ? AND aktif = TRUE LIMIT 1`,
		`SELECT lat, lng FROM cold_storage WHERE (kode = ? OR id::text = ?) AND status = 'AKTIF' LIMIT 1`,
		`SELECT lat, lng FROM kabupaten WHERE kode = ? LIMIT 1`,
	}
	for i, sql := range queries {
		var tx *gorm.DB
		if i == 2 {
			tx = h.db.WithContext(ctx).Raw(sql, kode, kode)
		} else {
			tx = h.db.WithContext(ctx).Raw(sql, kode)
		}
		if err := tx.Scan(&res).Error; err == nil && (res.Lat != 0 || res.Lng != 0) {
			return res.Lat, res.Lng, true
		}
	}
	return 0, 0, false
}

func (h *HandlerTitik) findNearestFacilities(
	ctx context.Context,
	jenis string,
	asalLat, asalLng, faktor float64,
	limit int,
) []FasilitasTerdekat {
	type row struct {
		Kode          string
		Nama          string
		KabupatenKode string
		Lat           float64
		Lng           float64
		KapasitasTon  *float64
	}
	var rows []row
	if jenis == "PASAR" {
		_ = h.db.WithContext(ctx).Raw(
			`SELECT kode, nama, kabupaten_kode, lat, lng, NULL::float8 AS kapasitas_ton
			   FROM pasar
			  WHERE aktif = TRUE`,
		).Scan(&rows).Error
	} else {
		_ = h.db.WithContext(ctx).Raw(
			`SELECT COALESCE(NULLIF(kode, ''), id::text) AS kode, nama, kabupaten_kode, lat, lng, kapasitas_ton::float8 AS kapasitas_ton
			   FROM cold_storage
			  WHERE status = 'AKTIF'`,
		).Scan(&rows).Error
	}
	if len(rows) == 0 {
		return []FasilitasTerdekat{}
	}

	candidates := make([]FasilitasTerdekat, 0, len(rows))
	for _, r := range rows {
		lurus := domain.JarakKm(asalLat, asalLng, r.Lat, r.Lng)
		jalanKm := math.Round(lurus*faktor*10) / 10
		menit := int(math.Round((jalanKm / 45.0) * 60.0))
		if menit < 1 && jalanKm > 0 {
			menit = 1
		}
		candidates = append(candidates, FasilitasTerdekat{
			Jenis:               jenis,
			Kode:                r.Kode,
			Nama:                r.Nama,
			KabupatenKode:       r.KabupatenKode,
			Lat:                 r.Lat,
			Lng:                 r.Lng,
			JarakLurusKm:        math.Round(lurus*10) / 10,
			JarakJalanKm:        jalanKm,
			EstimasiMenitTempuh: menit,
			KapasitasTon:        r.KapasitasTon,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].JarakLurusKm < candidates[j].JarakLurusKm
	})

	poolSize := len(candidates)
	if poolSize > 8 {
		poolSize = 8
	}
	pool := candidates[:poolSize]

	if h.router != nil && h.router.Enabled() && len(pool) > 0 {
		targets := make([]routing.MatrixTarget, len(pool))
		for i, c := range pool {
			targets[i] = routing.MatrixTarget{Lat: c.Lat, Lng: c.Lng}
		}
		if cells, err := h.router.FetchMatrix(ctx, asalLat, asalLng, targets); err == nil && len(cells) == len(pool) {
			for i, cell := range cells {
				if cell.Valid {
					pool[i].JarakJalanKm = cell.JarakKm
					pool[i].EstimasiMenitTempuh = cell.DurasiMenit
				}
			}
			sort.Slice(pool, func(i, j int) bool {
				if pool[i].EstimasiMenitTempuh != pool[j].EstimasiMenitTempuh {
					return pool[i].EstimasiMenitTempuh < pool[j].EstimasiMenitTempuh
				}
				return pool[i].JarakJalanKm < pool[j].JarakJalanKm
			})
		}
	}

	if len(pool) > limit {
		pool = pool[:limit]
	}
	return pool
}

func foodLossRiskFromMinutes(menit int) string {
	switch {
	case menit >= 240:
		return "TINGGI"
	case menit >= 120:
		return "SEDANG"
	default:
		return "RENDAH"
	}
}
