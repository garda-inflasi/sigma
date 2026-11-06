package seeder

import (
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/internal/seedkit"
)

// Izin yang disyaratkan tahap tertentu (selain intervensi.kelola bawaan).
const (
	izinSetuju       = "intervensi.setujui"
	izinSetujuFiskal = "intervensi.setujui_fiskal"
)

type tahapSeed struct {
	siklus   string
	kode     string
	label    string
	petunjuk string
	izin     string // kosong = domain.IzinBawaan
	slaHari  int    // 0 = tanpa SLA (tahap penutup)
}

// tahapSeeds mengikuti alur Layer 4 (PRD §4.1) dan sama dengan
// apps/web/src/lib/data/siklus.ts; dijaga tes fixture packages/contracts/intervensi-tahap.json.
// Urutan tahap = urutan kemunculan per siklus (mulai 1). SLA adalah angka awal.
var tahapSeeds = []tahapSeed{
	{domain.SiklusNormal, "n-pantau", "Pantau rutin data produsen", "Agregasi harian dari agen lapangan", "", 1},
	{domain.SiklusNormal, "n-verifikasi", "Verifikasi silang BPS", "Cocokkan dengan rilis statistik resmi", "", 3},
	{domain.SiklusNormal, "n-jadwal", "Optimasi jadwal pengiriman", "Susun rute & slot armada", "", 3},
	{domain.SiklusNormal, "n-agregasi", "Agregasi volume reguler", "Konsolidasi pasokan produsen", "", 5},
	{domain.SiklusNormal, "n-selesai", "Selesai", "Ditutup dan terdokumentasi", "", 0},

	{domain.SiklusTekanan, "t-ews", "EWS terpicu", "Alarm otomatis dari model", "", 1},
	{domain.SiklusTekanan, "t-verifikasi", "Verifikasi cepat BI/BPS", "Konfirmasi sinyal dengan data resmi", "", 2},
	{domain.SiklusTekanan, "t-petakan", "Petakan suplai sentra surplus", "Butuh persetujuan Dewan Pengarah", izinSetuju, 3},
	{domain.SiklusTekanan, "t-logistik", "Usulan intervensi logistik kilat", "Rute & volume pengiriman darurat", "", 3},
	{domain.SiklusTekanan, "t-fiskal", "Dukungan instrumen belanja", "Persetujuan Kanwil DJPb", izinSetujuFiskal, 5},
	{domain.SiklusTekanan, "t-selesai", "Selesai", "Harga kembali ke koridor", izinSetuju, 0},

	{domain.SiklusSurplus, "s-deteksi", "Deteksi over-supply", "Harga produsen jatuh di bawah koridor", "", 2},
	{domain.SiklusSurplus, "s-coldstorage", "Arahkan ke cold storage", "Alokasi kapasitas & shelf-life", "", 3},
	{domain.SiklusSurplus, "s-olahan", "Rekomendasi pengolahan turunan", "Uji mutu oleh Unit Mutu & Pascapanen", "", 7},
	{domain.SiklusSurplus, "s-ekspansi", "Ekspansi ke pembeli antardaerah", "Pencocokan pembeli & logistik", "", 7},
	{domain.SiklusSurplus, "s-selesai", "Selesai", "Surplus terserap", izinSetuju, 0},
}

// Penanggung jawab dipilih dari akun demo auth.demo bertipe tim operasional dan manajer.
// id adalah UUID akun (seedkit.IDPengguna dari slug demo).
var timDemo = []struct{ id, nama string }{
	{seedkit.IDPengguna("manajer_program"), "Rina Safitri, S.T., M.T."},
	{seedkit.IDPengguna("manajer_data"), "Bambang Wijaya, S.Kom., M.Cs."},
	{seedkit.IDPengguna("manajer_kebijakan"), "Dr. Nurul Aini, S.E., M.E."},
	{seedkit.IDPengguna("manajer_risiko"), "Dian Permata, S.Si., M.M."},
	{seedkit.IDPengguna("manajer_mutu"), "Hendra Gunawan, S.T., M.T."},
	{seedkit.IDPengguna("koordinator_lapangan"), "Zulkifli, S.P."},
	{seedkit.IDPengguna("agregator_data"), "Maya Sari, S.Stat."},
}

