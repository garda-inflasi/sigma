package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func teksPtr(s string) *string { return &s }

// Baris lama (dibuat sebelum snapshot pengguna menyamarkan nomor) tersimpan dengan nomor utuh: pembaca harus
// menyamarkannya, tanpa menyentuh field lain.
func TestSamarkanBaris_BarisLamaBernomorUtuh(t *testing.T) {
	row := AuditLog{
		Entitas: "pengguna",
		Sebelum: teksPtr(`{"nama":"Ani","peran":"OPS_MAHASISWA","telepon":"+6281234567890"}`),
		Sesudah: teksPtr(`{"nama":"Ani","peran":"OPS_MAHASISWA","telepon":"+6285712345678"}`),
	}

	got := SamarkanBaris(row)

	assert.JSONEq(t, `{"nama":"Ani","peran":"OPS_MAHASISWA","telepon":"+62********890"}`, *got.Sebelum)
	assert.JSONEq(t, `{"nama":"Ani","peran":"OPS_MAHASISWA","telepon":"+62********678"}`, *got.Sesudah)
	assert.NotContains(t, *got.Sebelum, "1234567", "digit tengah tidak boleh tersisa")
	assert.NotContains(t, *got.Sesudah, "1234567")
}

func TestSamarkanBaris_TidakMengubahMasukan(t *testing.T) {
	sebelum := `{"telepon":"+6281234567890"}`
	row := AuditLog{Entitas: "pengguna", Sebelum: &sebelum}

	_ = SamarkanBaris(row)

	assert.Equal(t, `{"telepon":"+6281234567890"}`, sebelum, "data mentah (dasar hash rantai) tidak boleh berubah")
	assert.Equal(t, `{"telepon":"+6281234567890"}`, *row.Sebelum)
}

func TestSamarkanBaris_YangTidakPerluDisamarkanDikembalikanApaAdanya(t *testing.T) {
	cases := map[string]AuditLog{
		"entitas lain, walau memuat kunci telepon": {Entitas: "pedagang", Sebelum: teksPtr(`{"telepon":"+6281234567890"}`)},
		"tanpa sebelum/sesudah/metadata":           {Entitas: "pengguna"},
		"telepon null (tidak ada nomor)":           {Entitas: "pengguna", Sesudah: teksPtr(`{"nama":"Ani","telepon":null}`)},
		"tanpa kunci telepon":                      {Entitas: "pengguna", Sesudah: teksPtr(`{"nama":"Ani","peran":"X"}`)},
		"sudah tersamar":                           {Entitas: "pengguna", Sesudah: teksPtr(`{"telepon":"+62********890"}`)},
		"telepon bukan teks":                       {Entitas: "pengguna", Sesudah: teksPtr(`{"telepon":12345}`)},
		"JSON rusak":                               {Entitas: "pengguna", Sesudah: teksPtr(`{bukan json`)},
		"bukan objek":                              {Entitas: "pengguna", Sesudah: teksPtr(`"+6281234567890"`)},
	}
	for nama, row := range cases {
		t.Run(nama, func(t *testing.T) {
			got := SamarkanBaris(row)

			assert.Equal(t, row, got)
			// Penunjuk yang sama: tidak ada penulisan ulang yang menggeser teks tersimpan.
			assert.Same(t, row.Sebelum, got.Sebelum)
			assert.Same(t, row.Sesudah, got.Sesudah)
		})
	}
}

func TestSamarkanBaris_MenyamarkanDiKedalamanMana_Pun(t *testing.T) {
	row := AuditLog{
		Entitas: "pengguna",
		Sesudah: teksPtr(`{"profil":{"telepon":"+6281234567890","kontak":[{"telepon":"+6285712345678"},{"telepon":null}]},"nama":"Ani"}`),
	}

	got := SamarkanBaris(row)

	assert.JSONEq(t,
		`{"profil":{"telepon":"+62********890","kontak":[{"telepon":"+62********678"},{"telepon":null}]},"nama":"Ani"}`,
		*got.Sesudah)
}

func TestSamarkanBaris_MetadataIkutDisamarkan(t *testing.T) {
	row := AuditLog{Entitas: "pengguna", Metadata: teksPtr(`{"telepon":"+6281234567890","telepon_diubah":true}`)}

	got := SamarkanBaris(row)

	assert.JSONEq(t, `{"telepon":"+62********890","telepon_diubah":true}`, *got.Metadata)
}

// Angka dipertahankan persis (tanpa jadi float) dan karakter HTML tidak di-escape: teks baris yang disajikan tidak
// bergeser dari yang tersimpan selain pada nomor teleponnya.
func TestSamarkanBaris_AngkaBesarDanKarakterHTMLTetapUtuh(t *testing.T) {
	row := AuditLog{
		Entitas: "pengguna",
		Sesudah: teksPtr(`{"id":12345678901234567890,"lembaga":"A & B <C>","pecahan":1.50,"telepon":"+6281234567890"}`),
	}

	got := SamarkanBaris(row)

	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(*got.Sesudah), &m))
	assert.Equal(t, `12345678901234567890`, string(m["id"]))
	assert.Equal(t, `1.50`, string(m["pecahan"]))
	assert.Equal(t, `"A & B <C>"`, string(m["lembaga"]), "tanpa escape \\u0026")
	assert.Equal(t, `"+62********890"`, string(m["telepon"]))
}

