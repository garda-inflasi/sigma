package domain

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Optimasi rute distribusi (12-C): masalah transportasi min-cost flow dari wilayah
// surplus ke wilayah defisit. Penyelesaian eksak (successive shortest path), bukan
// heuristik serakah: volume yang tersalurkan dimaksimalkan lebih dulu, lalu di antara
// semua rencana dengan volume maksimum dipilih yang jarak ton-km-nya terkecil.
// Tarif per ton-km konstan, sehingga meminimalkan ton-km sama dengan meminimalkan biaya.

// satuanTon adalah resolusi volume: neraca disimpan dengan satu desimal ton.
const satuanTon = 10

// Jalur adalah jarak dan waktu tempuh antara dua wilayah.
type Jalur struct{ JarakKm, JamTempuh float64 }

// JarakFunc mengembalikan jalur asal -> tujuan.
type JarakFunc func(asal, tujuan string) Jalur

// SisaWilayah adalah volume (ton) yang tersisa pada satu wilayah.
type SisaWilayah struct {
	Kode string
	Ton  float64
}

// ParamOptimasi mengatur biaya dan kendala. MaksJamTempuh > 0 melarang rute yang
// lebih lama dari batas itu (mis. komoditas mudah rusak).
type ParamOptimasi struct {
	TarifTonKm    float64
	MaksJamTempuh float64
}

// Alokasi adalah satu rute dalam rencana: volume dari asal ke tujuan.
type Alokasi struct {
	AsalKode, TujuanKode string
	VolumeTon            float64
	JarakKm, JamTempuh   float64
	Biaya                int64
}

// HasilOptimasi adalah rencana distribusi untuk satu komoditas.
type HasilOptimasi struct {
	Alokasi        []Alokasi
	TotalVolumeTon float64
	TotalBiaya     int64
	// BelumTerpenuhi adalah defisit yang tidak tertutup (pasokan kurang atau rute terlarang).
	BelumTerpenuhi []SisaWilayah
	// SurplusSisa adalah surplus yang tidak tersalurkan.
	SurplusSisa []SisaWilayah
}

// ErrOptimasiTidakValid dibungkus oleh galat masukan optimasi.
var ErrOptimasiTidakValid = errors.New("masukan optimasi tidak valid")

const infFlow = math.MaxInt64 / 4

type busur struct {
	ke, kap int
	biaya   int64
	balik   int
}

type graf struct {
	adj [][]busur
}

func (g *graf) tambah(dari, ke int, kap int, biaya int64) int {
	g.adj[dari] = append(g.adj[dari], busur{ke: ke, kap: kap, biaya: biaya, balik: len(g.adj[ke])})
	g.adj[ke] = append(g.adj[ke], busur{ke: dari, kap: 0, biaya: -biaya, balik: len(g.adj[dari]) - 1})
	return len(g.adj[dari]) - 1
}

