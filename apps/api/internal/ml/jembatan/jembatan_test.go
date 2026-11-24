package jembatan

import (
	"context"
	"strings"
	"testing"
	"time"

	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func s(v string) *string   { return &v }
func f(v float64) *float64 { return &v }

var siklusUji = domain.Siklus{CycleID: "C20261124T09Z", Status: domain.StatusBerhasil, CycleTimestamp: time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)}

var master = struct{ kom, kab map[string]bool }{
	kom: map[string]bool{"CBM-KRT": true, "BWM": true},
	kab: map[string]bool{"1171": true, "1174": true, "1107": true},
}

func TestJenisAlert(t *testing.T) {
	kasus := map[string]domain.SinyalEws{
		ewsdomain.JenisLonjakanHarga + "/naik":  {Jenis: domain.SinyalAnomaliHarga, SinyalKunci: "ANOMALI_HARGA:CBM-KRT:1171:NAIK"},
		ewsdomain.JenisAnomaliData + "/turun":   {Jenis: domain.SinyalAnomaliHarga, SinyalKunci: "ANOMALI_HARGA:CBM-KRT:1171:TURUN"},
		ewsdomain.JenisLonjakanHarga + "/proy":  {Jenis: domain.SinyalProyeksiLonjakan},
		ewsdomain.JenisLonjakanHarga + "/tekan": {Jenis: domain.SinyalTekananInflasi},
		ewsdomain.JenisAnomaliData + "/sumber":  {Jenis: domain.SinyalSumberData},
	}
	for nama, sg := range kasus {
		want, _, _ := strings.Cut(nama, "/")
		assert.Equal(t, want, JenisAlert(sg), nama)
	}
}

func TestAlertDari(t *testing.T) {
	a, err := AlertDari(domain.SinyalEws{SinyalKunci: "ANOMALI_HARGA:CBM-KRT:1171:NAIK", Severity: "SIAGA", Jenis: domain.SinyalAnomaliHarga,
		KomoditasKode: s("CBM-KRT"), KabupatenKode: s("1171"), Nilai: f(12), Ambang: f(10), Metode: s("zscore_musiman"),
		Judul: "Anomali", Pesan: "Harga menyimpang.", Aktif: true}, siklusUji, master.kom, master.kab)
	require.NoError(t, err)
	assert.Equal(t, "ML|ANOMALI_HARGA:CBM-KRT:1171:NAIK", a.DedupKey)
	assert.Equal(t, "1171", a.KabupatenKode)
	assert.Equal(t, "CBM-KRT", *a.KomoditasKode)
	assert.Equal(t, ewsdomain.SumberPipelineML, a.Sumber)
	assert.Zero(t, a.AturanID, "tanpa aturan: disimpan NULL")
	assert.Equal(t, CooldownJam, a.CooldownJam)
	assert.Equal(t, 72, a.SkorDampak)
	assert.Equal(t, "%", a.SatuanNilai)
	assert.Contains(t, a.Pesan, "siklus C20261124T09Z, metode zscore_musiman")

	sys, err := AlertDari(domain.SinyalEws{SinyalKunci: "SUMBER_DATA:BPS:inflasi", Severity: "SIAGA", Jenis: domain.SinyalSumberData,
		Judul: "Sumber gagal", Pesan: "Status ERROR."}, siklusUji, master.kom, master.kab)
	require.NoError(t, err)
	assert.Empty(t, sys.KabupatenKode, "tingkat sistem tanpa kabupaten")
	assert.Nil(t, sys.KomoditasKode)
	assert.Zero(t, sys.Nilai, "nilai null menjadi 0 karena kolom NOT NULL")
	assert.Equal(t, "hari", sys.SatuanNilai)
	assert.Contains(t, sys.Pesan, "Tingkat sistem.")

	prov, err := AlertDari(domain.SinyalEws{SinyalKunci: "TEKANAN_INFLASI:11", Severity: "WASPADA", Jenis: domain.SinyalTekananInflasi,
		KabupatenKode: s("11"), Nilai: f(62), Ambang: f(60)}, siklusUji, master.kom, master.kab)
	require.NoError(t, err)
	assert.Empty(t, prov.KabupatenKode)
	assert.Equal(t, "skor", prov.SatuanNilai)

	_, err = AlertDari(domain.SinyalEws{KomoditasKode: s("XYZ"), KabupatenKode: s("1171")}, siklusUji, master.kom, master.kab)
	assert.ErrorContains(t, err, "XYZ")
	_, err = AlertDari(domain.SinyalEws{KabupatenKode: s("9999")}, siklusUji, master.kom, master.kab)
	assert.ErrorContains(t, err, "9999")

	panjang := DedupKey(strings.Repeat("x", 300))
	assert.LessOrEqual(t, len(panjang), 255)
	assert.True(t, strings.HasPrefix(panjang, "ML|sha256:"))
}

// ── Alur dengan palsu ────────────────────────────────────────────────

type sumberPalsu struct {
	snap *domain.Snapshot
	k    usecase.Keadaan
}

