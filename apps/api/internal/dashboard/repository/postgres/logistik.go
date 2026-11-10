package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"gorm.io/gorm"
)

// barisRute adalah satu baris rute beserta total semua rute aktif dalam cakupan (fungsi jendela dihitung sebelum LIMIT,
// jadi total tidak ikut terpotong oleh batas jumlah item).
type barisRute struct {
	domain.RuteAktif
	TotalJumlah int     `gorm:"column:total_jumlah"`
	TotalVolume float64 `gorm:"column:total_volume"`
	TotalBiaya  int64   `gorm:"column:total_biaya"`
}

// Logistik membaca tiga hal dalam satu snapshot (dalamSnapshot): tonase transaksi stok per komoditas dan jenis,
// transaksi terbaru, dan rute distribusi aktif.
//
// Cakupan transaksi: fasilitas asal ATAU tujuan di kabupaten efektif (kondisiAsalAtauTujuan); kabupaten diambil dari
// fasilitasnya, bukan dari pelaku. Jendela [Awal, Akhir) memakai batas timestamp pada transaksi_stok.waktu. Jenis
// transaksi tidak disaring: MASUK, KELUAR, dan PINDAH semuanya dihitung satu kali (PINDAH tidak dihitung ganda sebagai
// keluar dan masuk).
//
// Rute aktif: status selain DIGANTIKAN (aturan modul spasial, partial index idx_rute_distribusi_aktif), diurutkan menurut
// biaya menurun lalu ID seperti daftar rute spasial. Rute menghubungkan kabupaten, jadi cakupannya kabupaten asal ATAU
// tujuan.
func (r *Repository) Logistik(ctx context.Context, q domain.LogistikQuery) (domain.LogistikFakta, error) {
	var hasil domain.LogistikFakta
	err := r.dalamSnapshot(ctx, func(tx *gorm.DB) error {
		// Ringkasan per komoditas dan jenis.
		agregat := &bangun{}
		agregat.tulis(`
			SELECT s.komoditas_kode, k.nama AS komoditas_nama, t.jenis,
			       COUNT(*) AS jumlah, COALESCE(SUM(t.volume_ton), 0) AS volume_ton
			  FROM transaksi_stok t
			  JOIN stok_cold_storage s ON s.id = t.lot_id
			  JOIN komoditas k ON k.kode = s.komoditas_kode
			  LEFT JOIN cold_storage fa ON fa.id = t.fasilitas_asal_id
			  LEFT JOIN cold_storage ft ON ft.id = t.fasilitas_tujuan_id
			 WHERE t.waktu >= ? AND t.waktu < ? AND `, q.Awal, q.Akhir)
		tulisSyaratPergerakan(agregat, q)
		agregat.tulis(` GROUP BY s.komoditas_kode, k.nama, t.jenis ORDER BY s.komoditas_kode, t.jenis`)
		if err := tx.Raw(agregat.sql.String(), agregat.args...).Scan(&hasil.Agregat).Error; err != nil {
			return fmt.Errorf("get ringkasan pergerakan: %w", err)
		}

		// Pergerakan terbaru, tanpa pelaku dan alasan.
		terbaru := &bangun{}
		terbaru.tulis(`
			SELECT t.id, t.waktu, t.jenis, t.volume_ton, s.komoditas_kode, k.nama AS komoditas_nama,
			       fa.id::text AS asal_id, fa.kode AS asal_kode, fa.nama AS asal_nama,
			       fa.kabupaten_kode AS asal_kabupaten_kode, ka.nama AS asal_kabupaten_nama,
			       ft.id::text AS tujuan_id, ft.kode AS tujuan_kode, ft.nama AS tujuan_nama,
			       ft.kabupaten_kode AS tujuan_kabupaten_kode, kt.nama AS tujuan_kabupaten_nama
			  FROM transaksi_stok t
			  JOIN stok_cold_storage s ON s.id = t.lot_id
			  JOIN komoditas k ON k.kode = s.komoditas_kode
			  LEFT JOIN cold_storage fa ON fa.id = t.fasilitas_asal_id
			  LEFT JOIN kabupaten ka ON ka.kode = fa.kabupaten_kode
			  LEFT JOIN cold_storage ft ON ft.id = t.fasilitas_tujuan_id
			  LEFT JOIN kabupaten kt ON kt.kode = ft.kabupaten_kode
			 WHERE t.waktu >= ? AND t.waktu < ? AND `, q.Awal, q.Akhir)
		tulisSyaratPergerakan(terbaru, q)
		terbaru.tulis(` ORDER BY t.waktu DESC, t.id DESC LIMIT ?`, q.MaksTerbaru)
		if err := tx.Raw(terbaru.sql.String(), terbaru.args...).Scan(&hasil.Terbaru).Error; err != nil {
			return fmt.Errorf("get pergerakan terbaru: %w", err)
		}

		// Rute aktif.
		rute := &bangun{}
		rute.tulis(`
			SELECT r.id, r.kode, r.asal_kode, ka.nama AS asal, r.tujuan_kode, kt.nama AS tujuan,
			       r.komoditas_kode, k.nama AS komoditas_nama, r.volume_ton::float8 AS volume_ton, r.jarak_km,
			       r.jam_tempuh::float8 AS jam_tempuh, r.biaya, r.status, r.sumber, r.dibuat_pada,
			       (COUNT(*) OVER ())::int AS total_jumlah,
			       COALESCE(SUM(r.volume_ton) OVER (), 0)::float8 AS total_volume,
			       COALESCE(SUM(r.biaya) OVER (), 0)::bigint AS total_biaya
			  FROM rute_distribusi r
			  JOIN kabupaten ka ON ka.kode = r.asal_kode
			  JOIN kabupaten kt ON kt.kode = r.tujuan_kode
			  JOIN komoditas k ON k.kode = r.komoditas_kode
			 WHERE r.status <> ? AND `, domain.StatusRuteDigantikan)
		kab, a := kondisiAsalAtauTujuan("r.asal_kode", "r.tujuan_kode", q.Cakupan, q.KabupatenKode)
		rute.tulis(kab, a...)
		rute.tulis(" AND ")
		kom, kk := kondisiKomoditas("r.komoditas_kode", q.KomoditasKode)
		rute.tulis(kom, kk...)
		rute.tulis(` ORDER BY r.biaya DESC, r.id LIMIT ?`, q.MaksRute)
		var baris []barisRute
		if err := tx.Raw(rute.sql.String(), rute.args...).Scan(&baris).Error; err != nil {
			return fmt.Errorf("get rute aktif: %w", err)
		}
		if len(baris) > 0 {
			hasil.Rute.Jumlah, hasil.Rute.VolumeTon, hasil.Rute.Biaya = baris[0].TotalJumlah, baris[0].TotalVolume, baris[0].TotalBiaya
		}
		hasil.Rute.Items = make([]domain.RuteAktif, len(baris))
		for i, br := range baris {
			hasil.Rute.Items[i] = br.RuteAktif
		}
		return nil
	})
	if err != nil {
		return domain.LogistikFakta{}, err
	}
	return hasil, nil
}

// tulisSyaratPergerakan menulis syarat cakupan (asal atau tujuan), filter komoditas lot, dan tidak ada yang lain;
// ujung WHERE pergerakan, setelah syarat jendela waktu.
func tulisSyaratPergerakan(b *bangun, q domain.LogistikQuery) {
	kab, a := kondisiAsalAtauTujuan("fa.kabupaten_kode", "ft.kabupaten_kode", q.Cakupan, q.KabupatenKode)
	b.tulis(kab, a...)
	b.tulis(" AND ")
	kom, k := kondisiKomoditas("s.komoditas_kode", q.KomoditasKode)
	b.tulis(kom, k...)
}
