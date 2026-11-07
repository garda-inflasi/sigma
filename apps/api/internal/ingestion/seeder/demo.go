package seeder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DemoSummary melaporkan baris yang ditulis SeedDemo.
type DemoSummary struct {
	Eksekusi, Mentah, Harga, Cuaca int
}

// Zona waktu hari demo: eksekusi contoh jatuh pada jam tetap WIB di hari-hari sebelum hari ini,
// sehingga menjalankan seeder berulang pada hari yang sama tidak menambah baris.
var wib = time.FixedZone("WIB", 7*60*60)

const (
	kodeHarga    = kodeBI
	kodeCuaca    = kodeBMKG
	hariHarga    = 14
	hariCuaca    = 7
	galatDemoBPS = "konfigurasi.variabel SRC-BPS belum lengkap, isi id variabel BPS untuk: ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd"
)

// sumberHarga adalah UUID sumber_data PIHPS yang dipakai harga eksternal contoh.
var sumberHarga = sumberseeder.IDSumber(kodeHarga)

// Kota yang punya harga eksternal contoh: pasar utama dengan data lapangan contoh.
var kotaHargaDemo = []string{"1171", "1108", "1174", "1110", "1109"}

var komoditasHargaDemo = []string{"BRS-MED", "CBM-KRT", "BWM", "TLR"}

type eksekusiDemo struct {
	// Sumber adalah kode sumber (sumber_data.kode); UUID-nya dihitung dari kode itu.
	Sumber    string
	HariLalu  int
	Jam       int
	Menit     int
	Status    string
	Dokumen   int
	Diambil   int
	TakPeta   int
	TidakVal  int
	LatensiMs int
	DurasiMs  int
	Galat     string
}

// jadwalDemo meniru jadwal katalog: PIHPS harian 10.00, BMKG tiap 3 jam, BPS harian 06.00.
var jadwalDemo = []eksekusiDemo{
	{Sumber: "SRC-BI", HariLalu: 3, Jam: 10, Status: domain.StatusSukses, Dokumen: 5, Diambil: 640, TakPeta: 15, TidakVal: 30, LatensiMs: 910, DurasiMs: 9800},
	{Sumber: "SRC-BI", HariLalu: 2, Jam: 10, Status: domain.StatusSukses, Dokumen: 5, Diambil: 655, TakPeta: 15, TidakVal: 20, LatensiMs: 880, DurasiMs: 9300},
	{Sumber: "SRC-BI", HariLalu: 1, Jam: 10, Status: domain.StatusSebagian, Dokumen: 4, Diambil: 520, TakPeta: 12, TidakVal: 45, LatensiMs: 1450, DurasiMs: 21000, Galat: "wilayah=104: GET https://www.bi.go.id/hargapangan/WebSite/TabelHarga/GetGridDataDaerah: status 503"},
	{Sumber: "SRC-BMKG", HariLalu: 1, Jam: 6, Status: domain.StatusSukses, Dokumen: 23, Diambil: 46, TidakVal: 23, LatensiMs: 480, DurasiMs: 30500},
	{Sumber: "SRC-BMKG", HariLalu: 1, Jam: 9, Status: domain.StatusSukses, Dokumen: 23, Diambil: 46, TidakVal: 23, LatensiMs: 495, DurasiMs: 30900},
	{Sumber: "SRC-BMKG", HariLalu: 1, Jam: 12, Status: domain.StatusSebagian, Dokumen: 22, Diambil: 44, TidakVal: 22, LatensiMs: 520, DurasiMs: 31200, Galat: "wilayah=11.05.01.2001: GET https://api.bmkg.go.id/publik/prakiraan-cuaca: status 404"},
	{Sumber: "SRC-BMKG", HariLalu: 1, Jam: 15, Status: domain.StatusSukses, Dokumen: 23, Diambil: 46, TidakVal: 23, LatensiMs: 470, DurasiMs: 30100},
	{Sumber: "SRC-BPS", HariLalu: 2, Jam: 6, Status: domain.StatusGagal, Galat: galatDemoBPS, DurasiMs: 40},
	{Sumber: "SRC-BPS", HariLalu: 1, Jam: 6, Status: domain.StatusGagal, Galat: galatDemoBPS, DurasiMs: 38},
}