func (p sumberPalsu) Snapshot(context.Context, string) (*domain.Snapshot, usecase.Keadaan) {
	return p.snap, p.k
}

type keadaanPalsu struct {
	data   map[string]domain.EwsSinyalML
	mereda []idx.ID
}

func (k *keadaanPalsu) MuatKeadaan(_ context.Context, kunci []string) (map[string]domain.EwsSinyalML, error) {
	out := map[string]domain.EwsSinyalML{}
	for _, x := range kunci {
		if v, ok := k.data[x]; ok {
			out[x] = v
		}
	}
	return out, nil
}
func (k *keadaanPalsu) SimpanKeadaan(_ context.Context, v domain.EwsSinyalML) error {
	k.data[v.SinyalKunci] = v
	return nil
}
func (k *keadaanPalsu) AlertAktif(context.Context, string) (*idx.ID, error) { return nil, nil }
func (k *keadaanPalsu) CatatMereda(_ context.Context, id idx.ID, _ string, _ time.Time) (bool, error) {
	k.mereda = append(k.mereda, id)
	return true, nil
}
func (k *keadaanPalsu) KodeMaster(context.Context) (map[string]bool, map[string]bool, error) {
	return master.kom, master.kab, nil
}

type penulisPalsu struct {
	urutan []string
	id     idx.ID
}

func (p *penulisPalsu) Terapkan(_ context.Context, in ewsdomain.NewAlert, _ time.Time) (ewsdomain.HasilTulis, *ewsdomain.Alert, error) {
	p.urutan = append(p.urutan, in.Severity+":"+in.DedupKey)
	p.id++
	return ewsdomain.HasilDibuat, &ewsdomain.Alert{EwsAlert: ewsdomain.EwsAlert{ID: p.id, Severity: in.Severity}}, nil
}

func TestJalankan_UrutanIdempotenINFOdanPulih(t *testing.T) {
	snap := &domain.Snapshot{Siklus: siklusUji, Ews: []domain.SinyalEws{
		{SinyalKunci: "PROYEKSI_LONJAKAN:BWM:1174", Severity: "WASPADA", Jenis: domain.SinyalProyeksiLonjakan, KomoditasKode: s("BWM"), KabupatenKode: s("1174"), Aktif: true},
		{SinyalKunci: "SUMBER_DATA:BMKG:prakiraan", Severity: "INFO", Jenis: domain.SinyalSumberData, Aktif: true},
		{SinyalKunci: "ANOMALI_HARGA:CBM-KRT:1171:NAIK", Severity: "KRITIS", Jenis: domain.SinyalAnomaliHarga, KomoditasKode: s("CBM-KRT"), KabupatenKode: s("1171"), Aktif: true},
		{SinyalKunci: "ANOMALI_HARGA:BWM:1171:NAIK", Severity: "INFO", Jenis: domain.SinyalAnomaliHarga, Aktif: false},
	}}
	lamaID := idx.ID(77)
	keadaan := &keadaanPalsu{data: map[string]domain.EwsSinyalML{
		"ANOMALI_HARGA:BWM:1171:NAIK": {SinyalKunci: "ANOMALI_HARGA:BWM:1171:NAIK", Aktif: true, CycleID: "C20261124T08Z", AlertID: &lamaID},
	}}
	penulis := &penulisPalsu{}
	j := New(sumberPalsu{snap: snap, k: usecase.Keadaan{Tersedia: true}}, keadaan, penulis, nil, nil)

	r, err := j.Jalankan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, r.Dibuat)
	assert.Equal(t, 1, r.DiabaikanInfo)
	assert.Equal(t, 1, r.Mereda)
	assert.Equal(t, []idx.ID{lamaID}, keadaan.mereda)
	require.Len(t, penulis.urutan, 2)
	assert.True(t, strings.HasPrefix(penulis.urutan[0], "KRITIS:"), "KRITIS diterapkan lebih dulu")
	assert.False(t, keadaan.data["ANOMALI_HARGA:BWM:1171:NAIK"].Aktif)
	_, adaInfo := keadaan.data["SUMBER_DATA:BMKG:prakiraan"]
	assert.False(t, adaInfo, "sinyal INFO tanpa riwayat tidak dicatat")

	r, err = j.Jalankan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, r.SudahDiterapkan, "siklus yang sama tidak diterapkan ulang")
	assert.Len(t, penulis.urutan, 2)
}

func TestJalankan_SumberTidakTersedia(t *testing.T) {
	j := New(sumberPalsu{k: usecase.Keadaan{Alasan: usecase.AlasanBelumAdaSiklus}}, &keadaanPalsu{}, &penulisPalsu{}, nil, nil)
	r, err := j.Jalankan(context.Background())
	assert.NoError(t, err, "belum ada siklus bukan galat")
	assert.Zero(t, r.Sinyal)

	j = New(sumberPalsu{k: usecase.Keadaan{Alasan: usecase.AlasanMongoTidakTersedia}}, &keadaanPalsu{}, &penulisPalsu{}, nil, nil)
	_, err = j.Jalankan(context.Background())
	assert.ErrorIs(t, err, ErrSumberTidakTersedia)
}
