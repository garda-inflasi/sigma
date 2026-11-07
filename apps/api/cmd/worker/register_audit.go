package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/platform/jobs"
)

// Jeda bawaan job integritas audit (03-C).
const (
	auditPartisiInterval    = 24 * time.Hour
	auditVerifikasiInterval = 24 * time.Hour
	auditPartisiKeDepan     = 3
)

// Job audit_partisi dan audit_verifikasi (docs/todos/03-audit.md §03-C). Konfigurasi env:
//
//	JOB_AUDIT_PARTISI_INTERVAL     jeda penyiapan partisi + retensi (bawaan 24h, off = nonaktif)
//	AUDIT_PARTISI_KE_DEPAN         partisi bulanan yang disiapkan di depan bulan ini (bawaan 3; 0..24)
//	AUDIT_RETENSI_BULAN            masa simpan audit di basis data (12..1200 bulan); kosong/off = retensi mati
//	AUDIT_ARSIP_DIR                direktori penyimpanan dingin untuk berkas arsip (wajib bila retensi aktif)
//	JOB_AUDIT_VERIFIKASI_INTERVAL  jeda verifikasi rantai hash (bawaan 24h, off = nonaktif)
//
// Verifikasi terjadwal dilewati bila verifikasi terakhir dimulai kurang dari setengah interval yang lalu (worker yang
// dinyalakan ulang tidak memindai ulang seluruh rantai); `worker run audit_verifikasi` selalu menjalankannya.
func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		intervalPartisi, err := jobs.IntervalFromEnv(d.getenv, "audit_partisi", auditPartisiInterval)
		if err != nil {
			return nil, err
		}
		intervalVerifikasi, err := jobs.IntervalFromEnv(d.getenv, "audit_verifikasi", auditVerifikasiInterval)
		if err != nil {
			return nil, err
		}
		keDepan, retensi, err := konfigAuditIntegritas(d.getenv)
		if err != nil {
			return nil, err
		}
		uc := audit.NewIntegritas(d.db)

		partisi := jobs.Job{
			Name:       "audit_partisi",
			Interval:   intervalPartisi,
			Timeout:    30 * time.Minute,
			RunOnStart: true,
			// Idempoten: partisi yang sudah ada dilewati, partisi yang sudah diarsipkan tidak lagi terdaftar.
			Run: func(ctx context.Context) error {
				dibuat, err := uc.SiapkanPartisi(ctx, keDepan)
				if err != nil {
					return err
				}
				if len(dibuat) > 0 {
					d.log.Info("audit: partisi dibuat", map[string]interface{}{"partisi": dibuat})
				}
				if retensi == nil {
					return nil
				}
				hasil, err := uc.Retensi(ctx, *retensi)
				for _, h := range hasil {
					d.log.Info("audit: partisi diarsipkan", map[string]interface{}{
						"partisi": h.Partisi, "baris": h.JumlahBaris, "berkas": h.Berkas, "arsip_id": h.ArsipID,
					})
				}
				return err
			},
		}

		verifikasi := jobs.Job{
			Name:       "audit_verifikasi",
			Interval:   intervalVerifikasi,
			Timeout:    time.Hour,
			RunOnStart: true,
			Run: func(ctx context.Context) error {
				pemicu := domain.PemicuManual
				if !jobs.Manual(ctx) {
					pemicu = domain.PemicuJadwal
					jeda := intervalVerifikasi / 2
					if perlu, err := uc.PerluVerifikasi(ctx, jeda); err != nil || !perlu {
						return err
					}
				}
				hasil, err := uc.Verifikasi(ctx, pemicu)
				if err != nil {
					return err
				}
				info := map[string]interface{}{
					"status": hasil.Status, "baris": hasil.JumlahBaris, "segmen_arsip": hasil.JumlahSegmen,
					"masalah": hasil.JumlahMasalah, "durasi": hasil.Selesai.Sub(hasil.Mulai).String(),
				}
				if hasil.Status != domain.StatusValid {
					return fmt.Errorf("rantai hash audit %s: %d masalah (lihat GET /v1/audit/integritas)", hasil.Status, hasil.JumlahMasalah)
				}
				d.log.Info("audit: rantai hash utuh", info)
				return nil
			},
		}
		return []jobs.Job{partisi, verifikasi}, nil
	})
}

// konfigAuditIntegritas membaca AUDIT_PARTISI_KE_DEPAN, AUDIT_RETENSI_BULAN, dan AUDIT_ARSIP_DIR. Retensi nil berarti
// mati. Nilai yang salah menggagalkan start worker.
func konfigAuditIntegritas(getenv func(string) string) (int, *audit.KonfigRetensi, error) {
	keDepan := auditPartisiKeDepan
	if raw := strings.TrimSpace(getenv("AUDIT_PARTISI_KE_DEPAN")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 24 {
			return 0, nil, fmt.Errorf("AUDIT_PARTISI_KE_DEPAN=%q harus bilangan 0..24", raw)
		}
		keDepan = n
	}

	raw := strings.TrimSpace(getenv("AUDIT_RETENSI_BULAN"))
	switch strings.ToLower(raw) {
	case "", "0", "off", "false", "disabled":
		return keDepan, nil, nil
	}
	bulan, err := strconv.Atoi(raw)
	if err != nil {
		return 0, nil, fmt.Errorf("AUDIT_RETENSI_BULAN=%q bukan bilangan bulan", raw)
	}
	retensi := audit.KonfigRetensi{Bulan: bulan, Dir: strings.TrimSpace(getenv("AUDIT_ARSIP_DIR"))}
	if err := retensi.Validasi(); err != nil {
		return 0, nil, fmt.Errorf("AUDIT_RETENSI_BULAN/AUDIT_ARSIP_DIR: %w", err)
	}
	return keDepan, &retensi, nil
}
