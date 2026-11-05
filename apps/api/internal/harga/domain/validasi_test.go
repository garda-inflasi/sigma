package domain_test

import (
	"testing"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pasarLat, pasarLng = 5.5603, 95.3237
	radius             = 400
)

func konteks() domain.KonteksValidasi {
	return domain.KonteksValidasi{
		Batas:    &domain.BatasHarga{BatasMin: 13000, BatasMaks: 156000},
		PasarLat: pasarLat, PasarLng: pasarLng, RadiusMeter: radius,
		HargaAcuan: 52000,
		Histori:    []int{51000, 52000, 53000, 52500, 51500, 52000, 52000, 50500, 53500},
	}
}

func entri(harga int) domain.EntriHarga {
	lat, lng := pasarLat+0.001, pasarLng // sekitar 111 m dari pasar
	return domain.EntriHarga{Harga: harga, Tipe: domain.TipeKonsumen, Lat: &lat, Lng: &lng}
}

func TestValidasi_NormalPriceRaisesNoFlag(t *testing.T) {
	hasil := domain.Validasi(entri(52000), konteks())

	assert.Equal(t, domain.FlagValidasi{Median: 52000, JarakMeter: hasil.Flag.JarakMeter}, hasil.Flag)
	assert.Positive(t, hasil.Flag.JarakMeter)
	assert.True(t, hasil.DalamGeofence)
	assert.Zero(t, hasil.DeviasiPct)
	assert.Less(t, hasil.SkorAnomali, 0.1)
}

func TestValidasi_PriceOutsideReasonableRange(t *testing.T) {
	k := konteks()

	tinggi := domain.Validasi(entri(520000), k)
	rendah := domain.Validasi(entri(5200), k)
	batas := domain.Validasi(entri(156000), k)

	assert.True(t, tinggi.Flag.DiLuarRentang, "salah satuan: sepuluh kali lipat")
	assert.True(t, rendah.Flag.DiLuarRentang)
	assert.False(t, batas.Flag.DiLuarRentang, "tepat di batas masih wajar")
	assert.GreaterOrEqual(t, tinggi.SkorAnomali, 0.99)
}

func TestValidasi_NoRangeConfiguredNeverFlagsRange(t *testing.T) {
	k := konteks()
	k.Batas = nil

	assert.False(t, domain.Validasi(entri(9_000_000), k).Flag.DiLuarRentang)
}

func TestValidasi_OutlierUsesRobustZScoreAgainstHistory(t *testing.T) {
	k := konteks()

	assert.True(t, domain.Validasi(entri(80000), k).Flag.Outlier, "lonjakan 54% jauh di atas sebaran histori")
	assert.True(t, domain.Validasi(entri(30000), k).Flag.Outlier)
	assert.False(t, domain.Validasi(entri(54500), k).Flag.Outlier, "kenaikan wajar")

	hasil := domain.Validasi(entri(80000), k)
	assert.InDelta(t, 53.8, hasil.DeviasiPct, 0.1, "deviasi terhadap median histori")
	assert.Equal(t, 52000, hasil.Flag.Median)
}

func TestValidasi_FlatHistoryUsesSpreadFloor(t *testing.T) {
	k := konteks()
	k.Histori = []int{50000, 50000, 50000, 50000, 50000}

	assert.False(t, domain.Validasi(entri(50500), k).Flag.Outlier, "selisih 1% di bawah lantai sebaran 2%")
	assert.True(t, domain.Validasi(entri(60000), k).Flag.Outlier)
}

func TestValidasi_ShortHistoryFallsBackToReferencePriceWithoutOutlierFlag(t *testing.T) {
	k := konteks()
	k.Histori = []int{50000, 51000}

	hasil := domain.Validasi(entri(80000), k)

	assert.False(t, hasil.Flag.Outlier, "tanpa histori cukup outlier tidak ditandai")
	assert.Equal(t, 52000, hasil.Flag.Median, "pembanding adalah harga acuan master")
	assert.InDelta(t, 53.8, hasil.DeviasiPct, 0.1)
}

func TestValidasi_Geofence(t *testing.T) {
	k := konteks()
	luarLat := pasarLat + 0.02 // sekitar 2,2 km
	luar := entri(52000)
	luar.Lat = &luarLat

	hasil := domain.Validasi(luar, k)

	assert.True(t, hasil.Flag.LuarGeofence)
	assert.False(t, hasil.DalamGeofence)
	assert.InDelta(t, 2200, hasil.Flag.JarakMeter, 100)
	assert.False(t, hasil.Flag.TanpaLokasi)
}

func TestValidasi_MissingCoordinatesAreFlaggedNotPenalizedAsOutside(t *testing.T) {
	e := entri(52000)
	e.Lat, e.Lng = nil, nil

	hasil := domain.Validasi(e, konteks())

	assert.True(t, hasil.Flag.TanpaLokasi)
	assert.False(t, hasil.Flag.LuarGeofence)
	assert.True(t, hasil.DalamGeofence)
}

func TestValidasi_DuplicateFlagFollowsContext(t *testing.T) {
	k := konteks()
	k.Duplikat = true

	hasil := domain.Validasi(entri(52000), k)

	assert.True(t, hasil.Flag.Duplikat)
	assert.InDelta(t, 0.2, hasil.SkorAnomali, 0.001)
}

func TestValidasi_ScoreIsCappedAndRounded(t *testing.T) {
	k := konteks()
	k.Duplikat = true
	luarLat := pasarLat + 0.05
	e := entri(900000)
	e.Lat = &luarLat

	hasil := domain.Validasi(e, k)

	assert.Equal(t, 1.0, hasil.SkorAnomali)
	assert.True(t, hasil.Flag.DiLuarRentang && hasil.Flag.Outlier && hasil.Flag.Duplikat && hasil.Flag.LuarGeofence)
}

func TestAcuanTipe(t *testing.T) {
	assert.Equal(t, 50000, domain.AcuanTipe(50000, domain.TipeKonsumen))
	assert.Equal(t, 36000, domain.AcuanTipe(50000, domain.TipeProdusen))
}

func TestFlagValidasi_JSONRoundTrip(t *testing.T) {
	awal := domain.FlagValidasi{Outlier: true, Duplikat: true, Median: 52000, JarakMeter: 120}

	nilai, err := awal.Value()
	require.NoError(t, err)
	var kembali domain.FlagValidasi
	require.NoError(t, kembali.Scan(nilai))
	assert.Equal(t, awal, kembali)

	require.NoError(t, kembali.Scan(nil))
	assert.Equal(t, domain.FlagValidasi{}, kembali, "NULL berarti tanpa bendera")
	require.NoError(t, kembali.Scan([]byte(`{"outlier":true}`)))
	assert.True(t, kembali.Outlier)
	assert.Error(t, kembali.Scan(42))
}
