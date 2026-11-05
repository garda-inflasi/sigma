package domain

import "fmt"

// Id fakta tetap. Fakta berkelompok (andil, harga, peringatan, proyeksi) berindeks dan
// dibentuk dengan IDFakta; hanya yang datanya ada yang masuk Konteks.
const (
	FaktaPeriode        = "PERIODE"
	FaktaWilayah        = "WILAYAH"
	FaktaInflasiPeriode = "INFLASI_PERIODE"
	FaktaInflasiMtm     = "INFLASI_MTM"
	FaktaInflasiYoy     = "INFLASI_YOY"
	FaktaInflasiYtd     = "INFLASI_YTD"
	FaktaTargetTahun    = "TARGET_TAHUN"
	FaktaTargetBawah    = "TARGET_BAWAH"
	FaktaTargetAtas     = "TARGET_ATAS"
	FaktaStatusSasaran  = "STATUS_SASARAN"
	FaktaEwsTotal       = "EWS_TOTAL"
	FaktaEwsKritis      = "EWS_KRITIS"
	FaktaEwsSiaga       = "EWS_SIAGA"
	FaktaEwsWaspada     = "EWS_WASPADA"
	FaktaProyeksiHoriz  = "PROYEKSI_HORIZON"
	FaktaProyeksiAsal   = "PROYEKSI_SUMBER"
	FaktaCatatanData    = "CATATAN_DATA"
)

// Kelompok fakta berindeks.
const (
	KelompokAndil    = "ANDIL"
	KelompokHarga    = "HARGA"
	KelompokEws      = "EWS"
	KelompokProyeksi = "PROYEKSI"
)

// IDFakta membentuk id fakta berindeks, mis. IDFakta(KelompokHarga, 1, "PERUBAHAN") = "HARGA_1_PERUBAHAN".
func IDFakta(kelompok string, urut int, atribut string) string {
	return fmt.Sprintf("%s_%d_%s", kelompok, urut, atribut)
}
