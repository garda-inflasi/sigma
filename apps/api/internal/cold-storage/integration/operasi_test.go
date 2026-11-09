package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fakta seed yang dipakai uji (db/seeds): kapasitas dan
// stok awal CS-01 400/347 ton, CS-02 300/240, CS-03 250/193, CS-04 150/33
// (PERAWATAN), CS-05 120/0 (RENCANA).

func post(t *testing.T, s *suite, path, email string, body any) (int, map[string]any) {
	t.Helper()
	w := s.call(t, http.MethodPost, path, email, body)
	return w.Code, bodyOf(t, w)
}

func dataMap(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	d, ok := body["data"].(map[string]any)
	require.True(t, ok, "data bukan objek: %v", body)
	return d
}

// volumeLot membaca lot menurut ID angka (string desimal) atau kode seed (STK-CS01-05) dan mengembalikan volume,
// kode fasilitasnya (CS-03), dan status.
func volumeLot(t *testing.T, s *suite, ref string) (volume float64, fasilitas, status string) {
	t.Helper()
	var row struct {
		VolumeTon float64
		Kode      string
		Status    string
	}
	require.NoError(t, s.db.Raw(`SELECT s.volume_ton, c.kode, s.status FROM stok_cold_storage s JOIN cold_storage c ON c.id = s.cold_storage_id
		WHERE s.id::text = ? OR s.kode = ?`, ref, ref).Scan(&row).Error)
	return row.VolumeTon, row.Kode, row.Status
}

// terisi menjumlahkan lot yang belum habis di fasilitas berkode fasilitas (CS-02).
func terisi(t *testing.T, s *suite, fasilitas string) float64 {
	t.Helper()
	var v float64
	require.NoError(t, s.db.Raw(`SELECT COALESCE(SUM(volume_ton), 0) FROM stok_cold_storage
		WHERE cold_storage_id = (SELECT id FROM cold_storage WHERE kode = ?) AND status <> 'HABIS'`, fasilitas).Scan(&v).Error)
	return v
}

func auditRows(t *testing.T, s *suite, aksi string) []struct {
	AktorID, EntitasID, Sebelum, Sesudah string
} {
	t.Helper()
	var rows []struct {
		AktorID, EntitasID, Sebelum, Sesudah string
	}
	require.NoError(t, s.db.Raw(`SELECT aktor_id, entitas_id, COALESCE(sebelum::text, '') AS sebelum, COALESCE(sesudah::text, '') AS sesudah
		FROM audit_log WHERE aksi = ? ORDER BY waktu`, aksi).Scan(&rows).Error)
	return rows
}

// ── Migrasi 000100 ────────────────────────────────────────────────────

func TestMigration000100_KeepsFacilitiesAndLotsOnDown(t *testing.T) {
	db := pg.NewDatabase(t, "mig_000100")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), 100))
	require.NoError(t, seed.SeedReference(t.Context(), db))
	execSQL(t, db, `
		INSERT INTO cold_storage (id, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status)
		VALUES ('CS-M', 'Fasilitas M', '1171', 5.5, 95.3, 100, 4, 'OPERASIONAL');
		INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, kedaluwarsa_pada, pemilik, risiko_food_loss)
		VALUES ('STK-M', 'CS-M', 'CBM-KRT', 10, now(), now() + interval '10 day', 'Pemilik M', 0.2);
		INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c) VALUES ('CS-M', now(), 4.2);
	`, "data pasca-000100")

	t.Run("bentuk kolom dan batas CHECK", func(t *testing.T) {
		var status string
		require.NoError(t, db.Raw(`SELECT status FROM stok_cold_storage WHERE id = 'STK-M'`).Scan(&status).Error)
		assert.Equal(t, "AKTIF", status, "lot yang ada otomatis AKTIF")

		assert.Error(t, db.Exec(`UPDATE cold_storage SET suhu_min_c = 5, suhu_maks_c = 5 WHERE id = 'CS-M'`).Error, "min harus < maks")
		assert.Error(t, db.Exec(`INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c) VALUES ('CS-M', now() + interval '1 s', 99)`).Error, "suhu di luar -80..60")
		assert.Error(t, db.Exec(`INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c, kelembapan_pct) VALUES ('CS-M', now() + interval '2 s', 4, 120)`).Error, "kelembapan > 100")
		assert.Error(t, db.Exec(`UPDATE stok_cold_storage SET status = 'LAIN' WHERE id = 'STK-M'`).Error)
		assert.Error(t, db.Exec(`INSERT INTO transaksi_stok (id, lot_id, jenis, volume_ton, fasilitas_asal_id, fasilitas_tujuan_id)
			VALUES ('T-X', 'STK-M', 'PINDAH', 1, 'CS-M', 'CS-M')`).Error, "pindah ke fasilitas yang sama")
		assert.Error(t, db.Exec(`INSERT INTO transaksi_stok (id, lot_id, jenis, volume_ton, fasilitas_tujuan_id)
			VALUES ('T-Y', 'STK-M', 'MASUK', 0, 'CS-M')`).Error, "volume harus > 0")
	})

	t.Run("telemetri unik per fasilitas dan waktu", func(t *testing.T) {
		assert.Error(t, db.Exec(`INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c)
			SELECT fasilitas_id, waktu, 5 FROM telemetri_suhu LIMIT 1`).Error)
	})

	t.Run("Down menyisakan fasilitas dan lot, Up setelahnya berhasil", func(t *testing.T) {
		require.NoError(t, migrator.DownTo(t.Context(), 99))

		var n int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM stok_cold_storage WHERE id = 'STK-M'`).Scan(&n).Error)
		assert.EqualValues(t, 1, n, "data lot tidak boleh hilang saat Down")
		var tabel []string
		require.NoError(t, db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_name IN ('telemetri_suhu', 'transaksi_stok')`).Scan(&tabel).Error)
		assert.Empty(t, tabel)

		require.NoError(t, migrator.UpTo(t.Context(), 100))
	})
}

// ── Otorisasi ─────────────────────────────────────────────────────────

