// Package jembatan mengubah sinyal aktif datasafe_ews_signals (siklus ML final terbaru) menjadi alert ews_alert di
// PostgreSQL, sehingga sinyal pipeline ikut alur akui/eskalasi/selesai, aliran SSE, dan notifikasi EWS yang sudah ada.
//
// Aturan:
//   - Idempoten lintas siklus: satu kondisi (sinyal_kunci) = satu dedup_key `ML|<sinyal_kunci>`, sehingga hanya satu
//     alert aktif per kondisi (indeks unik parsial ews_alert). Tiap kunci diterapkan sekali per siklus (ews_sinyal_ml),
//     jadi job yang berjalan beberapa kali dalam satu jam tidak membuat alert ganda.
//   - KRITIS mengikuti aturan modul EWS (repository Terapkan): tidak pernah diredam cooldown, naik ke KRITIS membuka
//     kembali alert ke BARU, dan tidak pernah ditutup otomatis. Jembatan ini tidak menutup alert apa pun: kondisi yang
//     pulih hanya dicatat di riwayat alert, penutupan tetap oleh pengguna.
//   - Sinyal INFO (termasuk SUMBER_DATA untuk sumber yang sengaja dilewati) tidak menjadi alert. Kegagalan crawler
//     nyata (SUMBER_DATA ERROR/basi, SIAGA/WASPADA/KRITIS) menjadi alert tingkat sistem tanpa kabupaten.
//   - Observer yang dipanggil hanya aliran SSE dan notifikasi; alert pipeline tidak otomatis membuat kartu intervensi
//     dan tidak dihitung mesin siklus ekonomi (lihat internal/siklus), karena labelnya bisa berbasis indeks pangan.
package jembatan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
)

// CooldownJam adalah masa peredaman setelah alert non-KRITIS ditutup (sama dengan bawaan aturan_ews.cooldown_jam).
const CooldownJam = 24

// awalanDedup membedakan kunci alert pipeline dari kunci rule engine (JENIS|komoditas|kabupaten).
const awalanDedup = "ML|"

// Batas panjang kolom ews_alert.
const (
	maksJudul = 255
	maksDedup = 255
)

// SumberSinyal membaca siklus final terbaru beserta sinyalnya.
type SumberSinyal interface {
	Snapshot(ctx context.Context, cycleID string) (*domain.Snapshot, usecase.Keadaan)
}

// Keadaan adalah penyimpanan keadaan jembatan dan bacaan pendukung di PostgreSQL.
type Keadaan interface {
	MuatKeadaan(ctx context.Context, kunci []string) (map[string]domain.EwsSinyalML, error)
	SimpanKeadaan(ctx context.Context, k domain.EwsSinyalML) error
	AlertAktif(ctx context.Context, dedupKey string) (*idx.ID, error)
	CatatMereda(ctx context.Context, alertID idx.ID, catatan string, now time.Time) (bool, error)
	KodeMaster(ctx context.Context) (komoditas, kabupaten map[string]bool, err error)
}

// PenulisAlert menulis alert idempoten menurut dedup key (ews repository postgres).
type PenulisAlert interface {
	Terapkan(ctx context.Context, in ewsdomain.NewAlert, now time.Time) (ewsdomain.HasilTulis, *ewsdomain.Alert, error)
}

// Ringkasan melaporkan satu putaran jembatan.
type Ringkasan struct {
	CycleID         string `json:"cycle_id"`
	Sinyal          int    `json:"sinyal"`
	Dibuat          int    `json:"dibuat"`
	Dinaikkan       int    `json:"dinaikkan"`
	Dilewati        int    `json:"dilewati"`
	Diredam         int    `json:"diredam"`
	Mereda          int    `json:"mereda"`
	DiabaikanInfo   int    `json:"diabaikan_info"`
	SudahDiterapkan int    `json:"sudah_diterapkan"`
	Galat           int    `json:"galat"`
}

// Jembatan menjalankan satu putaran penerapan sinyal.
type Jembatan struct {
	sumber    SumberSinyal
	keadaan   Keadaan
	penulis   PenulisAlert
	audit     audit.Writer
	observers []ewsdomain.Observer
	log       *logger.Logger
	now       func() time.Time
}

