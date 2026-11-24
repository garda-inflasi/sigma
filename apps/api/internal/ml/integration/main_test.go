// Package integration menguji modul ML terhadap MongoDB sungguhan (container mongo:7, mongotest) dan PostgreSQL
// sungguhan (pgtest): repository baca, endpoint /v1/ml/*, lampiran `ml` di endpoint lama, RBAC, perilaku saat Mongo
// mati, dan jembatan EWS. Fixture Mongo meniru keluaran apps/ml/garda_ml/pipeline (docs/PIPELINE-DATA-ML.md §3).
package integration

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/mongotest"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/gin-gonic/gin"
)

var (
	pg *pgtest.Server
	mg *mongotest.Server
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	flag.Parse()
	if !testing.Short() {
		srv, err := mongotest.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mongotest: %v\n", err)
			os.Exit(1)
		}
		mg = srv
	}
	code := pgtest.Main(m, &pg)
	if mg != nil {
		if err := mg.Purge(); err != nil {
			fmt.Fprintf(os.Stderr, "mongotest: gagal membuang container: %v\n", err)
		}
	}
	os.Exit(code)
}

func butuhDocker(t *testing.T) {
	t.Helper()
	if pg == nil || mg == nil {
		t.Skip("tes integrasi dilewati (-short, tanpa Docker)")
	}
}