// Menyamarkan nilai yang sudah tersamar tidak mengubah apa pun, jadi baris baru tidak ditulis ulang.
func TestSamarkanBaris_Idempoten(t *testing.T) {
	row := AuditLog{Entitas: "pengguna", Sesudah: teksPtr(`{"telepon":"+6281234567890"}`)}

	sekali := SamarkanBaris(row)
	dua := SamarkanBaris(sekali)

	assert.Equal(t, sekali, dua)
	assert.Same(t, sekali.Sesudah, dua.Sesudah)
}

func TestSamarkanPerubahan(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	t.Run("nilai telepon disamarkan, field lain utuh", func(t *testing.T) {
		masuk := []Perubahan{
			{Path: "nama", Jenis: PerubahanDiubah, Sebelum: raw(`"Ani"`), Sesudah: raw(`"Ana"`)},
			{Path: "telepon", Jenis: PerubahanDiubah, Sebelum: raw(`"+6281234567890"`), Sesudah: raw(`"+6285712345678"`)},
		}

		got := SamarkanPerubahan("pengguna", masuk)

		require.Len(t, got, 2)
		assert.Equal(t, masuk[0], got[0])
		assert.Equal(t, `"+62********890"`, string(got[1].Sebelum))
		assert.Equal(t, `"+62********678"`, string(got[1].Sesudah))
		assert.Equal(t, `"+6281234567890"`, string(masuk[1].Sebelum), "masukan tidak diubah")
	})

	t.Run("ditambah dan dihapus: sisi yang kosong tetap kosong", func(t *testing.T) {
		got := SamarkanPerubahan("pengguna", []Perubahan{
			{Path: "telepon", Jenis: PerubahanDitambah, Sesudah: raw(`"+6281234567890"`)},
			{Path: "telepon", Jenis: PerubahanDihapus, Sebelum: raw(`"+6281234567890"`)},
			{Path: "telepon", Jenis: PerubahanDiubah, Sebelum: raw(`null`), Sesudah: raw(`"+6281234567890"`)},
		})

		assert.Empty(t, got[0].Sebelum)
		assert.Equal(t, `"+62********890"`, string(got[0].Sesudah))
		assert.Equal(t, `"+62********890"`, string(got[1].Sebelum))
		assert.Empty(t, got[1].Sesudah)
		assert.Equal(t, `null`, string(got[2].Sebelum))
		assert.Equal(t, `"+62********890"`, string(got[2].Sesudah))
	})

	t.Run("path bersarang berakhiran .telepon", func(t *testing.T) {
		got := SamarkanPerubahan("pengguna", []Perubahan{
			{Path: "profil.telepon", Jenis: PerubahanDiubah, Sebelum: raw(`"+6281234567890"`), Sesudah: raw(`"+6281234567891"`)},
			{Path: "profil.telepon_diubah", Jenis: PerubahanDitambah, Sesudah: raw(`true`)},
		})

		assert.Equal(t, `"+62********890"`, string(got[0].Sebelum))
		assert.Equal(t, `"+62********891"`, string(got[0].Sesudah))
		assert.Equal(t, `true`, string(got[1].Sesudah), "hanya field bernama telepon")
	})

	t.Run("entitas lain tidak disentuh", func(t *testing.T) {
		masuk := []Perubahan{{Path: "telepon", Jenis: PerubahanDiubah, Sebelum: raw(`"+6281234567890"`), Sesudah: raw(`"+6281234567891"`)}}

		assert.Equal(t, masuk, SamarkanPerubahan("pedagang", masuk))
	})
}

// Selisih dihitung dari data mentah: perubahan nomor yang hanya menyentuh digit tengah tetap muncul sebagai bidang
// yang berubah (dengan kedua sisi tersamar sama), bukan lenyap seperti bila selisih dihitung dari data tersamar.
func TestSamarkanPerubahan_PerubahanDigitTengahTetapTerlihat(t *testing.T) {
	sebelum, sesudah := `{"telepon":"+6281234567890"}`, `{"telepon":"+6281299997890"}`

	diff, err := DiffState(&sebelum, &sesudah)
	require.NoError(t, err)
	got := SamarkanPerubahan("pengguna", diff)

	require.Len(t, got, 1)
	assert.Equal(t, "telepon", got[0].Path)
	assert.Equal(t, PerubahanDiubah, got[0].Jenis)
	assert.Equal(t, `"+62********890"`, string(got[0].Sebelum))
	assert.Equal(t, `"+62********890"`, string(got[0].Sesudah))
}
