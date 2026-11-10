// Package engine adalah mesin penentuan siklus ekonomi otomatis (08-B): mengukur aturan aktif (aturan_siklus) terhadap
// data nyata (alert EWS, stok cold storage), memutuskan siklus wilayah (Putuskan), lalu menulisnya secara atomik.
//
// Keamanan multi-instance: satu putaran berjalan di dalam satu transaksi yang memegang kunci advisory per wilayah,
// jadi job terjadwal, evaluasi manual, dan pemicu alert (di api maupun beberapa worker) bergantian, tidak pernah
// bersamaan. Menutup siklus lama dan membuka yang baru terjadi di transaksi yang sama (CHECK akhir_pada > mulai_pada
// dijaga), dan indeks unik parsial uq_siklus_ekonomi_aktif tetap menjadi pengaman terakhir bila ada penulis lain.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// Sumber adalah pelaku perubahan siklus yang dicatat di audit bila bukan pengguna.
const Sumber = "Mesin siklus"

// Aksi audit untuk pergantian siklus oleh mesin.
const AksiAudit = "GANTI_SIKLUS"

// Engine menjalankan putaran evaluasi. Aman dipanggil dari banyak goroutine dan banyak proses.
type Engine struct {
	repo  domain.EngineRepository
	audit audit.Writer
	log   *logger.Logger
}

var _ domain.Evaluator = (*Engine)(nil)

// New membuat Engine.
func New(repo domain.EngineRepository, auditWriter audit.Writer, log *logger.Logger) *Engine {
	return &Engine{repo: repo, audit: auditWriter, log: log}
}

// Evaluasi mengukur semua aturan aktif untuk wilayah dan mengganti siklus bila keputusan berbeda dari siklus aktif.
//
//   - Tanpa aturan aktif atau wilayah tak dikenal: dilewati, tidak ada yang berubah dan tidak galat.
//   - Belum ada siklus aktif (basis data kosong): dibuka siklus hasil keputusan (NORMAL bila tidak ada pemicu), supaya
//     antarmuka punya siklus yang jelas dan GET /v1/siklus/aktif tidak lagi 404.
//   - Satu aturan gagal diukur: seluruh putaran dibatalkan dan galat dikembalikan, tidak ada yang berubah. "Tidak
//     tahu" tidak boleh dibaca "tidak ada pemicu", yang bisa melepas siklus yang sebenarnya masih berlangsung.
//   - Siklus sama dengan keputusan: tidak ada baris baru, tidak ada audit. Yang mungkin ditulis hanya penanda masa
//     pulih (pulih_sejak) siklus aktif, itupun hanya saat berubah.
//
// Pergantian siklus dicatat ke audit sesudah transaksi berhasil (pola modul lain: kegagalan audit hanya dilog).
func (e *Engine) Evaluasi(ctx context.Context, wilayahKode string, opsi domain.OpsiEvaluasi) (domain.HasilEvaluasi, error) {
	var (
		hasil domain.HasilEvaluasi
		lama  *domain.SiklusEkonomi
		err   error
	)
	// Satu ulangan bila penulis lain (di luar mesin) membuka siklus aktif tepat di antara baca dan tulis.
	for percobaan := 0; percobaan < 2; percobaan++ {
		hasil, lama, err = e.jalankan(ctx, wilayahKode)
		if !errors.Is(err, domain.ErrBentrokSiklus) {
			break
		}
	}
	if err != nil {
		return domain.HasilEvaluasi{WilayahKode: wilayahKode}, err
	}
	if hasil.Berubah {
		e.catat(ctx, hasil, lama, opsi)
	}
	return hasil, nil
}

