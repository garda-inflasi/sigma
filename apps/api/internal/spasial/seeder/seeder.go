// Package seeder mengisi data demo peta: neraca wilayah, usulan rute distribusi,
// dan kandidat cold storage. Semuanya berlabel sumber DEMO. Angka yang tampak
// resmi (neraca pangan) tidak boleh ada di produksi, jadi tidak ada lapis reference.
// Wilayah, komoditas, dan koordinat berasal dari master.reference.
package seeder

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SumberDemo menandai baris yang bukan data resmi.
const SumberDemo = "DEMO"

// Parameter rute: tarif per ton-km, faktor jalan terhadap jarak garis lurus, kecepatan rata-rata (km/jam).
const (
	tarifTonKm  = 4200.0
	faktorJalan = 1.35
	kecepatanKm = 45.0
	jumlahRute  = 5
	// Wilayah dengan deviasi di atas batas ini dianggap tertekan dan layak mendapat pasokan.
	batasTertekan = 5.0
)

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Neraca, Rute, Kandidat int
}

type Koordinat struct{ Lat, Lng float64 }

// SeedDemo meng-upsert neraca (periode = awal bulan now), rute, dan kandidat.
// Idempoten dalam bulan yang sama. Hanya untuk demo.
//
// Prasyarat: seeder master.reference sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	var sum DemoSummary
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wilayah []struct {
			Kode     string
			Lat, Lng float64
		}
		if err := tx.Raw("SELECT kode, lat, lng FROM kabupaten").Scan(&wilayah).Error; err != nil {
			return fmt.Errorf("baca kabupaten: %w", err)
		}
		koordinat := make(map[string]Koordinat, len(wilayah))
		for _, w := range wilayah {
			koordinat[w.Kode] = Koordinat{w.Lat, w.Lng}
		}
		if len(koordinat) == 0 {
			return fmt.Errorf("kabupaten kosong; jalankan seeder master.reference lebih dulu (make db/seed/reference)")
		}

		neraca := BuildNeraca(now, KodeWilayah(koordinat))
		rute := BuildRute(neraca, koordinat, now)
		kandidat, err := BuildKandidat(now)
		if err != nil {
			return err
		}

		steps := []struct {
			name string
			rows any
			key  []string
			cols []string
		}{
			{"neraca", &neraca, []string{"komoditas_kode", "kabupaten_kode", "periode"}, []string{"surplus_defisit_ton", "delta_harga_pct", "sumber"}},
			{"rute", &rute, []string{"id"}, []string{"asal_kode", "tujuan_kode", "komoditas_kode", "volume_ton", "jarak_km", "jam_tempuh", "biaya", "status", "sumber", "dibuat_pada"}},
			{"kandidat", &kandidat, []string{"id"}, []string{"kabupaten_kode", "lokasi", "skor", "komponen_skor", "alasan", "status", "sumber", "dibuat_pada"}},
		}
		for _, step := range steps {
			keys := make([]clause.Column, len(step.key))
			for i, k := range step.key {
				keys[i] = clause.Column{Name: k}
			}
			err := tx.Clauses(clause.OnConflict{Columns: keys, DoUpdates: clause.AssignmentColumns(step.cols)}).
				CreateInBatches(step.rows, 100).Error
			if err != nil {
				return fmt.Errorf("seed %s: %w", step.name, err)
			}
		}
		sum = DemoSummary{Neraca: len(neraca), Rute: len(rute), Kandidat: len(kandidat)}
		return nil
	})
	return sum, err
}

// KodeWilayah mengembalikan kode wilayah terurut dari peta koordinat.
func KodeWilayah(koordinat map[string]Koordinat) []string {
	kode := make([]string, 0, len(koordinat))
	for k := range koordinat {
		kode = append(kode, k)
	}
	sort.Strings(kode)
	return kode
}

