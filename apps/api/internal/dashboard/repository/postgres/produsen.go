package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// Produsen membaca harga produsen dan konsumen pembanding dari agregat_harga_harian (hanya entri DIVALIDASI dan
// DIPUBLIKASI, sudah per tanggal Aceh, komoditas, kabupaten, dan tipe) serta produksi dari neraca_pangan, dalam satu
// pernyataan.
//
// Harga: rata-rata TERTIMBANG jumlah entri sepanjang jendela (SUM(rata_rata x jumlah) / SUM(jumlah)), yaitu rata-rata
// entri yang sebenarnya, bukan rata-rata rata-rata harian yang memberi bobot sama pada hari sepi dan hari ramai. Harga
// konsumen hanya dari kabupaten yang juga punya harga produsen komoditas itu (lihat domain.ProdusenFakta). Indeks
// idx_agregat_harga_harian_tanggal membatasi bacaan pada tujuh hari; kabupaten dalam cakupan disaring di agregat.
//
// Produksi: SATU wilayah saja. Wilayah neraca_pangan berupa provinsi (2 digit) atau kabupaten (4 digit); menjumlah
// keduanya menghitung produksi dua kali, jadi tampilan seluruh Aceh tanpa filter kabupaten memakai baris provinsi
// (domain.WilayahProvinsi) dan tampilan satu kabupaten memakai baris kabupaten itu; tanpa baris, produksinya kosong
// (bukan nol dan bukan jumlah kabupaten yang mungkin tidak lengkap). Periode dan sumber dipilih dengan aturan yang
// sama dengan dashboard eksekutif (eksekutif/repository NeracaTerbaru): angka resmi lebih diutamakan daripada DEMO,
// lalu periode terbaru milik wilayah itu, sehingga angka yang sama tidak berbeda antar dashboard.
func (r *Repository) Produsen(ctx context.Context, q domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
	kabupaten, ada := q.Cakupan.Irisi(q.KabupatenKode)
	if !ada {
		return nil, nil
	}
	wilayah := kabupaten
	if wilayah == "" {
		wilayah = domain.WilayahProvinsi
	}
	dari, sampai := tanggalSQL(q.Dari), tanggalSQL(q.Sampai)

	b := &bangun{}
	b.tulis(`
		WITH param AS (SELECT ?::text AS wilayah, ?::text AS demo),
		produsen AS (
			SELECT komoditas_kode, kabupaten_kode, SUM(rata_rata * jumlah) AS nilai, SUM(jumlah) AS n
			  FROM agregat_harga_harian
			 WHERE tipe = ? AND tanggal BETWEEN ?::date AND ?::date AND `,
		wilayah, domain.SumberNeracaDemo, domain.TipeProdusen, dari, sampai)
	b.kondisi("kabupaten_kode", "komoditas_kode", q.Cakupan, q.KabupatenKode, q.KomoditasKode)
	b.tulis(`
			 GROUP BY komoditas_kode, kabupaten_kode
		),
		konsumen AS (
			SELECT a.komoditas_kode, a.kabupaten_kode, SUM(a.rata_rata * a.jumlah) AS nilai, SUM(a.jumlah) AS n
			  FROM agregat_harga_harian a
			 WHERE a.tipe = ? AND a.tanggal BETWEEN ?::date AND ?::date AND `,
		domain.TipeKonsumen, dari, sampai)
	b.kondisi("a.kabupaten_kode", "a.komoditas_kode", q.Cakupan, q.KabupatenKode, q.KomoditasKode)
	b.tulis(`
			   AND EXISTS (SELECT 1 FROM produsen p
			                WHERE p.komoditas_kode = a.komoditas_kode AND p.kabupaten_kode = a.kabupaten_kode)
			 GROUP BY a.komoditas_kode, a.kabupaten_kode
		),
		sisi_produsen AS (
			SELECT komoditas_kode, SUM(nilai) / SUM(n) AS harga, SUM(n)::int AS entri, COUNT(*)::int AS kabupaten
			  FROM produsen GROUP BY komoditas_kode
		),
		sisi_konsumen AS (
			SELECT komoditas_kode, SUM(nilai) / SUM(n) AS harga, SUM(n)::int AS entri, COUNT(*)::int AS kabupaten
			  FROM konsumen GROUP BY komoditas_kode
		),
		pilih AS (
			SELECT (n.sumber = pr.demo) AS demo, n.periode
			  FROM neraca_pangan n, param pr
			 WHERE n.wilayah_kode = pr.wilayah
			 ORDER BY (n.sumber = pr.demo), n.periode DESC
			 LIMIT 1
		),
		produksi AS (
			SELECT n.komoditas_kode, n.produksi_ton, n.periode, n.sumber
			  FROM neraca_pangan n, param pr, pilih
			 WHERE n.wilayah_kode = pr.wilayah AND (n.sumber = pr.demo) = pilih.demo AND n.periode = pilih.periode AND `)
	kom, k := kondisiKomoditas("n.komoditas_kode", q.KomoditasKode)
	b.tulis(kom, k...)
	b.tulis(`
		)
		SELECT k.kode AS komoditas_kode, k.nama AS komoditas_nama, k.satuan, k.pilot,
		       sp.harga::float8 AS harga_produsen, COALESCE(sp.entri, 0) AS entri_produsen, COALESCE(sp.kabupaten, 0) AS kab_produsen,
		       sk.harga::float8 AS harga_konsumen, COALESCE(sk.entri, 0) AS entri_konsumen, COALESCE(sk.kabupaten, 0) AS kab_konsumen,
		       pd.produksi_ton::float8 AS produksi_ton, pd.periode AS produksi_periode, pd.sumber AS produksi_sumber
		  FROM komoditas k
		  LEFT JOIN sisi_produsen sp ON sp.komoditas_kode = k.kode
		  LEFT JOIN sisi_konsumen sk ON sk.komoditas_kode = k.kode
		  LEFT JOIN produksi pd ON pd.komoditas_kode = k.kode
		 WHERE sp.komoditas_kode IS NOT NULL OR pd.komoditas_kode IS NOT NULL
		 ORDER BY k.pilot DESC, k.nama, k.kode`)

	var items []domain.ProdusenFakta
	if err := r.db.WithContext(ctx).Raw(b.sql.String(), b.args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get produsen: %w", err)
	}
	return items, nil
}
