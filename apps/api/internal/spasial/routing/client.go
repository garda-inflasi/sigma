package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultSearchCutoffMeters = 5000
	defaultRequestTimeout     = 5 * time.Second
	defaultPublicOSRMURL      = "https://router.project-osrm.org"
)

type GeoJSONLineString struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type GangguanRef struct {
	ID         string  `json:"id"`
	Kode       string  `json:"kode"`
	Judul      string  `json:"judul"`
	Jenis      string  `json:"jenis"`
	Tingkat    string  `json:"tingkat"`
	RuasJalan  string  `json:"ruas_jalan,omitempty"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	RadiusM    int     `json:"radius_m"`
	Keterangan string  `json:"keterangan,omitempty"`
}

type OpsiRuteJalan struct {
	Tipe              string            `json:"tipe"` // "Jalur Utama", "Jalur Pengalihan", "Jalur Alternatif"
	RingkasanJalan    string            `json:"ringkasan_jalan"`
	JarakKm           float64           `json:"jarak_km"`
	DurasiMenit       int               `json:"durasi_menit"`
	RisikoFoodLoss    string            `json:"risiko_food_loss"`
	SnapAsalM         float64           `json:"snap_asal_m,omitempty"`
	SnapTujuanM       float64           `json:"snap_tujuan_m,omitempty"`
	TerdampakGangguan bool              `json:"terdampak_gangguan"`
	SelisihKm         float64           `json:"selisih_km,omitempty"`
	SelisihMenit      int               `json:"selisih_menit,omitempty"`
	Geometry          GeoJSONLineString `json:"geometry"`
}

type HasilRute struct {
	SumberRute         string          `json:"sumber_rute"`
	ProfilKendaraan    string          `json:"profil_kendaraan"`
	JalurUtamaDiblokir bool            `json:"jalur_utama_diblokir"`
	Rute               []OpsiRuteJalan `json:"rute"`
	RuteDiblokir       *OpsiRuteJalan  `json:"rute_diblokir,omitempty"`
	GangguanTerkait    []GangguanRef   `json:"gangguan_terkait,omitempty"`
}

type MatrixTarget struct {
	Lat float64
	Lng float64
}

type MatrixCell struct {
	JarakKm     float64
	DurasiMenit int
	Valid       bool
}

type Client struct {
	baseURL    string
	osrmURL    string
	httpClient *http.Client
	profile    string
}

func NewClientFromEnv() *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("VALHALLA_URL")), "/")
	rawOSRM, hasOSRM := os.LookupEnv("OSRM_URL")
	osrmURL := defaultPublicOSRMURL
	if hasOSRM {
		trimmed := strings.TrimRight(strings.TrimSpace(rawOSRM), "/")
		switch strings.ToLower(trimmed) {
		case "", "off", "none", "false", "0":
			osrmURL = ""
		default:
			osrmURL = trimmed
		}
	}
	profile := strings.TrimSpace(os.Getenv("ROUTING_PROFILE"))
	if profile == "" {
		profile = "truck"
	}
	c := NewClient(baseURL, profile, nil)
	c.osrmURL = osrmURL
	return c
}

func NewClient(baseURL, profile string, httpClient *http.Client) *Client {
	if profile == "" {
		profile = "truck"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: httpClient,
		profile:    profile,
	}
}

func (c *Client) WithOSRM(osrmURL string) *Client {
	if c == nil {
		return nil
	}
	return &Client{
		baseURL:    c.baseURL,
		osrmURL:    strings.TrimRight(strings.TrimSpace(osrmURL), "/"),
		httpClient: c.httpClient,
		profile:    c.profile,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && (c.baseURL != "" || c.osrmURL != "")
}

func (c *Client) HasValhalla() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) HasOSRM() bool {
	return c != nil && c.osrmURL != ""
}

func (c *Client) CacheScope() string {
	if c == nil {
		return "none"
	}
	if c.baseURL != "" {
		return "valhalla-" + c.profile
	}
	if c.osrmURL != "" {
		return "osrm-" + c.profile
	}
	return "haversine-" + c.profile
}

func (c *Client) Profile() string {
	return c.profile
}

// WithProfile mengembalikan salinan Client dengan profil kendaraan berbeda ('truck' / 'auto') tanpa mutasi.
func (c *Client) WithProfile(profile string) *Client {
	if c == nil {
		return nil
	}
	trimmed := strings.TrimSpace(profile)
	if trimmed == "" || trimmed == c.profile {
		return c
	}
	return &Client{
		baseURL:    c.baseURL,
		osrmURL:    c.osrmURL,
		httpClient: c.httpClient,
		profile:    trimmed,
	}
}

type valhallaLocation struct {
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	SearchCutoff int     `json:"search_cutoff,omitempty"`
}

type valhallaAvoidLocation struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type valhallaRouteReq struct {
	Locations        []valhallaLocation      `json:"locations"`
	Costing          string                  `json:"costing"`
	Alternates       int                     `json:"alternates"`
	Units            string                  `json:"units"`
	Language         string                  `json:"language"`
	ExcludeLocations []valhallaAvoidLocation `json:"exclude_locations,omitempty"`
}

type valhallaTrip struct {
	Status  int `json:"status"`
	Summary struct {
		Length float64 `json:"length"`
		Time   float64 `json:"time"`
	} `json:"summary"`
	Legs []struct {
		Shape   string `json:"shape"`
		Summary struct {
			Length float64 `json:"length"`
			Time   float64 `json:"time"`
		} `json:"summary"`
		Maneuvers []struct {
			StreetNames []string `json:"street_names"`
		} `json:"maneuvers"`
	} `json:"legs"`
}

type valhallaRouteResp struct {
	Trip       valhallaTrip `json:"trip"`
	Alternates []struct {
		Trip valhallaTrip `json:"trip"`
	} `json:"alternates"`
}

// FetchRoadRoutes memanggil engine routing (Valhalla atau OSRM) untuk mendapatkan rute jaringan jalan raya.
func (c *Client) FetchRoadRoutes(
	ctx context.Context,
	asalLat, asalLng, tujuanLat, tujuanLng float64,
	avoid []GangguanRef,
) ([]OpsiRuteJalan, error) {
	routes, _, err := c.FetchRoadRoutesWithSource(ctx, asalLat, asalLng, tujuanLat, tujuanLng, avoid)
	return routes, err
}

// FetchRoadRoutesWithSource mengembalikan rute jalan raya beserta identitas sumbernya (VALHALLA_OSM / OSRM_OSM).
func (c *Client) FetchRoadRoutesWithSource(
	ctx context.Context,
	asalLat, asalLng, tujuanLat, tujuanLng float64,
	avoid []GangguanRef,
) ([]OpsiRuteJalan, string, error) {
	if !c.Enabled() {
		return nil, "", errors.New("engine routing belum dikonfigurasi")
	}

	if c.HasValhalla() {
		if routes, err := c.fetchValhallaRoutes(ctx, asalLat, asalLng, tujuanLat, tujuanLng, avoid); err == nil {
			return routes, "VALHALLA_OSM", nil
		}
	}

	if c.HasOSRM() {
		routes, err := c.fetchOSRMRoutes(ctx, asalLat, asalLng, tujuanLat, tujuanLng, nil)
		if err == nil {
			return routes, "OSRM_OSM", nil
		}
		return nil, "", err
	}

	return nil, "", errors.New("valhalla gagal dan OSRM tidak aktif")
}

func (c *Client) fetchValhallaRoutes(
	ctx context.Context,
	asalLat, asalLng, tujuanLat, tujuanLng float64,
	avoid []GangguanRef,
) ([]OpsiRuteJalan, error) {
	reqBody := valhallaRouteReq{
		Locations: []valhallaLocation{
			{Lat: asalLat, Lon: asalLng, SearchCutoff: defaultSearchCutoffMeters},
			{Lat: tujuanLat, Lon: tujuanLng, SearchCutoff: defaultSearchCutoffMeters},
		},
		Costing:    c.profile,
		Alternates: 2,
		Units:      "km",
		Language:   "id-ID",
	}
	for _, g := range avoid {
		reqBody.ExcludeLocations = append(reqBody.ExcludeLocations, valhallaAvoidLocation{Lat: g.Lat, Lon: g.Lng})
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/route", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("valhalla tidak terjangkau: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("valhalla HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed valhallaRouteResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	trips := make([]valhallaTrip, 0, 1+len(parsed.Alternates))
	trips = append(trips, parsed.Trip)
	for _, alt := range parsed.Alternates {
		trips = append(trips, alt.Trip)
	}

	routes := make([]OpsiRuteJalan, 0, len(trips))
	for i, trip := range trips {
		if len(trip.Legs) == 0 {
			continue
		}
		coords := DecodePolyline6(trip.Legs[0].Shape)
		if len(coords) < 2 {
			continue
		}
		jarakKm := math.Round(trip.Summary.Length*10) / 10
		menit := int(math.Round(trip.Summary.Time / 60.0))
		if menit < 1 && jarakKm > 0 {
			menit = 1
		}
		summaryStreet := extractMainStreets(trip)
		if summaryStreet == "" {
			summaryStreet = fmt.Sprintf("Koridor %d", i+1)
		}
		tipe := "Jalur Alternatif"
		if i == 0 {
			tipe = "Jalur Utama"
		}
		routes = append(routes, OpsiRuteJalan{
			Tipe:           tipe,
			RingkasanJalan: summaryStreet,
			JarakKm:        jarakKm,
			DurasiMenit:    menit,
			RisikoFoodLoss: foodLossRisk(menit),
			Geometry: GeoJSONLineString{
				Type:        "LineString",
				Coordinates: coords,
			},
		})
	}
	if len(routes) == 0 {
		return nil, errors.New("valhalla tidak mengembalikan geometri rute")
	}
	return routes, nil
}

type osrmRouteResp struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64           `json:"distance"` // meter
		Duration float64           `json:"duration"` // detik
		Geometry GeoJSONLineString `json:"geometry"`
		Legs     []struct {
			Steps []struct {
				Name string `json:"name"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
}

