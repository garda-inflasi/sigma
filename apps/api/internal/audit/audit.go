// Package audit adalah pintu masuk modul lain ke jejak audit: satu Writer yang
// disuntikkan lewat konstruktor, dan RequestContext untuk membawa asal permintaan.
//
// Aturan proyek: setiap aksi yang mengubah data wajib memanggil Writer.Write
// (docs/todos/00-cara-kerja.md, DoD).
package audit

import (
	"context"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/audit/repository/postgres"
	"github.com/garda-inflasi/api/internal/audit/usecase"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type (
	// Writer mencatat satu baris audit.
	Writer = domain.Writer
	// Entry adalah satu aksi yang dicatat.
	Entry = domain.Entry
	// Origin adalah asal permintaan (IP, user agent, correlation id).
	Origin = domain.Origin
)

// Hasil sebuah aksi.
const (
	HasilBerhasil = domain.HasilBerhasil
	HasilGagal    = domain.HasilGagal
	HasilDitolak  = domain.HasilDitolak
)

// NewWriter membuat Writer yang menyimpan ke tabel audit_log.
func NewWriter(db *gorm.DB) Writer {
	return usecase.New(postgres.New(db))
}

// RequestContext mengembalikan context permintaan yang membawa Origin, supaya
// Writer mencatat IP, user agent, dan correlation id tanpa parameter tambahan.
func RequestContext(c *gin.Context) context.Context {
	correlationID, _ := c.Get("request_id")
	id, _ := correlationID.(string)
	return domain.WithOrigin(c.Request.Context(), Origin{
		IP:            c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		CorrelationID: id,
	})
}

// Integritas memelihara rantai hash, partisi bulanan, dan arsip audit_log (03-C).
type Integritas = usecase.Integritas

// KonfigRetensi adalah kebijakan masa simpan audit.
type KonfigRetensi = usecase.KonfigRetensi

// NewIntegritas membuat usecase integritas audit; temuan dan arsip dicatat lewat Writer basis data yang sama.
func NewIntegritas(db *gorm.DB) *Integritas {
	repo := postgres.New(db)
	return usecase.NewIntegritas(repo, usecase.New(repo))
}
