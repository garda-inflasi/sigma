package http

import (
	"errors"
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// LokasiSpasialRow adalah representasi master sentra pertanian dan perkampungan.
type LokasiSpasialRow struct {
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
	Aktif            bool     `json:"aktif" gorm:"column:aktif"`
	Versi            int      `json:"versi" gorm:"column:versi"`
}

// HandlerLangsung menyediakan endpoint baca lokasi spasial serta input/ubah langsung master data khusus ADMIN_SISTEM.
type HandlerLangsung struct {
	db    *gorm.DB
	audit audit.Writer
}

func NewLangsung(db *gorm.DB, auditWriter audit.Writer) *HandlerLangsung {
	return &HandlerLangsung{db: db, audit: auditWriter}
}

func pgErrKode(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code
	}
	return ""
}

// ListLokasiSpasial handles GET /v1/master/lokasi-spasial?jenis=&kabupaten_kode=&q=&page=&limit=
func (h *HandlerLangsung) ListLokasiSpasial(c *gin.Context) {
	jenis := strings.ToUpper(strings.TrimSpace(c.Query("jenis")))
	kab := strings.TrimSpace(c.Query("kabupaten_kode"))
	q := strings.TrimSpace(c.Query("q"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}

	tx := h.db.WithContext(c.Request.Context()).
		Table("lokasi_spasial AS l").
		Joins("JOIN kabupaten kab ON kab.kode = l.kabupaten_kode")

	if jenis == "PERTANIAN" || jenis == "PERKAMPUNGAN" {
		tx = tx.Where("l.jenis = ?", jenis)
	}
	if kab != "" {
		tx = tx.Where("l.kabupaten_kode = ?", kab)
	}
	if q != "" {
		pat := "%" + strings.ToLower(q) + "%"
		tx = tx.Where("(LOWER(l.kode) LIKE ? OR LOWER(l.nama) LIKE ? OR LOWER(l.komoditas_utama) LIKE ?)", pat, pat, pat)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		response.InternalError(c, "Gagal menghitung lokasi spasial")
		return
	}

	var rows []LokasiSpasialRow
	err := tx.Select("l.id::text, l.kode, l.jenis, l.nama, l.kabupaten_kode, kab.nama AS kabupaten_nama, l.kecamatan_kode, l.komoditas_utama, l.luas_ha::float8, l.estimasi_panen_ton::float8, l.populasi, l.lat, l.lng, l.aktif, l.versi").
		Order("l.jenis ASC, l.kode ASC").
		Limit(limit).
		Offset((page - 1) * limit).
		Scan(&rows).Error
	if err != nil {
		response.InternalError(c, "Gagal membaca lokasi spasial")
		return
	}
	if rows == nil {
		rows = []LokasiSpasialRow{}
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, rows)
}

type komoditasLangsungReq struct {
	Kode          string `json:"kode" binding:"required"`
	Nama          string `json:"nama" binding:"required"`
	Satuan        string `json:"satuan" binding:"required"`
	Kategori      string `json:"kategori" binding:"required"`
	HargaAcuan    int    `json:"harga_acuan" binding:"min=0"`
	ShelfLifeHari int    `json:"shelf_life_hari" binding:"min=1"`
	Pilot         bool   `json:"pilot"`
	Status        string `json:"status"`
}

type pasarLangsungReq struct {
	Kode          string  `json:"kode" binding:"required"`
	Nama          string  `json:"nama" binding:"required"`
	KabupatenKode string  `json:"kabupaten_kode" binding:"required"`
	KecamatanKode *string `json:"kecamatan_kode"`
	Lat           float64 `json:"lat" binding:"required"`
	Lng           float64 `json:"lng" binding:"required"`
	RadiusMeter   int     `json:"radius_meter"`
	Aktif         *bool   `json:"aktif"`
}

type kecamatanLangsungReq struct {
	Kode          string  `json:"kode" binding:"required"`
	Nama          string  `json:"nama" binding:"required"`
	KabupatenKode string  `json:"kabupaten_kode" binding:"required"`
	KodeDagri     *string `json:"kode_dagri"`
}

type satuanLangsungReq struct {
	Kode string `json:"kode" binding:"required"`
	Nama string `json:"nama" binding:"required"`
}

type pedagangLangsungReq struct {
	Kode    string `json:"kode" binding:"required"`
	Nama    string `json:"nama" binding:"required"`
	PasarID string `json:"pasar_id" binding:"required"`
	Aktif   *bool  `json:"aktif"`
}

type lokasiSpasialReq struct {
	Kode             string   `json:"kode" binding:"required"`
	Nama             string   `json:"nama" binding:"required"`
	KabupatenKode    string   `json:"kabupaten_kode" binding:"required"`
	KecamatanKode    *string  `json:"kecamatan_kode"`
	KomoditasUtama   string   `json:"komoditas_utama"`
	LuasHa           *float64 `json:"luas_ha"`
	EstimasiPanenTon *float64 `json:"estimasi_panen_ton"`
	Populasi         *int     `json:"populasi"`
	Lat              float64  `json:"lat" binding:"required"`
	Lng              float64  `json:"lng" binding:"required"`
	Aktif            *bool    `json:"aktif"`
}

func validKoordinat(lat, lng float64) bool {
	return lat >= -11 && lat <= 6.5 && lng >= 94 && lng <= 141
}

// CreateLangsung handles POST /v1/master/langsung/:entitas (ADMIN_SISTEM).
func (h *HandlerLangsung) CreateLangsung(c *gin.Context) {
	h.simpanLangsung(c, false)
}

// UpdateLangsung handles PUT /v1/master/langsung/:entitas/:kode (ADMIN_SISTEM).
func (h *HandlerLangsung) UpdateLangsung(c *gin.Context) {
	h.simpanLangsung(c, true)
}

func (h *HandlerLangsung) simpanLangsung(c *gin.Context, isUpdate bool) {
	entitas := strings.ToLower(strings.TrimSpace(c.Param("entitas")))
	kodeParam := strings.TrimSpace(c.Param("kode"))
	ctx := c.Request.Context()
	aktorID := c.GetString("user_id")

	writeAudit := func(aksi, entitasNama, entitasID string, meta map[string]any) {
		if h.audit == nil {
			return
		}
		_ = h.audit.Write(ctx, audit.Entry{
			AktorID:   aktorID,
			Aksi:      aksi,
			Entitas:   entitasNama,
			EntitasID: entitasID,
			Hasil:     audit.HasilBerhasil,
			Metadata:  meta,
		})
	}

	switch entitas {
	case "komoditas":
		var req komoditasLangsungReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi komoditas gagal: "+err.Error())
			return
		}
		kode := strings.ToUpper(strings.TrimSpace(req.Kode))
		if isUpdate && kodeParam != "" {
			kode = kodeParam
		}
		status := strings.ToUpper(strings.TrimSpace(req.Status))
		if status == "" {
			status = domain.StatusAktif
		}
		var err error
		if !isUpdate {
			err = h.db.WithContext(ctx).Exec(
				`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari, pilot, status, versi)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
				kode, strings.TrimSpace(req.Nama), strings.TrimSpace(req.Satuan), strings.TrimSpace(req.Kategori),
				req.HargaAcuan, req.ShelfLifeHari, req.Pilot, status,
			).Error
		} else {
			res := h.db.WithContext(ctx).Exec(
				`UPDATE komoditas
				 SET nama = ?, satuan = ?, kategori = ?, harga_acuan = ?, shelf_life_hari = ?, pilot = ?, status = ?, versi = versi + 1
				 WHERE kode = ?`,
				strings.TrimSpace(req.Nama), strings.TrimSpace(req.Satuan), strings.TrimSpace(req.Kategori),
				req.HargaAcuan, req.ShelfLifeHari, req.Pilot, status, kode,
			)
			err = res.Error
			if err == nil && res.RowsAffected == 0 {
				response.NotFound(c, "Komoditas tidak ditemukan")
				return
			}
		}
		if err != nil {
			if pgErrKode(err) == "23505" {
				response.Error(c, stdhttp.StatusConflict, "409", "Kode komoditas sudah ada")
				return
			}
			response.BadRequest(c, "Gagal menyimpan komoditas: "+err.Error())
			return
		}
		writeAudit(map[bool]string{true: "MASTER_LANGSUNG_UBAH", false: "MASTER_LANGSUNG_TAMBAH"}[isUpdate], "komoditas", kode, map[string]any{"nama": req.Nama})
		response.Success(c, gin.H{"kode": kode, "entitas": entitas})

	case "pasar":
		var req pasarLangsungReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi pasar gagal: "+err.Error())
			return
		}
		if !validKoordinat(req.Lat, req.Lng) {
			response.BadRequest(c, "Koordinat pasar di luar wilayah Indonesia")
			return
		}
		kode := strings.TrimSpace(req.Kode)
		if isUpdate && kodeParam != "" {
			kode = kodeParam
		}
		radius := req.RadiusMeter
		if radius <= 0 {
			radius = 400
		}
		aktif := true
		if req.Aktif != nil {
			aktif = *req.Aktif
		}
		var err error
		if !isUpdate {
			err = h.db.WithContext(ctx).Exec(
				`INSERT INTO pasar (kode, nama, kabupaten_kode, kecamatan_kode, lat, lng, radius_meter, aktif, versi)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
				kode, strings.TrimSpace(req.Nama), req.KabupatenKode, req.KecamatanKode, req.Lat, req.Lng, radius, aktif,
			).Error
		} else {
			res := h.db.WithContext(ctx).Exec(
				`UPDATE pasar
				 SET nama = ?, kabupaten_kode = ?, kecamatan_kode = ?, lat = ?, lng = ?, radius_meter = ?, aktif = ?, versi = versi + 1
				 WHERE kode = ?`,
				strings.TrimSpace(req.Nama), req.KabupatenKode, req.KecamatanKode, req.Lat, req.Lng, radius, aktif, kode,
			)
			err = res.Error
			if err == nil && res.RowsAffected == 0 {
				response.NotFound(c, "Pasar tidak ditemukan")
				return
			}
		}
		if err != nil {
			if pgErrKode(err) == "23505" {
				response.Error(c, stdhttp.StatusConflict, "409", "Kode pasar sudah ada")
				return
			}
			response.BadRequest(c, "Gagal menyimpan pasar: "+err.Error())
			return
		}
		writeAudit(map[bool]string{true: "MASTER_LANGSUNG_UBAH", false: "MASTER_LANGSUNG_TAMBAH"}[isUpdate], "pasar", kode, map[string]any{"nama": req.Nama, "lat": req.Lat, "lng": req.Lng})
		response.Success(c, gin.H{"kode": kode, "entitas": entitas})

	case "kecamatan":
		var req kecamatanLangsungReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi kecamatan gagal: "+err.Error())
			return
		}
		kode := strings.TrimSpace(req.Kode)
		if isUpdate && kodeParam != "" {
			kode = kodeParam
		}
		var err error
		if !isUpdate {
			err = h.db.WithContext(ctx).Exec(
				`INSERT INTO kecamatan (kode, nama, kabupaten_kode, kode_dagri) VALUES (?, ?, ?, ?)`,
				kode, strings.TrimSpace(req.Nama), req.KabupatenKode, req.KodeDagri,
			).Error
		} else {
			res := h.db.WithContext(ctx).Exec(
				`UPDATE kecamatan SET nama = ?, kabupaten_kode = ?, kode_dagri = ? WHERE kode = ?`,
				strings.TrimSpace(req.Nama), req.KabupatenKode, req.KodeDagri, kode,
			)
			err = res.Error
			if err == nil && res.RowsAffected == 0 {
				response.NotFound(c, "Kecamatan tidak ditemukan")
				return
			}
		}
		if err != nil {
			response.BadRequest(c, "Gagal menyimpan kecamatan: "+err.Error())
			return
		}
		writeAudit(map[bool]string{true: "MASTER_LANGSUNG_UBAH", false: "MASTER_LANGSUNG_TAMBAH"}[isUpdate], "kecamatan", kode, map[string]any{"nama": req.Nama})
		response.Success(c, gin.H{"kode": kode, "entitas": entitas})

	case "satuan":
		var req satuanLangsungReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi satuan gagal: "+err.Error())
			return
		}
		kode := strings.ToLower(strings.TrimSpace(req.Kode))
		if isUpdate && kodeParam != "" {
			kode = kodeParam
		}
		var err error
		if !isUpdate {
			err = h.db.WithContext(ctx).Exec(`INSERT INTO satuan (kode, nama) VALUES (?, ?)`, kode, strings.TrimSpace(req.Nama)).Error
		} else {
			res := h.db.WithContext(ctx).Exec(`UPDATE satuan SET nama = ? WHERE kode = ?`, strings.TrimSpace(req.Nama), kode)
			err = res.Error
			if err == nil && res.RowsAffected == 0 {
				response.NotFound(c, "Satuan tidak ditemukan")
				return
			}
		}
		if err != nil {
			response.BadRequest(c, "Gagal menyimpan satuan: "+err.Error())
			return
		}
		writeAudit(map[bool]string{true: "MASTER_LANGSUNG_UBAH", false: "MASTER_LANGSUNG_TAMBAH"}[isUpdate], "satuan", kode, map[string]any{"nama": req.Nama})
		response.Success(c, gin.H{"kode": kode, "entitas": entitas})

	case "pedagang":
		var req pedagangLangsungReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi pedagang gagal: "+err.Error())
			return
		}
		kode := strings.TrimSpace(req.Kode)
		if isUpdate && kodeParam != "" {
			kode = kodeParam
		}
		aktif := true
		if req.Aktif != nil {
			aktif = *req.Aktif
		}
		var err error
		if !isUpdate {
			err = h.db.WithContext(ctx).Exec(
				`INSERT INTO pedagang (kode, nama, pasar_id, aktif) VALUES (?, ?, ?, ?)`,
				kode, strings.TrimSpace(req.Nama), req.PasarID, aktif,
			).Error
		} else {
			res := h.db.WithContext(ctx).Exec(
				`UPDATE pedagang SET nama = ?, pasar_id = ?, aktif = ? WHERE kode = ?`,
				strings.TrimSpace(req.Nama), req.PasarID, aktif, kode,
			)
			err = res.Error
			if err == nil && res.RowsAffected == 0 {
				response.NotFound(c, "Pedagang tidak ditemukan")
				return
			}
		}
		if err != nil {
			response.BadRequest(c, "Gagal menyimpan pedagang: "+err.Error())
			return
		}
		writeAudit(map[bool]string{true: "MASTER_LANGSUNG_UBAH", false: "MASTER_LANGSUNG_TAMBAH"}[isUpdate], "pedagang", kode, map[string]any{"nama": req.Nama})
		response.Success(c, gin.H{"kode": kode, "entitas": entitas})

	case "pertanian", "perkampungan":
		var req lokasiSpasialReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi lokasi gagal: "+err.Error())
			return
		}
		if !validKoordinat(req.Lat, req.Lng) {
			response.BadRequest(c, "Koordinat lokasi di luar wilayah Indonesia")
			return
		}
		jenis := "PERTANIAN"
		if entitas == "perkampungan" {
			jenis = "PERKAMPUNGAN"
		}
		kode := strings.ToUpper(strings.TrimSpace(req.Kode))
		if isUpdate && kodeParam != "" {
			kode = kodeParam
		}
		aktif := true
		if req.Aktif != nil {
			aktif = *req.Aktif
		}
		var err error
		if !isUpdate {
			err = h.db.WithContext(ctx).Exec(
				`INSERT INTO lokasi_spasial (kode, jenis, nama, kabupaten_kode, kecamatan_kode, komoditas_utama, luas_ha, estimasi_panen_ton, populasi, lat, lng, aktif, versi)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
				kode, jenis, strings.TrimSpace(req.Nama), req.KabupatenKode, req.KecamatanKode,
				strings.TrimSpace(req.KomoditasUtama), req.LuasHa, req.EstimasiPanenTon, req.Populasi,
				req.Lat, req.Lng, aktif,
			).Error
		} else {
			res := h.db.WithContext(ctx).Exec(
				`UPDATE lokasi_spasial
				 SET nama = ?, kabupaten_kode = ?, kecamatan_kode = ?, komoditas_utama = ?, luas_ha = ?, estimasi_panen_ton = ?, populasi = ?, lat = ?, lng = ?, aktif = ?, versi = versi + 1, diperbarui_pada = now()
				 WHERE kode = ? AND jenis = ?`,
				strings.TrimSpace(req.Nama), req.KabupatenKode, req.KecamatanKode,
				strings.TrimSpace(req.KomoditasUtama), req.LuasHa, req.EstimasiPanenTon, req.Populasi,
				req.Lat, req.Lng, aktif, kode, jenis,
			)
			err = res.Error
			if err == nil && res.RowsAffected == 0 {
				response.NotFound(c, "Lokasi tidak ditemukan")
				return
			}
		}
		if err != nil {
			if pgErrKode(err) == "23505" {
				response.Error(c, stdhttp.StatusConflict, "409", "Kode lokasi sudah ada")
				return
			}
			response.BadRequest(c, "Gagal menyimpan lokasi: "+err.Error())
			return
		}
		writeAudit(map[bool]string{true: "MASTER_LANGSUNG_UBAH", false: "MASTER_LANGSUNG_TAMBAH"}[isUpdate], "lokasi_spasial", kode, map[string]any{"jenis": jenis, "nama": req.Nama, "lat": req.Lat, "lng": req.Lng})
		response.Success(c, gin.H{"kode": kode, "jenis": jenis})

	default:
		response.BadRequest(c, "Entitas master tidak didukung: "+entitas)
	}
}

// DeleteLangsung handles DELETE /v1/master/langsung/:entitas/:kode (ADMIN_SISTEM).
func (h *HandlerLangsung) DeleteLangsung(c *gin.Context) {
	entitas := strings.ToLower(strings.TrimSpace(c.Param("entitas")))
	kode := strings.TrimSpace(c.Param("kode"))
	ctx := c.Request.Context()

	var err error
	var rowsAffected int64
	switch entitas {
	case "pertanian", "perkampungan":
		jenis := "PERTANIAN"
		if entitas == "perkampungan" {
			jenis = "PERKAMPUNGAN"
		}
		res := h.db.WithContext(ctx).Exec(`DELETE FROM lokasi_spasial WHERE kode = ? AND jenis = ?`, kode, jenis)
		err, rowsAffected = res.Error, res.RowsAffected
	case "kecamatan":
		res := h.db.WithContext(ctx).Exec(`DELETE FROM kecamatan WHERE kode = ?`, kode)
		err, rowsAffected = res.Error, res.RowsAffected
	case "satuan":
		res := h.db.WithContext(ctx).Exec(`DELETE FROM satuan WHERE kode = ?`, kode)
		err, rowsAffected = res.Error, res.RowsAffected
	case "pedagang":
		res := h.db.WithContext(ctx).Exec(`DELETE FROM pedagang WHERE kode = ?`, kode)
		err, rowsAffected = res.Error, res.RowsAffected
	default:
		response.BadRequest(c, "Gunakan arsip atau endpoint hapus standar untuk entitas "+entitas)
		return
	}

	if err != nil {
		if pgErrKode(err) == "23503" {
			response.Error(c, stdhttp.StatusConflict, kodeHapusDirujuk, "Data masih dirujuk transaksi lain dan tidak dapat dihapus.")
			return
		}
		response.InternalError(c, "Gagal menghapus data master")
		return
	}
	if rowsAffected == 0 {
		response.NotFound(c, "Data master tidak ditemukan")
		return
	}
	if h.audit != nil {
		_ = h.audit.Write(ctx, audit.Entry{
			AktorID:   c.GetString("user_id"),
			Aksi:      "MASTER_LANGSUNG_HAPUS",
			Entitas:   entitas,
			EntitasID: kode,
			Hasil:     audit.HasilBerhasil,
		})
	}
	response.NoContent(c)
}
