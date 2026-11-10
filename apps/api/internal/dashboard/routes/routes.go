package routes

import (
	"github.com/garda-inflasi/api/internal/dashboard/delivery/http"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/dashboard/repository/postgres"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Logger *logger.Logger
	Token  *token.Manager
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	// Cache hasil agregasi dibatalkan setiap kali refresher (satu per koneksi basis data, dipakai bersama modul
	// harga dan ingestion) selesai menyegarkan view: data setelah publikasi harga langsung tampil, tanpa menunggu TTL.
	uc := usecase.New(repo, deps.Logger, usecase.WithEpoch(refresher.Shared(deps.DB, deps.Logger).Epoch))
	handler := http.New(uc)

	// Operasional — requires dashboard.operasional permission. Semua endpoint di grup ini dibatasi cakupan baris
	// pemanggil (02-B); endpoint baru wajib memakai handler.cakupan dan meneruskannya ke usecase.
	operasional := engine.Group("/dashboard/operasional")
	operasional.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.operasional"))
	{
		operasional.GET("/kpi", handler.GetKpiOperasional)
		operasional.GET("/heatmap", handler.GetHeatmap)
		operasional.GET("/aktivitas-lapangan", handler.GetAktivitasLapangan)
		operasional.GET("/pembanding-harga", handler.GetPembandingHarga)
		operasional.GET("/cuaca", handler.GetCuaca)
		// Ringkasan 08-B. Aktivitas per petugas memuat nama dan ID petugas, jadi di samping dashboard.operasional ia
		// memerlukan harga.lihat: izin yang sudah menjaga nama dan ID yang sama pada daftar entri harga. Seluruh
		// peran bawaan pemegang dashboard.operasional sudah memegang harga.lihat (dijaga tes), jadi peran bawaan tidak
		// kehilangan akses; syarat ini hanya menutup peran kustom atau izin langsung yang punya yang pertama saja.
		operasional.GET("/aktivitas-petugas", middleware.RequirePermission("harga.lihat"), handler.GetAktivitasPetugas)
		operasional.GET("/stok", handler.GetStok)
		operasional.GET("/produsen", handler.GetProdusen)
		operasional.GET("/logistik", handler.GetLogistik)
	}

	// Eksekutif: modul internal/eksekutif (GET /dashboard/eksekutif).
}
