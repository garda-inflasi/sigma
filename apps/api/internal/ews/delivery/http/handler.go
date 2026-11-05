// Package http adalah lapisan HTTP modul EWS.
package http

import (
	"errors"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ews/dto"
	"github.com/garda-inflasi/api/internal/ews/notifikasi"
	"github.com/garda-inflasi/api/internal/ews/stream"
	"github.com/garda-inflasi/api/internal/ews/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// totalCountHeader membawa jumlah seluruh hasil filter untuk klien yang memaginasi.
const totalCountHeader = "X-Total-Count"

type Handler struct {
	uc     *usecase.UseCase
	stream *stream.Streamer
	notif  *notifikasi.Service
}

func New(uc *usecase.UseCase, streamer *stream.Streamer, notif *notifikasi.Service) *Handler {
	return &Handler{uc: uc, stream: streamer, notif: notif}
}

// Stream handles GET /v1/ews/stream (Server-Sent Events, event `alert`).
// Klien yang menyambung ulang mengirim Last-Event-ID dan melanjutkan tanpa celah atau duplikat.
func (h *Handler) Stream(c *gin.Context) {
	ctx := c.Request.Context()
	start, err := h.stream.StartID(ctx, c.GetHeader("Last-Event-ID"))
	if err != nil {
		response.InternalError(c, "Gagal memulai aliran peringatan")
		return
	}

	// Server memakai WriteTimeout untuk request biasa; aliran harus hidup lebih lama.
	// Galat diabaikan: writer yang tidak mendukung deadline (mis. recorder tes) tak punya batas.
	_ = nethttp.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})

	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("X-Accel-Buffering", "no") // cegah buffering oleh proxy
	c.Status(nethttp.StatusOK)

	_ = h.stream.Serve(ctx, c.Writer, c.Writer.Flush, start)
}

// List handles GET /v1/ews?status=&severity=&jenis=&kabupaten_kode=&page=&limit=
func (h *Handler) List(c *gin.Context) {
	var req dto.ListEWSRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, total, err := h.uc.List(c.Request.Context(), domain.ListQuery{
		Status: req.Status, Severity: req.Severity, Jenis: req.Jenis, KabupatenKode: req.KabupatenKode,
		Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		response.InternalError(c, "Gagal mengambil data EWS")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToAlertResponses(items))
}

// GetByID handles GET /v1/ews/:id
func (h *Handler) GetByID(c *gin.Context) {
	alert, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Alert EWS tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil alert EWS")
		return
	}
	response.Success(c, dto.ToAlertResponse(*alert))
}

// ListAturan handles GET /v1/ews/aturan
func (h *Handler) ListAturan(c *gin.Context) {
	items, err := h.uc.ListAturan(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil aturan ambang EWS")
		return
	}
	response.Success(c, dto.ToAturanResponses(items))
}

// UpdateAturan handles PUT /v1/ews/aturan/:id
func (h *Handler) UpdateAturan(c *gin.Context) {
	var req dto.UpdateAturanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	updated, err := h.uc.UpdateAturan(audit.RequestContext(c), c.Param("id"), domain.AturanUpdate{
		Versi: req.Versi, AmbangWaspada: req.AmbangWaspada, AmbangSiaga: req.AmbangSiaga, AmbangKritis: req.AmbangKritis,
		JendelaHari: req.JendelaHari, CooldownJam: req.CooldownJam, Kanal: req.Kanal, Aktif: *req.Aktif, Keterangan: req.Keterangan,
	}, c.GetString("user_id"))
	switch {
	case err == nil:
		response.Success(c, dto.ToAturanResponse(*updated))
	case errors.Is(err, usecase.ErrAmbangTidakValid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, usecase.ErrAturanNotFound):
		response.NotFound(c, "Aturan ambang tidak ditemukan")
	case errors.Is(err, usecase.ErrVersiUsang):
		response.Conflict(c, "Aturan telah diubah pengguna lain. Muat ulang lalu ulangi perubahan.")
	default:
		response.InternalError(c, "Gagal menyimpan ambang")
	}
}

