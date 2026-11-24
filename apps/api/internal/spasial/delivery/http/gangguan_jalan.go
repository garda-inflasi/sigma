package http

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/spasial/routing"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandlerGangguan mengelola titik gangguan/bencana jalan raya yang memicu pengalihan rute otomatis.
type HandlerGangguan struct {
	db    *gorm.DB
	cache *routing.Cache
	audit audit.Writer
}

func NewGangguan(db *gorm.DB, cache *routing.Cache, auditWriter audit.Writer) *HandlerGangguan {
	return &HandlerGangguan{db: db, cache: cache, audit: auditWriter}
}

type GangguanJalanItem struct {
	ID               string     `json:"id"`
	Kode             string     `json:"kode"`
	Judul            string     `json:"judul"`
	Jenis            string     `json:"jenis"`
	Tingkat          string     `json:"tingkat"`
	KabupatenKode    *string    `json:"kabupaten_kode,omitempty"`
	KabupatenNama    string     `json:"kabupaten_nama,omitempty"`
	RuasJalan        string     `json:"ruas_jalan"`
	Lat              float64    `json:"lat"`
	Lng              float64    `json:"lng"`
	RadiusM          int        `json:"radius_m"`
	Keterangan       string     `json:"keterangan"`
	Aktif            bool       `json:"aktif"`
	Mulai            time.Time  `json:"mulai"`
	PerkiraanSelesai *time.Time `json:"perkiraan_selesai,omitempty"`
}

type createGangguanReq struct {
	Kode             string  `json:"kode"`
	Judul            string  `json:"judul" binding:"required,min=3,max=180"`
	Jenis            string  `json:"jenis" binding:"required"`
	Tingkat          string  `json:"tingkat"`
	KabupatenKode    *string `json:"kabupaten_kode"`
	RuasJalan        string  `json:"ruas_jalan"`
	Lat              float64 `json:"lat" binding:"required"`
	Lng              float64 `json:"lng" binding:"required"`
	RadiusM          int     `json:"radius_m"`
	Keterangan       string  `json:"keterangan"`
	Aktif            *bool   `json:"aktif"`
	PerkiraanSelesai *string `json:"perkiraan_selesai"`
}

type updateGangguanReq struct {
	Judul            *string  `json:"judul"`
	Jenis            *string  `json:"jenis"`
	Tingkat          *string  `json:"tingkat"`
	RuasJalan        *string  `json:"ruas_jalan"`
	Lat              *float64 `json:"lat"`
	Lng              *float64 `json:"lng"`
	RadiusM          *int     `json:"radius_m"`
	Keterangan       *string  `json:"keterangan"`
	Aktif            *bool    `json:"aktif"`
	PerkiraanSelesai *string  `json:"perkiraan_selesai"`
}

var allowedJenisGangguan = map[string]struct{}{
	"LONGSOR":        {},
	"BANJIR":         {},
	"JEMBATAN_PUTUS": {},
	"JALAN_RUSAK":    {},
	"PENUTUPAN":      {},
}

var allowedTingkatGangguan = map[string]struct{}{
	"TUTUP_TOTAL": {},
	"TERHAMBAT":   {},
}

func loadActiveDisruptions(ctx context.Context, db *gorm.DB) []routing.GangguanRef {
	if db == nil {
		return nil
	}
	var rows []routing.GangguanRef
	_ = db.WithContext(ctx).Raw(`
		SELECT id::text AS id, kode, judul, jenis, tingkat, ruas_jalan, lat, lng, radius_m, keterangan
		  FROM gangguan_jalan
		 WHERE aktif = TRUE
		 ORDER BY CASE WHEN tingkat = 'TUTUP_TOTAL' THEN 0 ELSE 1 END, mulai DESC`).
		Scan(&rows).Error
	return rows
}

func (h *HandlerGangguan) ListGangguanJalan(c *gin.Context) {
	if h.db == nil {
		response.Success(c, []GangguanJalanItem{})
		return
	}
	aktifParam := strings.ToLower(strings.TrimSpace(c.Query("aktif")))
	sql := `
		SELECT g.id::text AS id, g.kode, g.judul, g.jenis, g.tingkat,
		       g.kabupaten_kode, COALESCE(k.nama, '') AS kabupaten_nama,
		       g.ruas_jalan, g.lat, g.lng, g.radius_m, g.keterangan,
		       g.aktif, g.mulai, g.perkiraan_selesai
		  FROM gangguan_jalan g
		  LEFT JOIN kabupaten k ON k.kode = g.kabupaten_kode`
	args := make([]any, 0, 1)
	if aktifParam == "true" {
		sql += ` WHERE g.aktif = TRUE`
	} else if aktifParam == "false" {
		sql += ` WHERE g.aktif = FALSE`
	}
	sql += ` ORDER BY g.aktif DESC, g.mulai DESC`

	var items []GangguanJalanItem
	if err := h.db.WithContext(c.Request.Context()).Raw(sql, args...).Scan(&items).Error; err != nil {
		response.InternalError(c, "Gagal memuat daftar gangguan jalan")
		return
	}
	if items == nil {
		items = []GangguanJalanItem{}
	}
	response.Success(c, items)
}

