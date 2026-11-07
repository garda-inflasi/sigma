package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/internal/cold-storage/dto"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// conflictMessages memetakan penolakan bisnis yang berstatus 409 ke pesan pengguna.
var conflictMessages = map[error]string{
	domain.ErrLotHabis:             "Lot sudah habis",
	domain.ErrStokTidakCukup:       "Volume melebihi stok lot",
	domain.ErrKapasitasTidakCukup:  "Kapasitas fasilitas tujuan tidak cukup",
	domain.ErrFasilitasTidakAktif:  "Fasilitas tidak sedang beroperasi",
	domain.ErrKapasitasDiBawahStok: "Kapasitas baru lebih kecil dari stok yang tersimpan",
}

// writeOperasiError memetakan galat operasi ke respons HTTP.
func writeOperasiError(c *gin.Context, err error, fallback string) {
	var input *usecase.InputError
	switch {
	case errors.As(err, &input):
		response.BadRequest(c, input.Msg)
	case errors.Is(err, domain.ErrFasilitasTidakAda):
		response.NotFound(c, "Fasilitas tidak ditemukan")
	case errors.Is(err, domain.ErrLotTidakAda):
		response.NotFound(c, "Lot tidak ditemukan")
	case errors.Is(err, domain.ErrKomoditasTidakAda):
		response.NotFound(c, "Komoditas tidak ditemukan")
	case errors.Is(err, domain.ErrFasilitasSama):
		response.BadRequest(c, "Fasilitas asal dan tujuan sama")
	case errors.Is(err, domain.ErrBatasSuhuTidakValid):
		response.BadRequest(c, "Suhu minimum harus lebih kecil dari suhu maksimum")
	default:
		for target, msg := range conflictMessages {
			if errors.Is(err, target) {
				response.Conflict(c, msg)
				return
			}
		}
		response.InternalError(c, fallback)
	}
}

// maksKodeFasilitas adalah panjang terbesar kode fasilitas (kolom kode varchar(64)).
const maksKodeFasilitas = 64

// rujukanFasilitas membaca :id sebagai UUID atau kode fasilitas (alias CS-01); selain itu 404.
func rujukanFasilitas(c *gin.Context) (idx.RujukanUUID, bool) {
	return idx.ParamUUIDAtauKode(c, "id", maksKodeFasilitas, "Fasilitas tidak ditemukan")
}

// Detail handles GET /v1/cold-storage/fasilitas/:id (UUID atau kode fasilitas)
func (h *Handler) Detail(c *gin.Context) {
	ref, ok := rujukanFasilitas(c)
	if !ok {
		return
	}
	var req dto.DetailFasilitasRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	d, err := h.ops.Detail(c.Request.Context(), ref, req.Jam)
	if err != nil {
		writeOperasiError(c, err, "Gagal mengambil detail fasilitas")
		return
	}

	out := dto.DetailFasilitasResponse{
		Fasilitas: dto.ToFasilitasResponse(d.Fasilitas),
		Telemetri: make([]dto.TitikSuhuResponse, len(d.Telemetri)),
		Stok:      make([]dto.StokColdStorageResponse, len(d.Stok)),
		Transaksi: make([]dto.TransaksiStokResponse, len(d.Transaksi)),
	}
	for i, t := range d.Telemetri {
		out.Telemetri[i] = dto.TitikSuhuResponse{Waktu: t.Waktu, SuhuC: t.SuhuC, KelembapanPct: t.KelembapanPct, Sumber: t.Sumber, DiLuarRentang: t.DiLuarRentang}
	}
	for i, s := range d.Stok {
		out.Stok[i] = dto.ToStokResponse(s)
	}
	for i, t := range d.Transaksi {
		out.Transaksi[i] = dto.ToTransaksiResponse(t)
	}
	response.Success(c, out)
}

// UbahFasilitas handles PATCH /v1/cold-storage/fasilitas/:id (UUID atau kode fasilitas)
func (h *Handler) UbahFasilitas(c *gin.Context) {
	ref, ok := rujukanFasilitas(c)
	if !ok {
		return
	}
	var req dto.UbahFasilitasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	f, err := h.ops.UbahFasilitas(audit.RequestContext(c), ref, domain.FasilitasPatch{
		KapasitasTon: req.KapasitasTon, SuhuMinC: req.SuhuMinC, SuhuMaksC: req.SuhuMaksC, Status: req.Status,
	}, c.GetString("user_id"))
	if err != nil {
		writeOperasiError(c, err, "Gagal mengubah fasilitas")
		return
	}
	response.Success(c, dto.ToFasilitasResponse(*f))
}

// IngestTelemetri handles POST /v1/cold-storage/telemetri
func (h *Handler) IngestTelemetri(c *gin.Context) {
	var req dto.IngestTelemetriRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	in := usecase.IngestInput{ActorID: c.GetString("user_id"), Pembacaan: make([]usecase.Pembacaan, len(req.Pembacaan))}
	for i, p := range req.Pembacaan {
		in.Pembacaan[i] = usecase.Pembacaan{FasilitasID: p.FasilitasID, Waktu: p.Waktu, SuhuC: *p.SuhuC, KelembapanPct: p.KelembapanPct, Sumber: p.Sumber}
	}

	out, err := h.ops.Ingest(audit.RequestContext(c), in)
	if err != nil {
		writeOperasiError(c, err, "Gagal menyimpan telemetri")
		return
	}
	alarm := make([]dto.AlarmSuhuResponse, len(out.DiLuarRentang))
	for i, a := range out.DiLuarRentang {
		alarm[i] = dto.AlarmSuhuResponse{FasilitasID: a.FasilitasID, Waktu: a.Waktu, SuhuC: a.SuhuC, BatasMinC: a.BatasMinC, BatasMaksC: a.BatasMaksC}
	}
	response.Success(c, dto.IngestTelemetriResponse{Diterima: out.Diterima, Duplikat: out.Duplikat, DiLuarRentang: alarm})
}

// Transaksi handles POST /v1/cold-storage/transaksi
func (h *Handler) Transaksi(c *gin.Context) {
	var req dto.TransaksiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	// Rute mengizinkan coldstorage.kelola atau logistik.kelola; hanya perpindahan
	// yang boleh dilakukan pemegang logistik.kelola.
	if !(middleware.HasPermission(c, "coldstorage.kelola") || (req.Jenis == domain.TransaksiPindah && middleware.HasPermission(c, "logistik.kelola"))) {
		response.Forbidden(c, "Akses ditolak")
		return
	}

	hasil, err := h.ops.Transaksi(audit.RequestContext(c), usecase.TransaksiRequest{
		Jenis: req.Jenis, LotID: req.LotID, FasilitasID: req.FasilitasID, FasilitasTujuanID: req.FasilitasTujuanID,
		KomoditasKode: req.KomoditasKode, Pemilik: req.Pemilik, Alasan: req.Alasan, VolumeTon: req.VolumeTon,
		Semua: req.Semua, KedaluwarsaPada: req.KedaluwarsaPada, ActorID: c.GetString("user_id"),
	})
	if err != nil {
		writeOperasiError(c, err, "Gagal memproses transaksi stok")
		return
	}
	response.Created(c, dto.TransaksiHasilResponse{
		TransaksiID: hasil.TransaksiID, Jenis: hasil.Jenis, LotID: hasil.LotID, LotBaruID: hasil.LotBaruID,
		VolumeTon: hasil.VolumeTon, VolumeSisaTon: hasil.VolumeSisaTon, StatusLot: hasil.StatusLot,
	})
}
