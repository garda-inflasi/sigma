package domain

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/garda-inflasi/api/pkg/pii"
)

// Penyamaran data pribadi (UU PDP) pada baris audit yang DIBACA.
//
// Snapshot pengguna kini menyamarkan nomor telepon saat ditulis (auth usecase.userSnapshot), tetapi audit_log
// append-only dan rantai hash-nya dihitung dari data mentah: baris lama yang sudah memuat nomor utuh tidak dapat
// ditulis ulang. Pembaca (daftar, detail, dan ekspor) menyamarkannya saat menyajikan, sehingga baris lama pun
// tidak tampil utuh di layar maupun di berkas ekspor. Data tersimpan dan hash tidak berubah; verifikasi rantai
// membaca baris lewat repositori, bukan lewat pembaca ini.

const (
	// entitasPengguna adalah entitas baris audit yang snapshot-nya memuat telepon pengguna.
	entitasPengguna = "pengguna"
	// kunciTelepon adalah nama field telepon pada snapshot dan metadata.
	kunciTelepon = "telepon"
)

// SamarkanBaris mengembalikan salinan baris dengan nilai teks field "telepon" pada sebelum, sesudah, dan metadata
// disamarkan (pii.TeleponTersamar). Hanya baris entitas pengguna yang disentuh; baris lain dikembalikan apa adanya.
// Menyamarkan nilai yang sudah tersamar tidak mengubahnya, jadi aman bagi baris baru.
func SamarkanBaris(row AuditLog) AuditLog {
	if row.Entitas != entitasPengguna {
		return row
	}
	row.Sebelum = samarkanJSON(row.Sebelum)
	row.Sesudah = samarkanJSON(row.Sesudah)
	row.Metadata = samarkanJSON(row.Metadata)
	return row
}

// SamarkanPerubahan menyamarkan nilai perubahan pada field telepon (path "telepon" atau berakhiran ".telepon") untuk
// baris entitas pengguna. Selisihnya dihitung dari data mentah supaya perubahan nomor tetap terlihat sebagai
// bidang yang berubah, tanpa membuka digitnya. Hasilnya salinan; masukan tidak diubah.
func SamarkanPerubahan(entitas string, perubahan []Perubahan) []Perubahan {
	if entitas != entitasPengguna {
		return perubahan
	}
	out := make([]Perubahan, len(perubahan))
	for i, p := range perubahan {
		if p.Path == kunciTelepon || strings.HasSuffix(p.Path, "."+kunciTelepon) {
			p.Sebelum = samarkanNilai(p.Sebelum)
			p.Sesudah = samarkanNilai(p.Sesudah)
		}
		out[i] = p
	}
	return out
}

// samarkanNilai menyamarkan satu nilai JSON bila berupa teks; null, angka, dan yang kosong tidak berubah.
func samarkanNilai(raw json.RawMessage) json.RawMessage {
	// Dibaca sebagai any: null dan angka bukan teks, sedangkan Unmarshal null ke string tidak galat dan memberi "".
	var nilai any
	if len(raw) == 0 || json.Unmarshal(raw, &nilai) != nil {
		return raw
	}
	teks, ok := nilai.(string)
	if !ok {
		return raw
	}
	tersamar := pii.TeleponTersamar(teks)
	if tersamar == teks {
		return raw
	}
	if encoded, err := encodeJSON(tersamar); err == nil {
		return encoded
	}
	return raw
}

// samarkanJSON menyamarkan setiap field "telepon" bernilai teks, di kedalaman mana pun, pada dokumen JSON. Dokumen
// yang tidak memuatnya (atau bukan JSON yang sah) dikembalikan tanpa diubah; angka dipertahankan persis seperti
// aslinya (json.Number) dan karakter HTML tidak di-escape, supaya teksnya tidak bergeser dari yang tersimpan.
func samarkanJSON(raw *string) *string {
	if raw == nil {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(*raw))
	dec.UseNumber()
	var dokumen any
	if err := dec.Decode(&dokumen); err != nil {
		return raw
	}
	if !samarkanTelepon(dokumen) {
		return raw
	}
	encoded, err := encodeJSON(dokumen)
	if err != nil {
		return raw
	}
	hasil := string(encoded)
	return &hasil
}

// samarkanTelepon menyamarkan field "telepon" bertipe teks pada objek dan larik bersarang; true bila ada yang diubah.
func samarkanTelepon(nilai any) bool {
	berubah := false
	switch v := nilai.(type) {
	case map[string]any:
		for kunci, anak := range v {
			if teks, ok := anak.(string); ok {
				if kunci == kunciTelepon {
					if tersamar := pii.TeleponTersamar(teks); tersamar != teks {
						v[kunci] = tersamar
						berubah = true
					}
				}
				continue
			}
			berubah = samarkanTelepon(anak) || berubah
		}
	case []any:
		for _, anak := range v {
			berubah = samarkanTelepon(anak) || berubah
		}
	}
	return berubah
}

// encodeJSON menulis v tanpa meng-escape <, >, dan & (bawaan json.Marshal) dan tanpa baris baru penutup.
func encodeJSON(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
