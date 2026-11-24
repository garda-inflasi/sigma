package integration

import (
	"io"
	"strings"
	"testing"

	mlhttp "github.com/garda-inflasi/api/internal/ml/delivery/http"
	"github.com/garda-inflasi/api/internal/modules"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Setiap pola rute yang diperkaya benar-benar ada (GET) di router penuh cmd/api, supaya salah ketik pola tidak
// diam-diam mematikan lampiran ml.
func TestPetaRute_SemuaPolaAdaDiRouterPenuh(t *testing.T) {
	db := dbDemo(t)
	router := gin.New()
	modules.RegisterAll(router.Group("/v1"), db, logger.New(logger.ERROR, io.Discard), token.NewManager(jwtSecret))
	get := map[string]bool{}
	for _, r := range router.Routes() {
		if r.Method == "GET" {
			get[r.Path] = true
		}
	}
	assert.True(t, get["/v1/ml/cycles"])
	assert.True(t, get["/v1/ml/cycles/:cycle_id"])
	assert.True(t, get["/v1/ml/freshness"])
	for _, pola := range mlhttp.PolaDiperkaya() {
		if strings.HasSuffix(pola, "/") {
			ada := false
			for p := range get {
				ada = ada || strings.HasPrefix(p, pola)
			}
			assert.Truef(t, ada, "tidak ada rute GET di bawah %s", pola)
			continue
		}
		assert.Truef(t, get[pola], "rute GET %s tidak ada", pola)
	}
}
