package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// AktivitasPetugas membaca aktivitas tiap mahasiswa agen aktif yang kabupaten tugasnya lolos cakupan dan filter.
// Satu pernyataan (satu snapshot). Biaya dijaga indeks yang ada:
//   - entri jendela 7 hari per petugas: idx_entri_harga_petugas_dicatat (petugas_id, dicatat_pada DESC);
//   - entri terakhir: pembacaan terbalik indeks yang sama (berhenti di baris pertama yang bukan DRAF);
//   - antrean DIAJUKAN: idx_entri_harga_antrean (parsial, hanya status DIAJUKAN) dibaca sekali untuk semua petugas,
//     bukan sekali per petugas (tidak ada indeks (petugas_id, status));
//   - tugas per petugas: idx_tugas_lapangan_petugas.
//
// Status efektif tugas direplikasi dari lapangan/domain.TugasView.StatusEfektif (SELESAI; lewat tenggat TERLAMBAT;
// sudah ada progres BERJALAN; selain itu status tersimpan). Tes integrasi membandingkan keduanya pada semua
// kombinasi, jadi pergeseran salah satunya langsung ketahuan.
func (r *Repository) AktivitasPetugas(ctx context.Context, q domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
	b := &bangun{}
	b.tulis(`
		WITH param AS (
			SELECT ?::timestamptz AS awal_hari, ?::timestamptz AS akhir_hari,
			       ?::timestamptz AS awal_jendela, ?::timestamptz AS sekarang
		),
		petugas AS (
			SELECT p.id, p.nama, p.kabupaten_kode, kb.nama AS kabupaten_nama
			  FROM pengguna p
			  JOIN kabupaten kb ON kb.kode = p.kabupaten_kode
			 WHERE p.peran = ? AND p.status = 'AKTIF' AND `,
		q.AwalHari, q.AkhirHari, q.AwalJendela, q.Sekarang, domain.PeranPetugas)
	kab, a := kondisiKabupaten("p.kabupaten_kode", q.Cakupan, q.KabupatenKode)
	b.tulis(kab, a...)
	b.tulis(`
		),
		antrean AS (
			SELECT e.petugas_id, COUNT(*) AS jumlah
			  FROM entri_harga e
			 WHERE e.status = ? AND e.petugas_id IN (SELECT id FROM petugas)
			 GROUP BY e.petugas_id
		)
		SELECT p.id::text AS id, p.nama, p.kabupaten_kode, p.kabupaten_nama,
		       COALESCE(en.entri_hari_ini, 0) AS entri_hari_ini,
		       (SELECT COUNT(*) FROM komoditas WHERE status = 'AKTIF') AS target_harian,
		       COALESCE(an.jumlah, 0) AS menunggu_validasi,
		       COALESCE(en.ditolak, 0) AS ditolak_jendela,
		       tr.dicatat_pada AS entri_terakhir_pada,
		       COALESCE(tg.aktif, 0) AS tugas_aktif,
		       COALESCE(tg.selesai, 0) AS tugas_selesai,
		       COALESCE(tg.terlambat, 0) AS tugas_terlambat
		  FROM petugas p
		  CROSS JOIN param pr
		  LEFT JOIN antrean an ON an.petugas_id = p.id
		  LEFT JOIN LATERAL (
		        SELECT COUNT(*) FILTER (WHERE e.tipe = ? AND e.status <> ? AND e.dicatat_pada >= pr.awal_hari) AS entri_hari_ini,
		               COUNT(*) FILTER (WHERE e.status = ?) AS ditolak
		          FROM entri_harga e
		         WHERE e.petugas_id = p.id AND e.dicatat_pada >= pr.awal_jendela AND e.dicatat_pada < pr.akhir_hari
		       ) en ON TRUE
		  LEFT JOIN LATERAL (
		        SELECT e.dicatat_pada
		          FROM entri_harga e
		         WHERE e.petugas_id = p.id AND e.status <> ?
		         ORDER BY e.dicatat_pada DESC
		         LIMIT 1
		       ) tr ON TRUE
		  LEFT JOIN LATERAL (
		        SELECT COUNT(*) FILTER (WHERE x.efektif IN (?, ?)) AS aktif,
		               COUNT(*) FILTER (WHERE x.efektif = ?) AS selesai,
		               COUNT(*) FILTER (WHERE x.efektif = ?) AS terlambat
		          FROM (SELECT CASE WHEN t.status = ? THEN ?::text
		                            WHEN t.tenggat < pr.sekarang THEN ?::text
		                            WHEN t.jumlah_selesai > 0 THEN ?::text
		                            ELSE t.status::text END AS efektif
		                  FROM tugas_lapangan t
		                 WHERE t.petugas_id = p.id AND t.tenggat >= pr.awal_jendela) x
		       ) tg ON TRUE
		 ORDER BY p.kabupaten_nama, p.nama, p.id`,
		domain.EntriDiajukan,
		domain.TipeKonsumen, domain.EntriDraf, domain.EntriDitolak,
		domain.EntriDraf,
		domain.TugasBelum, domain.TugasBerjalan, domain.TugasSelesai, domain.TugasTerlambat,
		domain.TugasSelesai, domain.TugasSelesai, domain.TugasTerlambat, domain.TugasBerjalan)

	var items []domain.AktivitasPetugas
	if err := r.db.WithContext(ctx).Raw(b.sql.String(), b.args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get aktivitas petugas: %w", err)
	}
	return items, nil
}
