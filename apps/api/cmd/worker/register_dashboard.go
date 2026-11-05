package main

import (
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/platform/jobs"
)

// dashboardRefreshInterval adalah jeda bawaan penyegaran agregat dashboard
// (sama dengan nilai lama di cmd/api).
const dashboardRefreshInterval = 5 * time.Minute

func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		interval, err := jobs.IntervalFromEnv(d.getenv, "dashboard_refresh", dashboardRefreshInterval)
		if err != nil {
			return nil, err
		}
		r := refresher.Shared(d.db, d.log)
		return []jobs.Job{{
			Name:       "dashboard_refresh",
			Interval:   interval,
			Timeout:    5 * time.Minute,
			RunOnStart: true,
			Run:        r.Refresh,
		}}, nil
	})
}
