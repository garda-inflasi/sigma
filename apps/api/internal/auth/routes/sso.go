package routes

import (
	"github.com/garda-inflasi/api/internal/auth/delivery/http"
	"github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/sso"
	"github.com/gin-gonic/gin"
)

// ssoSettings menentukan konfigurasi SSO Google: Options.SSO bila Options diberikan (tes; nil = nonaktif),
// selain itu dari lingkungan (SSO_GOOGLE_*). Mengembalikan nil bila SSO tidak dinyalakan.
func ssoSettings(opts []Options, getenv func(string) string) *sso.Config {
	if len(opts) > 0 {
		return opts[0].SSO
	}
	cfg := sso.ConfigFromEnv(getenv)
	if !cfg.Enabled() {
		return nil
	}
	return &cfg
}

// registerSSO memasang rute SSO publik. Rute selalu terpasang: tanpa konfigurasi (atau konfigurasi yang
// salah) authorize dan callback menjawab 503 SSO_NOT_CONFIGURED dan status melaporkan google=false.
// Pembuatan provider tidak menghubungi penyedia; penemuan dilakukan malas pada permintaan pertama,
// sehingga API tetap start walau penerbit tidak terjangkau.
func registerSSO(public *gin.RouterGroup, repo *postgres.Repository, uc *usecase.UseCase, log *logger.Logger, cfg *sso.Config) {
	var provider usecase.SSOProvider
	if cfg != nil {
		p, err := sso.New(*cfg)
		if err != nil {
			log.Error("SSO Google dinonaktifkan karena konfigurasi tidak valid", map[string]interface{}{"error": err.Error()})
		} else {
			provider = p
			log.Info("SSO Google aktif", map[string]interface{}{"issuer": cfg.Issuer, "hosted_domain": cfg.HostedDomain})
		}
	}

	handler := http.NewSSO(usecase.NewSSO(uc, repo, usecase.SSOConfig{Provider: provider}))
	public.GET("/sso/status", handler.Status)
	public.POST("/sso/google/authorize", handler.Authorize)
	public.POST("/sso/google/callback", handler.Callback)
}