func (c *Client) fetchOSRMRoutes(
	ctx context.Context,
	asalLat, asalLng, tujuanLat, tujuanLng float64,
	waypoints []valhallaAvoidLocation,
) ([]OpsiRuteJalan, error) {
	if !c.HasOSRM() {
		return nil, errors.New("OSRM tidak aktif")
	}

	coordsParts := []string{fmt.Sprintf("%.6f,%.6f", asalLng, asalLat)}
	for _, wp := range waypoints {
		coordsParts = append(coordsParts, fmt.Sprintf("%.6f,%.6f", wp.Lon, wp.Lat))
	}
	coordsParts = append(coordsParts, fmt.Sprintf("%.6f,%.6f", tujuanLng, tujuanLat))

	altFlag := "true"
	if len(waypoints) > 0 {
		altFlag = "false"
	}
	url := fmt.Sprintf(
		"%s/route/v1/driving/%s?alternatives=%s&overview=full&geometries=geojson&steps=true",
		c.osrmURL,
		strings.Join(coordsParts, ";"),
		altFlag,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSRM tidak terjangkau: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM HTTP %d", resp.StatusCode)
	}

	var parsed osrmRouteResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Code != "Ok" || len(parsed.Routes) == 0 {
		return nil, errors.New("OSRM tidak menemukan rute jalan")
	}

	truckFactor := 1.0
	if c.profile == "truck" {
		truckFactor = 1.15
	}

	out := make([]OpsiRuteJalan, 0, len(parsed.Routes))
	for i, rt := range parsed.Routes {
		if len(rt.Geometry.Coordinates) < 2 {
			continue
		}
		jarakKm := math.Round((rt.Distance/1000.0)*10) / 10
		menit := int(math.Round((rt.Duration * truckFactor) / 60.0))
		if menit < 1 && jarakKm > 0 {
			menit = 1
		}
		streets := extractOSRMStreets(rt.Legs)
		if streets == "" {
			streets = fmt.Sprintf("Jaringan Jalan OSM #%d", i+1)
		}
		tipe := "Jalur Alternatif"
		if i == 0 {
			tipe = "Jalur Utama"
		}
		out = append(out, OpsiRuteJalan{
			Tipe:           tipe,
			RingkasanJalan: streets,
			JarakKm:        jarakKm,
			DurasiMenit:    menit,
			RisikoFoodLoss: foodLossRisk(menit),
			Geometry: GeoJSONLineString{
				Type:        "LineString",
				Coordinates: rt.Geometry.Coordinates,
			},
		})
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("geometri OSRM kosong")
	}
	return out, nil
}

