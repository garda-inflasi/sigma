package pii

import "testing"

func TestTeleponTersamar(t *testing.T) {
	kasus := []struct{ nama, masuk, harap string }{
		{"kosong tetap kosong", "", ""},
		{"E.164 Indonesia 11 digit nasional", "+6281234567890", "+62********890"},
		{"E.164 Indonesia 8 digit nasional (batas bawah)", "+6212345678", "+62*****678"},
		{"E.164 Indonesia 13 digit nasional (batas atas)", "+621234567890123", "+62**********123"},
		{"tujuh karakter: sebagian tersamar", "+628123", "+62*123"},
		{"enam karakter: seluruhnya tersamar", "+62812", "******"},
		{"satu karakter", "8", "*"},
		{"unicode dihitung per karakter, bukan per byte", "+62١٢٣٤٥٦٧٨٩", "+62******٧٨٩"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			if got := TeleponTersamar(k.masuk); got != k.harap {
				t.Errorf("TeleponTersamar(%q) = %q, mau %q", k.masuk, got, k.harap)
			}
		})
	}
}

func TestTeleponTersamar_PanjangSama(t *testing.T) {
	for _, nomor := range []string{"", "1", "+62812", "+6281234567890", "+621234567890123"} {
		if got, mau := len([]rune(TeleponTersamar(nomor))), len([]rune(nomor)); got != mau {
			t.Errorf("panjang hasil %d, mau %d untuk %q", got, mau, nomor)
		}
	}
}