// New membuat Jembatan; observers dipanggil untuk tiap alert yang dibuat atau naik tingkat (SSE, notifikasi).
func New(sumber SumberSinyal, keadaan Keadaan, penulis PenulisAlert, auditWriter audit.Writer, log *logger.Logger, observers ...ewsdomain.Observer) *Jembatan {
	return &Jembatan{sumber: sumber, keadaan: keadaan, penulis: penulis, audit: auditWriter, observers: observers, log: log, now: time.Now}
}

// SetJam mengganti sumber waktu (untuk tes).
func (j *Jembatan) SetJam(now func() time.Time) { j.now = now }

// ErrSumberTidakTersedia: siklus tidak dapat dibaca (Mongo tidak dikonfigurasi, mati, atau lambat).
var ErrSumberTidakTersedia = errors.New("hasil pipeline ML tidak tersedia")

// Jalankan menerapkan sinyal siklus final terbaru. Belum adanya siklus bukan galat (tidak ada yang diterapkan).
func (j *Jembatan) Jalankan(ctx context.Context) (Ringkasan, error) {
	var r Ringkasan
	snap, k := j.sumber.Snapshot(ctx, "")
	if !k.Tersedia {
		if k.Alasan == usecase.AlasanBelumAdaSiklus {
			return r, nil
		}
		return r, fmt.Errorf("%w: %s", ErrSumberTidakTersedia, k.Alasan)
	}
	r.CycleID, r.Sinyal = snap.Siklus.CycleID, len(snap.Ews)

	sinyal := append([]domain.SinyalEws(nil), snap.Ews...)
	// KRITIS lebih dulu: bila putaran terpotong timeout, kondisi terparah sudah tertulis.
	sort.SliceStable(sinyal, func(a, b int) bool {
		return ewsdomain.SeverityRank(sinyal[a].Severity) > ewsdomain.SeverityRank(sinyal[b].Severity)
	})
	kunci := make([]string, len(sinyal))
	for i, s := range sinyal {
		kunci[i] = s.SinyalKunci
	}
	keadaan, err := j.keadaan.MuatKeadaan(ctx, kunci)
	if err != nil {
		return r, err
	}
	komoditas, kabupaten, err := j.keadaan.KodeMaster(ctx)
	if err != nil {
		return r, err
	}

	for _, s := range sinyal {
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		lama, ada := keadaan[s.SinyalKunci]
		if ada && lama.CycleID == snap.Siklus.CycleID {
			r.SudahDiterapkan++
			continue
		}
		if !s.MenjadiAlert() {
			j.mereda(ctx, snap.Siklus, s, lama, ada, &r)
			continue
		}
		if err := j.terapkan(ctx, snap.Siklus, s, lama, ada, komoditas, kabupaten, &r); err != nil {
			r.Galat++
			j.logWarn("ml: jembatan EWS gagal menerapkan sinyal", map[string]interface{}{"sinyal": s.SinyalKunci, "error": err.Error()})
		}
	}
	return r, nil
}

// mereda menangani sinyal yang tidak menjadi alert: sinyal pulih (aktif=false) atau INFO.
func (j *Jembatan) mereda(ctx context.Context, siklus domain.Siklus, s domain.SinyalEws, lama domain.EwsSinyalML, ada bool, r *Ringkasan) {
	if !ada {
		if s.Aktif {
			r.DiabaikanInfo++
		}
		return
	}
	if lama.Aktif && lama.AlertID != nil {
		catatan := fmt.Sprintf("Pipeline ML siklus %s: kondisi %s tidak lagi terdeteksi (tingkat sinyal kini %s). "+
			"Alert tidak ditutup otomatis; tutup setelah diverifikasi.", siklus.CycleID, s.SinyalKunci, s.Severity)
		ok, err := j.keadaan.CatatMereda(ctx, *lama.AlertID, catatan, j.now())
		if err != nil {
			r.Galat++
			j.logWarn("ml: jembatan EWS gagal mencatat kondisi mereda", map[string]interface{}{"sinyal": s.SinyalKunci, "error": err.Error()})
			return
		}
		if ok {
			r.Mereda++
		}
	} else if s.Aktif {
		r.DiabaikanInfo++
	}
	lama.Aktif, lama.Severity, lama.CycleID, lama.CycleTimestamp, lama.JenisML, lama.DiperbaruiPada =
		false, s.Severity, siklus.CycleID, siklus.CycleTimestamp, s.Jenis, j.now()
	if err := j.keadaan.SimpanKeadaan(ctx, lama); err != nil {
		r.Galat++
		j.logWarn("ml: jembatan EWS gagal menyimpan keadaan", map[string]interface{}{"sinyal": s.SinyalKunci, "error": err.Error()})
	}
}