func TestOperasi_AuthorizationMatrix(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_authz"))
	ingest := map[string]any{"pembacaan": []map[string]any{{"fasilitas_id": "CS-01", "waktu": time.Now().Add(-time.Hour), "suhu_c": 3.8}}}
	masuk := map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-01", "komoditas_kode": "CBM-KRT", "pemilik": "Uji", "volume_ton": 1}
	pindah := map[string]any{"jenis": "PINDAH", "lot_id": s.lot(t, "STK-CS01-05"), "fasilitas_tujuan_id": "CS-03", "volume_ton": 1}
	patch := map[string]any{"suhu_min_c": 1.0}

	cases := []struct {
		name, method, path, email string
		body                      any
		want                      int
	}{
		{"detail: tanpa token", http.MethodGet, "/v1/cold-storage/fasilitas/CS-01", "", nil, http.StatusUnauthorized},
		{"detail: tanpa izin", http.MethodGet, "/v1/cold-storage/fasilitas/CS-01", userTanpaIzin, nil, http.StatusForbidden},
		{"detail: lihat saja", http.MethodGet, "/v1/cold-storage/fasilitas/CS-01", userLihatSaja, nil, http.StatusOK},
		{"telemetri: lihat saja", http.MethodPost, "/v1/cold-storage/telemetri", userLihatSaja, ingest, http.StatusForbidden},
		{"telemetri: logistik", http.MethodPost, "/v1/cold-storage/telemetri", userLogistik, ingest, http.StatusForbidden},
		{"telemetri: kelola", http.MethodPost, "/v1/cold-storage/telemetri", userKelola, ingest, http.StatusOK},
		{"transaksi masuk: lihat saja", http.MethodPost, "/v1/cold-storage/transaksi", userLihatSaja, masuk, http.StatusForbidden},
		{"transaksi masuk: logistik ditolak", http.MethodPost, "/v1/cold-storage/transaksi", userLogistik, masuk, http.StatusForbidden},
		{"transaksi masuk: kelola", http.MethodPost, "/v1/cold-storage/transaksi", userKelola, masuk, http.StatusCreated},
		{"transaksi pindah: logistik", http.MethodPost, "/v1/cold-storage/transaksi", userLogistik, pindah, http.StatusCreated},
		{"patch: lihat saja", http.MethodPatch, "/v1/cold-storage/fasilitas/CS-01", userLihatSaja, patch, http.StatusForbidden},
		{"patch: logistik", http.MethodPatch, "/v1/cold-storage/fasilitas/CS-01", userLogistik, patch, http.StatusForbidden},
		{"patch: kelola", http.MethodPatch, "/v1/cold-storage/fasilitas/CS-01", userKelola, patch, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := s.call(t, tc.method, tc.path, tc.email, tc.body)
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

// ── Telemetri ─────────────────────────────────────────────────────────

func TestIngestTelemetri(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_telemetri"))
	// Telemetri demo berakhir di "sekarang"; dikosongkan agar pembacaan uji menjadi yang terbaru.
	require.NoError(t, s.db.Exec(`DELETE FROM telemetri_suhu`).Error)
	base := time.Now().UTC().Truncate(time.Minute)
	reading := func(id string, ago time.Duration, suhu float64) map[string]any {
		return map[string]any{"fasilitas_id": id, "waktu": base.Add(-ago), "suhu_c": suhu, "kelembapan_pct": 80.5}
	}

	t.Run("suhu aktual diturunkan dari pembacaan terbaru, bukan urutan kiriman", func(t *testing.T) {
		code, body := post(t, s, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{
			reading("CS-01", 10*time.Minute, 4.4), // terbaru walau dikirim pertama
			reading("CS-01", 40*time.Minute, 9.9),
			reading("CS-01", 70*time.Minute, 4.1),
		}})

		require.Equal(t, http.StatusOK, code, body)
		data := dataMap(t, body)
		assert.EqualValues(t, 3, data["diterima"])
		assert.EqualValues(t, 0, data["duplikat"])
		alarm := list(t, data["di_luar_rentang"])
		require.Len(t, alarm, 1, "9,9 di luar 2..6 (target 4 ± 2)")
		assert.Equal(t, 9.9, alarm[0]["suhu_c"])
		assert.Equal(t, 2.0, alarm[0]["batas_min_c"])
		assert.Equal(t, 6.0, alarm[0]["batas_maks_c"])

		var f struct {
			SuhuAktualC         float64
			KelembapanAktualPct float64
		}
		require.NoError(t, s.db.Raw(`SELECT suhu_aktual_c, kelembapan_aktual_pct FROM cold_storage WHERE kode = 'CS-01'`).Scan(&f).Error)
		assert.Equal(t, 4.4, f.SuhuAktualC)
		assert.Equal(t, 80.5, f.KelembapanAktualPct)
	})

	t.Run("kirim ulang idempoten", func(t *testing.T) {
		code, body := post(t, s, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{
			reading("CS-01", 10*time.Minute, 4.4), reading("CS-01", 5*time.Minute, 4.6),
		}})

		require.Equal(t, http.StatusOK, code, body)
		assert.EqualValues(t, 1, dataMap(t, body)["diterima"])
		assert.EqualValues(t, 1, dataMap(t, body)["duplikat"])
		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM telemetri_suhu WHERE fasilitas_id = ? AND sumber = 'SENSOR'`, fasID("CS-01")).Scan(&n).Error)
		assert.EqualValues(t, 4, n)
	})

	t.Run("batas alarm khusus fasilitas dipakai", func(t *testing.T) {
		w := s.call(t, http.MethodPatch, "/v1/cold-storage/fasilitas/CS-03", userKelola, map[string]any{"suhu_min_c": 5.5, "suhu_maks_c": 6.5})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		_, body := post(t, s, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{reading("CS-03", time.Minute, 7.0)}})

		alarm := list(t, dataMap(t, body)["di_luar_rentang"])
		require.Len(t, alarm, 1)
		assert.Equal(t, 6.5, alarm[0]["batas_maks_c"])
	})

	t.Run("aksi tercatat di audit trail", func(t *testing.T) {
		rows := auditRows(t, s, "INGEST_TELEMETRI")
		require.NotEmpty(t, rows)
		assert.Equal(t, uid("manajer_mutu"), rows[0].AktorID)
		assert.Equal(t, fasID("CS-01"), rows[0].EntitasID, "satu fasilitas: entitas_id adalah UUID-nya")
	})

	t.Run("fasilitas boleh ditulis dengan UUID atau kode dan hasilnya sama", func(t *testing.T) {
		waktuKode, waktuUUID := base.Add(-3*time.Hour), base.Add(-4*time.Hour)
		for ref, waktu := range map[string]time.Time{"CS-02": waktuKode, fasID("CS-02"): waktuUUID} {
			code, body := post(t, s, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{
				{"fasilitas_id": ref, "waktu": waktu, "suhu_c": -20.2},
			}})
			require.Equal(t, http.StatusOK, code, body)
		}
		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM telemetri_suhu WHERE fasilitas_id = ? AND waktu IN (?, ?)`, fasID("CS-02"), waktuKode, waktuUUID).Scan(&n).Error)
		assert.EqualValues(t, 2, n, "kedua cara menulis menunjuk fasilitas yang sama")
	})

	t.Run("kiriman banyak fasilitas dicatat sebagai ringkasan dengan daftar UUID di metadata", func(t *testing.T) {
		code, body := post(t, s, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{
			reading("CS-01", 20*time.Minute, 4.0), reading("CS-02", 20*time.Minute, -20.0),
		}})
		require.Equal(t, http.StatusOK, code, body)
		var row struct{ EntitasID, Metadata string }
		require.NoError(t, s.db.Raw(`SELECT entitas_id, metadata::text AS metadata FROM audit_log
			WHERE aksi = 'INGEST_TELEMETRI' ORDER BY waktu DESC, id DESC LIMIT 1`).Scan(&row).Error)
		assert.Equal(t, "2 fasilitas", row.EntitasID)
		assert.Contains(t, row.Metadata, fasID("CS-01"))
		assert.Contains(t, row.Metadata, fasID("CS-02"))
	})

	t.Run("penolakan", func(t *testing.T) {
		cases := map[string]struct {
			body any
			want int
		}{
			"fasilitas tak ada":       {map[string]any{"pembacaan": []map[string]any{reading("CS-99", time.Minute, 4)}}, http.StatusNotFound},
			"uuid tak ada":            {map[string]any{"pembacaan": []map[string]any{reading(fasID("CS-99"), time.Minute, 4)}}, http.StatusNotFound},
			"fasilitas > 64 karakter": {map[string]any{"pembacaan": []map[string]any{reading(strings.Repeat("a", 65), time.Minute, 4)}}, http.StatusBadRequest},
			"fasilitas rencana":       {map[string]any{"pembacaan": []map[string]any{reading("CS-05", time.Minute, 4)}}, http.StatusConflict},
			"waktu di masa depan":     {map[string]any{"pembacaan": []map[string]any{reading("CS-01", -time.Hour, 4)}}, http.StatusBadRequest},
			"suhu mustahil":           {map[string]any{"pembacaan": []map[string]any{reading("CS-01", time.Minute, 120)}}, http.StatusBadRequest},
			"kelembapan > 100":        {map[string]any{"pembacaan": []map[string]any{{"fasilitas_id": "CS-01", "waktu": base.Add(-time.Hour), "suhu_c": 4, "kelembapan_pct": 101}}}, http.StatusBadRequest},
			"suhu tidak dikirim":      {map[string]any{"pembacaan": []map[string]any{{"fasilitas_id": "CS-01", "waktu": base.Add(-time.Hour)}}}, http.StatusBadRequest},
			"sumber DEMO dicadangkan": {map[string]any{"pembacaan": []map[string]any{{"fasilitas_id": "CS-01", "waktu": base.Add(-time.Hour), "suhu_c": 4, "sumber": "DEMO"}}}, http.StatusBadRequest},
			"larik kosong":            {map[string]any{"pembacaan": []map[string]any{}}, http.StatusBadRequest},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				w := s.call(t, http.MethodPost, "/v1/cold-storage/telemetri", userKelola, tc.body)
				assert.Equal(t, tc.want, w.Code, w.Body.String())
			})
		}
	})

	t.Run("kiriman yang gagal tidak menyimpan sebagian", func(t *testing.T) {
		var before int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM telemetri_suhu`).Scan(&before).Error)

		code, _ := post(t, s, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{
			reading("CS-01", 3*time.Minute, 4.0), reading("CS-99", 3*time.Minute, 4.0),
		}})

		assert.Equal(t, http.StatusNotFound, code)
		var after int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM telemetri_suhu`).Scan(&after).Error)
		assert.Equal(t, before, after)
	})
}