// jalankan melakukan satu putaran di dalam transaksi. lama adalah siklus aktif sebelum pergantian (nil bila tidak ada).
func (e *Engine) jalankan(ctx context.Context, wilayahKode string) (hasil domain.HasilEvaluasi, lama *domain.SiklusEkonomi, err error) {
	err = e.repo.Dalam(ctx, wilayahKode, func(s domain.Sesi) error {
		// fn bisa dijalankan ulang pada ulangan; mulai dari keadaan bersih.
		hasil, lama = domain.HasilEvaluasi{WilayahKode: wilayahKode}, nil

		ada, err := s.WilayahAda(ctx)
		if err != nil {
			return fmt.Errorf("periksa wilayah: %w", err)
		}
		if !ada {
			hasil.Dilewati, hasil.SebabDilewati = true, domain.SebabWilayahTakDikenal
			hasil.Ringkasan = fmt.Sprintf("Evaluasi dilewati: wilayah %s belum ada di master provinsi.", wilayahKode)
			return nil
		}
		aktif, err := s.SiklusAktif(ctx)
		if err != nil {
			return fmt.Errorf("baca siklus aktif: %w", err)
		}
		if aktif != nil {
			hasil.Sebelumnya, hasil.Aktif = aktif.Siklus, aktif
		}
		aturan, err := s.AturanAktif(ctx)
		if err != nil {
			return fmt.Errorf("baca aturan: %w", err)
		}
		if len(aturan) == 0 {
			hasil.Dilewati, hasil.SebabDilewati = true, domain.SebabTanpaAturan
			hasil.Ringkasan = "Evaluasi dilewati: tidak ada aturan penentuan siklus yang aktif; siklus tidak diubah."
			return nil
		}

		sekarang, err := s.Sekarang(ctx)
		if err != nil {
			return fmt.Errorf("baca jam basis data: %w", err)
		}
		hasilAturan := make([]domain.HasilAturan, 0, len(aturan))
		for _, a := range aturan {
			p, err := s.Ukur(ctx, a, sekarang)
			if err != nil {
				return fmt.Errorf("ukur aturan %s: %w", a.Kode, err)
			}
			hasilAturan = append(hasilAturan, domain.HasilAturan{Aturan: a, Pengukuran: p, Memicu: a.Memicu(p)})
		}

		keputusan := Putuskan(Kondisi{Sekarang: sekarang, Aktif: aktif, Hasil: hasilAturan})
		hasil.Aturan, hasil.Ringkasan = hasilAturan, keputusan.Alasan.Ringkasan

		if keputusan.Ganti {
			baru, err := s.Ganti(ctx, aktif, domain.SiklusBaru{WilayahKode: wilayahKode, Siklus: keputusan.Siklus, Alasan: keputusan.Alasan}, sekarang)
			if err != nil {
				return err
			}
			lama, hasil.Berubah, hasil.Aktif = aktif, true, baru
			return nil
		}
		if aktif == nil { // tak mungkin menurut Putuskan; jaga agar tidak panik bila kontraknya berubah
			return errors.New("keputusan mempertahankan siklus aktif yang tidak ada")
		}
		if !samaWaktu(aktif.PulihSejak, keputusan.PulihSejak) {
			if err := s.CatatPulih(ctx, aktif.ID, keputusan.PulihSejak); err != nil {
				return fmt.Errorf("catat masa pulih: %w", err)
			}
			salinan := *aktif
			salinan.PulihSejak = keputusan.PulihSejak
			hasil.Aktif = &salinan
		}
		return nil
	})
	return hasil, lama, err
}

// catat menulis audit pergantian siklus dan log. Konteks dilepaskan dari pembatalan: pergantian sudah berlaku, dan
// jejaknya tidak boleh hilang karena pemanggil kebetulan berhenti tepat sesudah commit.
func (e *Engine) catat(ctx context.Context, hasil domain.HasilEvaluasi, lama *domain.SiklusEkonomi, opsi domain.OpsiEvaluasi) {
	baru := hasil.Aktif
	pemicu := opsi.Pemicu
	if pemicu == "" {
		pemicu = domain.PemicuJadwal
	}

	var sebelum any
	if lama != nil {
		ringkas := ringkasSiklus(lama)
		ringkas["akhir_pada"] = baru.MulaiPada // siklus lama ditutup pada instan siklus baru dibuka
		sebelum = ringkas
	}
	sesudah := ringkasSiklus(baru)
	if alasan := alasanDokumen(baru); alasan != nil {
		sesudah["alasan"] = alasan
	}
	err := e.audit.Write(context.WithoutCancel(ctx), audit.Entry{
		AktorID: opsi.AktorID, Aksi: AksiAudit, Entitas: "siklus_ekonomi", EntitasID: baru.ID.String(), Hasil: audit.HasilBerhasil,
		Sebelum: sebelum, Sesudah: sesudah,
		Metadata: map[string]any{"sumber": Sumber, "wilayah_kode": hasil.WilayahKode, "pemicu": pemicu, "ringkasan": hasil.Ringkasan},
	})
	if err != nil {
		e.log.Warn("siklus: gagal mencatat audit", map[string]interface{}{"siklus_id": baru.ID, "error": err.Error()})
	}
	e.log.Info("siklus: berganti", map[string]interface{}{
		"wilayah": hasil.WilayahKode, "dari": hasil.Sebelumnya, "ke": baru.Siklus, "pemicu": pemicu, "ringkasan": hasil.Ringkasan,
	})
}

// Run mengevaluasi segera lalu tiap interval sampai ctx selesai. Interval <= 0 menonaktifkannya. Dipakai api saat tidak
// ada worker (API_BACKGROUND_JOBS bukan false); worker memakai penjadwal job (internal/platform/jobs).
func (e *Engine) Run(ctx context.Context, wilayahKode string, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := e.Evaluasi(ctx, wilayahKode, domain.OpsiEvaluasi{Pemicu: domain.PemicuJadwal}); err != nil && ctx.Err() == nil {
			e.log.Error("siklus: evaluasi terjadwal gagal", map[string]interface{}{"wilayah": wilayahKode, "error": err.Error()})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ringkasSiklus memilih bagian siklus yang bermakna untuk jejak audit.
func ringkasSiklus(s *domain.SiklusEkonomi) map[string]any {
	out := map[string]any{
		"id": s.ID.String(), "wilayah_kode": s.WilayahKode, "siklus": s.Siklus, "mulai_pada": s.MulaiPada, "otomatis": s.Otomatis,
	}
	if s.AkhirPada != nil {
		out["akhir_pada"] = *s.AkhirPada
	}
	return out
}

// alasanDokumen mengurai alasan JSON siklus agar masuk audit sebagai objek, bukan teks bersusun.
func alasanDokumen(s *domain.SiklusEkonomi) json.RawMessage {
	if s.Alasan == nil {
		return nil
	}
	return json.RawMessage(*s.Alasan)
}

// samaWaktu membandingkan dua penanda waktu opsional menurut instan, bukan zona.
func samaWaktu(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
