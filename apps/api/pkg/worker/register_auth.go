package worker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	authpg "github.com/garda-inflasi/api/internal/auth/repository/postgres"
	authuc "github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/platform/jobs"
)

// authPembersihanInterval adalah jeda bawaan job pembersihan tabel autentikasi.
const authPembersihanInterval = time.Hour

const (
	authPembersihanLoginHariMaks = 3650
	authPembersihanResetHariMaks = 365
	authPembersihanBatchMaks     = 100_000
)

// Job auth_pembersihan: menghapus baris autentikasi yang sudah mati (jwt_blacklist, refresh_tokens,
// password_reset_tokens kedaluwarsa/terpakai, dan login_attempts yang lewat masa simpan). Konfigurasi env:
//
//	JOB_AUTH_PEMBERSIHAN_INTERVAL     jeda antar putaran (bawaan 1h, off = nonaktif)
//	JOB_AUTH_PEMBERSIHAN_LOGIN_HARI   masa simpan login_attempts (bawaan 90; minimal 1; off = tidak dihapus)
//	JOB_AUTH_PEMBERSIHAN_RESET_HARI   masa simpan token reset sesudah mati (bawaan 7; 0 = langsung dihapus)
//	JOB_AUTH_PEMBERSIHAN_BATCH        baris per DELETE (bawaan 1000)
func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		interval, err := jobs.IntervalFromEnv(d.getenv, "auth_pembersihan", authPembersihanInterval)
		if err != nil {
			return nil, err
		}
		cfg, err := konfigAuthPembersihan(d.getenv)
		if err != nil {
			return nil, err
		}
		uc := authuc.NewPembersihan(authpg.New(d.db), cfg, d.log)
		return []jobs.Job{{
			Name:       "auth_pembersihan",
			Interval:   interval,
			Timeout:    5 * time.Minute,
			RunOnStart: true,
			// Idempoten: yang sudah dihapus tidak ada lagi; putaran yang terpotong dilanjutkan putaran berikutnya.
			Run: func(ctx context.Context) error {
				_, err := uc.Jalankan(ctx)
				return err
			},
		}}, nil
	})
}

// konfigAuthPembersihan membaca env JOB_AUTH_PEMBERSIHAN_* ke konfigurasi usecase. Nilai di luar rentang
// ditolak (worker gagal start) dan tidak dibulatkan diam-diam, supaya salah ketik tampak sejak awal.
func konfigAuthPembersihan(getenv func(string) string) (authuc.KonfigPembersihan, error) {
	cfg := authuc.DefaultKonfigPembersihan()

	if raw := strings.TrimSpace(getenv("JOB_AUTH_PEMBERSIHAN_LOGIN_HARI")); raw != "" {
		switch strings.ToLower(raw) {
		case "off", "false", "disabled":
			cfg.RetensiLoginAttempt = 0
		default:
			hari, err := bilanganBulat(raw, 1, authPembersihanLoginHariMaks)
			if err != nil {
				return cfg, fmt.Errorf("JOB_AUTH_PEMBERSIHAN_LOGIN_HARI=%q harus bilangan 1..%d hari (atau off untuk tidak menghapus login_attempts; lantai 1 hari melindungi perhitungan penguncian akun)", raw, authPembersihanLoginHariMaks)
			}
			cfg.RetensiLoginAttempt = time.Duration(hari) * 24 * time.Hour
		}
	}
	if raw := strings.TrimSpace(getenv("JOB_AUTH_PEMBERSIHAN_RESET_HARI")); raw != "" {
		hari, err := bilanganBulat(raw, 0, authPembersihanResetHariMaks)
		if err != nil {
			return cfg, fmt.Errorf("JOB_AUTH_PEMBERSIHAN_RESET_HARI=%q harus bilangan 0..%d hari", raw, authPembersihanResetHariMaks)
		}
		cfg.RetensiResetToken = time.Duration(hari) * 24 * time.Hour
	}
	if raw := strings.TrimSpace(getenv("JOB_AUTH_PEMBERSIHAN_BATCH")); raw != "" {
		n, err := bilanganBulat(raw, 1, authPembersihanBatchMaks)
		if err != nil {
			return cfg, fmt.Errorf("JOB_AUTH_PEMBERSIHAN_BATCH=%q harus bilangan 1..%d", raw, authPembersihanBatchMaks)
		}
		cfg.UkuranBatch = n
	}
	return cfg, nil
}

func bilanganBulat(raw string, terkecil, terbesar int) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n < terkecil || n > terbesar {
		return 0, fmt.Errorf("di luar rentang %d..%d", terkecil, terbesar)
	}
	return n, nil
}