// ── Detail fasilitas ──────────────────────────────────────────────────

func TestDetailFasilitas_UsesDemoTelemetryAndLedger(t *testing.T) {
	s := sharedSuite(t)

	t.Run("CS-04 pulih dari lonjakan suhu: riwayat memuat titik di luar rentang, saat ini normal", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-04", userLihatSaja, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		data := dataMap(t, bodyOf(t, w))

		fas := data["fasilitas"].(map[string]any)
		assert.Equal(t, fasID("CS-04"), fas["id"])
		assert.Equal(t, "CS-04", fas["kode"])
		assert.Equal(t, false, fas["alarm_suhu"])
		assert.Equal(t, 4.8, fas["suhu_aktual_c"])
		assert.Equal(t, 2.0, fas["batas_min_c"])
		assert.Equal(t, 6.0, fas["batas_maks_c"])
		assert.NotNil(t, fas["telemetri_pada"])

		titik := list(t, data["telemetri"])
		// Grid 30 menit anchored ke saat seed; titik tertua dapat jatuh sedikit di luar jendela 48 jam.
		require.GreaterOrEqual(t, len(titik), 90)
		require.LessOrEqual(t, len(titik), 96)
		var luar int
		prev := ""
		for _, p := range titik {
			waktu := p["waktu"].(string)
			assert.Greater(t, waktu, prev, "terlama ke terbaru")
			prev = waktu
			if p["di_luar_rentang"] == true {
				luar++
			}
		}
		assert.GreaterOrEqual(t, luar, 2)
		assert.Equal(t, 4.8, titik[len(titik)-1]["suhu_c"], "titik terbaru sama dengan suhu aktual kartu")
	})

	t.Run("rentang jam dibatasi", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-01?jam=6", userLihatSaja, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.LessOrEqual(t, len(list(t, dataMap(t, bodyOf(t, w))["telemetri"])), 13)

		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-01?jam=721", userLihatSaja, nil).Code)
	})

	t.Run("lot dan transaksi masuk demo", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-03", userLihatSaja, nil)
		data := dataMap(t, bodyOf(t, w))

		assert.Len(t, list(t, data["stok"]), 7)
		trx := list(t, data["transaksi"])
		require.Len(t, trx, 7)
		for _, x := range trx {
			assert.Equal(t, "MASUK", x["jenis"])
			assert.Equal(t, fasID("CS-03"), x["fasilitas_tujuan_id"])
			assert.Equal(t, "CS-03", x["fasilitas_tujuan_kode"])
			assert.Nil(t, x["fasilitas_asal_id"])
			assert.Nil(t, x["fasilitas_asal_kode"])
			assert.Regexp(t, `^[1-9][0-9]*$`, x["id"], "id transaksi BIGINT sebagai string desimal")
			assert.Regexp(t, `^TRX-DEMO-STK-CS03-0[1-7]$`, x["kode"])
		}
		for _, st := range list(t, data["stok"]) {
			assert.Regexp(t, `^[1-9][0-9]*$`, st["id"])
			assert.Regexp(t, `^STK-CS03-0[1-7]$`, st["kode"])
			assert.Equal(t, fasID("CS-03"), st["cold_storage_id"])
		}
	})

	t.Run("rute memakai UUID atau kode, huruf besar dinormalkan", func(t *testing.T) {
		kode := dataMap(t, bodyOf(t, s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-03", userLihatSaja, nil)))["fasilitas"]
		for _, ref := range []string{fasID("CS-03"), strings.ToUpper(fasID("CS-03"))} {
			w := s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/"+ref, userLihatSaja, nil)
			require.Equal(t, http.StatusOK, w.Code, ref)
			assert.Equal(t, kode, dataMap(t, bodyOf(t, w))["fasilitas"], ref)
		}
	})

	t.Run("id salah bentuk atau tidak ada dijawab 404 tanpa menyentuh kolom uuid", func(t *testing.T) {
		for name, ref := range map[string]string{
			"kode tak ada":         "CS-99",
			"uuid tak ada":         fasID("CS-99"),
			"lebih dari 64":        strings.Repeat("a", 65),
			"angka (bukan kode)":   "12345",
			"uuid tanpa tanda -":   strings.ReplaceAll(fasID("CS-03"), "-", ""),
			"bentuk uuid berisi x": "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
		} {
			w := s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/"+ref, userLihatSaja, nil)
			assert.Equal(t, http.StatusNotFound, w.Code, "%s: %s", name, w.Body.String())
			w = s.call(t, http.MethodPatch, "/v1/cold-storage/fasilitas/"+ref, userKelola, map[string]any{"suhu_min_c": 1})
			assert.Equal(t, http.StatusNotFound, w.Code, "PATCH %s: %s", name, w.Body.String())
		}
	})

	t.Run("fasilitas rencana tanpa telemetri", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-05", userLihatSaja, nil)
		require.Equal(t, http.StatusOK, w.Code)
		data := dataMap(t, bodyOf(t, w))
		assert.Empty(t, list(t, data["telemetri"]))
		assert.Nil(t, data["fasilitas"].(map[string]any)["suhu_aktual_c"])
		assert.Equal(t, false, data["fasilitas"].(map[string]any)["alarm_suhu"])
	})

	t.Run("fasilitas tak ada", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-99", userLihatSaja, nil).Code)
	})
}

