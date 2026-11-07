// Package workflow adalah otomasi workflow intervensi: membuat kartu dari alert EWS (ditugasi
// otomatis ke unit, dengan tenggat dari SLA tahap pertama) dan mengeskalasi kartu yang melewati
// SLA tahapnya. Persetujuan berjenjang oleh pengguna ada di internal/intervensi/usecase.
package workflow

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// Nilai bawaan.
const (
	// DefaultKartuOtomatisMin: alert SIAGA atau lebih memicu kartu otomatis.
	DefaultKartuOtomatisMin = ewsdomain.SeveritySiaga
	// PeranPenanggungJawab adalah unit yang ditugasi otomatis pada kartu buatan sistem.
	PeranPenanggungJawab = "MANAJER_PROGRAM"
	// hariTenggatCadangan dipakai bila tahap pertama tidak punya SLA.
	hariTenggatCadangan = 3
	// maksJudul adalah panjang kolom intervensi.judul.
	maksJudul = 255
)

// Config mengatur otomasi workflow.
type Config struct {
	// SLAInterval: jeda pemeriksaan SLA tahap; 0 mematikan eskalasi otomatis (INTERVENSI_SLA_INTERVAL).
	SLAInterval time.Duration
	// KartuOtomatisMin: tingkat alert minimum yang memicu kartu; kosong mematikan (INTERVENSI_KARTU_OTOMATIS_MIN).
	KartuOtomatisMin string
}

