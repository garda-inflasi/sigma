// Package http adalah lapisan HTTP modul spasial.
package http

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/pkg/middleware"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/internal/spasial/dto"
	"github.com/garda-inflasi/api/internal/spasial/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc  *usecase.UseCase
	opt *usecase.Optimasi
}

func New(uc *usecase.UseCase, opt *usecase.Optimasi) *Handler {
	return &Handler{uc: uc, opt: opt}
}

// ListWilayah handles GET /v1/spasial/wilayah
func (h *Handler) ListWilayah(c *gin.Context) {
	items, err := h.uc.ListMarker(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data wilayah")
		return
	}
	response.Success(c, dto.ToMarkerResponses(items))
}

// ListNeraca handles GET /v1/spasial/neraca?komoditas=&periode=YYYY-MM-DD
func (h *Handler) ListNeraca(c *gin.Context) {
	var req dto.NeracaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	items, err := h.uc.ListNeraca(c.Request.Context(), domain.NeracaQuery{KomoditasKode: req.Komoditas, Periode: req.Periode})
	if err != nil {
		response.InternalError(c, "Gagal mengambil neraca wilayah")
		return
	}
	response.Success(c, dto.ToNeracaResponses(items))
}

// ListRute handles GET /v1/spasial/rute
func (h *Handler) ListRute(c *gin.Context) {
	items, err := h.uc.ListRute(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil rute distribusi")
		return
	}
	response.Success(c, dto.ToRuteResponses(items))
}

// ListKandidat handles GET /v1/spasial/kandidat-cold-storage
func (h *Handler) ListKandidat(c *gin.Context) {
	items, err := h.uc.ListKandidat(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil kandidat cold storage")
		return
	}
	out, err := dto.ToKandidatResponses(items)
	if err != nil {
		response.InternalError(c, "Data kandidat cold storage tidak valid")
		return
	}
	response.Success(c, out)
}

// ListGeometri handles GET /v1/spasial/geometri?zoom=&bbox=minLng,minLat,maxLng,maxLat
func (h *Handler) ListGeometri(c *gin.Context) {
	var req dto.GeometriRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	bbox, err := parseBBox(req.BBox)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	zoom := -1
	if req.Zoom != nil {
		zoom = *req.Zoom
	}
	items, err := h.uc.Geometri(c.Request.Context(), zoom, bbox)
	if err != nil {
		response.InternalError(c, "Gagal mengambil geometri wilayah")
		return
	}
	response.Success(c, dto.ToGeometriResponse(items))
}

// Lokasi handles GET /v1/spasial/lokasi?lat=&lng=
func (h *Handler) Lokasi(c *gin.Context) {
	var req dto.LokasiRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	lokasi, err := h.uc.Lokasi(c.Request.Context(), *req.Lat, *req.Lng)
	if err != nil {
		if errors.Is(err, usecase.ErrTidakAdaWilayah) {
			response.NotFound(c, "Belum ada data wilayah")
			return
		}
		response.InternalError(c, "Gagal mencari wilayah")
		return
	}
	response.Success(c, dto.LokasiResponse{Kode: lokasi.Kode, Nama: lokasi.Nama, DiDalam: lokasi.DiDalam, JarakPusatKm: lokasi.JarakPusatKm})
}

// parseBBox membaca "minLng,minLat,maxLng,maxLat"; kosong berarti tanpa batas.
func parseBBox(raw string) (*domain.BBox, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, errors.New("bbox harus berformat minLng,minLat,maxLng,maxLat")
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, errors.New("bbox harus berisi empat angka")
		}
		v[i] = f
	}
	if v[0] > v[2] || v[1] > v[3] || v[0] < -180 || v[2] > 180 || v[1] < -90 || v[3] > 90 {
		return nil, errors.New("bbox di luar rentang atau terbalik")
	}
	return &domain.BBox{MinLng: v[0], MinLat: v[1], MaxLng: v[2], MaxLat: v[3]}, nil
}

// Optimasi handles POST /v1/spasial/optimasi-rute. Menyimpan rencana (simpan=true)
// mengubah data dan karenanya butuh logistik.kelola; simulasi cukup spasial.lihat.
func (h *Handler) Optimasi(c *gin.Context) {
	var req dto.OptimasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	if req.Simpan && !middleware.HasPermission(c, "logistik.kelola") {
		response.Forbidden(c, "Menyimpan rencana memerlukan izin logistik.kelola")
		return
	}
	in := usecase.OptimasiInput{
		KomoditasKode: req.KomoditasKode, TarifTonKm: req.TarifPerTonKm, FaktorJalan: req.FaktorJalan,
		MaksJamTempuh: req.MaksJamTempuh, Simpan: req.Simpan, ActorID: c.GetString("user_id"),
	}
	if req.Periode != "" {
		periode, err := time.Parse(time.DateOnly, req.Periode)
		if err != nil {
			response.BadRequest(c, "periode harus berformat YYYY-MM-DD")
			return
		}
		in.Periode = periode
	}
	for _, p := range req.Penyesuaian {
		in.Penyesuaian = append(in.Penyesuaian, usecase.Penyesuaian{KabupatenKode: p.KabupatenKode, Ton: p.Ton})
	}

	out, err := h.opt.Hitung(audit.RequestContext(c), in)
	if err != nil {
		var input *usecase.InputError
		switch {
		case errors.As(err, &input):
			response.BadRequest(c, input.Msg)
		case errors.Is(err, usecase.ErrKomoditasKosong), errors.Is(err, usecase.ErrSkenarioTidakSimpan):
			response.BadRequest(c, err.Error())
		case errors.Is(err, usecase.ErrNeracaKosong):
			response.NotFound(c, "Belum ada neraca untuk komoditas dan periode ini")
		default:
			response.InternalError(c, "Gagal menghitung optimasi rute")
		}
		return
	}
	response.Success(c, dto.ToOptimasiResponse(out))
}

// RiwayatOptimasi handles GET /v1/spasial/optimasi-rute
func (h *Handler) RiwayatOptimasi(c *gin.Context) {
	items, err := h.opt.Riwayat(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil rencana optimasi")
		return
	}
	response.Success(c, dto.ToRencanaOptimasiResponses(items))
}