func (j *Jembatan) terapkan(ctx context.Context, siklus domain.Siklus, s domain.SinyalEws, lama domain.EwsSinyalML, ada bool,
	komoditas, kabupaten map[string]bool, r *Ringkasan) error {
	keadaan := domain.EwsSinyalML{
		SinyalKunci: s.SinyalKunci, JenisML: s.Jenis, Severity: s.Severity, Aktif: true,
		CycleID: siklus.CycleID, CycleTimestamp: siklus.CycleTimestamp, DiperbaruiPada: j.now(),
	}
	if ada {
		keadaan.AlertID = lama.AlertID
	}
	in, err := AlertDari(s, siklus, komoditas, kabupaten)
	if err != nil {
		// Rujukan master yang tidak dikenal tidak akan berubah dalam siklus ini: catat sekali, coba lagi siklus depan.
		_ = j.keadaan.SimpanKeadaan(ctx, keadaan)
		return err
	}
	hasil, alert, err := j.penulis.Terapkan(ctx, in, j.now())
	if err != nil {
		return err
	}
	switch hasil {
	case ewsdomain.HasilDibuat:
		r.Dibuat++
	case ewsdomain.HasilDinaikkan:
		r.Dinaikkan++
	case ewsdomain.HasilDiredam:
		r.Diredam++
	default:
		r.Dilewati++
	}
	if alert != nil {
		keadaan.AlertID = &alert.ID
		j.catatAudit(ctx, hasil, *alert, in, siklus, s)
		for _, o := range j.observers {
			o.AlertDitulis(ctx, *alert, hasil)
		}
	} else if hasil == ewsdomain.HasilDilewati {
		if id, err := j.keadaan.AlertAktif(ctx, in.DedupKey); err == nil && id != nil {
			keadaan.AlertID = id
		}
	}
	return j.keadaan.SimpanKeadaan(ctx, keadaan)
}

func (j *Jembatan) catatAudit(ctx context.Context, hasil ewsdomain.HasilTulis, alert ewsdomain.Alert, in ewsdomain.NewAlert, siklus domain.Siklus, s domain.SinyalEws) {
	if j.audit == nil {
		return
	}
	aksi := "BUAT_OTOMATIS"
	if hasil == ewsdomain.HasilDinaikkan {
		aksi = "NAIK_TINGKAT"
	}
	err := j.audit.Write(ctx, audit.Entry{
		Aksi: aksi, Entitas: "ews_alert", EntitasID: alert.ID.String(), Hasil: audit.HasilBerhasil,
		Sesudah:  map[string]any{"severity": alert.Severity, "nilai": alert.Nilai, "dedup_key": in.DedupKey},
		Metadata: map[string]any{"sumber": ewsdomain.SumberPipelineML, "sinyal_kunci": s.SinyalKunci, "cycle_id": siklus.CycleID, "jenis_ml": s.Jenis},
	})
	if err != nil {
		j.logWarn("ml: jembatan EWS gagal mencatat audit", map[string]interface{}{"alert": alert.ID, "error": err.Error()})
	}
}

func (j *Jembatan) logWarn(pesan string, f map[string]interface{}) {
	if j.log != nil {
		j.log.Warn(pesan, f)
	}
}

// DedupKey adalah kunci dedup alert untuk sinyal_kunci (hash bila melebihi panjang kolom).
func DedupKey(sinyalKunci string) string {
	k := awalanDedup + sinyalKunci
	if len(k) <= maksDedup {
		return k
	}
	h := sha256.Sum256([]byte(sinyalKunci))
	return awalanDedup + "sha256:" + hex.EncodeToString(h[:])
}

