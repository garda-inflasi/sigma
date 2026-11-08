package domain

import "time"

// PenggunaPatch adalah perubahan sebagian atas profil pengguna (PATCH admin, ganti peran): hanya kolom yang
// non-nil yang ditulis, kolom lain tidak disentuh. Penulisan per kolom (bukan GORM Save seluruh baris) menjaga
// perubahan yang terjadi serentak pada kolom lain (penonaktifan, reset kata sandi, aktivasi MFA) tetap utuh.
type PenggunaPatch struct {
	Nama          *string
	Email         *string
	Peran         *string
	Lapis         *string
	Lembaga       *string
	KabupatenKode *string
	Status        *string
	// Telepon: pointer ke "" menghapus nomor, yang ditulis sebagai NULL (bukan string kosong); nilai lain sudah
	// dinormalkan pemanggil (usecase.NormalisasiTelepon).
	Telepon *string
}

// Kosong true bila tidak ada kolom yang diubah.
func (p PenggunaPatch) Kosong() bool { return len(p.Kolom()) == 0 }

// Kolom adalah nama kolom tabel pengguna beserta nilai barunya, hanya untuk kolom yang diubah.
func (p PenggunaPatch) Kolom() map[string]any {
	kolom := map[string]any{}
	set := func(nama string, nilai *string) {
		if nilai != nil {
			kolom[nama] = *nilai
		}
	}
	set("nama", p.Nama)
	set("email", p.Email)
	set("peran", p.Peran)
	set("lapis", p.Lapis)
	set("lembaga", p.Lembaga)
	set("kabupaten_kode", p.KabupatenKode)
	set("status", p.Status)
	if p.Telepon != nil {
		kolom["telepon"] = nilJikaKosong(*p.Telepon)
	}
	return kolom
}

// nilJikaKosong memetakan "" ke nil agar nomor yang dihapus ditulis NULL, bukan string kosong.
func nilJikaKosong(nilai string) any {
	if nilai == "" {
		return nil
	}
	return nilai
}

// Terapkan menulis kolom yang diubah ke salinan pengguna di memori (untuk penyimpanan tiruan dan tes).
func (p PenggunaPatch) Terapkan(u *Pengguna) {
	if p.Nama != nil {
		u.Nama = *p.Nama
	}
	if p.Email != nil {
		u.Email = *p.Email
	}
	if p.Peran != nil {
		u.Peran = *p.Peran
	}
	if p.Lapis != nil {
		u.Lapis = *p.Lapis
	}
	if p.Lembaga != nil {
		u.Lembaga = *p.Lembaga
	}
	if p.KabupatenKode != nil {
		kode := *p.KabupatenKode
		u.KabupatenKode = &kode
	}
	if p.Status != nil {
		u.Status = *p.Status
	}
	if p.Telepon != nil {
		u.Telepon = nil
		if *p.Telepon != "" {
			telepon := *p.Telepon
			u.Telepon = &telepon
		}
	}
}

// GantiPassword adalah penulisan kolom kata sandi saja: hash, penanda wajib ganti, dan waktu perubahan.
type GantiPassword struct {
	Hash       string
	WajibGanti bool
	At         time.Time
	// HashSebelumnya, bila diisi, mensyaratkan hash kata sandi yang tersimpan masih persis sama (compare-and-swap):
	// pengguna yang mengganti kata sandi setelah memverifikasi yang lama tidak boleh menimpa kata sandi yang
	// diubah pihak lain (reset admin, penggantian serentak) di sela-selanya. Nil = tulis tanpa syarat.
	HashSebelumnya *string
}