// Optimalkan menghitung rencana distribusi min-cost. pasokan dan kebutuhan
// berisi volume positif per wilayah (ton, resolusi 0,1); kode yang sama tidak boleh
// muncul di keduanya. Rute dengan waktu tempuh melebihi p.MaksJamTempuh dilarang.
func Optimalkan(pasokan, kebutuhan []SisaWilayah, jarak JarakFunc, p ParamOptimasi) (HasilOptimasi, error) {
	sup, dem, err := periksaMasukan(pasokan, kebutuhan, p)
	if err != nil {
		return HasilOptimasi{}, err
	}

	m, n := len(sup), len(dem)
	sumber, sink := 0, m+n+1
	g := &graf{adj: make([][]busur, m+n+2)}
	for i, s := range sup {
		g.tambah(sumber, 1+i, satuan(s.Ton), 0)
	}
	for j, d := range dem {
		g.tambah(1+m+j, sink, satuan(d.Ton), 0)
	}
	type indeksBusur struct{ i, j, idx int }
	var arcs []indeksBusur
	jalur := make(map[[2]int]Jalur)
	for i, s := range sup {
		for j, d := range dem {
			t := jarak(s.Kode, d.Kode)
			if t.JarakKm < 0 || t.JamTempuh < 0 || math.IsNaN(t.JarakKm) {
				return HasilOptimasi{}, fmt.Errorf("%w: jarak %s-%s tidak valid", ErrOptimasiTidakValid, s.Kode, d.Kode)
			}
			if p.MaksJamTempuh > 0 && t.JamTempuh > p.MaksJamTempuh {
				continue
			}
			jalur[[2]int{i, j}] = t
			// Biaya busur dalam seperseratus km per ton agar bilangan bulat.
			idx := g.tambah(1+i, 1+m+j, min(satuan(s.Ton), satuan(d.Ton)), int64(math.Round(t.JarakKm*100)))
			arcs = append(arcs, indeksBusur{i, j, idx})
		}
	}

	g.alirMinBiaya(sumber, sink)

	var hasil HasilOptimasi
	sisaSup := make([]int, m)
	sisaDem := make([]int, n)
	for i, s := range sup {
		sisaSup[i] = satuan(s.Ton)
	}
	for j, d := range dem {
		sisaDem[j] = satuan(d.Ton)
	}
	for _, a := range arcs {
		b := g.adj[1+a.i][a.idx]
		aliran := g.adj[b.ke][b.balik].kap // kapasitas busur balik = aliran maju
		if aliran == 0 {
			continue
		}
		t := jalur[[2]int{a.i, a.j}]
		vol := float64(aliran) / satuanTon
		biaya := int64(math.Round(vol * t.JarakKm * p.TarifTonKm))
		hasil.Alokasi = append(hasil.Alokasi, Alokasi{
			AsalKode: sup[a.i].Kode, TujuanKode: dem[a.j].Kode, VolumeTon: vol,
			JarakKm: t.JarakKm, JamTempuh: t.JamTempuh, Biaya: biaya,
		})
		hasil.TotalVolumeTon += vol
		hasil.TotalBiaya += biaya
		sisaSup[a.i] -= aliran
		sisaDem[a.j] -= aliran
	}
	hasil.TotalVolumeTon = math.Round(hasil.TotalVolumeTon*satuanTon) / satuanTon
	sort.Slice(hasil.Alokasi, func(x, y int) bool {
		if hasil.Alokasi[x].AsalKode != hasil.Alokasi[y].AsalKode {
			return hasil.Alokasi[x].AsalKode < hasil.Alokasi[y].AsalKode
		}
		return hasil.Alokasi[x].TujuanKode < hasil.Alokasi[y].TujuanKode
	})
	for i, s := range sisaSup {
		if s > 0 {
			hasil.SurplusSisa = append(hasil.SurplusSisa, SisaWilayah{sup[i].Kode, float64(s) / satuanTon})
		}
	}
	for j, d := range sisaDem {
		if d > 0 {
			hasil.BelumTerpenuhi = append(hasil.BelumTerpenuhi, SisaWilayah{dem[j].Kode, float64(d) / satuanTon})
		}
	}
	return hasil, nil
}

// periksaMasukan memeriksa dan menormalkan pasokan dan kebutuhan.
func periksaMasukan(pasokan, kebutuhan []SisaWilayah, p ParamOptimasi) (sup, dem []SisaWilayah, err error) {
	if p.TarifTonKm < 0 || p.MaksJamTempuh < 0 {
		return nil, nil, fmt.Errorf("%w: tarif dan batas waktu tidak boleh negatif", ErrOptimasiTidakValid)
	}
	if sup, err = normalisasi(pasokan, "pasokan"); err != nil {
		return nil, nil, err
	}
	if dem, err = normalisasi(kebutuhan, "kebutuhan"); err != nil {
		return nil, nil, err
	}
	for _, d := range dem {
		for _, s := range sup {
			if s.Kode == d.Kode {
				return nil, nil, fmt.Errorf("%w: wilayah %s tidak boleh surplus dan defisit sekaligus", ErrOptimasiTidakValid, s.Kode)
			}
		}
	}
	return sup, dem, nil
}

