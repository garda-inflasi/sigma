package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// HeaderDataEnv memilih sumber data aktif dari klien UI: "staging" (PostgreSQL demo) atau "prod" (MongoDB real).
	HeaderDataEnv  = "X-Garda-Data-Env"
	DataEnvStaging = "staging"
	DataEnvProd    = "prod"
)

// DataEnv membaca mode sumber data yang diminta pemanggil lewat header `X-Garda-Data-Env` atau query `data_env`.
// Mengembalikan "staging", "prod", atau string kosong bila tidak dikirim.
func DataEnv(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	raw := strings.TrimSpace(c.GetHeader(HeaderDataEnv))
	if raw == "" {
		raw = strings.TrimSpace(c.Query("data_env"))
	}
	switch strings.ToLower(raw) {
	case DataEnvStaging, "demo":
		return DataEnvStaging
	case DataEnvProd, "real", "production":
		return DataEnvProd
	default:
		return ""
	}
}
