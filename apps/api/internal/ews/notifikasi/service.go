// Package notifikasi mengantrekan dan mengirim notifikasi alert EWS lewat adaptor kanal
// yang dapat diganti (email SMTP, Web Push). Semua notifikasi lebih dulu dicatat di outbox
// ews_notifikasi; pengiriman nyata hanya terjadi bila adaptor kanalnya dikonfigurasi,
// selain itu barisnya ditandai DICATAT.
package notifikasi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
)

// PermissionPenerima adalah izin yang menentukan peran penerima notifikasi: mereka yang
// bertugas menindaklanjuti alert.
const PermissionPenerima = "ews.akui"

// Nilai bawaan pengiriman.
const (
	DefaultInterval  = 30 * time.Second
	DefaultMaxCoba   = 5
	batchPengiriman  = 50
	leasePengiriman  = 5 * time.Minute
	backoffAwal      = time.Minute
	backoffMaksimum  = time.Hour
	timeoutPengirim  = 30 * time.Second
	galatMaksPanjang = 500
)

var (
	// ErrKanalTidakValid: kanal di luar daftar yang dapat diatur pengguna.
	ErrKanalTidakValid = errors.New("kanal notifikasi tidak valid")
	// ErrSeverityTidakValid: tingkat minimum di luar INFO..KRITIS.
	ErrSeverityTidakValid = errors.New("tingkat minimum tidak valid")
)

// Config mengatur pengiriman dan eskalasi otomatis.
type Config struct {
	// Interval antar putaran pengiriman; <= 0 menonaktifkan Run.
	Interval time.Duration
	// MaxPercobaan adalah jumlah percobaan sebelum baris ditandai GAGAL.
	MaxPercobaan int
	// EskalasiSetelah: alert BARU berseverity >= EskalasiMin yang tidak diakui selama ini
	// dieskalasi otomatis. 0 menonaktifkan.
	EskalasiSetelah time.Duration
	EskalasiMin     string
	// VAPIDPublicKey diteruskan ke klien untuk berlangganan Web Push; kosong bila push tidak dikonfigurasi.
	VAPIDPublicKey string
}

// Service mengantrekan, mengirim, dan mengatur preferensi notifikasi.
type Service struct {
	repo    domain.NotifikasiRepository
	senders map[string]domain.Pengirim
	peran   []string
	cfg     Config
	audit   audit.Writer
	log     *logger.Logger
	now     func() time.Time
	wake    chan struct{}
	mu      sync.Mutex // menserialkan putaran Kirim
}

var _ domain.Observer = (*Service)(nil)

// New membuat Service. peran adalah peran penerima (lihat PermissionPenerima); pengirim
// hanya berisi adaptor kanal yang terkonfigurasi.
func New(repo domain.NotifikasiRepository, peran []string, cfg Config, auditWriter audit.Writer, log *logger.Logger, pengirim ...domain.Pengirim) *Service {
	if cfg.MaxPercobaan < 1 {
		cfg.MaxPercobaan = DefaultMaxCoba
	}
	if cfg.EskalasiMin == "" {
		cfg.EskalasiMin = domain.SeverityMinBawaan
	}
	senders := make(map[string]domain.Pengirim, len(pengirim))
	for _, p := range pengirim {
		senders[p.Kanal()] = p
	}
	return &Service{
		repo: repo, senders: senders, peran: peran, cfg: cfg, audit: auditWriter, log: log,
		now: time.Now, wake: make(chan struct{}, 1),
	}
}

// VAPIDPublicKey adalah kunci publik yang dibutuhkan browser untuk berlangganan Web Push.
func (s *Service) VAPIDPublicKey() string { return s.cfg.VAPIDPublicKey }

// Tersedia melaporkan apakah kanal punya adaptor terkonfigurasi (pengiriman nyata).
func (s *Service) Tersedia(kanal string) bool {
	_, ok := s.senders[kanal]
	return ok
}

