package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"gorm.io/gorm"
)

// FaktaStok membaca stok dan kapasitas dalam satu snapshot (dalamSnapshot), jadi utilisasi dan total tidak berasal
// dari dua keadaan basis data yang berbeda.
//
// Stok adalah lot yang belum HABIS (definisi modul cold-storage: AKTIF dan PRIORITAS_JUAL ada di gudang), dikelompokkan
// per komoditas x fasilitas x kelas lot (domain.KelasStok*). Kelas "berlaku" memakai aturan KPI cs (migrasi 000124):
// fasilitas OPERASIONAL, tanggal masuk <= hari ini dan tanggal kedaluwarsa > hari ini menurut waktu Aceh. Aturan
// tanggal itu ditulis sebagai perbandingan timestamp dengan tengah malam sesudah hari ini (domain.StokQuery.Batas),
// yang setara dan tetap bisa memakai indeks (cold_storage_id, kedaluwarsa_pada) yang sudah ada. Predikat cakupan
// jatuh pada fasilitas (cold_storage.kabupaten_kode), lalu lot dijangkau lewat idx_stok_cold_storage_fasilitas.
func (r *Repository) FaktaStok(ctx context.Context, q domain.StokQuery) (domain.FaktaStok, error) {
	var hasil domain.FaktaStok
	err := r.dalamSnapshot(ctx, func(tx *gorm.DB) error {
		b := &bangun{}
		b.tulis(`
			SELECT s.komoditas_kode, k.nama AS komoditas_nama, s.cold_storage_id::text AS fasilitas_id,
			       CASE WHEN c.status <> ? THEN ?::text
			            WHEN s.kedaluwarsa_pada < ? THEN ?::text
			            WHEN s.masuk_pada >= ? THEN ?::text
			            ELSE ?::text END AS kelas,
			       COUNT(*) AS jumlah_lot,
			       COALESCE(SUM(s.volume_ton), 0) AS volume_ton,
			       COALESCE(SUM(s.volume_ton) FILTER (WHERE s.kedaluwarsa_pada < ?), 0) AS mendekati_ton,
			       COALESCE(SUM(s.volume_ton) FILTER (WHERE s.risiko_food_loss > ?), 0) AS risiko_tinggi_ton
			  FROM stok_cold_storage s
			  JOIN cold_storage c ON c.id = s.cold_storage_id
			  JOIN komoditas k ON k.kode = s.komoditas_kode
			 WHERE s.status <> ? AND `,
			domain.StatusFasilitasOperasional, domain.KelasStokLuarOperasional,
			q.Batas, domain.KelasStokKedaluwarsa,
			q.Batas, domain.KelasStokBelumMasuk,
			domain.KelasStokBerlaku,
			q.BatasDekat, domain.AmbangRisikoTinggi, domain.StatusLotHabis)
		b.kondisi("c.kabupaten_kode", "s.komoditas_kode", q.Cakupan, q.KabupatenKode, q.KomoditasKode)
		// Posisi kolom (bukan ekspresi): CASE memuat parameter, dan PostgreSQL tidak menganggap dua ekspresi
		// berparameter sebagai ekspresi yang sama.
		b.tulis(` GROUP BY 1, 2, 3, 4 ORDER BY 1, 3, 4`)
		if err := tx.Raw(b.sql.String(), b.args...).Scan(&hasil.Baris).Error; err != nil {
			return fmt.Errorf("get stok: %w", err)
		}

		kap := &bangun{}
		kap.tulis(`
			SELECT COUNT(*) AS fasilitas, COALESCE(SUM(c.kapasitas_ton), 0) AS ton
			  FROM cold_storage c
			 WHERE c.status = ? AND `, domain.StatusFasilitasOperasional)
		kab, a := kondisiKabupaten("c.kabupaten_kode", q.Cakupan, q.KabupatenKode)
		kap.tulis(kab, a...)
		if err := tx.Raw(kap.sql.String(), kap.args...).Scan(&hasil.Kapasitas).Error; err != nil {
			return fmt.Errorf("get kapasitas stok: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.FaktaStok{}, err
	}
	return hasil, nil
}
