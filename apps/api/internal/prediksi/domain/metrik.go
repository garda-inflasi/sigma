package domain

import (
	"encoding/json"
	"fmt"
)

// MetrikSeri adalah model terlatih satu kabupaten di dalam model_registry.metrics:
// bahan untuk meramal ulang tanpa melatih lagi.
type MetrikSeri struct {
	Algoritma string             `json:"algoritma"`
	Params    map[string]float64 `json:"params"`
	GalatRMSE map[string]float64 `json:"galat_rmse"`
	Mape      float64            `json:"mape"`
}

// MetrikModel adalah isi kolom model_registry.metrics untuk model hasil pelatihan.
// MAPE dan RMSE berasal dari backtest walk-forward pada lipatan uji, dirata-ratakan
// antar kabupaten; BaselineMape adalah MAPE baseline naif (harga terakhir
// dipertahankan) pada lipatan yang sama.
type MetrikModel struct {
	Mape          float64               `json:"mape"`
	Rmse          float64               `json:"rmse"`
	BaselineMape  float64               `json:"baseline_mape"`
	BaselineRmse  float64               `json:"baseline_rmse"`
	Optimistis    bool                  `json:"optimistis"`
	Horizon       []int                 `json:"horizon"`
	RiwayatHari   int                   `json:"riwayat_hari"`
	NSeriOK       int                   `json:"n_seri_ok"`
	NSeriDilewati int                   `json:"n_seri_dilewati"`
	VersiLayanan  string                `json:"versi_layanan"`
	Algoritma     map[string]int        `json:"algoritma"`
	Seri          map[string]MetrikSeri `json:"seri"`
	Dilewati      map[string]string     `json:"dilewati"`
}

// BacaMetrik mengurai kolom metrics; "{}" (mis. model demo) menghasilkan MetrikModel kosong.
func BacaMetrik(raw string) (MetrikModel, error) {
	var m MetrikModel
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return MetrikModel{}, fmt.Errorf("metrics model tidak terbaca: %w", err)
	}
	return m, nil
}

// ImportansiFaktor adalah bobot relatif sebuah faktor pada model (jumlah seluruh bobot = 1).
type ImportansiFaktor struct {
	Nama  string  `json:"nama"`
	Bobot float64 `json:"bobot"`
}
