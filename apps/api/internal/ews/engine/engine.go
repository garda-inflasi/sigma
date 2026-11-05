// Package engine adalah rule engine EWS: mengukur data nyata (harga tervalidasi,
// stok cold storage, cuaca harian) terhadap aturan ambang dan menulis alert yang
// idempoten. Satu kondisi (jenis, komoditas, kabupaten) hanya punya satu alert aktif.
package engine

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// Sumber alert yang dibuat mesin.
const Sumber = "Rule engine EWS"

// Engine mengevaluasi seluruh aturan aktif. Aman dipanggil dari beberapa goroutine;
// putaran yang bersamaan dijalankan bergantian.
type Engine struct {
	repo      domain.EngineRepository
	audit     audit.Writer
	log       *logger.Logger
	observers []domain.Observer
	now       func() time.Time
	mu        sync.Mutex
}

var _ domain.Evaluator = (*Engine)(nil)

// New membuat Engine; observers dipanggil berurutan untuk tiap alert yang dibuat atau dinaikkan tingkatnya.
func New(repo domain.EngineRepository, auditWriter audit.Writer, log *logger.Logger, observers ...domain.Observer) *Engine {
	return &Engine{repo: repo, audit: auditWriter, log: log, observers: observers, now: time.Now}
}

// Severity menentukan tingkat dari besaran pengukuran terhadap ambang aturan;
// kosong bila di bawah ambang waspada.
func Severity(besaran float64, a domain.AturanEws) string {
	switch {
	case besaran >= a.AmbangKritis:
		return domain.SeverityKritis
	case besaran >= a.AmbangSiaga:
		return domain.SeveritySiaga
	case besaran >= a.AmbangWaspada:
		return domain.SeverityWaspada
	}
	return ""
}

// besaran adalah ukuran yang dibandingkan dengan ambang: kenaikan harga, besar
// penurunan stok (nilai tercatat negatif), atau curah hujan.
func besaran(jenis string, nilai float64) float64 {
	if jenis == domain.JenisKelangkaanStok {
		return -nilai
	}
	return nilai
}

// Evaluasi mengukur semua aturan aktif dan menulis alert. Kegagalan satu aturan
// atau kondisi dihitung sebagai galat dan dicatat di log; putaran tetap berlanjut.
func (e *Engine) Evaluasi(ctx context.Context) (domain.RingkasanEvaluasi, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var ringkasan domain.RingkasanEvaluasi
	aturan, err := e.repo.AturanAktif(ctx)
	if err != nil {
		return ringkasan, err
	}
	ringkasan.Aturan = len(aturan)

	now := e.now()
	for _, a := range aturan {
		ukuran, err := e.repo.Ukur(ctx, a, now)
		if err != nil {
			e.log.Error("ews: ukur aturan gagal", map[string]interface{}{"aturan": a.ID, "error": err.Error()})
			ringkasan.Galat++
			continue
		}
		ringkasan.Diukur += len(ukuran)
		for _, p := range ukuran {
			severity := Severity(besaran(a.Jenis, p.Nilai), a)
			if severity == "" {
				continue
			}
			if err := e.tulis(ctx, a, p, severity, now, &ringkasan); err != nil {
				e.log.Error("ews: tulis alert gagal", map[string]interface{}{"aturan": a.ID, "kabupaten": p.KabupatenKode, "error": err.Error()})
				ringkasan.Galat++
			}
		}
	}
	return ringkasan, nil
}

func (e *Engine) tulis(ctx context.Context, a domain.AturanEws, p domain.Pengukuran, severity string, now time.Time, ringkasan *domain.RingkasanEvaluasi) error {
	in := baru(a, p, severity)
	hasil, alert, err := e.repo.Terapkan(ctx, in, now)
	if err != nil {
		return err
	}
	switch hasil {
	case domain.HasilDibuat:
		ringkasan.Dibuat++
	case domain.HasilDinaikkan:
		ringkasan.Dinaikkan++
	case domain.HasilDiredam:
		ringkasan.Diredam++
	default:
		ringkasan.Dilewati++
	}
	if alert == nil {
		return nil
	}

	aksi := "BUAT_OTOMATIS"
	if hasil == domain.HasilDinaikkan {
		aksi = "NAIK_TINGKAT"
	}
	err = e.audit.Write(ctx, audit.Entry{
		Aksi: aksi, Entitas: "ews_alert", EntitasID: alert.ID, Hasil: audit.HasilBerhasil,
		Sesudah:  map[string]any{"severity": alert.Severity, "nilai": alert.Nilai, "dedup_key": in.DedupKey},
		Metadata: map[string]any{"aturan": a.ID, "sumber": Sumber},
	})
	if err != nil {
		e.log.Warn("ews: gagal mencatat audit", map[string]interface{}{"alert": alert.ID, "error": err.Error()})
	}
	for _, o := range e.observers {
		o.AlertDitulis(ctx, *alert, hasil)
	}
	return nil
}