// JenisAlert memetakan jenis sinyal pipeline ke enum jenis_ews yang sudah dikenal web. Anomali harga turun tidak
// dianggap over-supply (tidak ada dasar model untuk itu) dan dicatat sebagai ANOMALI_DATA.
func JenisAlert(s domain.SinyalEws) string {
	switch s.Jenis {
	case domain.SinyalAnomaliHarga:
		if strings.HasSuffix(s.SinyalKunci, ":TURUN") {
			return ewsdomain.JenisAnomaliData
		}
		return ewsdomain.JenisLonjakanHarga
	case domain.SinyalProyeksiLonjakan, domain.SinyalTekananInflasi:
		return ewsdomain.JenisLonjakanHarga
	}
	return ewsdomain.JenisAnomaliData
}

func satuanNilai(jenis string) string {
	switch jenis {
	case domain.SinyalTekananInflasi:
		return "skor"
	case domain.SinyalSumberData:
		return "hari"
	}
	return "%"
}

func potong(s string, maks int) string {
	if utf8.RuneCountInString(s) <= maks {
		return s
	}
	r := []rune(s)
	return string(r[:maks])
}

// skorDampak mengikuti pola rule engine: dasar per tingkat ditambah kedekatan dengan ambang (maks 19 poin).
func skorDampak(s domain.SinyalEws) int {
	dasar := map[string]int{ewsdomain.SeverityWaspada: 40, ewsdomain.SeveritySiaga: 60, ewsdomain.SeverityKritis: 80}[s.Severity]
	if s.Nilai == nil || s.Ambang == nil || *s.Ambang <= 0 {
		return dasar
	}
	return dasar + int(math.Max(0, math.Min(19, math.Abs(*s.Nilai) / *s.Ambang * 10)))
}

// AlertDari menyusun alert dari satu sinyal. Komoditas dan kabupaten harus ada di master (FK ews_alert); kode
// provinsi (2 digit) dan sinyal tanpa wilayah menjadi alert tanpa kabupaten.
func AlertDari(s domain.SinyalEws, siklus domain.Siklus, komoditas, kabupaten map[string]bool) (ewsdomain.NewAlert, error) {
	var kom *string
	if s.KomoditasKode != nil && *s.KomoditasKode != "" {
		if !komoditas[*s.KomoditasKode] {
			return ewsdomain.NewAlert{}, fmt.Errorf("komoditas %q tidak ada di master", *s.KomoditasKode)
		}
		k := *s.KomoditasKode
		kom = &k
	}
	kab, tingkat := "", "sistem"
	if s.KabupatenKode != nil && *s.KabupatenKode != "" {
		switch w := *s.KabupatenKode; {
		case len(w) <= 2:
			tingkat = "provinsi " + w
		case kabupaten[w]:
			kab, tingkat = w, ""
		default:
			return ewsdomain.NewAlert{}, fmt.Errorf("kabupaten %q tidak ada di master", w)
		}
	}
	nilai := 0.0
	if s.Nilai != nil {
		nilai = *s.Nilai
	}
	metode := ""
	if s.Metode != nil && *s.Metode != "" {
		metode = ", metode " + *s.Metode
	}
	pesan := strings.TrimSpace(s.Pesan)
	if tingkat != "" {
		pesan += " Tingkat " + tingkat + "."
	}
	pesan += fmt.Sprintf(" Sumber: pipeline ML siklus %s%s.", siklus.CycleID, metode)
	return ewsdomain.NewAlert{
		Jenis: JenisAlert(s), Severity: s.Severity, Judul: potong(s.Judul, maksJudul), Pesan: strings.TrimSpace(pesan),
		KomoditasKode: kom, KabupatenKode: kab, Nilai: nilai, SatuanNilai: satuanNilai(s.Jenis), SkorDampak: skorDampak(s),
		Sumber: ewsdomain.SumberPipelineML, DedupKey: DedupKey(s.SinyalKunci), CooldownJam: CooldownJam,
		CatatanRiwayat: fmt.Sprintf("Dibuat oleh jembatan pipeline ML dari sinyal %s (siklus %s)", s.SinyalKunci, siklus.CycleID),
	}, nil
}
