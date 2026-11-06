package usecase

// Peta peran -> izin di berkas ini adalah matriks BAWAAN, bukan lagi sumber izin yang berlaku: sejak 02-C
// (Opsi B, migrasi 000107) izin yang ditegakkan API dibaca dari basis data (role_permissions + user_permissions),
// dan matriks ini hanya dipakai seeder lapis reference untuk mengisi tabel (ON CONFLICT DO NOTHING, perubahan
// admin tidak pernah tertimpa) serta sebagai pembanding "bawaan" di UI. Katalog izin yang dikenal sistem ada di
// rbac_katalog.go. Berkas packages/contracts/permissions.json dan apps/web/src/lib/rbac/permissions.ts
// (katalog + matriks bawaan) dijaga sama dengannya oleh tes.

// roleOrder menjaga urutan baku peran seperti di web (lib/rbac/roles.ts ROLE_IDS)
// agar daftar peran dari API stabil.
var roleOrder = []string{
	"STRATEGIS_FMIPA",
	"STRATEGIS_BI",
	"STRATEGIS_BPS",
	"STRATEGIS_DJPB",
	"MANAJER_PROGRAM",
	"MANAJER_DATA",
	"MANAJER_KEBIJAKAN",
	"MANAJER_PEMBIAYAAN",
	"MANAJER_RISIKO",
	"MANAJER_MUTU",
	"OPS_KOORDINATOR",
	"OPS_AGREGATOR",
	"OPS_MAHASISWA",
	"ADMIN_SISTEM",
}

// roleLapis memetakan peran ke lapisnya. Tabel pengguna menyimpan keduanya,
// jadi harus ikut berubah setiap kali peran berubah.
var roleLapis = map[string]string{
	"STRATEGIS_FMIPA":    "STRATEGIS",
	"STRATEGIS_BI":       "STRATEGIS",
	"STRATEGIS_BPS":      "STRATEGIS",
	"STRATEGIS_DJPB":     "STRATEGIS",
	"MANAJER_PROGRAM":    "MANAJERIAL",
	"MANAJER_DATA":       "MANAJERIAL",
	"MANAJER_KEBIJAKAN":  "MANAJERIAL",
	"MANAJER_PEMBIAYAAN": "MANAJERIAL",
	"MANAJER_RISIKO":     "MANAJERIAL",
	"MANAJER_MUTU":       "MANAJERIAL",
	"OPS_KOORDINATOR":    "OPERASIONAL",
	"OPS_AGREGATOR":      "OPERASIONAL",
	"OPS_MAHASISWA":      "OPERASIONAL",
	"ADMIN_SISTEM":       "SISTEM",
}

// rolePermissions adalah matriks bawaan peran -> izin (cermin ROLE_PERMISSIONS di web lib/rbac/permissions.ts).
// Jangan dipakai untuk memutuskan akses: itu urusan basis data (lihat izin.go).
var rolePermissions = map[string][]string{
	"STRATEGIS_FMIPA": {
		"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
		"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
		"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "brief.lihat",
		"brief.setujui", "kpi.lihat", "master.lihat", "master.setujui", "sumber.lihat", "audit.lihat",
	},
	"STRATEGIS_BI": {
		"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
		"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
		"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "brief.lihat",
		"brief.setujui", "kpi.lihat", "master.lihat", "sumber.lihat", "audit.lihat",
	},
	"STRATEGIS_BPS": {
		"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
		"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
		"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "brief.lihat",
		"brief.setujui", "kpi.lihat", "master.lihat", "master.setujui", "sumber.lihat", "audit.lihat",
	},
	"STRATEGIS_DJPB": {
		"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
		"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
		"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "intervensi.setujui_fiskal",
		"brief.lihat", "brief.setujui", "kpi.lihat", "master.lihat", "sumber.lihat", "audit.lihat",
	},
	"MANAJER_PROGRAM": {
		"dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "harga.ekspor",
		"tugas.kelola", "ews.lihat", "ews.akui", "ews.eskalasi", "prediksi.lihat",
		"spasial.lihat", "logistik.kelola", "coldstorage.lihat", "fiskal.lihat",
		"intervensi.lihat", "intervensi.kelola", "brief.lihat", "brief.susun",
		"kpi.lihat", "master.lihat", "master.ajukan", "sumber.lihat", "audit.lihat",
	},
	"MANAJER_DATA": {
		"dashboard.operasional", "harga.lihat", "harga.publikasi", "harga.ekspor",
		"ews.lihat", "ews.akui", "ews.eskalasi", "ews.atur_ambang", "prediksi.lihat",
		"model.kelola", "spasial.lihat", "intervensi.lihat", "brief.lihat",
		"kpi.lihat", "master.lihat", "master.ajukan", "sumber.lihat", "sumber.kelola", "audit.lihat",
	},
	"MANAJER_KEBIJAKAN": {
		"dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "ews.lihat",
		"prediksi.lihat", "spasial.lihat", "fiskal.lihat", "intervensi.lihat",
		"intervensi.kelola", "brief.lihat", "brief.susun", "kpi.lihat",
	},
	"MANAJER_PEMBIAYAAN": {
		"dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "spasial.lihat",
		"coldstorage.lihat", "fiskal.lihat", "intervensi.lihat", "brief.lihat", "kpi.lihat",
	},
	"MANAJER_RISIKO": {
		"dashboard.operasional", "harga.lihat", "harga.publikasi", "harga.ekspor",
		"ews.lihat", "ews.akui", "ews.eskalasi", "coldstorage.lihat",
		"intervensi.lihat", "kpi.lihat", "master.lihat", "audit.lihat",
	},
	"MANAJER_MUTU": {
		"dashboard.operasional", "harga.lihat", "ews.lihat", "prediksi.lihat",
		"coldstorage.lihat", "coldstorage.kelola", "intervensi.lihat", "intervensi.kelola", "kpi.lihat",
	},
	"OPS_KOORDINATOR": {
		"dashboard.operasional", "harga.input", "harga.lihat", "harga.validasi",
		"tugas.kelola", "tugas.lihat_sendiri", "ews.lihat", "ews.akui",
		"spasial.lihat", "intervensi.lihat", "master.lihat",
	},
	"OPS_AGREGATOR": {
		"dashboard.operasional", "harga.input", "harga.lihat", "tugas.lihat_sendiri",
		"ews.lihat", "coldstorage.lihat", "coldstorage.kelola", "logistik.kelola", "intervensi.lihat",
	},
	"OPS_MAHASISWA": {
		"harga.input", "tugas.lihat_sendiri",
	},
	"ADMIN_SISTEM": {
		"pengguna.kelola", "master.lihat", "master.ajukan", "sumber.lihat", "sumber.kelola", "audit.lihat",
	},
}
