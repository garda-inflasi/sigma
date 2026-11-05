package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func angka(v float64) *float64 { return &v }

func TestStatusCapaian(t *testing.T) {
	naik := &Target{Target: 75, AmbangWaspada: angka(67.5)}
	turun := &Target{Target: 5.4, AmbangWaspada: angka(5.94)}
	tanpaAmbang := &Target{Target: 75}

	tests := []struct {
		name   string
		arah   string
		nilai  *float64
		target *Target
		want   string
	}{
		{"naik: tepat di target", ArahNaik, angka(75), naik, StatusTercapai},
		{"naik: di atas target", ArahNaik, angka(82.1), naik, StatusTercapai},
		{"naik: di antara ambang dan target", ArahNaik, angka(70), naik, StatusWaspada},
		{"naik: tepat di ambang", ArahNaik, angka(67.5), naik, StatusWaspada},
		{"naik: di bawah ambang", ArahNaik, angka(60), naik, StatusTidakTercapai},
		{"turun: tepat di target", ArahTurun, angka(5.4), turun, StatusTercapai},
		{"turun: lebih rendah lebih baik", ArahTurun, angka(3), turun, StatusTercapai},
		{"turun: di antara target dan ambang", ArahTurun, angka(5.7), turun, StatusWaspada},
		{"turun: di atas ambang", ArahTurun, angka(8), turun, StatusTidakTercapai},
		{"tanpa ambang: di bawah target langsung tidak tercapai", ArahNaik, angka(70), tanpaAmbang, StatusTidakTercapai},
		{"belum ada nilai", ArahNaik, nil, naik, StatusMenungguData},
		{"belum ada nilai dan target", ArahNaik, nil, nil, StatusMenungguData},
		{"ada nilai tanpa target", ArahNaik, angka(70), nil, StatusTanpaTarget},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StatusCapaian(tc.arah, tc.nilai, tc.target))
		})
	}
}
