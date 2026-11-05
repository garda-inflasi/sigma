package main

import (
	"fmt"

	"github.com/garda-inflasi/api/internal/platform/jobs"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// deps adalah ketergantungan yang tersedia bagi pembuat job.
type deps struct {
	db     *gorm.DB
	log    *logger.Logger
	getenv func(string) string
}

// builder membuat nol atau lebih job dari konfigurasi env. Job yang tidak
// diinginkan cukup diberi interval 0 (JOB_<NAMA>_INTERVAL=off), bukan dihilangkan.
type builder func(d deps) ([]jobs.Job, error)

// builders diisi oleh init() di tiap berkas register_<modul>.go, mengikuti pola
// cmd/seed/register_*.go: satu berkas per modul agar penambahan job tidak
// bentrok. Lihat "Penjadwal" di apps/api/README.md.
var builders []builder

func registerBuilder(b builder) { builders = append(builders, b) }

// buildScheduler mendaftarkan semua job dari builders ke scheduler baru.
func buildScheduler(d deps) (*jobs.Scheduler, error) {
	s := jobs.New(d.db, d.log)
	for _, b := range builders {
		list, err := b(d)
		if err != nil {
			return nil, err
		}
		for _, j := range list {
			if err := s.Add(j); err != nil {
				return nil, fmt.Errorf("daftarkan job: %w", err)
			}
		}
	}
	return s, nil
}
