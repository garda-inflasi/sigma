package postgres

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

const (
	// wilayahProvinsi adalah kode BPS provinsi Aceh pada tabel inflasi_bulanan/andil_inflasi.
	wilayahProvinsi = "11"
	// kabupatenProyeksi dan horizonProyeksi: proyeksi yang diringkas adalah Kota Banda Aceh, 14 hari
	// (bawaan halaman prediksi).
	kabupatenProyeksi = "1171"
	horizonProyeksi   = 14
	// maksAlert adalah jumlah peringatan terparah yang dibawa ke konteks.
	maksAlert = 3
)

// Kumpulkan membaca data nyata untuk brief periode (awal bulan), apa adanya tanpa narasi.
// Bagian yang datanya tidak ada dibiarkan nil/kosong.
func (r *Repository) Kumpulkan(ctx context.Context, periode time.Time) (*domain.DataPeriode, error) {
	awal := time.Date(periode.Year(), periode.Month(), 1, 0, 0, 0, 0, time.UTC)
	data := &domain.DataPeriode{Periode: awal}

	steps := []struct {
		nama  string
		jalan func(context.Context, time.Time, *domain.DataPeriode) error
	}{
		{"inflasi", r.inflasi},
		{"harga", r.harga},
		{"peringatan dini", r.ews},
		{"proyeksi", r.proyeksi},
	}
	for _, s := range steps {
		if err := s.jalan(ctx, awal, data); err != nil {
			return nil, fmt.Errorf("%s: %w", s.nama, err)
		}
	}
	return data, nil
}

// inflasi membaca inflasi provinsi pada bulan terbaru yang tidak melewati periode, sasaran tahunnya,
// dan komoditas penyumbang terbesar bulan itu.
func (r *Repository) inflasi(ctx context.Context, awal time.Time, data *domain.DataPeriode) error {
	var rows []struct {
		Periode time.Time
		Mtm     float64
		Yoy     float64
		Ytd     float64
		Sumber  string
	}
	err := r.db.WithContext(ctx).Table("inflasi_bulanan").
		Select("periode, inflasi_mtm AS mtm, inflasi_yoy AS yoy, inflasi_ytd AS ytd, sumber").
		Where("wilayah_kode = ? AND periode <= ?", wilayahProvinsi, awal).
		Order("periode DESC").Limit(1).Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("baca inflasi: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	in := rows[0]
	data.Inflasi = &domain.InflasiData{Periode: in.Periode, Mtm: in.Mtm, Yoy: in.Yoy, Ytd: in.Ytd, Sumber: in.Sumber}

	var targets []domain.TargetData
	err = r.db.WithContext(ctx).Table("target_inflasi").
		Select("tahun, batas_bawah AS bawah, batas_atas AS atas").
		Where("tahun = ?", in.Periode.Year()).Scan(&targets).Error
	if err != nil {
		return fmt.Errorf("baca sasaran inflasi: %w", err)
	}
	if len(targets) > 0 {
		data.Target = &targets[0]
	}

	err = r.db.WithContext(ctx).Table("andil_inflasi AS a").
		Select("k.nama AS komoditas, a.andil, a.sumber").
		Joins("JOIN komoditas k ON k.kode = a.komoditas_kode").
		Where("a.wilayah_kode = ? AND a.periode = ? AND a.andil > 0", wilayahProvinsi, in.Periode).
		Order("a.andil DESC, k.nama").Limit(3).Scan(&data.Andil).Error
	if err != nil {
		return fmt.Errorf("baca andil inflasi: %w", err)
	}
	return nil
}

// harga membandingkan rata-rata harga konsumen tervalidasi bulan brief dengan bulan sebelumnya per komoditas.
func (r *Repository) harga(ctx context.Context, awal time.Time, data *domain.DataPeriode) error {
	lalu, akhir := awal.AddDate(0, -1, 0), awal.AddDate(0, 1, 0)
	var rows []struct {
		Komoditas string
		RataIni   *int
		RataLalu  *int
	}
	err := r.db.WithContext(ctx).Table("entri_harga AS e").
		Select(`k.nama AS komoditas,
			ROUND(AVG(e.harga) FILTER (WHERE e.dicatat_pada >= ? AND e.dicatat_pada < ?))::int AS rata_ini,
			ROUND(AVG(e.harga) FILTER (WHERE e.dicatat_pada >= ? AND e.dicatat_pada < ?))::int AS rata_lalu`,
			awal, akhir, lalu, awal).
		Joins("JOIN komoditas k ON k.kode = e.komoditas_kode").
		Where("e.tipe = 'KONSUMEN' AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')").
		Where("e.dicatat_pada >= ? AND e.dicatat_pada < ?", lalu, akhir).
		Group("k.nama").Order("k.nama").Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("baca harga: %w", err)
	}
	for _, row := range rows {
		if row.RataIni == nil || row.RataLalu == nil || *row.RataLalu <= 0 {
			continue
		}
		pct := float64(*row.RataIni-*row.RataLalu) / float64(*row.RataLalu) * 100
		data.Harga = append(data.Harga, domain.HargaData{Komoditas: row.Komoditas, RataIni: *row.RataIni, RataLalu: *row.RataLalu, PerubahanPct: pct})
	}
	return nil
}