// pembuatDemo membuat seluruh kartu contoh (UUID akun manajer_program).
var pembuatDemo = seedkit.IDPengguna("manajer_program")

type kartuSeed struct {
	judul     string
	siklus    string
	tahap     string
	komoditas string
	kabupaten string
	prioritas string
	ai        bool
	alert     string // id alert EWS (ews.demo); kosong = tanpa alert
	tenggat   int    // hari dari sekarang (negatif = terlambat)
	total     int
	selesai   int
}

// kartuSeeds meniru mockKanban (apps/web/src/lib/api/mock.ts): 7 kartu tekanan,
// 6 normal, 6 surplus, tersebar di semua tahap. Id = INT-<301 + indeks>.
var kartuSeeds = []kartuSeed{
	{"Lonjakan cabai merah Banda Aceh", domain.SiklusTekanan, "t-ews", "CBM-KRT", "1171", "TINGGI", true, "EWS-1000", 1, 5, 1},
	{"Pasokan cabai rawit Lhokseumawe", domain.SiklusTekanan, "t-ews", "CBR-RWT", "1174", "TINGGI", true, "EWS-1002", 2, 4, 0},
	{"Bawang merah Meulaboh naik 9%", domain.SiklusTekanan, "t-verifikasi", "BWM", "1107", "SEDANG", true, "EWS-1003", 3, 6, 2},
	{"Kirim cabai Takengon → Banda Aceh", domain.SiklusTekanan, "t-logistik", "CBM-KRT", "1106", "TINGGI", false, "", 4, 7, 3},
	{"Subsidi ongkos angkut Simeulue", domain.SiklusTekanan, "t-fiskal", "CBM-KRT", "1101", "SEDANG", false, "", 6, 5, 4},
	{"Verifikasi harga ikan tongkol Sabang", domain.SiklusTekanan, "t-verifikasi", "IKN-TGK", "1172", "RENDAH", false, "", -1, 3, 1},
	{"Operasi pasar cabai Peunayong", domain.SiklusTekanan, "t-petakan", "CBM-KRT", "1171", "TINGGI", false, "", 2, 6, 2},

	{"Rekap harga beras pekan 40", domain.SiklusNormal, "n-pantau", "BRS-MED", "1171", "RENDAH", false, "", 5, 4, 1},
	{"Cocokkan IHK BPS September", domain.SiklusNormal, "n-verifikasi", "BRS-MED", "1171", "SEDANG", false, "", 3, 5, 2},
	{"Jadwal armada Bireuen–Lhokseumawe", domain.SiklusNormal, "n-jadwal", "TLR", "1110", "RENDAH", false, "", 6, 4, 2},
	{"Agregasi telur peternak Bireuen", domain.SiklusNormal, "n-agregasi", "TLR", "1110", "SEDANG", false, "", 4, 6, 3},
	{"Pantau daging ayam Langsa", domain.SiklusNormal, "n-pantau", "DAG-AYM", "1173", "RENDAH", false, "", 1, 3, 0},
	{"Verifikasi gula pasir Pidie", domain.SiklusNormal, "n-selesai", "GLA", "1109", "RENDAH", false, "", -2, 3, 3},

	{"Kentang Gayo: harga produsen −18%", domain.SiklusSurplus, "s-deteksi", "KTG", "1106", "TINGGI", true, "EWS-1012", 2, 5, 1},
	{"Tomat Bener Meriah surplus 1.200 ton", domain.SiklusSurplus, "s-deteksi", "TMT", "1117", "TINGGI", true, "EWS-1013", 3, 4, 0},
	{"Alokasi 80 ton ke CS Takengon", domain.SiklusSurplus, "s-coldstorage", "KTG", "1106", "SEDANG", false, "", 4, 6, 3},
	{"Uji olahan saus tomat UMKM", domain.SiklusSurplus, "s-olahan", "TMT", "1117", "SEDANG", false, "", 7, 7, 2},
	{"Pembeli kentang dari Medan", domain.SiklusSurplus, "s-ekspansi", "KTG", "1106", "SEDANG", false, "", 5, 5, 3},
	{"Keripik kentang — mitra koperasi", domain.SiklusSurplus, "s-selesai", "KTG", "1106", "RENDAH", false, "", -1, 4, 4},
}