// AlertDitulis mengantrekan notifikasi untuk alert yang baru dibuat atau naik tingkat
// (domain.Observer). Pengguna yang sudah dinotifikasi untuk alert ini tidak dinotifikasi lagi.
func (s *Service) AlertDitulis(ctx context.Context, alert domain.Alert, _ domain.HasilTulis) {
	n, err := s.repo.Antrekan(ctx, alert.ID, s.peran, s.now())
	if err != nil {
		s.log.Error("ews: gagal mengantrekan notifikasi", map[string]interface{}{"alert": alert.ID, "error": err.Error()})
		return
	}
	if n > 0 {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// RingkasanKirim melaporkan satu putaran pengiriman.
type RingkasanKirim struct {
	Terkirim, Dicatat, Diulang, Gagal int
}

// Kirim mengirim notifikasi jatuh tempo. Aman dipanggil bersamaan (bergantian).
func (s *Service) Kirim(ctx context.Context) (RingkasanKirim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var r RingkasanKirim
	pesan, err := s.repo.Klaim(ctx, s.now(), batchPengiriman, leasePengiriman)
	if err != nil {
		return r, err
	}
	for _, p := range pesan {
		status, galat := s.kirimSatu(ctx, p)
		now := s.now()
		jadwal := now.Add(backoff(p.Percobaan + 1))
		if status == domain.NotifMenunggu && p.Percobaan+1 >= s.cfg.MaxPercobaan {
			status = domain.NotifGagal
		}
		if err := s.repo.Tandai(ctx, p.NotifikasiID, status, galat, jadwal, now); err != nil {
			s.log.Error("ews: gagal menyimpan hasil notifikasi", map[string]interface{}{"id": p.NotifikasiID, "error": err.Error()})
			continue
		}
		switch status {
		case domain.NotifTerkirim:
			r.Terkirim++
		case domain.NotifDicatat:
			r.Dicatat++
		case domain.NotifGagal:
			r.Gagal++
		default:
			r.Diulang++
		}
	}
	return r, nil
}

// kirimSatu menghasilkan status akhir percobaan ini: TERKIRIM, DICATAT (tanpa adaptor atau
// tujuan), atau MENUNGGU (galat sementara; pemanggil memutuskan GAGAL bila percobaan habis).
func (s *Service) kirimSatu(ctx context.Context, p domain.Pesan) (status, galat string) {
	sender, ok := s.senders[p.Kanal]
	if !ok {
		return domain.NotifDicatat, "adaptor kanal " + p.Kanal + " belum dikonfigurasi; hanya dicatat"
	}
	p.TautanPath = "/ews?id=" + p.AlertID.String()
	ctx, cancel := context.WithTimeout(ctx, timeoutPengirim)
	defer cancel()

	err := sender.Kirim(ctx, p)
	switch {
	case err == nil:
		return domain.NotifTerkirim, ""
	case errors.Is(err, domain.ErrTanpaTujuan):
		return domain.NotifDicatat, potong(err.Error())
	default:
		s.log.Warn("ews: pengiriman notifikasi gagal", map[string]interface{}{"id": p.NotifikasiID, "kanal": p.Kanal, "error": err.Error()})
		return domain.NotifMenunggu, potong(err.Error())
	}
}

// Eskalasi mengeskalasi alert BARU yang tidak diakui dalam batas waktu (bila diaktifkan)
// dan mengembalikan id yang dieskalasi.
func (s *Service) Eskalasi(ctx context.Context) ([]idx.ID, error) {
	if s.cfg.EskalasiSetelah <= 0 {
		return nil, nil
	}
	ids, err := s.repo.EskalasiOtomatis(ctx, s.now(), s.cfg.EskalasiSetelah, s.cfg.EskalasiMin)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		err := s.audit.Write(ctx, audit.Entry{
			Aksi: "ESKALASI_OTOMATIS", Entitas: "ews_alert", EntitasID: id.String(), Hasil: audit.HasilBerhasil,
			Metadata: map[string]any{"setelah_menit": int(s.cfg.EskalasiSetelah.Minutes()), "minimum": s.cfg.EskalasiMin},
		})
		if err != nil {
			s.log.Warn("ews: gagal mencatat audit eskalasi otomatis", map[string]interface{}{"alert": id, "error": err.Error()})
		}
	}
	return ids, nil
}

// Run mengirim notifikasi dan mengeskalasi alert tiap interval (dan segera setelah ada
// antrean baru) sampai ctx selesai.
func (s *Service) Run(ctx context.Context) {
	if s.cfg.Interval <= 0 {
		return
	}
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		if _, err := s.Eskalasi(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("ews: eskalasi otomatis gagal", map[string]interface{}{"error": err.Error()})
		}
		if _, err := s.Kirim(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("ews: pengiriman notifikasi gagal", map[string]interface{}{"error": err.Error()})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
	}
}

// Preferensi adalah preferensi efektif satu kanal (bawaan bila belum diatur).
type Preferensi struct {
	Kanal       string
	SeverityMin string
	Aktif       bool
	// Tersedia: server memiliki adaptor terkonfigurasi untuk kanal ini.
	Tersedia bool
}

// DaftarPreferensi mengembalikan preferensi efektif tiap kanal outbox untuk pengguna.
func (s *Service) DaftarPreferensi(ctx context.Context, penggunaID string) ([]Preferensi, error) {
	tersimpan, err := s.repo.ListPreferensi(ctx, penggunaID)
	if err != nil {
		return nil, err
	}
	out := make([]Preferensi, 0, len(domain.KanalOutbox))
	for _, kanal := range domain.KanalOutbox {
		p := Preferensi{Kanal: kanal, SeverityMin: domain.SeverityMinBawaan, Aktif: true, Tersedia: s.Tersedia(kanal)}
		if i := slices.IndexFunc(tersimpan, func(t domain.PreferensiNotifikasi) bool { return t.Kanal == kanal }); i >= 0 {
			p.SeverityMin, p.Aktif = tersimpan[i].SeverityMin, tersimpan[i].Aktif
		}
		out = append(out, p)
	}
	return out, nil
}

// SimpanPreferensi memvalidasi dan menyimpan preferensi pengguna, lalu mencatatnya di audit.
func (s *Service) SimpanPreferensi(ctx context.Context, penggunaID string, prefs []Preferensi) error {
	rows := make([]domain.PreferensiNotifikasi, len(prefs))
	for i, p := range prefs {
		if !slices.Contains(domain.KanalOutbox, p.Kanal) {
			return fmt.Errorf("%w: %s", ErrKanalTidakValid, p.Kanal)
		}
		if domain.SeverityRank(p.SeverityMin) == 0 {
			return fmt.Errorf("%w: %s", ErrSeverityTidakValid, p.SeverityMin)
		}
		rows[i] = domain.PreferensiNotifikasi{PenggunaID: penggunaID, Kanal: p.Kanal, SeverityMin: p.SeverityMin, Aktif: p.Aktif}
	}
	if err := s.repo.SimpanPreferensi(ctx, penggunaID, rows); err != nil {
		return err
	}
	s.writeAudit(ctx, penggunaID, "UBAH_PREFERENSI_NOTIFIKASI", "preferensi_notifikasi", penggunaID, map[string]any{"preferensi": rows})
	return nil
}

// DaftarPush menyimpan langganan Web Push perangkat pengguna (mengganti bila endpoint sudah ada).
func (s *Service) DaftarPush(ctx context.Context, penggunaID string, l domain.LanggananPush) error {
	l.PenggunaID = penggunaID
	if err := s.repo.SimpanLangganan(ctx, l); err != nil {
		return err
	}
	s.writeAudit(ctx, penggunaID, "DAFTAR_PUSH", "langganan_push", penggunaID, nil)
	return nil
}

// BatalPush menghapus langganan Web Push; false bila tidak ada.
func (s *Service) BatalPush(ctx context.Context, penggunaID, endpoint string) (bool, error) {
	ada, err := s.repo.HapusLangganan(ctx, penggunaID, endpoint)
	if err != nil || !ada {
		return ada, err
	}
	s.writeAudit(ctx, penggunaID, "BATAL_PUSH", "langganan_push", penggunaID, nil)
	return true, nil
}

func (s *Service) writeAudit(ctx context.Context, actorID, aksi, entitas, entitasID string, metadata map[string]any) {
	err := s.audit.Write(ctx, audit.Entry{AktorID: actorID, Aksi: aksi, Entitas: entitas, EntitasID: entitasID, Hasil: audit.HasilBerhasil, Metadata: metadata})
	if err != nil {
		s.log.Warn("ews: gagal mencatat audit", map[string]interface{}{"aksi": aksi, "error": err.Error()})
	}
}

// backoff: 1 menit, 2, 4, ... maksimum 1 jam, untuk percobaan ke-n (n >= 1).
func backoff(n int) time.Duration {
	d := backoffAwal << min(max(n-1, 0), 10)
	return min(d, backoffMaksimum)
}

func potong(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > galatMaksPanjang {
		return s[:galatMaksPanjang]
	}
	return s
}
