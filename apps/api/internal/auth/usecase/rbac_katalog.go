package usecase

import "slices"

// Katalog izin dan peran didefinisikan di kode: route merujuk nama izinnya (RequirePermission), jadi UI tidak
// boleh menciptakan izin baru. Yang disimpan di basis data (Opsi B, migrasi 000107) adalah penugasannya;
// seeder lapis reference memuat katalog ini dan matriks bawaan (rolePermissions) dengan ON CONFLICT DO NOTHING.

// DefinisiPeran adalah satu peran (nilai enum peran) beserta label dan lapisnya.
type DefinisiPeran struct {
	Kunci string
	Label string
	Lapis string
	// Urutan baku (mulai 1), sama dengan roleOrder dan ROLE_IDS di web.
	Urutan int
}

// DefinisiIzin adalah satu izin domain.aksi di katalog.
type DefinisiIzin struct {
	Nama      string
	Deskripsi string
	Kelompok  string
}

var labelPeran = map[string]string{
	"STRATEGIS_FMIPA":    "Pimpinan FMIPA USK",
	"STRATEGIS_BI":       "Analis Bank Indonesia",
	"STRATEGIS_BPS":      "Statistisi BPS",
	"STRATEGIS_DJPB":     "Analis Kanwil DJPb",
	"MANAJER_PROGRAM":    "Manajer Program",
	"MANAJER_DATA":       "Unit Data & Statistik",
	"MANAJER_KEBIJAKAN":  "Unit Hubungan Pemerintah & Kebijakan",
	"MANAJER_PEMBIAYAAN": "Unit Pembiayaan & Investasi",
	"MANAJER_RISIKO":     "Unit Monitoring & Risiko",
	"MANAJER_MUTU":       "Unit Mutu & Pascapanen",
	"OPS_KOORDINATOR":    "Koordinator Lapangan",
	"OPS_AGREGATOR":      "Unit Agregator",
	"OPS_MAHASISWA":      "Agen Lapangan Mahasiswa",
	"ADMIN_SISTEM":       "Administrator Sistem",
}

// katalogIzin adalah satu-satunya daftar izin yang dikenal sistem, dalam urutan tampil (kelompok, lalu aksi).
var katalogIzin = []DefinisiIzin{
	{"dashboard.operasional", "Membuka dasbor operasional", "Dasbor"},
	{"dashboard.eksekutif", "Membuka dasbor eksekutif", "Dasbor"},
	{"harga.lihat", "Melihat data harga pangan", "Harga"},
	{"harga.input", "Menginput harga dari lapangan", "Harga"},
	{"harga.validasi", "Memvalidasi entri harga", "Harga"},
	{"harga.publikasi", "Mempublikasikan data harga", "Harga"},
	{"harga.ekspor", "Mengekspor data harga", "Harga"},
	{"tugas.lihat_sendiri", "Melihat tugas lapangan milik sendiri", "Tugas lapangan"},
	{"tugas.kelola", "Membuat dan mengelola tugas lapangan", "Tugas lapangan"},
	{"ews.lihat", "Melihat peringatan dini (EWS)", "Peringatan dini"},
	{"ews.akui", "Mengakui peringatan dini", "Peringatan dini"},
	{"ews.eskalasi", "Mengeskalasi peringatan dini", "Peringatan dini"},
	{"ews.atur_ambang", "Mengatur ambang peringatan dini", "Peringatan dini"},
	{"prediksi.lihat", "Melihat prediksi dan penjelasan model", "Prediksi"},
	{"model.kelola", "Mengelola model prediksi dan siklus MLOps", "Prediksi"},
	{"spasial.lihat", "Melihat peta dan analisis spasial", "Spasial dan logistik"},
	{"logistik.kelola", "Merencanakan dan menyimpan rute distribusi", "Spasial dan logistik"},
	{"coldstorage.lihat", "Melihat data cold storage", "Cold storage"},
	{"coldstorage.kelola", "Mengelola fasilitas dan stok cold storage", "Cold storage"},
	{"fiskal.lihat", "Melihat data dan simulasi fiskal", "Fiskal"},
	{"intervensi.lihat", "Melihat daftar intervensi", "Intervensi"},
	{"intervensi.kelola", "Menyusun dan memindahkan tahap intervensi", "Intervensi"},
	{"intervensi.setujui", "Menyetujui intervensi", "Intervensi"},
	{"intervensi.setujui_fiskal", "Menyetujui dukungan fiskal intervensi", "Intervensi"},
	{"brief.lihat", "Melihat policy brief", "Policy brief"},
	{"brief.susun", "Menyusun policy brief", "Policy brief"},
	{"brief.setujui", "Menyetujui policy brief", "Policy brief"},
	{"kpi.lihat", "Melihat indikator kinerja (KPI)", "KPI"},
	{"master.lihat", "Melihat master data", "Master data"},
	{"master.ajukan", "Mengajukan perubahan master data", "Master data"},
	{"master.setujui", "Menyetujui perubahan master data", "Master data"},
	{"sumber.lihat", "Melihat sumber data", "Sumber data"},
	{"sumber.kelola", "Mengelola sumber data dan ingesti", "Sumber data"},
	{"pengguna.kelola", "Mengelola pengguna, peran, matriks izin, dan izin langsung", "Pengguna dan audit"},
	{"audit.lihat", "Melihat jejak audit", "Pengguna dan audit"},
}

// DaftarPeran mengembalikan 14 peran dalam urutan baku.
func DaftarPeran() []DefinisiPeran {
	out := make([]DefinisiPeran, len(roleOrder))
	for i, kunci := range roleOrder {
		out[i] = DefinisiPeran{Kunci: kunci, Label: labelPeran[kunci], Lapis: roleLapis[kunci], Urutan: i + 1}
	}
	return out
}

// KatalogIzin mengembalikan salinan katalog izin dalam urutan tampil.
func KatalogIzin() []DefinisiIzin {
	return slices.Clone(katalogIzin)
}

// IzinBawaan mengembalikan salinan izin bawaan (matriks seed) sebuah peran; nil bila perannya tidak dikenal.
func IzinBawaan(peran string) []string {
	izin, ok := rolePermissions[peran]
	if !ok {
		return nil
	}
	return slices.Clone(izin)
}

// PeranDikenal true bila peran adalah salah satu dari 14 nilai enum peran.
func PeranDikenal(peran string) bool {
	_, ok := rolePermissions[peran]
	return ok
}

// IzinDikenal true bila nama izin ada di katalog.
func IzinDikenal(nama string) bool {
	return slices.ContainsFunc(katalogIzin, func(d DefinisiIzin) bool { return d.Nama == nama })
}