// ews merangkum peringatan dini yang dibuat pada bulan brief.
func (r *Repository) ews(ctx context.Context, awal time.Time, data *domain.DataPeriode) error {
	akhir := awal.AddDate(0, 1, 0)
	var hitung []struct {
		Severity string
		Jumlah   int
	}
	err := r.db.WithContext(ctx).Table("ews_alert").
		Select("severity::text AS severity, COUNT(*) AS jumlah").
		Where("dibuat_pada >= ? AND dibuat_pada < ?", awal, akhir).Group("severity").Scan(&hitung).Error
	if err != nil {
		return fmt.Errorf("hitung peringatan: %w", err)
	}
	for _, h := range hitung {
		data.Ews.Total += h.Jumlah
		switch h.Severity {
		case "KRITIS":
			data.Ews.Kritis = h.Jumlah
		case "SIAGA":
			data.Ews.Siaga = h.Jumlah
		case "WASPADA":
			data.Ews.Waspada = h.Jumlah
		}
	}
	if data.Ews.Total == 0 {
		return nil
	}

	err = r.db.WithContext(ctx).Table("ews_alert AS a").
		Select("COALESCE(k.nama, '') AS komoditas, kb.nama AS wilayah, a.severity::text AS level").
		Joins("JOIN kabupaten kb ON kb.kode = a.kabupaten_kode").
		Joins("LEFT JOIN komoditas k ON k.kode = a.komoditas_kode").
		Where("a.dibuat_pada >= ? AND a.dibuat_pada < ?", awal, akhir).
		Order("CASE a.severity WHEN 'KRITIS' THEN 4 WHEN 'SIAGA' THEN 3 WHEN 'WASPADA' THEN 2 ELSE 1 END DESC, a.skor_dampak DESC, a.id").
		Limit(maksAlert).Scan(&data.Ews.Teratas).Error
	if err != nil {
		return fmt.Errorf("baca peringatan teratas: %w", err)
	}
	return nil
}

// proyeksi membaca proyeksi terbaru model PRODUKSI tiap komoditas untuk Kota Banda Aceh pada horizon bawaan.
func (r *Repository) proyeksi(ctx context.Context, _ time.Time, data *domain.DataPeriode) error {
	var rows []struct {
		Komoditas  string
		Nilai      int
		HargaDasar int
		Sumber     string
	}
	err := r.db.WithContext(ctx).Table("prediksi_harga AS p").
		Select("DISTINCT ON (p.komoditas_kode) k.nama AS komoditas, p.nilai, p.harga_dasar, p.sumber").
		Joins("JOIN komoditas k ON k.kode = p.komoditas_kode").
		Joins("JOIN model_registry m ON m.id = p.model_id").
		Where("m.status = 'PRODUKSI' AND p.kabupaten_kode = ? AND p.horizon_hari = ?", kabupatenProyeksi, horizonProyeksi).
		Order("p.komoditas_kode, p.tanggal_target DESC").Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("baca proyeksi: %w", err)
	}
	for _, row := range rows {
		pct := float64(row.Nilai-row.HargaDasar) / float64(row.HargaDasar) * 100
		data.Proyeksi = append(data.Proyeksi, domain.ProyeksiData{Komoditas: row.Komoditas, HorizonHari: horizonProyeksi, PerubahanPct: pct, Sumber: row.Sumber})
	}
	// Urutan tetap (|perubahan| terbesar dulu, lalu nama) agar konteks deterministik.
	sort.SliceStable(data.Proyeksi, func(i, j int) bool {
		a, b := math.Abs(data.Proyeksi[i].PerubahanPct), math.Abs(data.Proyeksi[j].PerubahanPct)
		if a != b {
			return a > b
		}
		return data.Proyeksi[i].Komoditas < data.Proyeksi[j].Komoditas
	})
	return nil
}
