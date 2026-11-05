package routes

import (
	"context"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ingestion"
	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	ingestrepo "github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	"github.com/garda-inflasi/api/internal/sumber-data/delivery/http"
	"github.com/garda-inflasi/api/internal/sumber-data/repository/postgres"
	"github.com/garda-inflasi/api/internal/sumber-data/usecase"
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
	// Runner menjalankan ingestion. Nil: dirakit dari lingkungan (konektor dan Fetcher nyata);
	// tes menyuntikkan Runner ber-fixture agar tidak memanggil internet.
	Runner *engine.Runner
}

// ingestionAdapter menyatukan Runner dan repository riwayat menjadi usecase.Ingestion.
type ingestionAdapter struct {
	runner *engine.Runner
	repo   ingest.Repository
}

func (a ingestionAdapter) Punya(id string) bool { return a.runner.Punya(id) }

func (a ingestionAdapter) Mulai(ctx context.Context, id, pemicu string, aktor *string) (usecase.Berjalan, error) {
	run, err := a.runner.Mulai(ctx, id, pemicu, aktor)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (a ingestionAdapter) Eksekusi(ctx context.Context, id string, limit int) ([]ingest.EksekusiSumber, error) {
	return a.repo.Eksekusi(ctx, id, limit)
}

func (a ingestionAdapter) SampelMentah(ctx context.Context, id string, limit int) ([]ingest.SampelMentah, error) {
	return a.repo.SampelMentah(ctx, id, limit)
}

func Register(group *gin.RouterGroup, deps *Dependencies) {
	runner := deps.Runner
	if runner == nil {
		runner = ingestion.NewRunner(deps.DB, deps.Logger)
	}
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, ingestionAdapter{runner: runner, repo: ingestrepo.New(deps.DB)}, audit.NewWriter(deps.DB), deps.Logger)
	handler := http.New(uc)

	sumber := group.Group("/sumber-data")
	sumber.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("sumber.lihat"))
	{
		sumber.GET("", handler.List)
		sumber.GET("/:id", handler.Get)
		sumber.GET("/:id/eksekusi", handler.Eksekusi)
		sumber.GET("/:id/sampel", handler.Sampel)
		sumber.POST("/bulk", middleware.RequirePermission("sumber.kelola"), handler.Bulk)
		sumber.POST("/:id/jalankan", middleware.RequirePermission("sumber.kelola"), handler.Jalankan)
	}
}
