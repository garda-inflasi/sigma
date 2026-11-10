package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var _ domain.EngineRepository = (*Repository)(nil)

// BatasKunciBawaan adalah lama maksimum menunggu kunci evaluasi (evaluasi lain yang sedang berjalan) maupun kunci baris
// siklus (penulis di luar mesin). Satu putaran normal selesai dalam milidetik; batas ini hanya pagar bila ada yang macet.
const BatasKunciBawaan = 15 * time.Second

// kodeKunciHabis adalah SQLSTATE lock_not_available (lock_timeout terlampaui).
const kodeKunciHabis = "55P03"

// kunciEvaluasi adalah nama kunci advisory satu wilayah; dipakai hashtextextended seperti kunci job (internal/platform/jobs).
func kunciEvaluasi(wilayahKode string) string { return "garda.siklus.evaluasi." + wilayahKode }

// Dalam menjalankan fn dalam satu transaksi yang memegang kunci advisory evaluasi wilayah. Kunci transaksi dilepas
// otomatis saat commit, rollback, atau bila proses mati, jadi tidak ada kunci yatim.
//
// Batas tunggu (lock_timeout) berlaku untuk kunci evaluasi maupun kunci baris siklus (mis. penulis di luar mesin yang
// menahan baris aktif); keduanya yang terlampaui dilaporkan sebagai ErrEvaluasiSibuk.
func (r *Repository) Dalam(ctx context.Context, wilayahKode string, fn func(domain.Sesi) error) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf("SET LOCAL lock_timeout = %d", r.batasKunci.Milliseconds())).Error; err != nil {
			return fmt.Errorf("atur batas kunci: %w", err)
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", kunciEvaluasi(wilayahKode)).Error; err != nil {
			return fmt.Errorf("kunci evaluasi siklus: %w", err)
		}
		return fn(&sesi{tx: tx, wilayah: wilayahKode})
	})
	if adalahKunciHabis(err) {
		return domain.ErrEvaluasiSibuk
	}
	return err
}

func adalahKunciHabis(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == kodeKunciHabis
}

// sesi adalah akses data di dalam satu transaksi evaluasi untuk satu wilayah.
type sesi struct {
	tx      *gorm.DB
	wilayah string
}

var _ domain.Sesi = (*sesi)(nil)

func (s *sesi) db(ctx context.Context) *gorm.DB { return s.tx.WithContext(ctx) }

// kolomSiklus adalah daftar kolom siklus_ekonomi untuk entity; enum dan jsonb dibaca sebagai teks.
const kolomSiklus = `id, kode, wilayah_kode, siklus::text AS siklus, mulai_pada, akhir_pada, dibuat_pada, otomatis, alasan::text AS alasan, pulih_sejak`

func (s *sesi) WilayahAda(ctx context.Context) (bool, error) {
	var ada bool
	if err := s.db(ctx).Raw("SELECT EXISTS (SELECT 1 FROM provinsi WHERE kode = ?)", s.wilayah).Scan(&ada).Error; err != nil {
		return false, err
	}
	return ada, nil
}

// Sekarang memakai clock_timestamp(), bukan now(): now() adalah awal transaksi, yang bisa lebih tua daripada
// mulai_pada siklus yang baru saja ditulis evaluasi lain selagi transaksi ini menunggu kunci.
func (s *sesi) Sekarang(ctx context.Context) (time.Time, error) {
	var t time.Time
	if err := s.db(ctx).Raw("SELECT clock_timestamp()").Scan(&t).Error; err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func (s *sesi) AturanAktif(ctx context.Context) ([]domain.AturanSiklus, error) {
	var items []domain.AturanSiklus
	if err := s.db(ctx).Where("aktif").Order(urutanAturan).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *sesi) SiklusAktif(ctx context.Context) (*domain.SiklusEkonomi, error) {
	var rows []domain.SiklusEkonomi
	err := s.db(ctx).Raw(`SELECT `+kolomSiklus+` FROM siklus_ekonomi
		WHERE wilayah_kode = ? AND akhir_pada IS NULL ORDER BY mulai_pada DESC LIMIT 1 FOR UPDATE`, s.wilayah).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *sesi) CatatPulih(ctx context.Context, id idx.ID, sejak *time.Time) error {
	return s.db(ctx).Exec("UPDATE siklus_ekonomi SET pulih_sejak = ? WHERE id = ? AND akhir_pada IS NULL", sejak, id).Error
}

// Ganti menutup siklus lama dan membuka yang baru pada instan yang sama (tanpa celah dan tanpa tumpang tindih). Instan
// itu tidak pernah mendahului mulai_pada siklus lama: bila jam basis data mundur atau dua pergantian jatuh pada
// mikrodetik yang sama, instan digeser satu mikrodetik di atas mulai_pada lama, sehingga CHECK akhir_pada > mulai_pada
// tidak pernah dilanggar.
func (s *sesi) Ganti(ctx context.Context, lama *domain.SiklusEkonomi, baru domain.SiklusBaru, sekarang time.Time) (*domain.SiklusEkonomi, error) {
	instan := sekarang
	if lama != nil {
		if !instan.After(lama.MulaiPada) {
			instan = lama.MulaiPada.Add(time.Microsecond)
		}
		res := s.db(ctx).Exec("UPDATE siklus_ekonomi SET akhir_pada = ?, pulih_sejak = NULL WHERE id = ? AND akhir_pada IS NULL", instan, lama.ID)
		if res.Error != nil {
			return nil, fmt.Errorf("tutup siklus %s: %w", lama.ID, res.Error)
		}
		if res.RowsAffected != 1 {
			return nil, domain.ErrBentrokSiklus // sudah ditutup penulis lain
		}
	}

	alasan, err := json.Marshal(baru.Alasan)
	if err != nil {
		return nil, fmt.Errorf("susun alasan: %w", err)
	}
	var rows []domain.SiklusEkonomi
	err = s.db(ctx).Raw(`INSERT INTO siklus_ekonomi (wilayah_kode, siklus, mulai_pada, otomatis, alasan)
		VALUES (?, CAST(? AS siklus), ?, true, CAST(? AS jsonb))
		RETURNING `+kolomSiklus, baru.WilayahKode, baru.Siklus, instan, string(alasan)).Scan(&rows).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil, domain.ErrBentrokSiklus // penulis lain membuka siklus aktif lebih dulu (uq_siklus_ekonomi_aktif)
	}
	if err != nil {
		return nil, fmt.Errorf("buka siklus %s: %w", baru.Siklus, err)
	}
	if len(rows) != 1 {
		return nil, errors.New("buka siklus: tidak ada baris yang dikembalikan")
	}
	return &rows[0], nil
}

