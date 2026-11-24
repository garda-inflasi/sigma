// Package http adalah lapisan HTTP modul ML: endpoint /v1/ml/* dan pengaya lampiran `ml` untuk endpoint lama.
package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/internal/ml/dto"
	"github.com/garda-inflasi/api/internal/ml/orchestrator"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// Izin yang membuka data ML.
const (
	// IzinPrediksi membuka hasil training (siklus, metrik, forecast, sinyal inflasi, XAI, makro).
	IzinPrediksi = "prediksi.lihat"
	// IzinSumber membuka kesegaran sumber (crawl_runs, source_freshness, sinyal SUMBER_DATA).
	IzinSumber = "sumber.lihat"
	// IzinEws membuka sinyal EWS pipeline.
	IzinEws = "ews.lihat"
)

// Batas halaman riwayat siklus (bawaan satu hari siklus per jam, maksimum satu minggu).
const (
	defaultLimit = 24
	maksLimit    = 168
)

// anggaranEndpoint membatasi bacaan Mongo satu request endpoint /v1/ml/*.
const anggaranEndpoint = 3 * time.Second

var zonaWIB = time.FixedZone("WIB", 7*3600)

type Handler struct {
	svc  *usecase.Service
	pg   domain.PostgresRepository
	orch *orchestrator.Orchestrator
}

func New(svc *usecase.Service, pg domain.PostgresRepository, orch ...*orchestrator.Orchestrator) *Handler {
	var o *orchestrator.Orchestrator
	if len(orch) > 0 {
		o = orch[0]
	}
	return &Handler{svc: svc, pg: pg, orch: o}
}

// parseWaktu menerima YYYY-MM-DD (tanggal WIB) atau RFC3339. akhir=true menjadikan tanggal batas eksklusif hari
// berikutnya, sehingga `sampai=2026-11-24` mencakup seluruh tanggal itu.
func parseWaktu(raw string, akhir bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.ParseInLocation(time.DateOnly, raw, zonaWIB); err == nil {
		if akhir {
			t = t.AddDate(0, 0, 1)
		}
		return &t, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New("format tanggal harus YYYY-MM-DD atau RFC3339")
	}
	return &t, nil
}

func parseStatus(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.ToUpper(strings.TrimSpace(s))
		switch s {
		case domain.StatusBerjalan, domain.StatusBerhasil, domain.StatusSebagian, domain.StatusGagal:
			out = append(out, s)
		case "FINAL":
			out = append(out, domain.StatusFinal...)
		default:
			return nil, errors.New("status harus BERJALAN, BERHASIL, SEBAGIAN, GAGAL, atau FINAL")
		}
	}
	return out, nil
}

func angka(c *gin.Context, nama string, bawaan int) (int, bool) {
	raw := strings.TrimSpace(c.Query(nama))
	if raw == "" {
		return bawaan, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, false
	}
	return v, true
}