// SeedDemo menulis riwayat eksekusi contoh (pemicu DEMO) beserta satu payload bronze tiap run,
// harga eksternal 14 hari untuk komoditas pilot di lima kota, dan cuaca 7 hari untuk semua
// kabupaten/kota. Prasyarat: master dan katalog sumber. Idempoten dalam satu hari dan
// deterministik (PRNG tetap); baris hasil ingestion nyata tidak pernah ditimpa.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	var sum DemoSummary
	hariIni := time.Date(now.In(wib).Year(), now.In(wib).Month(), now.In(wib).Day(), 0, 0, 0, 0, time.UTC)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var eksekusiBI, mentahBI, eksekusiBMKG, mentahBMKG idx.ID
		for _, d := range jadwalDemo {
			mulai := time.Date(hariIni.Year(), hariIni.Month(), hariIni.Day()-d.HariLalu, d.Jam, d.Menit, 0, 0, wib)
			id, err := upsertEksekusi(tx, d, mulai)
			if err != nil {
				return err
			}
			sum.Eksekusi++
			if d.Dokumen == 0 {
				continue
			}
			mid, err := upsertMentah(tx, d.Sumber, id, mulai)
			if err != nil {
				return err
			}
			sum.Mentah++
			// Silver contoh menempel pada run terakhir sumbernya.
			switch d.Sumber {
			case kodeHarga:
				eksekusiBI, mentahBI = id, mid
			case kodeCuaca:
				eksekusiBMKG, mentahBMKG = id, mid
			}
		}

		harga := BuatHargaDemo(hariIni, eksekusiBI, mentahBI)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sumber_id"}, {Name: "komoditas_kode"}, {Name: "kabupaten_kode"}, {Name: "tanggal"}, {Name: "tipe"}},
			DoUpdates: clause.Assignments(map[string]any{"harga": gorm.Expr("EXCLUDED.harga"), "satuan": gorm.Expr("EXCLUDED.satuan"), "data_mentah_id": gorm.Expr("EXCLUDED.data_mentah_id"), "eksekusi_id": gorm.Expr("EXCLUDED.eksekusi_id")}),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "harga_eksternal.eksekusi_id IN (" + sqlEksekusiDemo + ")"}}},
		}).CreateInBatches(&harga, 500).Error; err != nil {
			return fmt.Errorf("seed harga eksternal demo: %w", err)
		}
		sum.Harga = len(harga)

		cuaca := BuatCuacaDemo(hariIni, eksekusiBMKG, mentahBMKG)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "kabupaten_kode"}, {Name: "tanggal"}, {Name: "sumber"}},
			DoUpdates: clause.AssignmentColumns([]string{"curah_hujan_mm", "suhu_c", "kelembapan", "anomali", "eksekusi_id", "data_mentah_id"}),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "cuaca_harian.eksekusi_id IN (" + sqlEksekusiDemo + ")"}}},
		}).CreateInBatches(&cuaca, 500).Error; err != nil {
			return fmt.Errorf("seed cuaca demo: %w", err)
		}
		sum.Cuaca = len(cuaca)
		return nil
	})
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed ingestion demo: %w", err)
	}

	// Gold (gold_harga_harian) dihitung dari agregat lapangan dan harga eksternal.
	if err := refresher.Shared(db, logger.NewDefault()).Refresh(ctx); err != nil {
		return DemoSummary{}, fmt.Errorf("seed ingestion demo: %w", err)
	}
	return sum, nil
}

// sqlEksekusiDemo memilih id eksekusi contoh; hanya baris silver yang berasal dari eksekusi
// contoh yang boleh diperbarui seeder.
const sqlEksekusiDemo = `SELECT id FROM eksekusi_sumber WHERE pemicu = 'DEMO'`

// upsertEksekusi membuat eksekusi contoh bila belum ada untuk (sumber, mulai) dan mengembalikan id-nya.
func upsertEksekusi(tx *gorm.DB, d eksekusiDemo, mulai time.Time) (idx.ID, error) {
	selesai := mulai.Add(time.Duration(d.DurasiMs) * time.Millisecond)
	gagal := 0
	if d.Galat != "" {
		gagal = 1
	}
	ringkasan := fmt.Sprintf(`{"permintaan":{"berhasil":%d,"gagal":%d}}`, d.Dokumen, gagal)
	var galat, latensi any
	if d.Galat != "" {
		galat = d.Galat
	}
	if d.LatensiMs > 0 {
		latensi = d.LatensiMs
	}

	sumberID := sumberseeder.IDSumber(d.Sumber)
	var ids []int64
	err := tx.Raw(`
		INSERT INTO eksekusi_sumber (sumber_id, pemicu, status, mulai, selesai, durasi_ms, latensi_ms, dokumen,
		                             baris_dibaca, baris_diambil, baris_tak_terpetakan, baris_tidak_valid, galat, ringkasan)
		SELECT ?::uuid, 'DEMO', ?::text, ?::timestamptz, ?::timestamptz, ?::int, ?::int, ?::int, ?::int, ?::int, ?::int, ?::int, ?::text, ?::jsonb
		 WHERE NOT EXISTS (SELECT 1 FROM eksekusi_sumber WHERE sumber_id = ?::uuid AND pemicu = 'DEMO' AND mulai = ?::timestamptz)
		RETURNING id`,
		sumberID, d.Status, mulai, selesai, d.DurasiMs, latensi, d.Dokumen,
		d.Diambil+d.TakPeta+d.TidakVal, d.Diambil, d.TakPeta, d.TidakVal, galat, ringkasan,
		sumberID, mulai).Scan(&ids).Error
	if err != nil {
		return 0, fmt.Errorf("seed eksekusi demo %s: %w", d.Sumber, err)
	}
	if len(ids) == 1 {
		return idx.ID(ids[0]), nil
	}
	var id int64
	if err := tx.Raw(`SELECT id FROM eksekusi_sumber WHERE sumber_id = ? AND pemicu = 'DEMO' AND mulai = ?::timestamptz`, sumberID, mulai).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("baca eksekusi demo %s: %w", d.Sumber, err)
	}
	return idx.ID(id), nil
}