// Run mengevaluasi segera lalu tiap interval sampai ctx selesai. Interval <= 0 menonaktifkannya.
func (e *Engine) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		ringkasan, err := e.Evaluasi(ctx)
		switch {
		case err == nil:
			e.log.Info("ews: evaluasi terjadwal selesai", map[string]interface{}{
				"aturan": ringkasan.Aturan, "dibuat": ringkasan.Dibuat, "dinaikkan": ringkasan.Dinaikkan, "galat": ringkasan.Galat,
			})
		case ctx.Err() == nil:
			e.log.Error("ews: evaluasi terjadwal gagal", map[string]interface{}{"error": err.Error()})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// baru menyusun alert dari satu pengukuran yang melampaui ambang.
func baru(a domain.AturanEws, p domain.Pengukuran, severity string) domain.NewAlert {
	size := besaran(a.Jenis, p.Nilai)
	wilayah := judulKapital(p.KabupatenNama)
	nama := strings.ToLower(p.KomoditasNama)

	in := domain.NewAlert{
		AturanID: a.ID, Jenis: a.Jenis, Severity: severity, KomoditasKode: p.KomoditasKode,
		KabupatenKode: p.KabupatenKode, Nilai: p.Nilai, SatuanNilai: "%", Sumber: Sumber,
		SkorDampak: skor(size, severity, a), CooldownJam: a.CooldownJam,
		DedupKey: dedupKey(a.Jenis, p.KomoditasKode, p.KabupatenKode),
	}
	switch a.Jenis {
	case domain.JenisLonjakanHarga:
		in.Judul = "Lonjakan harga " + nama
		in.Pesan = fmt.Sprintf("Harga konsumen naik %s%% di atas rata-rata %d hari di %s. Harga rata-rata terbaru Rp%s/%s.",
			angka(size, 1), a.JendelaHari, wilayah, angka(p.Terbaru, 0), p.SatuanKomoditas)
	case domain.JenisKelangkaanStok:
		in.Judul = "Stok " + nama + " menipis"
		in.Pesan = fmt.Sprintf("Stok layak simpan turun %s%% dalam %d hari di %s; tersisa %s ton.",
			angka(size, 1), a.JendelaHari, wilayah, angka(p.Terbaru, 1))
	case domain.JenisAnomaliCuaca:
		in.SatuanNilai = "mm"
		in.Judul = "Curah hujan tinggi di " + wilayah
		in.Pesan = fmt.Sprintf("Curah hujan kumulatif %s mm dalam %d hari di %s.", angka(size, 0), a.JendelaHari, wilayah)
		if nama != "" {
			in.Pesan += " Risiko gangguan pasokan " + nama + " meningkat."
		}
	}
	return in
}

func dedupKey(jenis string, komoditas *string, kabupaten string) string {
	kode := "-"
	if komoditas != nil {
		kode = *komoditas
	}
	return jenis + "|" + kode + "|" + kabupaten
}

// skor memetakan tingkat dan kedekatan dengan ambang kritis ke skor dampak 0-99:
// dasar per tingkat (WASPADA 40, SIAGA 60, KRITIS 80) ditambah hingga 19 poin.
func skor(besaran float64, severity string, a domain.AturanEws) int {
	dasar := map[string]int{domain.SeverityWaspada: 40, domain.SeveritySiaga: 60, domain.SeverityKritis: 80}[severity]
	return dasar + int(math.Min(19, besaran/a.AmbangKritis*10))
}

// angka memformat bilangan gaya Indonesia (titik ribuan, koma desimal).
func angka(v float64, desimal int) string {
	s := fmt.Sprintf("%.*f", desimal, math.Abs(v))
	bulat, pecahan, _ := strings.Cut(s, ".")
	var sb strings.Builder
	for i, r := range bulat {
		if i > 0 && (len(bulat)-i)%3 == 0 {
			sb.WriteByte('.')
		}
		sb.WriteRune(r)
	}
	if pecahan != "" {
		sb.WriteString("," + pecahan)
	}
	return sb.String()
}

// judulKapital mengubah "KOTA BANDA ACEH" menjadi "Kota Banda Aceh".
func judulKapital(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
