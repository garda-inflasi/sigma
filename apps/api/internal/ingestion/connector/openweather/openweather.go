// Package openweather adalah konektor API OpenWeather (api.openweathermap.org):
// mengumpulkan cuaca aktual (suhu, kelembapan, curah hujan) per wilayah/sentra produksi
// dan menormalkannya ke tabel cuaca_harian.
package openweather

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// Kode adalah kode baris sumber_data OpenWeather (sumber_data.kode).
const Kode = "SRC-OWM"

// Sumber adalah nilai kolom cuaca_harian.sumber.
const Sumber = "OpenWeather"

// EnvKunci adalah variabel lingkungan berisi kunci API OpenWeather.
const EnvKunci = "OPENWEATHER_API_KEY"

const (
	jalurWeather = "/weather"
	kunciWilayah = "wilayah="
	anomaliCurah = 50.0 // mm
)

// wib adalah zona waktu Asia/Jakarta (UTC+7).
var wib = time.FixedZone("WIB", 7*3600)

type konfigurasi struct {
	APIKey string `json:"api_key"`
}

// Konektor adalah adaptor untuk OpenWeather API.
type Konektor struct {
	kunciAPI string
}

// New membuat konektor OpenWeather baru.
func New(kunciAPI string) *Konektor {
	if kunciAPI == "" {
		kunciAPI = os.Getenv(EnvKunci)
	}
	return &Konektor{kunciAPI: kunciAPI}
}

func (*Konektor) Kode() string { return Kode }

func (k *Konektor) Permintaan(in domain.Masukan) ([]domain.Permintaan, error) {
	if in.URLDasar == "" {
		return nil, fmt.Errorf("url_dasar %s belum diisi", Kode)
	}
	kunci := k.kunciAPI
	if kunci == "" && len(in.Konfigurasi) > 0 {
		var cfg konfigurasi
		_ = json.Unmarshal(in.Konfigurasi, &cfg)
		kunci = cfg.APIKey
	}
	if kunci == "" {
		return nil, fmt.Errorf("kunci API %s belum diatur (setel variabel lingkungan %s atau konfigurasi.api_key)", Kode, EnvKunci)
	}

	wilayah := in.Pemetaan.SemuaWilayah()
	if len(wilayah) == 0 {
		return nil, fmt.Errorf("pemetaan_sumber WILAYAH untuk %s kosong: petakan koordinat atau nama wilayah ke kode kabupaten/kota", Kode)
	}

	reqs := make([]domain.Permintaan, len(wilayah))
	for i, w := range wilayah {
		v := url.Values{
			"appid": {kunci},
			"units": {"metric"},
		}
		// Alias bisa berupa koordinat "lat,lon" atau nama wilayah "Banda Aceh".
		if parts := strings.Split(w.Alias, ","); len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			v.Set("lat", strings.TrimSpace(parts[0]))
			v.Set("lon", strings.TrimSpace(parts[1]))
		} else {
			v.Set("q", w.Alias)
		}

		reqs[i] = domain.Permintaan{
			Kunci:    kunciWilayah + w.Alias,
			URL:      strings.TrimRight(in.URLDasar, "/") + jalurWeather + "?" + v.Encode(),
			Samarkan: []string{kunci},
		}
	}
	return reqs, nil
}

type respons struct {
	Main struct {
		Temp     float64 `json:"temp"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Rain struct {
		H1 float64 `json:"1h"`
		H3 float64 `json:"3h"`
	} `json:"rain"`
	Dt  int64  `json:"dt"`
	Cod int    `json:"cod"`
	Msg string `json:"message"`
}

func (*Konektor) Normalisasi(in domain.Masukan, dok domain.Dokumen) (domain.Hasil, error) {
	var hasil domain.Hasil

	alias, ok := strings.CutPrefix(dok.Kunci, kunciWilayah)
	if !ok {
		return hasil, fmt.Errorf("kunci dokumen %q bukan permintaan OpenWeather", dok.Kunci)
	}

	kabupatenKode, cari := in.Pemetaan.Cari(domain.JenisWilayah, alias)
	switch cari {
	case domain.Diabaikan:
		return hasil, nil
	case domain.TakTerpetakan:
		hasil.TambahTakTerpetakan(domain.JenisWilayah, alias)
		return hasil, nil
	}

	var resp respons
	if err := json.Unmarshal(dok.Konten, &resp); err != nil {
		return hasil, fmt.Errorf("unmarshal JSON: %w", err)
	}
	if resp.Cod != 0 && resp.Cod != 200 {
		return hasil, fmt.Errorf("galat OpenWeather API (cod %d): %s", resp.Cod, resp.Msg)
	}

	curah := resp.Rain.H1
	if curah == 0 && resp.Rain.H3 > 0 {
		curah = resp.Rain.H3
	}

	waktu := in.Sekarang
	if resp.Dt > 0 {
		waktu = time.Unix(resp.Dt, 0).In(wib)
	}
	tanggal := time.Date(waktu.Year(), waktu.Month(), waktu.Day(), 0, 0, 0, 0, time.UTC)

	hasil.Cuaca = append(hasil.Cuaca, domain.CuacaHarian{
		KabupatenKode: kabupatenKode,
		Tanggal:       tanggal,
		CurahHujanMm:  curah,
		SuhuC:         resp.Main.Temp,
		Kelembapan:    resp.Main.Humidity,
		Anomali:       curah >= anomaliCurah,
		Sumber:        Sumber,
	})

	return hasil, nil
}
