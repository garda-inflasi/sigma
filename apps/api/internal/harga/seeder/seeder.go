// Package seeder mengisi histori entri harga demo: sembilan puluh hari entri
// petugas lapangan di pasar utama tiap kabupaten, dengan sebaran status,
// outlier, duplikat, dan entri di luar geofence. Komoditas, pasar, dan akun
// berasal dari seeder master.reference dan auth.demo.
package seeder

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Jumlah hari histori, termasuk hari ini.
const HariHistori = 90

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Entri   int
	Riwayat int
}

// alasanTolak adalah alasan penolakan entri demo; dipilih menurut urutan entri agar deterministik.
var alasanTolak = []string{
	"Harga di luar rentang wajar dan tanpa bukti pendukung",
	"Satuan harga diduga salah, mohon survei ulang",
	"Lokasi pencatatan di luar area pasar",
}

// Komoditas adalah komoditas master yang dicatat petugas.
type Komoditas struct {
	Kode       string `gorm:"column:kode"`
	Satuan     string `gorm:"column:satuan"`
	HargaAcuan int    `gorm:"column:harga_acuan"`
}

// Petugas adalah petugas lapangan beserta pasar utama di kabupatennya.
type Petugas struct {
	ID            string  `gorm:"column:id"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	PasarID       string  `gorm:"column:pasar_id"`
	PasarLat      float64 `gorm:"column:pasar_lat"`
	PasarLng      float64 `gorm:"column:pasar_lng"`
	RadiusMeter   int     `gorm:"column:radius_meter"`
}

// Input adalah data acuan yang dibaca dari basis data.
type Input struct {
	Komoditas []Komoditas
	Petugas   []Petugas
	// Validator memvalidasi entri; Publikator mempublikasikannya. Keduanya bukan petugas pencatat.
	Validator  string
	Publikator string
}

// guncangan menggeser harga sebuah komoditas: puncak di hari ke-pusat (0 = awal
// histori, HariHistori-1 = hari ini), selebar lebar hari, setinggi amp (negatif = turun).
type guncangan struct {
	pusat, lebar, amp float64
}

// Skenario demo: cabai dalam tekanan (curah hujan tinggi di sentra Gayo), bawang
// naik moderat, kentang dan tomat surplus di dataran tinggi.
var guncanganKomoditas = map[string]guncangan{
	"CBM-KRT": {93, 6.5, 0.38},
	"CBR-RWT": {92, 6, 0.30},
	"BWM":     {88, 14, 0.12},
	"IKN-TGK": {40, 6, 0.15},
	"KTG":     {86, 12, -0.18},
	"TMT":     {91, 7, -0.26},
}

// premiWilayah adalah selisih harga wilayah (kepulauan dan terpencil lebih mahal).
var premiWilayah = map[string]float64{"1101": 0.11, "1104": 0.06, "1103": 0.04, "1171": 0.03, "1106": -0.04, "1117": -0.03}

// komoditasProdusen dicatat juga sebagai harga produsen (kelompok tani) oleh tiap petugas.
var komoditasProdusen = map[string]bool{"CBM-KRT": true, "BWM": true, "KTG": true, "TMT": true}

const (
	rasioProdusen    = 0.72
	pOutlier         = 0.06
	pLuarGeofence    = 0.06
	pDuplikat        = 0.02
	pFoto            = 0.75
	jamMulaiLapangan = 6 * time.Hour
	rentangLapangan  = 8 * time.Hour
)

// SeedDemo mengisi histori entri harga. Idempoten: id entri ditentukan oleh
// urutan hari, petugas, dan komoditas, sehingga menjalankannya ulang menyegarkan
// tanggal relatif terhadap now tanpa menggandakan baris. Hanya untuk demo.
//
// Prasyarat: seeder master.reference dan auth.demo sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	in, err := loadInput(ctx, db)
	if err != nil {
		return DemoSummary{}, err
	}
	entri := Build(in, now)

	riwayat := BuildRiwayat(entri)

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).CreateInBatches(&entri, 500).Error
		if err != nil {
			return fmt.Errorf("seed entri harga: %w", err)
		}

		// Riwayat entri demo ditulis ulang bersama entrinya supaya waktunya mengikuti now.
		ids := make([]string, len(entri))
		for i, e := range entri {
			ids[i] = e.ID
		}
		for start := 0; start < len(ids); start += hapusBatch {
			batch := ids[start:min(start+hapusBatch, len(ids))]
			if err := tx.Exec("DELETE FROM riwayat_status_entri WHERE entri_id IN ?", batch).Error; err != nil {
				return fmt.Errorf("bersihkan riwayat status demo: %w", err)
			}
		}
		if err := tx.CreateInBatches(&riwayat, 1000).Error; err != nil {
			return fmt.Errorf("seed riwayat status: %w", err)
		}
		return nil
	})
	if err != nil {
		return DemoSummary{}, err
	}
	return DemoSummary{Entri: len(entri), Riwayat: len(riwayat)}, nil
}

// hapusBatch membatasi jumlah id per DELETE agar di bawah batas parameter PostgreSQL.
const hapusBatch = 5000

// BuildRiwayat menurunkan riwayat status dari jejak tiap entri demo: dicatat, divalidasi,
// dipublikasi, atau ditolak. Entri DRAF hanya tercatat sebagai draf.
func BuildRiwayat(entri []domain.EntriHarga) []domain.RiwayatStatusEntri {
	var out []domain.RiwayatStatusEntri
	tambah := func(e domain.EntriHarga, dari *string, ke string, oleh *string, waktu time.Time, alasan *string) {
		out = append(out, domain.RiwayatStatusEntri{EntriID: e.ID, Dari: dari, Ke: ke, Oleh: oleh, Waktu: waktu, Alasan: alasan})
	}
	str := func(s string) *string { return &s }

	for _, e := range entri {
		if e.Status == domain.StatusDraf {
			tambah(e, nil, domain.StatusDraf, e.PetugasID, e.DibuatPada, nil)
			continue
		}
		tambah(e, nil, domain.StatusDiajukan, e.PetugasID, e.DibuatPada, nil)
		switch e.Status {
		case domain.StatusDivalidasi, domain.StatusDipublikasi:
			tambah(e, str(domain.StatusDiajukan), domain.StatusDivalidasi, e.DivalidasiOleh, *e.DivalidasiPada, nil)
			if e.Status == domain.StatusDipublikasi {
				tambah(e, str(domain.StatusDivalidasi), domain.StatusDipublikasi, e.DipublikasiOleh, *e.DipublikasiPada, nil)
			}
		case domain.StatusDitolak:
			tambah(e, str(domain.StatusDiajukan), domain.StatusDitolak, e.DitolakOleh, *e.DitolakPada, e.AlasanPenolakan)
		}
	}
	return out
}

func loadInput(ctx context.Context, db *gorm.DB) (Input, error) {
	var in Input
	tx := db.WithContext(ctx)

	if err := tx.Raw(`SELECT kode, satuan, harga_acuan FROM komoditas WHERE status = 'AKTIF' ORDER BY kode`).Scan(&in.Komoditas).Error; err != nil {
		return in, fmt.Errorf("baca komoditas: %w", err)
	}
	err := tx.Raw(`
		SELECT u.id, u.kabupaten_kode, p.id AS pasar_id, p.lat AS pasar_lat, p.lng AS pasar_lng, p.radius_meter
		  FROM pengguna u
		  JOIN LATERAL (
		       SELECT id, lat, lng, radius_meter FROM pasar
		        WHERE kabupaten_kode = u.kabupaten_kode AND aktif ORDER BY id LIMIT 1
		  ) p ON true
		 WHERE u.peran = 'OPS_MAHASISWA' AND u.status = 'AKTIF'
		 ORDER BY u.kabupaten_kode, u.id`).Scan(&in.Petugas).Error
	if err != nil {
		return in, fmt.Errorf("baca petugas: %w", err)
	}
	if len(in.Komoditas) == 0 || len(in.Petugas) == 0 {
		return in, fmt.Errorf("komoditas atau petugas lapangan belum ada; jalankan seeder master.reference dan auth.demo lebih dulu")
	}

	for _, peran := range []struct {
		nama string
		dst  *string
	}{{"OPS_KOORDINATOR", &in.Validator}, {"MANAJER_DATA", &in.Publikator}} {
		var ids []string
		if err := tx.Raw(`SELECT id FROM pengguna WHERE peran = ? AND status = 'AKTIF' ORDER BY id LIMIT 1`, peran.nama).Scan(&ids).Error; err != nil {
			return in, fmt.Errorf("baca akun %s: %w", peran.nama, err)
		}
		if len(ids) == 0 {
			return in, fmt.Errorf("akun berperan %s belum ada; jalankan seeder auth.demo lebih dulu", peran.nama)
		}
		*peran.dst = ids[0]
	}
	return in, nil
}

// Build menyusun entri demo. Hasilnya deterministik untuk Input dan now yang sama.
func Build(in Input, now time.Time) []domain.EntriHarga {
	rng := rand.New(rand.NewPCG(0x47415244, 0x41494e46))
	lokal := now.In(domain.WIB)
	hariIni := time.Date(lokal.Year(), lokal.Month(), lokal.Day(), 0, 0, 0, 0, domain.WIB)

	var out []domain.EntriHarga
	for offset := 0; offset < HariHistori; offset++ { // offset 0 = hari ini
		awalHari := hariIni.AddDate(0, 0, -offset)
		indeksHari := float64(HariHistori - 1 - offset)
		batasWaktu := awalHari.Add(rentangLapangan + jamMulaiLapangan)
		if offset == 0 {
			batasWaktu = now
		}

		for pi, p := range in.Petugas {
			for ki, k := range in.Komoditas {
				tipe := []string{domain.TipeKonsumen}
				if komoditasProdusen[k.Kode] {
					tipe = append(tipe, domain.TipeProdusen)
				}
				for _, t := range tipe {
					if offset == 0 && rng.Float64() >= cakupanHariIni(pi, len(in.Petugas)) {
						continue
					}
					e := buatEntri(rng, in, p, k, t, offset, pi, ki, indeksHari, awalHari, batasWaktu, now)
					out = append(out, e)
					if offset <= 1 && rng.Float64() < pDuplikat {
						out = append(out, duplikat(e, now))
					}
				}
			}
		}
	}
	return out
}

// cakupanHariIni adalah peluang sebuah harga sudah tercatat hari ini; petugas
// berindeks lebih tinggi lebih lengkap, sehingga capaian antarwilayah bervariasi.
func cakupanHariIni(indeks, jumlah int) float64 {
	if jumlah <= 1 {
		return 1
	}
	return 0.45 + 0.5*float64(indeks)/float64(jumlah-1)
}

func buatEntri(rng *rand.Rand, in Input, p Petugas, k Komoditas, tipe string, offset, pi, ki int, indeksHari float64, awalHari, batasWaktu, now time.Time) domain.EntriHarga {
	acuan := float64(k.HargaAcuan) * (1 + premiWilayah[p.KabupatenKode]) * (1 + geser(k.Kode, indeksHari))
	if tipe == domain.TipeProdusen {
		acuan *= rasioProdusen
	}

	deviasi := rng.NormFloat64() * 2
	outlier := rng.Float64() < pOutlier
	if outlier {
		deviasi = (28 + rng.Float64()*30) * tanda(rng)
	}
	harga := max(500, int(math.Round(acuan*(1+deviasi/100)/100)*100))

	luar := rng.Float64() < pLuarGeofence
	lat, lng := posisi(rng, p, luar)

	dicatat := waktuCatat(rng, awalHari, batasWaktu, offset)
	suffix := "K"
	if tipe == domain.TipeProdusen {
		suffix = "P"
	}
	id := fmt.Sprintf("HRG-D%02d-%02d-%02d-%s", offset, pi, ki, suffix)

	petugas := p.ID
	pedagang := fmt.Sprintf("PDG-%03d", (pi*37+ki*11+offset)%400)
	e := domain.EntriHarga{
		ID: id, KomoditasKode: k.Kode, PasarID: p.PasarID, Tipe: tipe, Harga: harga, Satuan: k.Satuan,
		Kualitas: pilihKualitas(rng), Sumber: "LAPANGAN", Status: domain.StatusDiajukan,
		DeviasiPct: math.Round(deviasi*10) / 10, SkorAnomali: math.Round(math.Min(1, math.Abs(deviasi)/40+rng.Float64()*0.05)*100) / 100,
		Lat: &lat, Lng: &lng, DalamGeofence: !luar, KodePedagang: &pedagang,
		DicatatPada: dicatat, PetugasID: &petugas, DibuatPada: dicatat, DiupdatePada: dicatat,
	}
	if rng.Float64() < pFoto {
		foto := "demo/foto/" + id + ".jpg"
		e.FotoURL = &foto
	}

	e.Status = pilihStatus(rng, offset, outlier)
	terapkanJejak(&e, in, dicatat, now, pi*100+ki)
	return e
}

// geser mengembalikan guncangan harga komoditas pada hari ke-indeksHari (gelombang
// berbentuk lonceng) ditambah riak musiman kecil.
func geser(kode string, indeksHari float64) float64 {
	g, ada := guncanganKomoditas[kode]
	if !ada {
		return 0
	}
	z := (indeksHari - g.pusat) / g.lebar
	return g.amp * math.Exp(-z*z)
}

func tanda(rng *rand.Rand) float64 {
	if rng.Float64() < 0.5 {
		return -1
	}
	return 1
}

// waktuCatat menyebar waktu pencatatan pada jam kerja lapangan (06.00 sampai 14.00 WIB),
// atau sampai batasWaktu bila hari itu belum selesai.
func waktuCatat(rng *rand.Rand, awalHari, batasWaktu time.Time, offset int) time.Time {
	mulai := awalHari.Add(jamMulaiLapangan)
	if offset == 0 {
		mulai = awalHari
	}
	span := batasWaktu.Sub(mulai)
	if span <= 0 {
		return batasWaktu
	}
	return mulai.Add(time.Duration(rng.Float64() * float64(span))).Truncate(time.Second)
}

// posisi menaruh titik GPS di dalam radius pasar, atau 1,2 sampai 2 km darinya bila luar.
func posisi(rng *rand.Rand, p Petugas, luar bool) (lat, lng float64) {
	jarak := rng.Float64() * 0.6 * float64(p.RadiusMeter)
	if luar {
		jarak = 1200 + rng.Float64()*800
	}
	sudut := rng.Float64() * 2 * math.Pi
	const meterPerDerajat = 111_320.0
	lat = p.PasarLat + jarak*math.Sin(sudut)/meterPerDerajat
	lng = p.PasarLng + jarak*math.Cos(sudut)/(meterPerDerajat*math.Cos(p.PasarLat*math.Pi/180))
	return math.Round(lat*1e6) / 1e6, math.Round(lng*1e6) / 1e6
}

func pilihKualitas(rng *rand.Rand) string {
	switch v := rng.Float64(); {
	case v < 0.2:
		return "PREMIUM"
	case v < 0.7:
		return "MEDIUM"
	default:
		return "ASALAN"
	}
}

// pilihStatus menyebar status menurut umur entri: yang lama hampir semuanya
// terbit, yang baru menunggu validasi. Outlier lama sudah ditolak.
func pilihStatus(rng *rand.Rand, offset int, outlier bool) string {
	if outlier {
		if offset <= 1 {
			return domain.StatusDiajukan
		}
		return domain.StatusDitolak
	}

	// Bobot kumulatif: DIPUBLIKASI, DIVALIDASI, DIAJUKAN, DITOLAK, sisanya DRAF.
	var bobot [4]float64
	switch {
	case offset >= 4:
		bobot = [4]float64{0.97, 0.97, 0.97, 1}
	case offset >= 2:
		bobot = [4]float64{0.55, 0.90, 0.96, 1}
	case offset == 1:
		bobot = [4]float64{0.20, 0.60, 0.90, 0.95}
	default:
		bobot = [4]float64{0.05, 0.20, 0.80, 0.85}
	}
	status := [4]string{domain.StatusDipublikasi, domain.StatusDivalidasi, domain.StatusDiajukan, domain.StatusDitolak}

	v := rng.Float64()
	for i, batas := range bobot {
		if v < batas {
			return status[i]
		}
	}
	return domain.StatusDraf
}

// terapkanJejak mengisi validator dan penerbit sesuai status, tidak melampaui now.
func terapkanJejak(e *domain.EntriHarga, in Input, dicatat, now time.Time, urutan int) {
	tidakLewatSekarang := func(t time.Time) time.Time {
		if t.After(now) {
			return now
		}
		return t
	}

	switch e.Status {
	case domain.StatusDivalidasi, domain.StatusDipublikasi:
		validator, pada := in.Validator, tidakLewatSekarang(dicatat.Add(2*time.Hour))
		e.DivalidasiOleh, e.DivalidasiPada, e.DiupdatePada = &validator, &pada, pada
	}
	if e.Status == domain.StatusDitolak {
		validator, pada, alasan := in.Validator, tidakLewatSekarang(dicatat.Add(2*time.Hour)), alasanTolak[urutan%len(alasanTolak)]
		e.DitolakOleh, e.DitolakPada, e.AlasanPenolakan, e.DiupdatePada = &validator, &pada, &alasan, pada
	}
	if e.Status == domain.StatusDipublikasi {
		penerbit, pada := in.Publikator, tidakLewatSekarang(e.DivalidasiPada.Add(3*time.Hour))
		e.DipublikasiOleh, e.DipublikasiPada, e.DiupdatePada = &penerbit, &pada, pada
	}
}

// duplikat menyalin entri sebagai pengiriman ulang lima menit kemudian.
func duplikat(e domain.EntriHarga, now time.Time) domain.EntriHarga {
	d := e
	d.ID = e.ID + "-dup"
	d.Status = domain.StatusDiajukan
	d.DivalidasiOleh, d.DivalidasiPada, d.DipublikasiOleh, d.DipublikasiPada = nil, nil, nil, nil
	d.DitolakOleh, d.DitolakPada, d.AlasanPenolakan = nil, nil, nil
	d.DicatatPada = e.DicatatPada.Add(5 * time.Minute)
	if d.DicatatPada.After(now) {
		d.DicatatPada = now
	}
	d.DibuatPada, d.DiupdatePada = d.DicatatPada, d.DicatatPada
	return d
}