// ── Transaksi stok ────────────────────────────────────────────────────

func TestTransaksi_LedgerStaysConsistentWithStock(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_transaksi"))
	trx := func(body map[string]any) (int, map[string]any) {
		return post(t, s, "/v1/cold-storage/transaksi", userKelola, body)
	}

	var lotMasuk string
	lot := func(kode string) string { return s.lot(t, kode) }
	t.Run("MASUK membuat lot dan menaikkan utilisasi", func(t *testing.T) {
		before := terisi(t, s, "CS-02")

		code, body := trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-02", "komoditas_kode": "IKN-TGK", "pemilik": "CV Uji", "volume_ton": 25.5})

		require.Equal(t, http.StatusCreated, code, body)
		data := dataMap(t, body)
		lotMasuk = data["lot_id"].(string)
		assert.Regexp(t, `^[1-9][0-9]*$`, lotMasuk, "ID lot dari identity basis data")
		assert.Regexp(t, `^[1-9][0-9]*$`, data["transaksi_id"], "ID transaksi dari identity basis data")
		assert.Equal(t, "AKTIF", data["status_lot"])
		assert.Equal(t, 25.5, data["volume_sisa_ton"])
		assert.InDelta(t, before+25.5, terisi(t, s, "CS-02"), 1e-9)

		var kedaluwarsaHari float64
		require.NoError(t, s.db.Raw(`SELECT EXTRACT(EPOCH FROM (kedaluwarsa_pada - masuk_pada)) / 86400 FROM stok_cold_storage WHERE id = ?`, lotMasuk).Scan(&kedaluwarsaHari).Error)
		assert.Equal(t, 120.0, kedaluwarsaHari, "masa simpan IKN-TGK dari master komoditas")

		rows := auditRows(t, s, "TRANSAKSI_STOK_MASUK")
		require.Len(t, rows, 1)
		assert.Equal(t, lotMasuk, rows[0].EntitasID)
		assert.Empty(t, rows[0].Sebelum)
		assert.JSONEq(t, fmt.Sprintf(`{"fasilitas_id":%q,"volume_ton":25.5,"status":"AKTIF"}`, fasID("CS-02")), rows[0].Sesudah)

		// Lot baru diberi kode tampilan dari ID-nya; transaksi baru tidak diberi kode.
		var kode, kodeTrx *string
		require.NoError(t, s.db.Raw(`SELECT kode FROM stok_cold_storage WHERE id = ?`, lotMasuk).Scan(&kode).Error)
		require.NotNil(t, kode)
		assert.Equal(t, "STK-"+strings.Repeat("0", 6-len(lotMasuk))+lotMasuk, *kode)
		require.NoError(t, s.db.Raw(`SELECT kode FROM transaksi_stok WHERE id = ?`, data["transaksi_id"]).Scan(&kodeTrx).Error)
		assert.Nil(t, kodeTrx)
	})

	t.Run("MASUK dengan fasilitas ditulis UUID atau kode menghasilkan lot di fasilitas yang sama", func(t *testing.T) {
		for _, ref := range []string{"CS-01", fasID("CS-01"), strings.ToUpper(fasID("CS-01"))} {
			code, body := trx(map[string]any{"jenis": "MASUK", "fasilitas_id": ref, "komoditas_kode": "CBM-KRT", "pemilik": "Uji " + ref, "volume_ton": 0.5})
			require.Equal(t, http.StatusCreated, code, "%s: %v", ref, body)
			_, fas, _ := volumeLot(t, s, dataMap(t, body)["lot_id"].(string))
			assert.Equal(t, "CS-01", fas, ref)
		}
		// Ketiganya dibatalkan agar angka kapasitas dan jumlah transaksi uji berikutnya tetap seperti semula.
		require.NoError(t, s.db.Exec(`DELETE FROM transaksi_stok WHERE lot_id IN (SELECT id FROM stok_cold_storage WHERE pemilik LIKE 'Uji %')`).Error)
		require.NoError(t, s.db.Exec(`DELETE FROM stok_cold_storage WHERE pemilik LIKE 'Uji %'`).Error)
	})

	t.Run("MASUK ditolak bila melebihi kapasitas, fasilitas tak beroperasi, atau komoditas tak ada", func(t *testing.T) {
		// CS-02: kapasitas 300, terisi 265,5.
		code, _ := trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-02", "komoditas_kode": "IKN-TGK", "pemilik": "x", "volume_ton": 35})
		assert.Equal(t, http.StatusConflict, code, "265,5 + 35 > 300")
		code, _ = trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-02", "komoditas_kode": "IKN-TGK", "pemilik": "x", "volume_ton": 34.5})
		assert.Equal(t, http.StatusCreated, code, "tepat memenuhi kapasitas")

		code, _ = trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-04", "komoditas_kode": "TLR", "pemilik": "x", "volume_ton": 1})
		assert.Equal(t, http.StatusConflict, code, "CS-04 dalam perawatan")
		code, _ = trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-05", "komoditas_kode": "TLR", "pemilik": "x", "volume_ton": 1})
		assert.Equal(t, http.StatusConflict, code, "CS-05 baru rencana")
		code, _ = trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-01", "komoditas_kode": "TIDAK-ADA", "pemilik": "x", "volume_ton": 1})
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = trx(map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-99", "komoditas_kode": "TLR", "pemilik": "x", "volume_ton": 1})
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("KELUAR sebagian lalu seluruhnya: lot habis dan tidak lagi tampil", func(t *testing.T) {
		code, body := trx(map[string]any{"jenis": "KELUAR", "lot_id": lotMasuk, "volume_ton": 10.5, "alasan": "Dijual ke pasar induk"})
		require.Equal(t, http.StatusCreated, code, body)
		assert.Equal(t, 15.0, dataMap(t, body)["volume_sisa_ton"])
		vol, _, status := volumeLot(t, s, lotMasuk)
		assert.Equal(t, 15.0, vol)
		assert.Equal(t, "AKTIF", status)

		code, body = trx(map[string]any{"jenis": "KELUAR", "lot_id": lotMasuk, "semua": true, "alasan": "Sisa dijual"})
		require.Equal(t, http.StatusCreated, code, body)
		assert.Equal(t, "HABIS", dataMap(t, body)["status_lot"])
		assert.EqualValues(t, 15, dataMap(t, body)["volume_ton"])
		_, _, status = volumeLot(t, s, lotMasuk)
		assert.Equal(t, "HABIS", status)

		data, stok := getCold(t, s, userLihatSaja, nil)
		for _, st := range stok {
			assert.NotEqual(t, lotMasuk, st["id"], "lot habis tidak tampil")
		}
		assert.EqualValues(t, 23+1, data["ringkasan"].(map[string]any)["total_lot"], "23 lot demo + 1 lot MASUK 34,5 ton; lot habis tidak dihitung")

		code, _ = trx(map[string]any{"jenis": "KELUAR", "lot_id": lotMasuk, "volume_ton": 1, "alasan": "x"})
		assert.Equal(t, http.StatusConflict, code, "lot habis tidak dapat ditransaksikan lagi")

		rows := auditRows(t, s, "TRANSAKSI_STOK_KELUAR")
		require.Len(t, rows, 2)
		cs02 := fasID("CS-02")
		assert.JSONEq(t, fmt.Sprintf(`{"fasilitas_id":%q,"volume_ton":25.5,"status":"AKTIF"}`, cs02), rows[0].Sebelum)
		assert.JSONEq(t, fmt.Sprintf(`{"fasilitas_id":%q,"volume_ton":15,"status":"AKTIF"}`, cs02), rows[0].Sesudah)
		assert.JSONEq(t, fmt.Sprintf(`{"fasilitas_id":%q,"volume_ton":0,"status":"HABIS"}`, cs02), rows[1].Sesudah)
	})

	t.Run("KELUAR ditolak: melebihi stok, tanpa alasan, volume tidak positif", func(t *testing.T) {
		code, _ := trx(map[string]any{"jenis": "KELUAR", "lot_id": lot("STK-CS01-05"), "volume_ton": 18.7, "alasan": "x"})
		assert.Equal(t, http.StatusConflict, code, "stok lot 18,6 ton")
		code, _ = trx(map[string]any{"jenis": "KELUAR", "lot_id": lot("STK-CS01-05"), "volume_ton": 1})
		assert.Equal(t, http.StatusBadRequest, code, "alasan wajib")
		code, _ = trx(map[string]any{"jenis": "KELUAR", "lot_id": lot("STK-CS01-05"), "volume_ton": 0, "alasan": "x"})
		assert.Equal(t, http.StatusBadRequest, code)
		code, _ = trx(map[string]any{"jenis": "KELUAR", "lot_id": lot("STK-CS01-05"), "volume_ton": -3, "alasan": "x"})
		assert.Equal(t, http.StatusBadRequest, code)
		code, _ = trx(map[string]any{"jenis": "KELUAR", "lot_id": lotTakAda, "volume_ton": 1, "alasan": "x"})
		assert.Equal(t, http.StatusNotFound, code)
		vol, _, _ := volumeLot(t, s, "STK-CS01-05")
		assert.Equal(t, 18.6, vol, "stok tidak berubah oleh penolakan")
	})

	t.Run("lot_id salah bentuk dijawab 400, bukan 404 dan bukan 500", func(t *testing.T) {
		for name, id := range map[string]any{
			"kode lama":         "STK-CS01-05",
			"uuid":              fasID("CS-01"),
			"nol":               "0",
			"negatif":           "-4",
			"pecahan":           1.5,
			"melebihi int64":    "9223372036854775808",
			"dengan spasi":      " 7 ",
			"awalan nol":        "007",
			"string kosong":     "",
			"bukan angka (obj)": map[string]any{"a": 1},
		} {
			code, _ := trx(map[string]any{"jenis": "KELUAR", "lot_id": id, "volume_ton": 1, "alasan": "x"})
			assert.Equal(t, http.StatusBadRequest, code, name)
		}
		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM transaksi_stok WHERE alasan = 'x'`).Scan(&n).Error)
		assert.Zero(t, n, "permintaan salah bentuk tidak menyentuh buku besar")
	})

	t.Run("lot_id boleh angka JSON maupun string desimal", func(t *testing.T) {
		id, _ := strconv.ParseInt(lot("STK-CS01-05"), 10, 64)
		for _, v := range []any{id, lot("STK-CS01-05")} {
			code, body := trx(map[string]any{"jenis": "KELUAR", "lot_id": v, "volume_ton": 0.1, "alasan": "bentuk id"})
			require.Equal(t, http.StatusCreated, code, "%T: %v", v, body)
		}
		vol, _, _ := volumeLot(t, s, "STK-CS01-05")
		assert.InDelta(t, 18.4, vol, 1e-9)
		// Kembalikan agar uji PINDAH di bawah memakai volume 18,6 ton seperti seed.
		require.NoError(t, s.db.Exec(`UPDATE stok_cold_storage SET volume_ton = 18.6 WHERE kode = 'STK-CS01-05'`).Error)
		require.NoError(t, s.db.Exec(`DELETE FROM transaksi_stok WHERE alasan = 'bentuk id'`).Error)
	})

	t.Run("PINDAH seluruh lot mengubah lokasinya", func(t *testing.T) {
		a, b := terisi(t, s, "CS-01"), terisi(t, s, "CS-03")

		code, body := trx(map[string]any{"jenis": "PINDAH", "lot_id": lot("STK-CS01-05"), "fasilitas_tujuan_id": "CS-03", "semua": true})

		require.Equal(t, http.StatusCreated, code, body)
		assert.Nil(t, dataMap(t, body)["lot_baru_id"])
		_, fas, _ := volumeLot(t, s, "STK-CS01-05")
		assert.Equal(t, "CS-03", fas)
		assert.InDelta(t, a-18.6, terisi(t, s, "CS-01"), 1e-9)
		assert.InDelta(t, b+18.6, terisi(t, s, "CS-03"), 1e-9)
	})

	t.Run("PINDAH sebagian membentuk lot baru dengan atribut sama", func(t *testing.T) {
		code, body := trx(map[string]any{"jenis": "PINDAH", "lot_id": lot("STK-CS01-04"), "fasilitas_tujuan_id": "CS-03", "volume_ton": 8.2})

		require.Equal(t, http.StatusCreated, code, body)
		data := dataMap(t, body)
		baru := data["lot_baru_id"].(string)
		require.Regexp(t, `^[1-9][0-9]*$`, baru, "ID lot baru dari RETURNING, bukan buatan aplikasi")
		assert.NotEqual(t, lot("STK-CS01-04"), baru)
		assert.InDelta(t, 30.0, data["volume_sisa_ton"], 1e-9)

		sumber, _, _ := volumeLot(t, s, "STK-CS01-04")
		assert.InDelta(t, 30.0, sumber, 1e-9, "38,2 - 8,2")
		vol, fas, status := volumeLot(t, s, baru)
		assert.InDelta(t, 8.2, vol, 1e-9)
		assert.Equal(t, "CS-03", fas)
		assert.Equal(t, "AKTIF", status)

		var sama int64
		require.NoError(t, s.db.Raw(`
			SELECT COUNT(*) FROM stok_cold_storage a JOIN stok_cold_storage b ON b.id = ?
			 WHERE a.kode = 'STK-CS01-04' AND a.komoditas_kode = b.komoditas_kode AND a.pemilik = b.pemilik
			   AND a.masuk_pada = b.masuk_pada AND a.kedaluwarsa_pada = b.kedaluwarsa_pada AND a.risiko_food_loss = b.risiko_food_loss`, baru).Scan(&sama).Error)
		assert.EqualValues(t, 1, sama)

		// Kode lot baru turunan ID-nya (bukan salinan kode lot sumber), dan transaksi menunjuk kedua lot.
		var kodeBaru string
		require.NoError(t, s.db.Raw(`SELECT kode FROM stok_cold_storage WHERE id = ?`, baru).Scan(&kodeBaru).Error)
		assert.NotEqual(t, "STK-CS01-04", kodeBaru)
		assert.Equal(t, "STK-"+strings.Repeat("0", 6-len(baru))+baru, kodeBaru)
		var trxLot struct{ LotID, LotBaruID string }
		require.NoError(t, s.db.Raw(`SELECT lot_id::text AS lot_id, lot_baru_id::text AS lot_baru_id FROM transaksi_stok WHERE id = ?`, data["transaksi_id"]).Scan(&trxLot).Error)
		assert.Equal(t, lot("STK-CS01-04"), trxLot.LotID)
		assert.Equal(t, baru, trxLot.LotBaruID)

		aud := auditRows(t, s, "TRANSAKSI_STOK_PINDAH")
		require.NotEmpty(t, aud)
		assert.Equal(t, lot("STK-CS01-04"), aud[len(aud)-1].EntitasID, "audit mencatat ID lot sumber")
	})

	t.Run("PINDAH ditolak: kapasitas tujuan, fasilitas sama, tujuan tak beroperasi", func(t *testing.T) {
		code, _ := trx(map[string]any{"jenis": "PINDAH", "lot_id": lot("STK-CS01-07"), "fasilitas_tujuan_id": "CS-02", "volume_ton": 40})
		assert.Equal(t, http.StatusConflict, code, "CS-02 hanya muat 25,5 ton lagi")
		code, _ = trx(map[string]any{"jenis": "PINDAH", "lot_id": lot("STK-CS01-07"), "fasilitas_tujuan_id": "CS-01", "volume_ton": 1})
		assert.Equal(t, http.StatusBadRequest, code)
		code, _ = trx(map[string]any{"jenis": "PINDAH", "lot_id": lot("STK-CS01-07"), "fasilitas_tujuan_id": "CS-04", "volume_ton": 1})
		assert.Equal(t, http.StatusConflict, code, "CS-04 dalam perawatan")
		code, _ = trx(map[string]any{"jenis": "PINDAH", "lot_id": lot("STK-CS01-07"), "fasilitas_tujuan_id": "CS-99", "volume_ton": 1})
		assert.Equal(t, http.StatusNotFound, code)
		vol, fas, _ := volumeLot(t, s, "STK-CS01-07")
		assert.Equal(t, 52.7, vol)
		assert.Equal(t, "CS-01", fas)
	})

	t.Run("jenis dan isian wajib divalidasi", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"jenis kosong":                 {},
			"jenis tak dikenal":            {"jenis": "BUANG"},
			"masuk tanpa komoditas":        {"jenis": "MASUK", "fasilitas_id": "CS-01", "pemilik": "x", "volume_ton": 1},
			"masuk tanpa volume":           {"jenis": "MASUK", "fasilitas_id": "CS-01", "komoditas_kode": "TLR", "pemilik": "x"},
			"masuk kedaluwarsa lampau":     {"jenis": "MASUK", "fasilitas_id": "CS-01", "komoditas_kode": "TLR", "pemilik": "x", "volume_ton": 1, "kedaluwarsa_pada": time.Now().Add(-time.Hour)},
			"pindah tanpa tujuan":          {"jenis": "PINDAH", "lot_id": lot("STK-CS01-07"), "volume_ton": 1},
			"keluar tanpa lot":             {"jenis": "KELUAR", "volume_ton": 1, "alasan": "x"},
			"keluar volume tak masuk akal": {"jenis": "KELUAR", "lot_id": lot("STK-CS01-07"), "volume_ton": 1e12, "alasan": "x"},
		} {
			code, _ := trx(body)
			assert.Equal(t, http.StatusBadRequest, code, name)
		}
	})

	t.Run("buku besar: volume tiap lot = MASUK - KELUAR, dan tiap perpindahan tercatat", func(t *testing.T) {
		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM transaksi_stok WHERE sumber = 'APLIKASI'`).Scan(&n).Error)
		assert.EqualValues(t, 6, n, "2 MASUK, 2 KELUAR, 2 PINDAH yang berhasil")

		var selisih float64
		require.NoError(t, s.db.Raw(`
			SELECT COALESCE(SUM(ABS(s.volume_ton - COALESCE(m.masuk, 0) + COALESCE(k.keluar, 0))), 0)
			  FROM stok_cold_storage s
			  LEFT JOIN (SELECT lot_id, SUM(volume_ton) AS masuk FROM transaksi_stok WHERE jenis = 'MASUK' GROUP BY lot_id) m ON m.lot_id = s.id
			  LEFT JOIN (SELECT lot_id, SUM(volume_ton) AS keluar FROM transaksi_stok WHERE jenis = 'KELUAR' GROUP BY lot_id) k ON k.lot_id = s.id
			 WHERE s.id = ?`, lotMasuk).Scan(&selisih).Error)
		assert.InDelta(t, 0, selisih, 1e-9)
	})
}

func TestUbahFasilitas(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_patch"))
	patch := func(id string, body any) (int, map[string]any) {
		w := s.call(t, http.MethodPatch, "/v1/cold-storage/fasilitas/"+id, userKelola, body)
		return w.Code, bodyOf(t, w)
	}

	code, body := patch("CS-01", map[string]any{"suhu_min_c": 2.5, "suhu_maks_c": 5.5, "kapasitas_ton": 500})

	require.Equal(t, http.StatusOK, code, body)
	fas := dataMap(t, body)
	assert.Equal(t, fasID("CS-01"), fas["id"])
	assert.Equal(t, "CS-01", fas["kode"])
	assert.Equal(t, 2.5, fas["batas_min_c"])
	assert.Equal(t, 5.5, fas["batas_maks_c"])
	assert.EqualValues(t, 500, fas["kapasitas_ton"])
	rows := auditRows(t, s, "UBAH_FASILITAS_COLD_STORAGE")
	require.Len(t, rows, 1)
	assert.Equal(t, fasID("CS-01"), rows[0].EntitasID, "audit memakai UUID walau URL memakai kode")
	assert.JSONEq(t, `{"kapasitas_ton":400,"suhu_min_c":null,"suhu_maks_c":null,"status":"OPERASIONAL"}`, rows[0].Sebelum)
	assert.JSONEq(t, `{"kapasitas_ton":500,"suhu_min_c":2.5,"suhu_maks_c":5.5,"status":"OPERASIONAL"}`, rows[0].Sesudah)

	for name, tc := range map[string]struct {
		id   string
		body any
		want int
	}{
		"min >= maks":             {"CS-01", map[string]any{"suhu_min_c": 6.0}, http.StatusBadRequest},
		"kapasitas di bawah stok": {"CS-01", map[string]any{"kapasitas_ton": 300}, http.StatusConflict},
		"kapasitas nol":           {"CS-01", map[string]any{"kapasitas_ton": 0}, http.StatusBadRequest},
		"status tak dikenal":      {"CS-01", map[string]any{"status": "TUTUP"}, http.StatusBadRequest},
		"tanpa perubahan":         {"CS-01", map[string]any{}, http.StatusBadRequest},
		"fasilitas tak ada":       {"CS-99", map[string]any{"suhu_min_c": 1}, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			code, _ := patch(tc.id, tc.body)
			assert.Equal(t, tc.want, code)
		})
	}
	assert.Len(t, auditRows(t, s, "UBAH_FASILITAS_COLD_STORAGE"), 1, "penolakan tidak tercatat sebagai perubahan")

	t.Run("UUID dan kode menunjuk fasilitas yang sama", func(t *testing.T) {
		code, body := patch(fasID("CS-02"), map[string]any{"suhu_min_c": -22.0})
		require.Equal(t, http.StatusOK, code, body)
		assert.Equal(t, "CS-02", dataMap(t, body)["kode"])
		code, body = patch("CS-02", map[string]any{"suhu_min_c": -21.0})
		require.Equal(t, http.StatusOK, code, body)
		assert.Equal(t, -21.0, dataMap(t, body)["batas_min_c"])
	})

	t.Run("status PERAWATAN menutup penerimaan stok", func(t *testing.T) {
		code, _ := patch("CS-03", map[string]any{"status": "PERAWATAN"})
		require.Equal(t, http.StatusOK, code)

		code, _ = post(t, s, "/v1/cold-storage/transaksi", userKelola, map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-03", "komoditas_kode": "TMT", "pemilik": "x", "volume_ton": 1})
		assert.Equal(t, http.StatusConflict, code)
	})
}

// ── Aksi massal yang mengubah stok ────────────────────────────────────

func TestBulk_PrioritasJualDanBuang(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_bulk"))

	t.Run("izin: logistik tidak boleh menandai atau membuang", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, bulk(t, s, userLogistik, "tandai_prioritas_jual", "STK-CS01-01").Code)
		assert.Equal(t, http.StatusForbidden, bulk(t, s, userLogistik, "buang", "STK-CS01-01").Code)
		assert.Equal(t, http.StatusForbidden, bulk(t, s, userLihatSaja, "buang", "STK-CS01-01").Code)
	})

	t.Run("tandai prioritas jual: status berubah, hasil per lot, idempoten", func(t *testing.T) {
		for range 2 {
			w := bulk(t, s, userKelola, "tandai_prioritas_jual", "STK-CS01-01", "STK-TIDAK-ADA", "STK-CS03-01")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			data := dataOf(t, w)
			assert.EqualValues(t, 2, data["berhasil"])
			assert.EqualValues(t, 1, data["gagal"])
			rincian := list(t, data["rincian"])
			require.Len(t, rincian, 3)
			assert.Equal(t, []any{s.lot(t, "STK-CS01-01"), true, ""}, []any{rincian[0]["id"], rincian[0]["berhasil"], rincian[0]["pesan"]})
			assert.Equal(t, false, rincian[1]["berhasil"])
			assert.NotEmpty(t, rincian[1]["pesan"])
			assert.Equal(t, true, rincian[2]["berhasil"])
		}
		_, _, status := volumeLot(t, s, "STK-CS01-01")
		assert.Equal(t, "PRIORITAS_JUAL", status)

		_, stok := getCold(t, s, userLihatSaja, map[string][]string{"cold_storage_id": {"CS-01"}, "limit": {"100"}})
		var ditandai int
		for _, st := range stok {
			if st["status"] == "PRIORITAS_JUAL" {
				ditandai++
			}
		}
		assert.Equal(t, 1, ditandai)
	})

	t.Run("buang: seluruh sisa lot keluar sebagai transaksi, lot yang habis dilaporkan gagal", func(t *testing.T) {
		before := terisi(t, s, "CS-03")

		w := bulk(t, s, userKelola, "buang", "STK-CS03-01", "STK-CS03-02", "STK-TIDAK-ADA")

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		data := dataOf(t, w)
		assert.EqualValues(t, 2, data["berhasil"])
		assert.EqualValues(t, 1, data["gagal"])
		assert.Equal(t, 46.4, data["volume_ton"], "31,0 + 15,4 ton")
		assert.InDelta(t, before-46.4, terisi(t, s, "CS-03"), 1e-9)
		_, _, status := volumeLot(t, s, "STK-CS03-01")
		assert.Equal(t, "HABIS", status)

		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM transaksi_stok WHERE jenis = 'KELUAR' AND alasan = 'Dibuang (aksi massal)' AND oleh_id = garda_uuid_stabil('pengguna', 'manajer_mutu')`).Scan(&n).Error)
		assert.EqualValues(t, 2, n)

		rows := auditRows(t, s, "BULK_buang")
		require.Len(t, rows, 1)
		assert.Equal(t, uid("manajer_mutu"), rows[0].AktorID)
		assert.Equal(t, "2 lot", rows[0].EntitasID)

		again := bulk(t, s, userKelola, "buang", "STK-CS03-01")
		assert.EqualValues(t, 0, dataOf(t, again)["berhasil"], "lot yang sudah habis tidak dapat dibuang lagi")
	})
}

// ── Konkurensi ────────────────────────────────────────────────────────

// concurrently menjalankan fn sebanyak n kali serentak dan mengembalikan kode
// status masing-masing. Tidak memakai require di dalam goroutine.
func concurrently(n int, fn func(i int) int) []int {
	codes := make([]int, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			codes[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return codes
}

func (s *suite) rawPost(path, email string, body any) int {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.tokens[email])
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w.Code
}

func count(codes []int, want int) int {
	n := 0
	for _, c := range codes {
		if c == want {
			n++
		}
	}
	return n
}

func TestTransaksi_ConcurrentRequestsNeverOverdrawOrOverfill(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_konkurensi"))
	const path = "/v1/cold-storage/transaksi"
	lotCS02, lotCS01_04 := s.lot(t, "STK-CS02-01"), s.lot(t, "STK-CS01-04")
	lotCS03_05, lotCS01_08 := s.lot(t, "STK-CS03-05"), s.lot(t, "STK-CS01-08")

	t.Run("dua pengguna mengeluarkan lot yang sama: tidak ada stok negatif", func(t *testing.T) {
		// STK-CS02-01 = 64,5 ton. Delapan permintaan 20 ton: tepat tiga yang muat.
		codes := concurrently(8, func(int) int {
			return s.rawPost(path, userKelola, map[string]any{"jenis": "KELUAR", "lot_id": lotCS02, "volume_ton": 20, "alasan": "uji"})
		})

		assert.Equal(t, 3, count(codes, http.StatusCreated), "%v", codes)
		assert.Equal(t, 5, count(codes, http.StatusConflict), "%v", codes)
		vol, _, status := volumeLot(t, s, lotCS02)
		assert.InDelta(t, 4.5, vol, 1e-9)
		assert.Equal(t, "AKTIF", status)
	})

	t.Run("pindah sebagian lot yang sama serentak: hanya yang muat berhasil dan volume terjaga", func(t *testing.T) {
		total := terisi(t, s, "CS-01") + terisi(t, s, "CS-03")

		// STK-CS01-04 = 38,2 ton: dua pindah 20 ton serentak, hanya satu yang muat.
		codes := concurrently(4, func(int) int {
			return s.rawPost(path, userLogistik, map[string]any{"jenis": "PINDAH", "lot_id": lotCS01_04, "fasilitas_tujuan_id": "CS-03", "volume_ton": 20})
		})

		assert.Equal(t, 1, count(codes, http.StatusCreated), "%v", codes)
		assert.Equal(t, 3, count(codes, http.StatusConflict), "%v", codes)
		assert.InDelta(t, total, terisi(t, s, "CS-01")+terisi(t, s, "CS-03"), 1e-9, "volume total tidak berubah oleh pemindahan")
	})

	t.Run("masuk serentak tidak melampaui kapasitas", func(t *testing.T) {
		// CS-02: kapasitas 300 ton, terisi 180 setelah uji pertama -> sisa 120 ton.
		// Sepuluh MASUK 20 ton serentak: tepat enam yang muat.
		codes := concurrently(10, func(int) int {
			return s.rawPost(path, userKelola, map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-02", "komoditas_kode": "IKN-TGK", "pemilik": "uji", "volume_ton": 20})
		})

		assert.Equal(t, 6, count(codes, http.StatusCreated), "%v", codes)
		assert.Equal(t, 4, count(codes, http.StatusConflict), "%v", codes)
		assert.InDelta(t, 300.0, terisi(t, s, "CS-02"), 1e-9)
	})

	t.Run("pindah silang antar dua fasilitas tidak saling mengunci", func(t *testing.T) {
		codes := concurrently(6, func(i int) int {
			lot, tujuan := lotCS03_05, "CS-01"
			if i%2 == 1 {
				lot, tujuan = lotCS01_08, "CS-03"
			}
			return s.rawPost(path, userKelola, map[string]any{"jenis": "PINDAH", "lot_id": lot, "fasilitas_tujuan_id": tujuan, "volume_ton": 1})
		})

		// Kunci fasilitas diambil menurut urutan id, jadi tidak ada deadlock (500).
		assert.Equal(t, 6, count(codes, http.StatusCreated), "%v", codes)
	})
}

// ── Kontrak OpenAPI ───────────────────────────────────────────────────

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	require.NoError(t, err)
	return n
}

func TestContract_OperasiResponsesMatchOpenAPI(t *testing.T) {
	s := newSuite(t, pg.NewDatabase(t, "cs_ops_contract"))
	spec := loadSpec(t)

	check := func(t *testing.T, path, method, status string, w *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, mustAtoi(t, status), w.Code, w.Body.String())
		conforms(t, spec, responseSchema(t, spec, path, method, status), bodyOf(t, w), "$")
	}

	t.Run("detail fasilitas", func(t *testing.T) {
		check(t, "/v1/cold-storage/fasilitas/{id}", "get", "200", s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-03", userLihatSaja, nil))
		check(t, "/v1/cold-storage/fasilitas/{id}", "get", "200", s.call(t, http.MethodGet, "/v1/cold-storage/fasilitas/CS-05?jam=1", userLihatSaja, nil))
	})
	t.Run("telemetri", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/cold-storage/telemetri", userKelola, map[string]any{"pembacaan": []map[string]any{
			{"fasilitas_id": "CS-01", "waktu": time.Now().Add(-time.Minute), "suhu_c": 9.0},
		}})
		check(t, "/v1/cold-storage/telemetri", "post", "200", w)
	})
	t.Run("transaksi", func(t *testing.T) {
		masuk := s.call(t, http.MethodPost, "/v1/cold-storage/transaksi", userKelola, map[string]any{"jenis": "MASUK", "fasilitas_id": "CS-01", "komoditas_kode": "CBM-KRT", "pemilik": "Uji", "volume_ton": 2})
		check(t, "/v1/cold-storage/transaksi", "post", "201", masuk)
		pindah := s.call(t, http.MethodPost, "/v1/cold-storage/transaksi", userKelola, map[string]any{"jenis": "PINDAH", "lot_id": s.lot(t, "STK-CS01-01"), "fasilitas_tujuan_id": "CS-03", "volume_ton": 1})
		check(t, "/v1/cold-storage/transaksi", "post", "201", pindah)
	})
	t.Run("ubah fasilitas", func(t *testing.T) {
		check(t, "/v1/cold-storage/fasilitas/{id}", "patch", "200", s.call(t, http.MethodPatch, "/v1/cold-storage/fasilitas/CS-01", userKelola, map[string]any{"suhu_min_c": 1.5}))
	})
	t.Run("aksi massal", func(t *testing.T) {
		for _, a := range []string{"tandai_prioritas_jual", "buang"} {
			check(t, "/v1/cold-storage/bulk", "post", "200", bulk(t, s, userKelola, a, "STK-CS04-01", "STK-TIDAK-ADA"))
		}
	})
}
