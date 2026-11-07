package seedkit

import "github.com/garda-inflasi/api/pkg/idx"

// IDPengguna adalah UUID akun seed untuk slug lamanya (superadmin, mahasiswa_9, manajer_kebijakan, ...).
//
// pengguna.id adalah UUID (migrasi 000110). Basis data lama dimigrasi dengan garda_uuid_stabil('pengguna', slug)
// dan basis data baru di-seed dengan nilai yang sama, sehingga seeder modul mana pun yang merujuk akun demo
// (pembuat intervensi, penyusun brief, pengaku alert, petugas lapangan) memakai satu rumus ini, bukan slug.
func IDPengguna(slug string) string { return idx.UUIDStabil("pengguna", slug) }

// SlugDemo adalah slug lama seluruh akun yang dibuat seeder: admin awal (auth.admin) dan 25 akun auth.demo.
// Dipakai SlugPengguna; tes di paket seeder auth memastikan daftar ini sama dengan akun demo.
var SlugDemo = []string{
	"admin-awal", "superadmin",
	"strategis_fmipa", "strategis_bi", "strategis_bps", "strategis_djpb",
	"manajer_program", "manajer_data", "manajer_kebijakan", "manajer_pembiayaan", "manajer_risiko", "manajer_mutu",
	"koordinator_lapangan", "agregator_data",
	"mahasiswa_1", "mahasiswa_2", "mahasiswa_3", "mahasiswa_4", "mahasiswa_5",
	"mahasiswa_6", "mahasiswa_7", "mahasiswa_8", "mahasiswa_9", "mahasiswa_10",
	"user_nonaktif", "user_mfa",
}

var slugDariID = func() map[string]string {
	m := make(map[string]string, len(SlugDemo))
	for _, s := range SlugDemo {
		m[IDPengguna(s)] = s
	}
	return m
}()

// SlugPengguna mengembalikan slug lama akun seed untuk UUID-nya, atau id itu sendiri bila bukan akun seed.
//
// Dipakai seeder yang menurunkan ID baris dari ID pengguna (mis. tugas lapangan "TGS-<slug>-1"): ID turunan
// tetap sama dengan yang ada di basis data yang dimigrasi dari ID lama, jadi seed ulang tidak menggandakan
// baris. Akun buatan aplikasi (UUID acak) tidak punya slug dan dipakai apa adanya.
func SlugPengguna(id string) string {
	if s, ok := slugDariID[id]; ok {
		return s
	}
	return id
}
