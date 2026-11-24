package http

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TitikDesaItem struct {
	ID               string   `json:"id" gorm:"column:id"`
	Kode             string   `json:"kode" gorm:"column:kode"`
	Jenis            string   `json:"jenis" gorm:"column:jenis"`
	Nama             string   `json:"nama" gorm:"column:nama"`
	KabupatenKode    string   `json:"kabupaten_kode" gorm:"column:kabupaten_kode"`
	KabupatenNama    string   `json:"kabupaten_nama" gorm:"column:kabupaten_nama"`
	KecamatanKode    *string  `json:"kecamatan_kode,omitempty" gorm:"column:kecamatan_kode"`
	KomoditasUtama   string   `json:"komoditas_utama" gorm:"column:komoditas_utama"`
	LuasHa           *float64 `json:"luas_ha,omitempty" gorm:"column:luas_ha"`
	EstimasiPanenTon *float64 `json:"estimasi_panen_ton,omitempty" gorm:"column:estimasi_panen_ton"`
	Populasi         *int     `json:"populasi,omitempty" gorm:"column:populasi"`
	Lat              float64  `json:"lat" gorm:"column:lat"`
	Lng              float64  `json:"lng" gorm:"column:lng"`
}

type FasilitasTerdekat struct {
	Kode          string  `json:"kode"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	JarakLurusKm  float64 `json:"jarak_lurus_km"`
	JarakJalanKm  float64 `json:"jarak_jalan_km"`
	MenitTempuh   int     `json:"menit_tempuh"`
	Status        string  `json:"status,omitempty"`
}

type JarakResponse struct {
	AsalLat             float64             `json:"asal_lat"`
	AsalLng             float64             `json:"asal_lng"`
	TujuanLat           *float64            `json:"tujuan_lat,omitempty"`
	TujuanLng           *float64            `json:"tujuan_lng,omitempty"`
	FaktorJalan         float64             `json:"faktor_jalan"`
	JarakLurusKm        *float64            `json:"jarak_lurus_km,omitempty"`
	JarakJalanKm        *float64            `json:"jarak_jalan_km,omitempty"`
	JamTempuh           *float64            `json:"jam_tempuh,omitempty"`
	MenitTempuh         *int                `json:"menit_tempuh,omitempty"`
	RisikoFoodLoss      string              `json:"risiko_food_loss,omitempty"`
	PasarTerdekat       []FasilitasTerdekat `json:"pasar_terdekat"`
	ColdStorageTerdekat []FasilitasTerdekat `json:"cold_storage_terdekat"`
}

type HandlerTitik struct {
	db *gorm.DB
}

func NewTitik(db *gorm.DB) *HandlerTitik {
	return &HandlerTitik{db: db}
}

// ListTitikDesa handles GET /v1/spasial/titik-desa?jenis=PERTANIAN|PERKAMPUNGAN&kabupaten_kode=
func (h *HandlerTitik) ListTitikDesa(c *gin.Context) {
	jenis := strings.ToUpper(strings.TrimSpace(c.Query("jenis")))
	kab := strings.TrimSpace(c.Query("kabupaten_kode"))

	tx := h.db.WithContext(c.Request.Context()).
		Table("lokasi_spasial AS l").
		Joins("JOIN kabupaten kab ON kab.kode = l.kabupaten_kode").
		Where("l.aktif = TRUE")

	if jenis == "PERTANIAN" || jenis == "PERKAMPUNGAN" {
		tx = tx.Where("l.jenis = ?", jenis)
	}
	if kab != "" {
		tx = tx.Where("l.kabupaten_kode = ?", kab)
	}

	var items []TitikDesaItem
	err := tx.Select("l.id::text, l.kode, l.jenis, l.nama, l.kabupaten_kode, kab.nama AS kabupaten_nama, l.kecamatan_kode, l.komoditas_utama, l.luas_ha::float8, l.estimasi_panen_ton::float8, l.populasi, l.lat, l.lng").
		Order("l.jenis ASC, l.kode ASC").
		Scan(&items).Error
	if err != nil {
		response.InternalError(c, "Gagal membaca titik sentra pertanian & perkampungan")
		return
	}
	if items == nil {
		items = []TitikDesaItem{}
	}
	response.Success(c, items)
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func risikoDurasi(jam float64) string {
	switch {
	case jam >= 4.0:
		return "TINGGI"
	case jam >= 2.0:
		return "SEDANG"
	default:
		return "RENDAH"
	}
}

// HitungJarak handles GET /v1/spasial/jarak?asal_lat=&asal_lng=&tujuan_lat=&tujuan_lng=&faktor_jalan=
func (h *HandlerTitik) HitungJarak(c *gin.Context) {
	asalLat, err1 := strconv.ParseFloat(c.Query("asal_lat"), 64)
	asalLng, err2 := strconv.ParseFloat(c.Query("asal_lng"), 64)
	if err1 != nil || err2 != nil || asalLat < -90 || asalLat > 90 || asalLng < -180 || asalLng > 180 {
		response.BadRequest(c, "asal_lat dan asal_lng wajib berupa koordinat yang valid")
		return
	}

	faktor := 1.28 // rasio jalan pegunungan/pesisir Aceh terhadap jarak udara
	if rawF := strings.TrimSpace(c.Query("faktor_jalan")); rawF != "" {
		if f, err := strconv.ParseFloat(rawF, 64); err == nil && f >= 1.0 && f <= 2.5 {
			faktor = f
		}
	}

	out := JarakResponse{
		AsalLat:             asalLat,
		AsalLng:             asalLng,
		FaktorJalan:         faktor,
		PasarTerdekat:       []FasilitasTerdekat{},
		ColdStorageTerdekat: []FasilitasTerdekat{},
	}

	if c.Query("tujuan_lat") != "" && c.Query("tujuan_lng") != "" {
		tLat, e1 := strconv.ParseFloat(c.Query("tujuan_lat"), 64)
		tLng, e2 := strconv.ParseFloat(c.Query("tujuan_lng"), 64)
		if e1 != nil || e2 != nil || tLat < -90 || tLat > 90 || tLng < -180 || tLng > 180 {
			response.BadRequest(c, "tujuan_lat dan tujuan_lng tidak valid")
			return
		}
		lurus := round1(domain.JarakKm(asalLat, asalLng, tLat, tLng))
		jalan := round1(lurus * faktor)
		jam := round1(jalan / 42.0) // kecepatan rata-rata truk logistik Aceh ~42 km/jam
		menit := int(math.Round(jam * 60))
		out.TujuanLat = &tLat
		out.TujuanLng = &tLng
		out.JarakLurusKm = &lurus
		out.JarakJalanKm = &jalan
		out.JamTempuh = &jam
		out.MenitTempuh = &menit
		out.RisikoFoodLoss = risikoDurasi(jam)
	}

	if h.db != nil {
		ctx := c.Request.Context()
		var pasarRows []struct {
			Kode          string  `gorm:"column:kode"`
			Nama          string  `gorm:"column:nama"`
			KabupatenKode string  `gorm:"column:kabupaten_kode"`
			KabupatenNama string  `gorm:"column:kabupaten_nama"`
			Lat           float64 `gorm:"column:lat"`
			Lng           float64 `gorm:"column:lng"`
		}
		_ = h.db.WithContext(ctx).Table("pasar p").
			Joins("JOIN kabupaten k ON k.kode = p.kabupaten_kode").
			Select("p.kode, p.nama, p.kabupaten_kode, k.nama AS kabupaten_nama, p.lat, p.lng").
			Where("p.aktif = TRUE").
			Scan(&pasarRows).Error

		for _, p := range pasarRows {
			lurus := round1(domain.JarakKm(asalLat, asalLng, p.Lat, p.Lng))
			jalan := round1(lurus * faktor)
			out.PasarTerdekat = append(out.PasarTerdekat, FasilitasTerdekat{
				Kode: p.Kode, Nama: p.Nama, KabupatenKode: p.KabupatenKode, KabupatenNama: p.KabupatenNama,
				Lat: p.Lat, Lng: p.Lng, JarakLurusKm: lurus, JarakJalanKm: jalan,
				MenitTempuh: int(math.Round((jalan / 42.0) * 60)),
			})
		}
		sort.Slice(out.PasarTerdekat, func(i, j int) bool {
			return out.PasarTerdekat[i].JarakLurusKm < out.PasarTerdekat[j].JarakLurusKm
		})
		if len(out.PasarTerdekat) > 5 {
			out.PasarTerdekat = out.PasarTerdekat[:5]
		}

		var csRows []struct {
			Kode          string  `gorm:"column:kode"`
			Nama          string  `gorm:"column:nama"`
			KabupatenKode string  `gorm:"column:kabupaten_kode"`
			KabupatenNama string  `gorm:"column:kabupaten_nama"`
			Lat           float64 `gorm:"column:lat"`
			Lng           float64 `gorm:"column:lng"`
			Status        string  `gorm:"column:status"`
		}
		_ = h.db.WithContext(ctx).Table("cold_storage cs").
			Joins("JOIN kabupaten k ON k.kode = cs.kabupaten_kode").
			Select("cs.kode, cs.nama, cs.kabupaten_kode, k.nama AS kabupaten_nama, cs.lat, cs.lng, cs.status::text AS status").
			Scan(&csRows).Error

		for _, cs := range csRows {
			lurus := round1(domain.JarakKm(asalLat, asalLng, cs.Lat, cs.Lng))
			jalan := round1(lurus * faktor)
			out.ColdStorageTerdekat = append(out.ColdStorageTerdekat, FasilitasTerdekat{
				Kode: cs.Kode, Nama: cs.Nama, KabupatenKode: cs.KabupatenKode, KabupatenNama: cs.KabupatenNama,
				Lat: cs.Lat, Lng: cs.Lng, JarakLurusKm: lurus, JarakJalanKm: jalan,
				MenitTempuh: int(math.Round((jalan / 42.0) * 60)),
				Status:      cs.Status,
			})
		}
		sort.Slice(out.ColdStorageTerdekat, func(i, j int) bool {
			return out.ColdStorageTerdekat[i].JarakLurusKm < out.ColdStorageTerdekat[j].JarakLurusKm
		})
		if len(out.ColdStorageTerdekat) > 5 {
			out.ColdStorageTerdekat = out.ColdStorageTerdekat[:5]
		}
	}

	response.Success(c, out)
}