// ListSiklus handles GET /v1/ml/cycles?page=&limit=&dari=&sampai=&status=
//
// Riwayat siklus training terbaru dahulu, semua status (GAGAL dan BERJALAN ikut, supaya kegagalan terlihat) kecuali
// disaring. Mongo yang tidak dikonfigurasi atau mati dijawab 200 dengan tersedia=false dan daftar kosong.
func (h *Handler) ListSiklus(c *gin.Context) {
	page, ok := angka(c, "page", 1)
	if !ok {
		response.BadRequest(c, "page harus bilangan bulat >= 1")
		return
	}
	limit, ok := angka(c, "limit", defaultLimit)
	if !ok || limit > maksLimit {
		response.BadRequest(c, "limit harus 1-"+strconv.Itoa(maksLimit))
		return
	}
	status, err := parseStatus(c.Query("status"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	dari, err := parseWaktu(c.Query("dari"), false)
	if err != nil {
		response.BadRequest(c, "dari: "+err.Error())
		return
	}
	sampai, err := parseWaktu(c.Query("sampai"), true)
	if err != nil {
		response.BadRequest(c, "sampai: "+err.Error())
		return
	}
	if dari != nil && sampai != nil && !dari.Before(*sampai) {
		response.BadRequest(c, "dari harus sebelum sampai")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), anggaranEndpoint)
	defer cancel()
	out := dto.DaftarSiklusResponse{Items: []dto.SiklusResponse{}, Page: page, Limit: limit}
	items, total, k := h.svc.ListSiklus(ctx, domain.ListSiklusQuery{Status: status, Dari: dari, Sampai: sampai, Page: page, Limit: limit})
	out.Tersedia, out.Alasan, out.Total = k.Tersedia, k.Alasan, total
	for _, s := range items {
		out.Items = append(out.Items, dto.ToSiklus(s, false))
	}
	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	response.Success(c, out)
}

// GetSiklus handles GET /v1/ml/cycles/:cycle_id. source_freshness hanya untuk pemegang sumber.lihat.
func (h *Handler) GetSiklus(c *gin.Context) {
	id := c.Param("cycle_id")
	if !domain.CycleIDValid(id) {
		response.BadRequest(c, "cycle_id harus berformat C{YYYYMMDD}T{HH}Z")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), anggaranEndpoint)
	defer cancel()
	siklus, k := h.svc.Siklus(ctx, id)
	if !k.Tersedia {
		switch k.Alasan {
		case usecase.AlasanSiklusTidakDitemukan:
			response.NotFound(c, "Siklus tidak ditemukan")
		default:
			response.ErrorWithData(c, nethttp.StatusServiceUnavailable, "503", "Hasil pipeline ML tidak tersedia",
				gin.H{"tersedia": false, "alasan": k.Alasan})
		}
		return
	}
	response.Success(c, dto.ToSiklus(*siklus, middleware.HasPermission(c, IzinSumber)))
}

// Freshness handles GET /v1/ml/freshness: status crawl terakhir per sumber (BACKFILL tidak dinilai), kesegaran
// menurut siklus terbaru, dan sinyal SUMBER_DATA (termasuk INFO untuk sumber yang sengaja dilewati).
func (h *Handler) Freshness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), anggaranEndpoint)
	defer cancel()
	response.Success(c, susunKesegaran(ctx, h.svc, h.pg))
}

// susunKesegaran menyusun blok kesegaran; Mongo yang tidak tersedia menghasilkan tersedia=false, bukan galat.
func susunKesegaran(ctx context.Context, svc *usecase.Service, pg domain.PostgresRepository) *dto.BlokKesegaran {
	hasil, k := svc.Kesegaran(ctx)
	if !k.Tersedia {
		return &dto.BlokKesegaran{Meta: dto.MetaMati(k.Alasan), DihitungPada: svc.Sekarang().UTC().Format(time.RFC3339),
			Sumber: []dto.SumberKesegaranResponse{}, SinyalSumber: []dto.SinyalEwsResponse{}}
	}
	kunci := make([]string, 0, len(hasil.Sinyal))
	for _, s := range hasil.Sinyal {
		kunci = append(kunci, s.SinyalKunci)
	}
	tautan := map[string]domain.TautanAlert{}
	if pg != nil {
		if t, err := pg.TautanAlert(ctx, kunci); err == nil {
			tautan = t
		}
	}
	return dto.ToKesegaran(hasil, tautan)
}

// cakupanDari menurunkan cakupan baris pemanggil (02-B) dari basis data.
func cakupanDari(ctx context.Context, pg domain.PostgresRepository, userID string) (hargadomain.Cakupan, error) {
	peran, kab, ditemukan, err := pg.PenggunaCakupan(ctx, userID)
	if err != nil || !ditemukan {
		return hargadomain.Cakupan{}, err
	}
	return hargadomain.CakupanPeran(peran, kab, userID), nil
}