func (s *sesi) Ukur(ctx context.Context, aturan domain.AturanSiklus, sekarang time.Time) (domain.Pengukuran, error) {
	switch aturan.Indikator {
	case domain.IndikatorAlertEws:
		return s.ukurAlert(ctx, aturan)
	case domain.IndikatorUtilisasiStok:
		return s.ukurUtilisasi(ctx, sekarang)
	}
	return domain.Pengukuran{}, fmt.Errorf("indikator %q belum didukung mesin siklus", aturan.Indikator)
}

// alertSQL menghitung alert yang belum SELESAI (BARU, DIAKUI, DIESKALASI: kondisinya masih dianggap berlangsung)
// di kabupaten milik provinsi, menurut jenis dan tingkat minimum (enum severity berurut INFO < WASPADA < SIAGA <
// KRITIS). Jumlah total ikut di tiap baris (jendela, dihitung sebelum LIMIT); tanpa baris berarti nol.
const alertSQL = `
SELECT count(*) OVER () AS jumlah, COALESCE(a.kode, a.id::text) AS rujukan
  FROM ews_alert a
  JOIN kabupaten k ON k.kode = a.kabupaten_kode
 WHERE k.provinsi_kode = @wilayah
   AND a.jenis = CAST(@jenis AS jenis_ews)
   AND a.status <> 'SELESAI'
   AND a.severity >= CAST(@minimal AS severity)
 ORDER BY a.severity DESC, a.dibuat_pada DESC, a.id DESC
 LIMIT 5`

func (s *sesi) ukurAlert(ctx context.Context, a domain.AturanSiklus) (domain.Pengukuran, error) {
	if a.JenisEws == nil || a.SeverityMin == nil {
		return domain.Pengukuran{}, errors.New("aturan ALERT_EWS tanpa jenis_ews atau severity_min")
	}
	var rows []struct {
		Jumlah  int
		Rujukan string
	}
	err := s.db(ctx).Raw(alertSQL, map[string]any{"wilayah": s.wilayah, "jenis": *a.JenisEws, "minimal": *a.SeverityMin}).Scan(&rows).Error
	if err != nil {
		return domain.Pengukuran{}, err
	}
	p := domain.Pengukuran{Terukur: true}
	for _, row := range rows {
		p.Nilai = float64(row.Jumlah)
		p.Rujukan = append(p.Rujukan, row.Rujukan)
	}
	return p, nil
}

// stokSQL menjumlahkan kapasitas fasilitas OPERASIONAL di provinsi dan volume lot layak simpan di dalamnya (sudah
// masuk, belum kedaluwarsa, belum HABIS), definisi yang sama dengan KPI utilisasi dasbor (mv_kpi_operasional "cs") dan
// stok layak simpan pada rule engine EWS.
const stokSQL = `
WITH fasilitas AS (
	SELECT c.id, c.kapasitas_ton
	  FROM cold_storage c
	  JOIN kabupaten k ON k.kode = c.kabupaten_kode
	 WHERE k.provinsi_kode = @wilayah AND c.status = 'OPERASIONAL'
)
SELECT (SELECT COALESCE(SUM(kapasitas_ton), 0) FROM fasilitas) AS kapasitas_ton,
       (SELECT COALESCE(SUM(s.volume_ton), 0)
          FROM stok_cold_storage s JOIN fasilitas f ON f.id = s.cold_storage_id
         WHERE s.status <> 'HABIS' AND s.masuk_pada <= @sekarang AND s.kedaluwarsa_pada > @sekarang) AS terisi_ton`

func (s *sesi) ukurUtilisasi(ctx context.Context, sekarang time.Time) (domain.Pengukuran, error) {
	var row struct {
		KapasitasTon float64
		TerisiTon    float64
	}
	if err := s.db(ctx).Raw(stokSQL, map[string]any{"wilayah": s.wilayah, "sekarang": sekarang}).Scan(&row).Error; err != nil {
		return domain.Pengukuran{}, err
	}
	if row.KapasitasTon <= 0 {
		return domain.Pengukuran{Rincian: "tidak ada fasilitas cold storage operasional"}, nil
	}
	// Kali 100 dulu, baru bagi: rasio bulat (mis. 450 dari 500 ton) menghasilkan persen bulat tanpa galat pecahan,
	// sehingga ambang tepat di batas memicu seperti yang tertulis.
	return domain.Pengukuran{
		Terukur: true,
		Nilai:   100 * row.TerisiTon / row.KapasitasTon,
		Rincian: fmt.Sprintf("%s dari %s ton", domain.FormatAngka(row.TerisiTon), domain.FormatAngka(row.KapasitasTon)),
	}, nil
}