// Serakah menghitung rencana pembanding: berulang kali pasangan pemasok-peminta
// dengan jarak terdekat yang masih bisa mengirim dipenuhi sebanyak mungkin. Cepat
// dan masuk akal bagi perencana manual, tetapi tidak optimal; dipakai sebagai
// pembanding penghematan biaya Optimalkan (KPI efisiensi biaya logistik).
func Serakah(pasokan, kebutuhan []SisaWilayah, jarak JarakFunc, p ParamOptimasi) (HasilOptimasi, error) {
	sup, dem, err := periksaMasukan(pasokan, kebutuhan, p)
	if err != nil {
		return HasilOptimasi{}, err
	}
	sisaSup, sisaDem := make([]int, len(sup)), make([]int, len(dem))
	for i, s := range sup {
		sisaSup[i] = satuan(s.Ton)
	}
	for j, d := range dem {
		sisaDem[j] = satuan(d.Ton)
	}

	var hasil HasilOptimasi
	for {
		bi, bj, terdekat := -1, -1, math.Inf(1)
		var jalur Jalur
		for i, s := range sup {
			for j, d := range dem {
				if sisaSup[i] == 0 || sisaDem[j] == 0 {
					continue
				}
				t := jarak(s.Kode, d.Kode)
				if t.JarakKm < 0 || t.JamTempuh < 0 || math.IsNaN(t.JarakKm) {
					return HasilOptimasi{}, fmt.Errorf("%w: jarak %s-%s tidak valid", ErrOptimasiTidakValid, s.Kode, d.Kode)
				}
				if p.MaksJamTempuh > 0 && t.JamTempuh > p.MaksJamTempuh {
					continue
				}
				if t.JarakKm < terdekat {
					bi, bj, terdekat, jalur = i, j, t.JarakKm, t
				}
			}
		}
		if bi < 0 {
			break
		}
		kirim := min(sisaSup[bi], sisaDem[bj])
		sisaSup[bi] -= kirim
		sisaDem[bj] -= kirim
		vol := float64(kirim) / satuanTon
		biaya := int64(math.Round(vol * jalur.JarakKm * p.TarifTonKm))
		hasil.Alokasi = append(hasil.Alokasi, Alokasi{
			AsalKode: sup[bi].Kode, TujuanKode: dem[bj].Kode, VolumeTon: vol,
			JarakKm: jalur.JarakKm, JamTempuh: jalur.JamTempuh, Biaya: biaya,
		})
		hasil.TotalVolumeTon += vol
		hasil.TotalBiaya += biaya
	}
	hasil.TotalVolumeTon = math.Round(hasil.TotalVolumeTon*satuanTon) / satuanTon
	return hasil, nil
}

func satuan(ton float64) int { return int(math.Round(ton * satuanTon)) }

// normalisasi memeriksa dan mengurutkan menurut kode; volume nol dibuang.
func normalisasi(in []SisaWilayah, nama string) ([]SisaWilayah, error) {
	seen := map[string]bool{}
	out := make([]SisaWilayah, 0, len(in))
	for _, w := range in {
		switch {
		case w.Kode == "":
			return nil, fmt.Errorf("%w: kode wilayah %s kosong", ErrOptimasiTidakValid, nama)
		case seen[w.Kode]:
			return nil, fmt.Errorf("%w: wilayah %s ganda pada %s", ErrOptimasiTidakValid, w.Kode, nama)
		case w.Ton < 0 || math.IsNaN(w.Ton) || math.IsInf(w.Ton, 0):
			return nil, fmt.Errorf("%w: volume %s %s tidak valid", ErrOptimasiTidakValid, nama, w.Kode)
		}
		seen[w.Kode] = true
		if satuan(w.Ton) > 0 {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kode < out[j].Kode })
	return out, nil
}

// alirMinBiaya mengalirkan sebanyak mungkin dari s ke t dengan biaya minimum
// (successive shortest path; SPFA menangani biaya negatif pada busur sisa).
func (g *graf) alirMinBiaya(s, t int) {
	n := len(g.adj)
	for {
		dist := make([]int64, n)
		for i := range dist {
			dist[i] = infFlow
		}
		dist[s] = 0
		dalamAntrean := make([]bool, n)
		prevNode := make([]int, n)
		prevBusur := make([]int, n)
		antrean := []int{s}
		for len(antrean) > 0 {
			u := antrean[0]
			antrean = antrean[1:]
			dalamAntrean[u] = false
			for k, e := range g.adj[u] {
				if e.kap > 0 && dist[u]+e.biaya < dist[e.ke] {
					dist[e.ke] = dist[u] + e.biaya
					prevNode[e.ke], prevBusur[e.ke] = u, k
					if !dalamAntrean[e.ke] {
						dalamAntrean[e.ke] = true
						antrean = append(antrean, e.ke)
					}
				}
			}
		}
		if dist[t] == infFlow {
			return
		}
		aliran := math.MaxInt
		for v := t; v != s; v = prevNode[v] {
			aliran = min(aliran, g.adj[prevNode[v]][prevBusur[v]].kap)
		}
		for v := t; v != s; v = prevNode[v] {
			e := &g.adj[prevNode[v]][prevBusur[v]]
			e.kap -= aliran
			g.adj[v][e.balik].kap += aliran
		}
	}
}
