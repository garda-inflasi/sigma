package idx

import (
	"github.com/gin-gonic/gin"

	"github.com/garda-inflasi/api/pkg/response"
)

// ParamID membaca parameter rute BIGINT (mis. /v1/harga/:id). Bila teksnya bukan ID yang sah, handler
// menjawab 404 dengan pesanTidakAda dan mengembalikan false: ID yang salah bentuk diperlakukan sama dengan ID
// yang tidak ada, tanpa membocorkan alasan dan tanpa menyentuh basis data.
func ParamID(c *gin.Context, nama, pesanTidakAda string) (ID, bool) {
	id, err := Parse(c.Param(nama))
	if err != nil {
		response.NotFound(c, pesanTidakAda)
		return 0, false
	}
	return id, true
}

// ParamUUID membaca parameter rute UUID (mis. /v1/users/:id) dalam bentuk kanonik huruf kecil. Bila bukan
// UUID yang sah, handler menjawab 404 dengan pesanTidakAda dan mengembalikan false, sehingga teks bebas tidak
// pernah sampai ke kolom uuid (yang akan menjadi galat 500 di PostgreSQL).
func ParamUUID(c *gin.Context, nama, pesanTidakAda string) (string, bool) {
	id, ok := NormalUUID(c.Param(nama))
	if !ok {
		response.NotFound(c, pesanTidakAda)
		return "", false
	}
	return id, true
}

// RujukanUUID adalah isi parameter rute untuk entitas UUID yang juga punya kode tampilan unik (mis.
// intervensi INT-1000, policy brief PB-2026-10). Tautan lama yang memakai kode tetap berlaku sebagai alias.
type RujukanUUID struct {
	// ID terisi bila parameter berupa UUID (bentuk kanonik huruf kecil).
	ID string
	// Kode terisi bila parameter bukan UUID; dicari pada kolom kode.
	Kode string
}

// ParamUUIDAtauKode membaca parameter rute yang boleh berupa UUID atau kode tampilan. Teks kosong atau lebih
// dari maksKode karakter dijawab 404 dan mengembalikan false.
func ParamUUIDAtauKode(c *gin.Context, nama string, maksKode int, pesanTidakAda string) (RujukanUUID, bool) {
	teks := c.Param(nama)
	if id, ok := NormalUUID(teks); ok {
		return RujukanUUID{ID: id}, true
	}
	if teks == "" || len(teks) > maksKode {
		response.NotFound(c, pesanTidakAda)
		return RujukanUUID{}, false
	}
	return RujukanUUID{Kode: teks}, true
}

// Kondisi mengembalikan potongan WHERE dan argumennya untuk mencari baris dengan rujukan ini, mis.
// db.Where(r.Kondisi("id", "kode")). Nama kolom boleh diberi awalan tabel ("i.id", "i.kode").
func (r RujukanUUID) Kondisi(kolomID, kolomKode string) (string, any) {
	if r.ID != "" {
		return kolomID + " = ?", r.ID
	}
	return kolomKode + " = ?", r.Kode
}

// RujukanID adalah isi parameter rute untuk entitas BIGINT yang punya kode tampilan atau ID lama di kolom kode
// (mis. ews_alert EWS-0901, tugas_lapangan TGS-…). Tautan lama yang memakai kode tetap berlaku sebagai alias.
type RujukanID struct {
	// ID terisi bila parameter berupa bilangan bulat positif.
	ID ID
	// Kode terisi bila parameter bukan ID; dicari pada kolom kode.
	Kode string
}

// ParamIDAtauKode membaca parameter rute yang boleh berupa ID BIGINT atau kode tampilan. Teks yang lolos Parse
// selalu dibaca sebagai ID, jadi kode tidak boleh berupa angka murni. Teks kosong atau lebih dari maksKode
// karakter dijawab 404 dan mengembalikan false.
func ParamIDAtauKode(c *gin.Context, nama string, maksKode int, pesanTidakAda string) (RujukanID, bool) {
	teks := c.Param(nama)
	if id, err := Parse(teks); err == nil {
		return RujukanID{ID: id}, true
	}
	if teks == "" || len(teks) > maksKode {
		response.NotFound(c, pesanTidakAda)
		return RujukanID{}, false
	}
	return RujukanID{Kode: teks}, true
}

// Kondisi mengembalikan potongan WHERE dan argumennya untuk mencari baris dengan rujukan ini, mis.
// db.Where(r.Kondisi("id", "kode")).
func (r RujukanID) Kondisi(kolomID, kolomKode string) (string, any) {
	if r.ID != 0 {
		return kolomID + " = ?", r.ID
	}
	return kolomKode + " = ?", r.Kode
}