// FetchOSRMDetour mencari jalur pengalihan di OSRM dengan menggeser titik bantu tegak lurus terhadap koridor dari titik gangguan.
func (c *Client) FetchOSRMDetour(
	ctx context.Context,
	asalLat, asalLng, tujuanLat, tujuanLng float64,
	avoid GangguanRef,
) ([]OpsiRuteJalan, error) {
	if !c.HasOSRM() {
		return nil, errors.New("OSRM tidak aktif")
	}
	dLat := tujuanLat - asalLat
	dLng := tujuanLng - asalLng
	norm := math.Hypot(dLat, dLng)
	if norm < 1e-6 {
		return nil, errors.New("asal dan tujuan berhimpit")
	}
	// Vektor satuan tegak lurus koridor: (-dLng/norm, dLat/norm).
	perpLat := -dLng / norm
	perpLng := dLat / norm

	radiusDeg := math.Max(float64(avoid.RadiusM)/111000.0*2.4, 0.015)
	offsets := []float64{radiusDeg, -radiusDeg, radiusDeg * 2.2, -radiusDeg * 2.2}

	bestScore := math.Inf(1)
	var bestRoutes []OpsiRuteJalan
	for _, off := range offsets {
		wp := valhallaAvoidLocation{
			Lat: avoid.Lat + perpLat*off,
			Lon: avoid.Lng + perpLng*off,
		}
		cand, err := c.fetchOSRMRoutes(ctx, asalLat, asalLng, tujuanLat, tujuanLng, []valhallaAvoidLocation{wp})
		if err != nil || len(cand) == 0 {
			continue
		}
		if PointToPolylineMeters(avoid.Lat, avoid.Lng, cand[0].Geometry.Coordinates) > float64(avoid.RadiusM) {
			if cand[0].JarakKm < bestScore {
				bestScore = cand[0].JarakKm
				bestRoutes = cand
			}
		}
	}
	if len(bestRoutes) == 0 {
		return nil, errors.New("tidak ditemukan rute pengalihan OSRM bebas gangguan")
	}
	return bestRoutes, nil
}