// Periode adalah tanggal 1 pada bulan now (UTC).
func Periode(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// BuildNeraca menyusun neraca tiap komoditas x wilayah. Deviasi harga acak
// (PRNG tetap) dibatasi di bawah batas tertekan, kecuali titik panas yang dituliskan
// eksplisit agar selaras dengan alert EWS demo. Surplus/defisit berlawanan arah
// dengan deviasi: harga naik berarti pasokan kurang.
func BuildNeraca(now time.Time, wilayah []string) []domain.NeracaWilayah {
	periode := Periode(now)
	rng := rand.New(rand.NewPCG(20261016, 7))
	out := make([]domain.NeracaWilayah, 0, len(komoditasNeraca)*len(wilayah))
	for _, k := range komoditasNeraca {
		for _, w := range wilayah {
			delta, hot := titikPanas[titik{k.kode, w}]
			if !hot {
				delta = math.Round(math.Max(-4.5, math.Min(4.5, rng.NormFloat64()*2.5))*10) / 10
			}
			out = append(out, domain.NeracaWilayah{
				KomoditasKode: k.kode, KabupatenKode: w, Periode: periode,
				SurplusDefisitTon: math.Round(-delta*k.tonPerPoin*10) / 10, DeltaHargaPct: delta, Sumber: SumberDemo,
			})
		}
	}
	return out
}

// BuildRute menyusun usulan rute dari sentra surplus ke lima wilayah paling tertekan
// (deviasi tertinggi di atas batas tertekan), bergiliran antar sentra.
func BuildRute(neraca []domain.NeracaWilayah, koordinat map[string]Koordinat, now time.Time) []domain.RuteDistribusi {
	teratas := map[string]float64{}
	for _, n := range neraca {
		if n.DeltaHargaPct > teratas[n.KabupatenKode] {
			teratas[n.KabupatenKode] = n.DeltaHargaPct
		}
	}
	type tertekan struct {
		kode  string
		delta float64
	}
	var daftar []tertekan
	for kode, delta := range teratas {
		if delta > batasTertekan {
			daftar = append(daftar, tertekan{kode, delta})
		}
	}
	sort.Slice(daftar, func(i, j int) bool {
		if daftar[i].delta != daftar[j].delta {
			return daftar[i].delta > daftar[j].delta
		}
		return daftar[i].kode < daftar[j].kode
	})
	if len(daftar) > jumlahRute {
		daftar = daftar[:jumlahRute]
	}

	var out []domain.RuteDistribusi
	for i, t := range daftar {
		sentra := sentraSurplus[i%len(sentraSurplus)]
		asal, ok1 := koordinat[sentra.kode]
		tujuan, ok2 := koordinat[t.kode]
		if !ok1 || !ok2 || sentra.kode == t.kode {
			continue
		}
		km := jarakMeter(asal, tujuan) / 1000 * faktorJalan
		volume := math.Round(8 + t.delta*1.6)
		out = append(out, domain.RuteDistribusi{
			ID: fmt.Sprintf("RT-%s-%s", sentra.kode, t.kode), AsalKode: sentra.kode, TujuanKode: t.kode,
			KomoditasKode: sentra.komoditas, VolumeTon: volume, JarakKm: int(math.Round(km)),
			JamTempuh: math.Round(km/kecepatanKm*10) / 10, Biaya: int64(math.Round(km * volume * tarifTonKm)),
			Status: "USULAN", Sumber: SumberDemo, DibuatPada: now.UTC(),
		})
	}
	return out
}

// BuildKandidat menyusun kandidat cold storage; skor = rata-rata tertimbang komponen.
func BuildKandidat(now time.Time) ([]domain.KandidatColdStorage, error) {
	out := make([]domain.KandidatColdStorage, len(kandidatSeeds))
	for i, k := range kandidatSeeds {
		komponen, err := json.Marshal(k.komponen)
		if err != nil {
			return nil, fmt.Errorf("komponen skor %s: %w", k.id, err)
		}
		out[i] = domain.KandidatColdStorage{
			ID: k.id, KabupatenKode: k.kabupaten, Lokasi: k.lokasi, Skor: Skor(k.komponen),
			KomponenSkor: string(komponen), Alasan: k.alasan, Status: k.status, Sumber: SumberDemo, DibuatPada: now.UTC(),
		}
	}
	return out, nil
}

// Skor menghitung skor 0-100 sebagai rata-rata tertimbang komponen (bobot di bobotSkor).
func Skor(komponen map[string]float64) int {
	var total float64
	for nama, bobot := range bobotSkor {
		total += komponen[nama] * bobot
	}
	return int(math.Round(total))
}

// jarakMeter adalah jarak garis lurus (haversine) dalam meter.
func jarakMeter(a, b Koordinat) float64 {
	const radiusBumi = 6371e3
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLng := rad(b.Lat-a.Lat), rad(b.Lng-a.Lng)
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(rad(a.Lat))*math.Cos(rad(b.Lat))*math.Pow(math.Sin(dLng/2), 2)
	return 2 * radiusBumi * math.Asin(math.Sqrt(h))
}