// ConfigFromEnv membaca INTERVENSI_SLA_INTERVAL (durasi Go; bawaan mati) dan
// INTERVENSI_KARTU_OTOMATIS_MIN (INFO..KRITIS; bawaan SIAGA; "off" mematikan). Nilai tak valid
// memakai bawaan dan dilaporkan sebagai galat.
func ConfigFromEnv() (Config, error) {
	cfg := Config{KartuOtomatisMin: DefaultKartuOtomatisMin}
	var problems []string

	if raw := strings.TrimSpace(os.Getenv("INTERVENSI_SLA_INTERVAL")); raw != "" && !strings.EqualFold(raw, "off") && raw != "0" {
		if d, err := time.ParseDuration(raw); err != nil || d < 0 {
			problems = append(problems, fmt.Sprintf("INTERVENSI_SLA_INTERVAL: durasi %q tidak valid", raw))
		} else {
			cfg.SLAInterval = d
		}
	}
	if raw := strings.ToUpper(strings.TrimSpace(os.Getenv("INTERVENSI_KARTU_OTOMATIS_MIN"))); raw != "" {
		switch {
		case raw == "OFF" || raw == "0":
			cfg.KartuOtomatisMin = ""
		case ewsdomain.SeverityRank(raw) > 0:
			cfg.KartuOtomatisMin = raw
		default:
			problems = append(problems, fmt.Sprintf("INTERVENSI_KARTU_OTOMATIS_MIN: tingkat %q tidak valid", raw))
		}
	}
	if len(problems) > 0 {
		return cfg, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return cfg, nil
}

// Service menjalankan otomasi workflow intervensi.
type Service struct {
	repo  domain.WorkflowRepository
	audit audit.Writer
	log   *logger.Logger
	cfg   Config
	now   func() time.Time
	mu    sync.Mutex // menserialkan pemeriksaan SLA
}

var _ ewsdomain.Observer = (*Service)(nil)

func New(repo domain.WorkflowRepository, auditWriter audit.Writer, cfg Config, log *logger.Logger) *Service {
	return &Service{repo: repo, audit: auditWriter, log: log, cfg: cfg, now: time.Now}
}

var (
	sharedMu sync.Mutex
	shared   = map[*gorm.DB]*Service{}
)

// Shared mengembalikan satu Service per koneksi basis data; konfigurasi dibaca dari env saat pertama dibuat.
func Shared(db *gorm.DB, log *logger.Logger) *Service {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if s, ok := shared[db]; ok {
		return s
	}
	cfg, err := ConfigFromEnv()
	if err != nil {
		log.Warn("intervensi: konfigurasi workflow tidak valid, memakai bawaan", map[string]interface{}{"error": err.Error()})
	}
	s := New(postgres.New(db), audit.NewWriter(db), cfg, log)
	shared[db] = s
	return s
}

// SiklusUntuk menentukan siklus intervensi yang dipicu jenis alert: tekanan pasokan atau
// harga memicu siklus Tekanan, over-supply memicu Surplus. false bila jenis tidak memicu kartu.
func SiklusUntuk(jenis string) (string, bool) {
	switch jenis {
	case ewsdomain.JenisLonjakanHarga, ewsdomain.JenisKelangkaanStok, ewsdomain.JenisAnomaliCuaca:
		return domain.SiklusTekanan, true
	case ewsdomain.JenisOverSupply:
		return domain.SiklusSurplus, true
	}
	return "", false
}

// PrioritasUntuk memetakan tingkat alert ke prioritas kartu.
func PrioritasUntuk(severity string) string {
	switch severity {
	case ewsdomain.SeverityKritis:
		return "TINGGI"
	case ewsdomain.SeveritySiaga:
		return "SEDANG"
	}
	return "RENDAH"
}

// AlertDitulis membuat kartu intervensi untuk alert baru atau yang naik tingkat bila memenuhi
// tingkat minimum (ewsdomain.Observer). Satu alert hanya punya satu kartu otomatis; kartu yang
// sudah ada (otomatis maupun buatan pengguna) tidak digandakan.
func (s *Service) AlertDitulis(ctx context.Context, alert ewsdomain.Alert, _ ewsdomain.HasilTulis) {
	siklus, ok := SiklusUntuk(alert.Jenis)
	if !ok || s.cfg.KartuOtomatisMin == "" || alert.KomoditasKode == nil ||
		ewsdomain.SeverityRank(alert.Severity) < ewsdomain.SeverityRank(s.cfg.KartuOtomatisMin) {
		return
	}
	judul := fmt.Sprintf("%s — %s", alert.Judul, kapital(alert.KabupatenNama))
	if len(judul) > maksJudul {
		judul = judul[:maksJudul]
	}
	kartu, err := s.repo.BuatKartuOtomatis(ctx, domain.KartuOtomatis{
		AlertID: alert.ID, AlertKode: kodeAlert(alert), Siklus: siklus, Judul: judul, Deskripsi: alert.Pesan,
		KomoditasKode: *alert.KomoditasKode, KabupatenKode: alert.KabupatenKode, Prioritas: PrioritasUntuk(alert.Severity),
		HariTenggatCadangan: hariTenggatCadangan, PeranPenanggungJawab: []string{PeranPenanggungJawab}, Sekarang: s.now(),
	})
	if err != nil {
		s.log.Error("intervensi: gagal membuat kartu dari alert", map[string]interface{}{"alert": alert.ID, "error": err.Error()})
		return
	}
	if kartu == nil {
		return
	}
	s.writeAudit(ctx, audit.Entry{
		Aksi: "BUAT_OTOMATIS", Entitas: "intervensi", EntitasID: kartu.ID, Hasil: audit.HasilBerhasil,
		Sesudah:  map[string]any{"siklus": kartu.Siklus, "tahap": kartu.Tahap, "judul": kartu.Judul, "prioritas": kartu.Prioritas},
		Metadata: map[string]any{"ews_alert_id": alert.ID.String()},
	})
}

// kodeAlert adalah kode tampilan alert (EWS-n) bila ada, untuk teks riwayat kartu.
func kodeAlert(a ewsdomain.Alert) string {
	if a.Kode == nil {
		return ""
	}
	return *a.Kode
}

// CekSLA mengeskalasi kartu yang lebih lama di tahapnya daripada SLA tahap itu dan mengembalikan jumlahnya.
// Kartu yang dieskalasi membuka permintaan persetujuan eskalasi bila alurnya aktif.
func (s *Service) CekSLA(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lewat, err := s.repo.KartuMelewatiSLA(ctx, s.now())
	if err != nil {
		return 0, err
	}
	var n int
	for _, k := range lewat {
		alasan := fmt.Sprintf("Melewati SLA tahap %q (%d hari)", k.Label, k.SLAHari)
		eskalasi, err := s.repo.EskalasiSLA(ctx, k.ID, alasan)
		if err != nil {
			s.log.Error("intervensi: eskalasi SLA gagal", map[string]interface{}{"id": k.ID, "error": err.Error()})
			continue
		}
		if !eskalasi {
			continue
		}
		n++
		s.writeAudit(ctx, audit.Entry{
			Aksi: "ESKALASI_SLA", Entitas: "intervensi", EntitasID: k.ID, Hasil: audit.HasilBerhasil,
			Metadata: map[string]any{"tahap": k.TahapKode, "sla_hari": k.SLAHari},
		})
	}
	return n, nil
}

// Run memeriksa SLA tiap SLAInterval sampai ctx selesai; interval 0 menonaktifkannya.
func (s *Service) Run(ctx context.Context) {
	if s.cfg.SLAInterval <= 0 {
		return
	}
	ticker := time.NewTicker(s.cfg.SLAInterval)
	defer ticker.Stop()
	for {
		if n, err := s.CekSLA(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("intervensi: pemeriksaan SLA gagal", map[string]interface{}{"error": err.Error()})
		} else if n > 0 {
			s.log.Info("intervensi: kartu dieskalasi karena melewati SLA", map[string]interface{}{"jumlah": n})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) writeAudit(ctx context.Context, entry audit.Entry) {
	if err := s.audit.Write(ctx, entry); err != nil {
		s.log.Warn("intervensi: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

// kapital mengubah "KOTA BANDA ACEH" menjadi "Kota Banda Aceh".
func kapital(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
