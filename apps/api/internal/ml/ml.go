// Package ml merakit pembaca hasil pipeline data dan ML (MongoDB dataunseen dan datasafe) yang dipakai bersama oleh
// route HTTP (cmd/api) dan jembatan EWS (cmd/worker). Kontrak data: docs/PIPELINE-DATA-ML.md.
package ml

import (
	"context"
	"os"
	"sync"
	"time"

	mongorepo "github.com/garda-inflasi/api/internal/ml/repository/mongo"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/platform/mongodb"
)

var (
	sharedOnce sync.Once
	shared     *usecase.Service
)

// Shared mengembalikan satu Service per proses yang dibaca dari env (MONGO_URI/MONGO_URL, MONGO_DB_UNSEEN,
// MONGO_DB_SAFE). Tanpa konfigurasi atau dengan URI tak valid Service tetap dibuat dan melaporkan alasannya.
func Shared(log *logger.Logger) *usecase.Service {
	sharedOnce.Do(func() { shared = DariEnv(os.Getenv, log) })
	return shared
}

// DariEnv membuat Service baru dari env.
func DariEnv(getenv func(string) string, log *logger.Logger) *usecase.Service {
	cfg := mongodb.ConfigDariEnv(getenv)
	if !cfg.Dikonfigurasi() {
		if log != nil {
			log.Info("ml: MONGO_URI/MONGO_URL kosong, hasil pipeline ML tidak dilampirkan", nil)
		}
		return usecase.New(nil, usecase.AlasanTidakDikonfigurasi, log)
	}
	klien, err := mongodb.Buka(cfg)
	if err != nil {
		if log != nil {
			log.Warn("ml: konfigurasi Mongo tidak valid, hasil pipeline ML tidak dilampirkan", map[string]interface{}{"error": err.Error()})
		}
		return usecase.New(nil, usecase.AlasanKonfigurasiInvalid, log)
	}
	if log != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := klien.Ping(ctx); err != nil {
			log.Warn("ml: Mongo belum menjawab; API tetap berjalan dan mencoba lagi pada request berikutnya", map[string]interface{}{"error": err.Error()})
		} else {
			log.Info("ml: Mongo pipeline terhubung", map[string]interface{}{"db_unseen": cfg.DBUnseen, "db_safe": cfg.DBSafe})
		}
	}
	return usecase.New(mongorepo.New(klien), "", log)
}
