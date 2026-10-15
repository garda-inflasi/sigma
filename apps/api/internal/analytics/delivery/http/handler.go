package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/analytics/domain"
	"github.com/garda-inflasi/api/internal/analytics/dto"
	"github.com/garda-inflasi/api/internal/analytics/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/dashboard/summary
func (h *Handler) GetDashboardSummary(c *gin.Context) {
	var req dto.DashboardSummaryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	data, err := h.uc.GetDashboardSummary(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil ringkasan dashboard")
		return
	}

	response.Success(c, dto.DashboardSummaryResponse{Data: data})
}

// GET /v1/inflation/trends
func (h *Handler) GetInflationTrends(c *gin.Context) {
	var req dto.InflationTrendsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, err := h.uc.GetInflationTrends(c.Request.Context(), req.Year, req.Region)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tren inflasi")
		return
	}

	response.Success(c, dto.InflationTrendsResponse{Items: items})
}

// GET /v1/food-balance
func (h *Handler) GetFoodBalance(c *gin.Context) {
	var req dto.FoodBalanceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, err := h.uc.GetFoodBalance(c.Request.Context(), req.Period)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data neraca pangan")
		return
	}

	response.Success(c, dto.FoodBalanceResponse{Items: items})
}

// GET /v1/fiscal-space
func (h *Handler) GetFiscalSpace(c *gin.Context) {
	data, err := h.uc.GetFiscalSpace(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data ruang fiskal")
		return
	}

	response.Success(c, dto.FiscalSpaceResponse{Data: data})
}

// GET /v1/operations/heatmap
func (h *Handler) GetHeatmap(c *gin.Context) {
	var req dto.HeatmapRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, err := h.uc.GetHeatmap(c.Request.Context(), req.Date)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data heatmap")
		return
	}

	response.Success(c, dto.HeatmapResponse{Items: items})
}

// GET /v1/operations/alerts
func (h *Handler) GetActiveAlerts(c *gin.Context) {
	items, err := h.uc.GetActiveAlerts(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data alert")
		return
	}

	response.Success(c, dto.AlertsResponse{Items: items})
}

// GET /v1/operations/trends
func (h *Handler) GetPriceTrends(c *gin.Context) {
	var req dto.PriceTrendsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, err := h.uc.GetPriceTrends(c.Request.Context(), req.Commodity, req.Region, req.Days)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tren harga")
		return
	}

	response.Success(c, dto.PriceTrendsResponse{Items: items})
}

// GET /v1/regions/status-map
func (h *Handler) GetRegionStatusMap(c *gin.Context) {
	items, err := h.uc.GetRegionStatusMap(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data peta status wilayah")
		return
	}

	response.Success(c, dto.RegionStatusMapResponse{Items: items})
}

// GET /v1/predictions/forecast
func (h *Handler) GetForecast(c *gin.Context) {
	var req dto.ForecastRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	data, err := h.uc.GetForecast(c.Request.Context(), req.Commodity, req.Days)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data prediksi")
		return
	}

	response.Success(c, dto.ForecastResponse{Data: data})
}

// GET /v1/predictions/features
func (h *Handler) GetFeatureImportance(c *gin.Context) {
	var req dto.FeatureImportanceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	data, err := h.uc.GetFeatureImportance(c.Request.Context(), req.Commodity)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data fitur penting")
		return
	}

	response.Success(c, dto.FeatureImportanceResponse{Data: data})
}

// POST /v1/predictions/simulate
func (h *Handler) SimulateScenario(c *gin.Context) {
	var req dto.SimulationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	data, err := h.uc.SimulateScenario(c.Request.Context(), domain.SimulationRequest{
		CommodityID:        req.CommodityID,
		RainfallPct:        req.RainfallPct,
		SupplyIncreasePct:  req.SupplyIncreasePct,
		TransportSubsidyPct: req.TransportSubsidyPct,
	})
	if err != nil {
		response.InternalError(c, "Gagal menjalankan simulasi")
		return
	}

	response.Success(c, dto.SimulationResponse{Data: data})
}

// GET /v1/predictions/all-commodities
func (h *Handler) GetAllCommoditiesForecast(c *gin.Context) {
	items, err := h.uc.GetAllCommoditiesForecast(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data prediksi seluruh komoditas")
		return
	}

	response.Success(c, dto.AllCommoditiesForecastResponse{Items: items})
}

// GET /v1/validations/queue
func (h *Handler) GetValidationQueue(c *gin.Context) {
	var req dto.ValidationQueueRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, total, err := h.uc.GetValidationQueue(c.Request.Context(), req.Page, req.Limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil antrean validasi")
		return
	}

	response.Success(c, dto.ValidationQueueResponse{
		Items: items,
		Total: total,
		Page:  req.Page,
		Limit: req.Limit,
	})
}

// POST /v1/validations/batch-approve
func (h *Handler) BatchApprove(c *gin.Context) {
	var req dto.BatchApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	// Get validated_by from JWT claims
	validatedBy, _ := c.Get("user_id")
	validatedByStr, _ := validatedBy.(string)

	count, err := h.uc.BatchApprove(c.Request.Context(), req.EntryIDs, validatedByStr)
	if err != nil {
		response.InternalError(c, "Gagal memvalidasi entri")
		return
	}

	response.Success(c, dto.BatchApproveResponse{Approved: count})
}

// GET /v1/field-activities/progress
func (h *Handler) GetFieldActivityProgress(c *gin.Context) {
	items, err := h.uc.GetFieldActivityProgress(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data progres lapangan")
		return
	}

	response.Success(c, dto.FieldActivityProgressResponse{Items: items})
}

// GET /v1/models/registry
func (h *Handler) GetModelRegistry(c *gin.Context) {
	items, err := h.uc.GetModelRegistry(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data registry model")
		return
	}

	response.Success(c, dto.ModelRegistryResponse{Items: items})
}