func (h *HandlerGangguan) CreateGangguanJalan(c *gin.Context) {
	if h.db == nil {
		response.InternalError(c, "Koneksi basis data tidak tersedia")
		return
	}
	var req createGangguanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gangguan jalan gagal: "+err.Error())
		return
	}
	jenis := strings.ToUpper(strings.TrimSpace(req.Jenis))
	if _, ok := allowedJenisGangguan[jenis]; !ok {
		response.BadRequest(c, "Jenis gangguan harus LONGSOR, BANJIR, JEMBATAN_PUTUS, JALAN_RUSAK, atau PENUTUPAN")
		return
	}
	tingkat := strings.ToUpper(strings.TrimSpace(req.Tingkat))
	if tingkat == "" {
		tingkat = "TUTUP_TOTAL"
	}
	if _, ok := allowedTingkatGangguan[tingkat]; !ok {
		response.BadRequest(c, "Tingkat gangguan harus TUTUP_TOTAL atau TERHAMBAT")
		return
	}
	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		response.BadRequest(c, "Koordinat gangguan jalan tidak valid")
		return
	}
	radius := req.RadiusM
	if radius == 0 {
		radius = 350
	}
	if radius < 50 || radius > 10000 {
		response.BadRequest(c, "Radius terdampak harus antara 50 dan 10000 meter")
		return
	}
	kode := strings.ToUpper(strings.TrimSpace(req.Kode))
	if kode == "" {
		kode = fmt.Sprintf("GGJ-%d", time.Now().UnixMilli()%100000000)
	}
	aktif := true
	if req.Aktif != nil {
		aktif = *req.Aktif
	}
	var selesai *time.Time
	if req.PerkiraanSelesai != nil && strings.TrimSpace(*req.PerkiraanSelesai) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.PerkiraanSelesai))
		if err != nil {
			response.BadRequest(c, "Format perkiraan_selesai harus RFC3339")
			return
		}
		selesai = &parsed
	}

	var kabKode *string
	if req.KabupatenKode != nil && strings.TrimSpace(*req.KabupatenKode) != "" {
		trimmed := strings.TrimSpace(*req.KabupatenKode)
		kabKode = &trimmed
	}

	var id string
	err := h.db.WithContext(c.Request.Context()).Raw(`
		INSERT INTO gangguan_jalan (
			kode, judul, jenis, tingkat, kabupaten_kode, ruas_jalan,
			lat, lng, radius_m, keterangan, aktif, perkiraan_selesai
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id::text`,
		kode, strings.TrimSpace(req.Judul), jenis, tingkat, kabKode, strings.TrimSpace(req.RuasJalan),
		req.Lat, req.Lng, radius, strings.TrimSpace(req.Keterangan), aktif, selesai,
	).Scan(&id).Error
	if err != nil {
		response.BadRequest(c, "Gagal menyimpan gangguan jalan: "+err.Error())
		return
	}

	h.cache.InvalidateDisruptions(c.Request.Context())
	h.writeAudit(c, "TAMBAH_GANGGUAN_JALAN", id, map[string]any{
		"kode": kode, "judul": req.Judul, "jenis": jenis, "tingkat": tingkat, "lat": req.Lat, "lng": req.Lng,
	})
	response.Created(c, gin.H{"id": id, "kode": kode})
}

