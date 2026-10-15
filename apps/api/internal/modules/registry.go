package modules

import (
	"github.com/gin-gonic/gin"
	auditRoutes "github.com/garda-inflasi/api/internal/audit/routes"
	coldStorageRoutes "github.com/garda-inflasi/api/internal/cold-storage/routes"
	dashboardRoutes "github.com/garda-inflasi/api/internal/dashboard/routes"
	ewsRoutes "github.com/garda-inflasi/api/internal/ews/routes"
	hargaRoutes "github.com/garda-inflasi/api/internal/harga/routes"
	intervensiRoutes "github.com/garda-inflasi/api/internal/intervensi/routes"
	lapanganRoutes "github.com/garda-inflasi/api/internal/lapangan/routes"
	masterRoutes "github.com/garda-inflasi/api/internal/master/routes"
	policyBriefRoutes "github.com/garda-inflasi/api/internal/policy-brief/routes"
	prediksiRoutes "github.com/garda-inflasi/api/internal/prediksi/routes"
	siklusRoutes "github.com/garda-inflasi/api/internal/siklus/routes"
	spasialRoutes "github.com/garda-inflasi/api/internal/spasial/routes"
	sumberDataRoutes "github.com/garda-inflasi/api/internal/sumber-data/routes"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// RegisterAll registers all module routes under the given router group.
func RegisterAll(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger) {
	siklusRoutes.Register(v1, &siklusRoutes.Dependencies{DB: db, Logger: log})
	dashboardRoutes.Register(v1, &dashboardRoutes.Dependencies{DB: db, Logger: log})
	hargaRoutes.Register(v1, &hargaRoutes.Dependencies{DB: db, Logger: log})
	ewsRoutes.Register(v1, &ewsRoutes.Dependencies{DB: db, Logger: log})
	prediksiRoutes.Register(v1, &prediksiRoutes.Dependencies{DB: db, Logger: log})
	intervensiRoutes.Register(v1, &intervensiRoutes.Dependencies{DB: db, Logger: log})
	coldStorageRoutes.Register(v1, &coldStorageRoutes.Dependencies{DB: db, Logger: log})
	sumberDataRoutes.Register(v1, &sumberDataRoutes.Dependencies{DB: db, Logger: log})
	auditRoutes.Register(v1, &auditRoutes.Dependencies{DB: db, Logger: log})
	policyBriefRoutes.Register(v1, &policyBriefRoutes.Dependencies{DB: db, Logger: log})
	lapanganRoutes.Register(v1, &lapanganRoutes.Dependencies{DB: db, Logger: log})
	masterRoutes.Register(v1, &masterRoutes.Dependencies{DB: db, Logger: log})
	spasialRoutes.Register(v1, &spasialRoutes.Dependencies{DB: db, Logger: log})
}
