package gambar

import (
	"bytes"
	"encoding/binary"
)

// Orientasi EXIF (tag 0x0112): bagaimana gambar tersimpan harus diputar/dicerminkan agar tegak saat ditampilkan.
// Nilainya 1 sampai 8; 1 berarti sudah tegak. Parser ini sengaja minimal (hanya satu tag) dan tidak pernah panik
// pada masukan rusak: apa pun yang tidak dapat dipastikan dianggap tegak (1). Pustaka EXIF penuh tidak dipakai:
// seluruh metadata dibuang, jadi satu-satunya yang dibutuhkan dari EXIF adalah orientasi ini.
const (
	orientasiTegak = 1
	orientasiMaks  = 8

	tagOrientasi = 0x0112
	tipeShort    = 3
	tipeLong     = 4

	// Satu entri IFD berukuran 12 byte: tag (2), tipe (2), jumlah (4), nilai atau offset (4).
	ukuranEntriIFD = 12
	// IFD0 berisi puluhan entri paling banyak; batas ini menahan IFD palsu yang mengaku memuat puluhan ribu.
	maksEntriIFD = 1024
)

// orientasiEXIF membaca orientasi dari segmen APP1 "Exif" pada data JPEG. Mengembalikan 1 bila data bukan JPEG,
// tidak punya EXIF, tidak memuat tag orientasi, atau strukturnya rusak atau nilainya di luar 1 sampai 8.
func orientasiEXIF(jpegData []byte) int {
	if len(jpegData) < 4 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		return orientasiTegak
	}
	pos := 2
	for pos+4 <= len(jpegData) {
		if jpegData[pos] != 0xFF {
			return orientasiTegak // bukan awal segmen: struktur rusak
		}
		for pos < len(jpegData) && jpegData[pos] == 0xFF {
			pos++ // byte pengisi 0xFF boleh mendahului penanda
		}
		if pos >= len(jpegData) {
			return orientasiTegak
		}
		penanda := jpegData[pos]
		pos++

		switch {
		case penanda == 0xD9 || penanda == 0xDA:
			return orientasiTegak // EOI atau awal data gambar: EXIF selalu ada sebelumnya
		case penanda == 0x01 || (penanda >= 0xD0 && penanda <= 0xD8):
			continue // penanda tanpa panjang (TEM, RSTn, SOI)
		}

		if pos+2 > len(jpegData) {
			return orientasiTegak
		}
		panjang := int(binary.BigEndian.Uint16(jpegData[pos:]))
		if panjang < 2 || pos+panjang > len(jpegData) {
			return orientasiTegak
		}
		isi := jpegData[pos+2 : pos+panjang]
		if penanda == 0xE1 && bytes.HasPrefix(isi, []byte("Exif\x00\x00")) {
			return orientasiDariTIFF(isi[len("Exif\x00\x00"):])
		}
		pos += panjang
	}
	return orientasiTegak
}

// orientasiDariTIFF membaca tag orientasi dari IFD0 pada struktur TIFF (isi segmen Exif sesudah awalannya).
func orientasiDariTIFF(tiff []byte) int {
	if len(tiff) < 8 {
		return orientasiTegak
	}
	var urutan binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		urutan = binary.LittleEndian
	case "MM":
		urutan = binary.BigEndian
	default:
		return orientasiTegak
	}
	if urutan.Uint16(tiff[2:]) != 42 {
		return orientasiTegak
	}

	// Offset diperiksa dalam int64: nilai uint32 yang besar tidak boleh membungkus menjadi negatif di int 32-bit.
	awal := int64(urutan.Uint32(tiff[4:]))
	if awal+2 > int64(len(tiff)) {
		return orientasiTegak
	}
	jumlah := int64(urutan.Uint16(tiff[awal:]))
	if jumlah > maksEntriIFD {
		return orientasiTegak
	}
	entri := awal + 2
	if entri+jumlah*ukuranEntriIFD > int64(len(tiff)) {
		return orientasiTegak
	}

	for i := int64(0); i < jumlah; i++ {
		e := tiff[entri+i*ukuranEntriIFD : entri+(i+1)*ukuranEntriIFD]
		if urutan.Uint16(e) != tagOrientasi {
			continue
		}
		if urutan.Uint32(e[4:]) != 1 {
			return orientasiTegak // orientasi selalu satu nilai
		}
		var nilai uint32
		switch urutan.Uint16(e[2:]) {
		case tipeShort:
			nilai = uint32(urutan.Uint16(e[8:])) // nilai SHORT rata kiri pada kolom nilai
		case tipeLong:
			nilai = urutan.Uint32(e[8:])
		default:
			return orientasiTegak
		}
		if nilai < orientasiTegak || nilai > orientasiMaks {
			return orientasiTegak
		}
		return int(nilai)
	}
	return orientasiTegak
}