func (h *HandlerGangguan) UpdateGangguanJalan(c *gin.Context) {
	if h.db == nil {
		response.InternalError(c, "Koneksi basis data tidak tersedia")
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		response.BadRequest(c, "ID gangguan wajib diisi")
		return
	}
	var req updateGangguanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Payload pembaruan tidak valid: "+err.Error())
		return
	}

	sets := make([]string, 0, 8)
	args := make([]any, 0, 10)
	if req.Judul != nil {
		sets = append(sets, "judul = ?")
		args = append(args, strings.TrimSpace(*req.Judul))
	}
	if req.Jenis != nil {
		j := strings.ToUpper(strings.TrimSpace(*req.Jenis))
		if _, ok := allowedJenisGangguan[j]; !ok {
			response.BadRequest(c, "Jenis gangguan tidak dikenal")
			return
		}
		sets = append(sets, "jenis = ?")
		args = append(args, j)
	}
	if req.Tingkat != nil {
		t := strings.ToUpper(strings.TrimSpace(*req.Tingkat))
		if _, ok := allowedTingkatGangguan[t]; !ok {
			response.BadRequest(c, "Tingkat gangguan harus TUTUP_TOTAL atau TERHAMBAT")
			return
		}
		sets = append(sets, "tingkat = ?")
		args = append(args, t)
	}
	if req.RuasJalan != nil {
		sets = append(sets, "ruas_jalan = ?")
		args = append(args, strings.TrimSpace(*req.RuasJalan))
	}
	if req.Lat != nil {
		if *req.Lat < -90 || *req.Lat > 90 {
			response.BadRequest(c, "Latitude di luar rentang")
			return
		}
		sets = append(sets, "lat = ?")
		args = append(args, *req.Lat)
	}
	if req.Lng != nil {
		if *req.Lng < -180 || *req.Lng > 180 {
			response.BadRequest(c, "Longitude di luar rentang")
			return
		}
		sets = append(sets, "lng = ?")
		args = append(args, *req.Lng)
	}
	if req.RadiusM != nil {
		if *req.RadiusM < 50 || *req.RadiusM > 10000 {
			response.BadRequest(c, "Radius terdampak harus 50..10000 meter")
			return
		}
		sets = append(sets, "radius_m = ?")
		args = append(args, *req.RadiusM)
	}
	if req.Keterangan != nil {
		sets = append(sets, "keterangan = ?")
		args = append(args, strings.TrimSpace(*req.Keterangan))
	}
	if req.Aktif != nil {
		sets = append(sets, "aktif = ?")
		args = append(args, *req.Aktif)
	}
	if req.PerkiraanSelesai != nil {
		trimmed := strings.TrimSpace(*req.PerkiraanSelesai)
		if trimmed == "" {
			sets = append(sets, "perkiraan_selesai = NULL")
		} else {
			parsed, err := time.Parse(time.RFC3339, trimmed)
			if err != nil {
				response.BadRequest(c, "Format perkiraan_selesai harus RFC3339")
				return
			}
			sets = append(sets, "perkiraan_selesai = ?")
			args = append(args, parsed)
		}
	}
	if len(sets) == 0 {
		response.BadRequest(c, "Tidak ada kolom yang diperbarui")
		return
	}
	sets = append(sets, "diperbarui_pada = now()")
	args = append(args, id)

	res := h.db.WithContext(c.Request.Context()).Exec(
		fmt.Sprintf("UPDATE gangguan_jalan SET %s WHERE id::text = ?", strings.Join(sets, ", ")),
		args...,
	)
	if res.Error != nil {
		response.BadRequest(c, "Gagal memperbarui gangguan jalan: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		response.NotFound(c, "Gangguan jalan tidak ditemukan")
		return
	}

	h.cache.InvalidateDisruptions(c.Request.Context())
	h.writeAudit(c, "UBAH_GANGGUAN_JALAN", id, map[string]any{"aktif": req.Aktif, "tingkat": req.Tingkat})
	response.Success(c, gin.H{"id": id, "updated": true})
}

func (h *HandlerGangguan) DeleteGangguanJalan(c *gin.Context) {
	if h.db == nil {
		response.InternalError(c, "Koneksi basis data tidak tersedia")
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		response.BadRequest(c, "ID gangguan wajib diisi")
		return
	}
	res := h.db.WithContext(c.Request.Context()).Exec("DELETE FROM gangguan_jalan WHERE id::text = ?", id)
	if res.Error != nil {
		response.BadRequest(c, "Gagal menghapus gangguan jalan: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		response.NotFound(c, "Gangguan jalan tidak ditemukan")
		return
	}

	h.cache.InvalidateDisruptions(c.Request.Context())
	h.writeAudit(c, "HAPUS_GANGGUAN_JALAN", id, nil)
	response.Success(c, gin.H{"id": id, "deleted": true})
}

func (h *HandlerGangguan) writeAudit(c *gin.Context, aksi, entitasID string, meta map[string]any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Write(audit.RequestContext(c), audit.Entry{
		AktorID:   c.GetString("user_id"),
		Aksi:      aksi,
		Entitas:   "gangguan_jalan",
		EntitasID: entitasID,
		Hasil:     audit.HasilBerhasil,
		Metadata:  meta,
	})
}
