package domain

import "time"

// Ringkasan adalah satu layar dashboard eksekutif. Blok yang datanya belum ada
// bernilai nil atau kosong; Demo benar bila ada angka ber-sumber DEMO di dalamnya.
type Ringkasan struct {
	Demo       bool
	Inflasi    *InflasiRingkas
	Neraca     *NeracaRingkas
	Fiskal     *FiskalRingkas
	EWS        map[string]int
	Intervensi map[string]IntervensiRingkas
	// SiklusDominan adalah siklus dengan kartu intervensi aktif terbanyak.
	SiklusDominan string
}

type InflasiRingkas struct {
	Periode     time.Time
	Sumber      string
	DiimporPada time.Time
	IHK         float64
	Yoy         float64
	Mtm         float64
	Ytd         float64
	Sasaran     Sasaran
	Seri        []TitikSeri
	Penyumbang  []Penyumbang
}

type Sasaran struct {
	Tahun       int
	Bawah, Atas float64
}

type TitikSeri struct {
	Periode time.Time
	Yoy     float64
}

type NeracaRingkas struct {
	Periode time.Time
	Sumber  string
	Items   []NeracaItem
}

type FiskalRingkas struct {
	Sumber     string
	PerTanggal time.Time
	Pagu       int64
	Realisasi  int64
	Skor       int
	Status     string
	Instrumen  []RuangFiskal
}

type IntervensiRingkas struct {
	Aktif, Selesai int
}