// FetchMatrix memanggil Valhalla /sources_to_targets atau OSRM /table untuk mengurutkan fasilitas menurut waktu tempuh jalan asli.
func (c *Client) FetchMatrix(
	ctx context.Context,
	asalLat, asalLng float64,
	targets []MatrixTarget,
) ([]MatrixCell, error) {
	if !c.Enabled() || len(targets) == 0 {
		return nil, errors.New("engine routing nonaktif atau target kosong")
	}

	if c.HasValhalla() {
		if cells, err := c.fetchValhallaMatrix(ctx, asalLat, asalLng, targets); err == nil {
			return cells, nil
		}
	}
	if c.HasOSRM() {
		return c.fetchOSRMMatrix(ctx, asalLat, asalLng, targets)
	}
	return nil, errors.New("matrix routing gagal")
}

func (c *Client) fetchValhallaMatrix(
	ctx context.Context,
	asalLat, asalLng float64,
	targets []MatrixTarget,
) ([]MatrixCell, error) {
	valTargets := make([]valhallaLocation, len(targets))
	for i, t := range targets {
		valTargets[i] = valhallaLocation{Lat: t.Lat, Lon: t.Lng, SearchCutoff: defaultSearchCutoffMeters}
	}
	reqBody := map[string]any{
		"sources": []valhallaLocation{{Lat: asalLat, Lon: asalLng, SearchCutoff: defaultSearchCutoffMeters}},
		"targets": valTargets,
		"costing": c.profile,
		"units":   "km",
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sources_to_targets", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("valhalla matrix HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		SourcesToTargets [][]struct {
			Distance *float64 `json:"distance"`
			Time     *float64 `json:"time"`
		} `json:"sources_to_targets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.SourcesToTargets) == 0 {
		return nil, errors.New("matrix kosong")
	}

	row := parsed.SourcesToTargets[0]
	out := make([]MatrixCell, len(targets))
	for i := 0; i < len(targets) && i < len(row); i++ {
		if row[i].Distance == nil || row[i].Time == nil {
			continue
		}
		out[i] = MatrixCell{
			JarakKm:     math.Round(*row[i].Distance*10) / 10,
			DurasiMenit: int(math.Round(*row[i].Time / 60.0)),
			Valid:       true,
		}
	}
	return out, nil
}

func (c *Client) fetchOSRMMatrix(
	ctx context.Context,
	asalLat, asalLng float64,
	targets []MatrixTarget,
) ([]MatrixCell, error) {
	coords := make([]string, 0, 1+len(targets))
	coords = append(coords, fmt.Sprintf("%.6f,%.6f", asalLng, asalLat))
	destIndices := make([]string, 0, len(targets))
	for i, t := range targets {
		coords = append(coords, fmt.Sprintf("%.6f,%.6f", t.Lng, t.Lat))
		destIndices = append(destIndices, fmt.Sprintf("%d", i+1))
	}
	url := fmt.Sprintf(
		"%s/table/v1/driving/%s?sources=0&destinations=%s&annotations=distance,duration",
		c.osrmURL,
		strings.Join(coords, ";"),
		strings.Join(destIndices, ";"),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM matrix HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		Code      string       `json:"code"`
		Distances [][]*float64 `json:"distances"`
		Durations [][]*float64 `json:"durations"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Code != "Ok" || len(parsed.Distances) == 0 || len(parsed.Durations) == 0 {
		return nil, errors.New("OSRM matrix kosong")
	}

	truckFactor := 1.0
	if c.profile == "truck" {
		truckFactor = 1.15
	}
	dists := parsed.Distances[0]
	durs := parsed.Durations[0]
	out := make([]MatrixCell, len(targets))
	for i := 0; i < len(targets) && i < len(dists) && i < len(durs); i++ {
		if dists[i] == nil || durs[i] == nil {
			continue
		}
		menit := int(math.Round((*durs[i] * truckFactor) / 60.0))
		km := math.Round((*dists[i]/1000.0)*10) / 10
		if menit < 1 && km > 0 {
			menit = 1
		}
		out[i] = MatrixCell{
			JarakKm:     km,
			DurasiMenit: menit,
			Valid:       true,
		}
	}
	return out, nil
}

func extractMainStreets(trip valhallaTrip) string {
	if len(trip.Legs) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	names := make([]string, 0, 3)
	for _, m := range trip.Legs[0].Maneuvers {
		for _, street := range m.StreetNames {
			clean := strings.TrimSpace(street)
			if clean == "" {
				continue
			}
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			names = append(names, clean)
			if len(names) >= 3 {
				return strings.Join(names, " → ")
			}
		}
	}
	return strings.Join(names, " → ")
}

func extractOSRMStreets(legs []struct {
	Steps []struct {
		Name string `json:"name"`
	} `json:"steps"`
}) string {
	seen := make(map[string]struct{})
	names := make([]string, 0, 3)
	for _, leg := range legs {
		for _, step := range leg.Steps {
			clean := strings.TrimSpace(step.Name)
			if clean == "" {
				continue
			}
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			names = append(names, clean)
			if len(names) >= 3 {
				return strings.Join(names, " → ")
			}
		}
	}
	return strings.Join(names, " → ")
}

func foodLossRisk(minutes int) string {
	switch {
	case minutes >= 240:
		return "TINGGI"
	case minutes >= 120:
		return "SEDANG"
	default:
		return "RENDAH"
	}
}