// Evaluasi handles POST /v1/ews/evaluasi: menjalankan rule engine sekarang.
func (h *Handler) Evaluasi(c *gin.Context) {
	ringkasan, err := h.uc.Evaluasi(audit.RequestContext(c), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal menjalankan evaluasi")
		return
	}
	response.Success(c, dto.ToEvaluasiResponse(ringkasan))
}

// ListRiwayat handles GET /v1/ews/:id/riwayat
func (h *Handler) ListRiwayat(c *gin.Context) {
	items, err := h.uc.ListRiwayat(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Alert EWS tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil riwayat alert")
		return
	}
	response.Success(c, dto.ToRiwayatResponses(items))
}

// Bulk handles POST /v1/ews/bulk. Route meminta ews.akui atau ews.eskalasi;
// izin yang tepat menurut aksi diperiksa di sini.
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkEWSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	if !middleware.HasPermission(c, izinAksi(req.Action)) {
		response.Forbidden(c, "Tidak memiliki izin untuk aksi ini")
		return
	}

	res, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Action, c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidAction) {
			response.BadRequest(c, "Aksi tidak valid")
			return
		}
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}
	response.Success(c, dto.BulkEWSResponse{Berhasil: res.Berhasil, Gagal: res.Gagal})
}

// izinAksi adalah izin yang dibutuhkan untuk satu aksi massal.
func izinAksi(action string) string {
	if action == string(domain.BulkEskalasi) {
		return "ews.eskalasi"
	}
	return "ews.akui"
}

// GetPreferensi handles GET /v1/ews/notifikasi/preferensi (milik pengguna yang masuk).
func (h *Handler) GetPreferensi(c *gin.Context) {
	prefs, err := h.notif.DaftarPreferensi(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal mengambil preferensi notifikasi")
		return
	}
	out := dto.PreferensiNotifikasiResponse{Preferensi: make([]dto.PreferensiResponse, len(prefs)), VAPIDPublicKey: h.notif.VAPIDPublicKey()}
	for i, p := range prefs {
		out.Preferensi[i] = dto.PreferensiResponse{Kanal: p.Kanal, SeverityMin: p.SeverityMin, Aktif: p.Aktif, Tersedia: p.Tersedia}
	}
	response.Success(c, out)
}

// PutPreferensi handles PUT /v1/ews/notifikasi/preferensi
func (h *Handler) PutPreferensi(c *gin.Context) {
	var req dto.SimpanPreferensiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	prefs := make([]notifikasi.Preferensi, len(req.Preferensi))
	for i, p := range req.Preferensi {
		prefs[i] = notifikasi.Preferensi{Kanal: p.Kanal, SeverityMin: p.SeverityMin, Aktif: *p.Aktif}
	}
	err := h.notif.SimpanPreferensi(audit.RequestContext(c), c.GetString("user_id"), prefs)
	switch {
	case err == nil:
		h.GetPreferensi(c)
	case errors.Is(err, notifikasi.ErrKanalTidakValid), errors.Is(err, notifikasi.ErrSeverityTidakValid):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "Gagal menyimpan preferensi notifikasi")
	}
}

// PutLanggananPush handles PUT /v1/ews/notifikasi/langganan-push
func (h *Handler) PutLanggananPush(c *gin.Context) {
	var req dto.DaftarPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	err := h.notif.DaftarPush(audit.RequestContext(c), c.GetString("user_id"), domain.LanggananPush{Endpoint: req.Endpoint, P256dh: req.P256dh, Auth: req.Auth})
	if err != nil {
		response.InternalError(c, "Gagal menyimpan langganan push")
		return
	}
	response.Success(c, nil)
}

// DeleteLanggananPush handles DELETE /v1/ews/notifikasi/langganan-push
func (h *Handler) DeleteLanggananPush(c *gin.Context) {
	var req dto.BatalPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	ada, err := h.notif.BatalPush(audit.RequestContext(c), c.GetString("user_id"), req.Endpoint)
	switch {
	case err != nil:
		response.InternalError(c, "Gagal membatalkan langganan push")
	case !ada:
		response.NotFound(c, "Langganan push tidak ditemukan")
	default:
		response.Success(c, nil)
	}
}