// upsertMentah menyimpan satu payload bronze contoh untuk eksekusi dan mengembalikan id-nya.
func upsertMentah(tx *gorm.DB, kode string, eksekusiID idx.ID, mulai time.Time) (idx.ID, error) {
	sumberID := sumberseeder.IDSumber(kode)
	payload := fmt.Sprintf(`{"demo":true,"sumber":%q,"catatan":"payload contoh, bukan respons layanan nyata","diambil":%q}`, kode, mulai.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(payload))
	checksum := hex.EncodeToString(sum[:])

	var id int64
	err := tx.Raw(`
		WITH ins AS (
		    INSERT INTO data_mentah (sumber_id, eksekusi_id, kunci_permintaan, diambil_pada, payload, checksum)
		    VALUES (?::uuid, ?::bigint, 'demo', ?::timestamptz, ?::jsonb, ?::text)
		    ON CONFLICT (sumber_id, checksum) DO NOTHING
		    RETURNING id)
		SELECT id FROM ins UNION ALL SELECT id FROM data_mentah WHERE sumber_id = ?::uuid AND checksum = ?::text LIMIT 1`,
		sumberID, eksekusiID.Int64(), mulai, payload, checksum, sumberID, checksum).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("seed data mentah demo %s: %w", kode, err)
	}
	return idx.ID(id), nil
}

// BuatHargaDemo membangun harga eksternal contoh: harga acuan komoditas pilot dengan noise tetap
// +-3% dan tren pelan, per hari untuk hariHarga hari yang berakhir kemarin. eksekusi dan mentah
// adalah lineage run contoh terakhir; nol berarti tanpa lineage.
func BuatHargaDemo(hariIni time.Time, eksekusi, mentah idx.ID) []domain.HargaEksternal {
	acuan := map[string]masterKomoditas{}
	for _, k := range masterseeder.Komoditas() {
		acuan[k.Kode] = masterKomoditas{harga: float64(k.HargaAcuan), satuan: k.Satuan}
	}

	rng := rand.New(rand.NewPCG(0x50494850, 0x53444d4f))
	var eid, mid *idx.ID
	if eksekusi != 0 {
		eid, mid = &eksekusi, &mentah
	}
	var rows []domain.HargaEksternal
	for _, kota := range kotaHargaDemo {
		for _, kode := range komoditasHargaDemo {
			k := acuan[kode]
			for h := hariHarga; h >= 1; h-- {
				tren := 1 + 0.002*float64(hariHarga-h)
				noise := 1 + (rng.Float64()-0.5)*0.06
				rows = append(rows, domain.HargaEksternal{
					SumberID: sumberHarga, KomoditasKode: kode, KabupatenKode: kota,
					Tanggal: hariIni.AddDate(0, 0, -h), Tipe: domain.TipeHargaKonsumen,
					Harga: math.Round(k.harga*tren*noise/100) * 100, Satuan: k.satuan,
					DataMentahID: mid, EksekusiID: eid,
				})
			}
		}
	}
	return rows
}

type masterKomoditas struct {
	harga  float64
	satuan string
}

// BuatCuacaDemo membangun cuaca harian contoh untuk semua kabupaten/kota selama hariCuaca hari
// yang berakhir hari ini. Hujan lebat (>= 50 mm) ditandai anomali, sama dengan konektor BMKG.
func BuatCuacaDemo(hariIni time.Time, eksekusi, mentah idx.ID) []domain.CuacaHarian {
	rng := rand.New(rand.NewPCG(0x424d4b47, 0x44454d4f))
	var eid, mid *idx.ID
	if eksekusi != 0 {
		eid, mid = &eksekusi, &mentah
	}
	var rows []domain.CuacaHarian
	for _, kab := range masterseeder.Kabupaten() {
		for h := hariCuaca - 1; h >= 0; h-- {
			tanggal := hariIni.AddDate(0, 0, -h)
			hujan := 0.0
			if rng.Float64() < 0.45 {
				hujan = math.Round(rng.Float64()*600) / 10 // 0-60 mm
			}
			rows = append(rows, domain.CuacaHarian{
				KabupatenKode: kab.Kode, Tanggal: tanggal,
				CurahHujanMm: hujan, SuhuC: math.Round((25+rng.Float64()*7)*10) / 10, Kelembapan: 70 + rng.IntN(26),
				Anomali: hujan >= 50, Sumber: "BMKG", EksekusiID: eid, DataMentahID: mid,
			})
		}
	}
	return rows
}
