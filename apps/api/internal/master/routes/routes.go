// Package routes provides route registration for the master module.
package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/delivery/http"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/internal/master/repository/postgres"
	"github.com/garda-inflasi/api/internal/master/usecase"
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
	auditWriter := audit.NewWriter(deps.DB)
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, auditWriter, deps.Logger).DenganPerubahan(repo)
	handler := http.New(uc)
	ucUsulan := usecase.NewPerubahan(repo, auditWriter, deps.Logger)
	perubahan := http.NewPerubahan(ucUsulan)

	ref := http.NewReferensi(usecase.NewReferensi(postgres.NewReferensi(deps.DB), auditWriter, deps.Logger)).
		DenganUsulan(ucUsulan)

	master := engine.Group("/master")
	master.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("master.lihat"))
	{
		master.GET("/komoditas", handler.ListKomoditas)
		master.GET("/wilayah", handler.ListWilayah)
		master.GET("/pasar", handler.ListPasar)
		// Izin per aksi diperiksa di handler (setujui ≠ ajukan/arsipkan).
		master.POST("/komoditas/bulk", middleware.RequireAnyPermission("master.ajukan", "master.setujui"), handler.BulkKomoditas)

		// Antrean usulan perubahan (04-B): pengaju (master.ajukan) dan penyetuju (master.setujui) dipisah; pengaju
		// tidak dapat memutuskan usulannya sendiri (diperiksa di repository).
		master.GET("/perubahan", perubahan.List)
		master.GET("/perubahan/:id", perubahan.Get)
		master.POST("/perubahan", middleware.RequirePermission("master.ajukan"), perubahan.Ajukan)
		master.POST("/perubahan/:id/setujui", middleware.RequirePermission("master.setujui"), perubahan.Setujui)
		master.POST("/perubahan/:id/tolak", middleware.RequirePermission("master.setujui"), perubahan.Tolak)
		master.POST("/perubahan/:id/batalkan", middleware.RequirePermission("master.ajukan"), perubahan.Batalkan)

		// Data referensi (04-B/04-C). Segmen statis ("bulk", "versi") menang atas ":kode" di gin, dan POST
		// /komoditas/bulk berada di pohon metode berbeda dari GET/DELETE /komoditas/:kode.
		master.GET("/kecamatan", ref.ListKecamatan)
		master.GET("/kecamatan/:kode", ref.GetKecamatan)
		master.GET("/satuan", ref.ListSatuan)
		master.GET("/pedagang", ref.ListPedagang)
		master.GET("/wilayah/:kode", ref.GetWilayah)
		master.GET("/pasar/:kode", ref.GetPasar)
		master.GET("/komoditas/:kode", ref.GetKomoditas)
		master.GET("/komoditas/:kode/versi", ref.VersiKomoditas)
		// Hapus hanya untuk penyetuju; master yang sudah dirujuk dijawab 409 (nonaktifkan/arsipkan, bukan hapus).
		master.DELETE("/komoditas/:kode", middleware.RequirePermission(domain.IzinSetujui), ref.HapusKomoditas)
		master.DELETE("/pasar/:kode", middleware.RequirePermission(domain.IzinSetujui), ref.HapusPasar)
	}
}
