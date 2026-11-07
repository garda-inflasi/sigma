package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
)

// Rantai hash audit_log (migrasi 000116). HitungHash adalah padanan Go dari fungsi SQL audit_log_hitung_hash:
// keduanya wajib menghasilkan nilai yang sama untuk baris yang sama (diuji di integration/rantai_test.go).
// Verifikasi dilakukan di Go agar tidak bergantung pada fungsi di basis data yang bisa saja ikut diganti.

// versiRantai adalah awalan masukan hash; mengubah format berarti versi baru.
const versiRantai = "garda-audit-v1"

// UkuranHash adalah panjang hash SHA-256 dalam bita.
const UkuranHash = sha256.Size

// HitungHash menghitung hash satu baris audit yang dirantai ke hashSebelumnya (nil untuk baris pertama).
// aktor_id sengaja tidak ikut (boleh dikosongkan FK ON DELETE SET NULL); identitas pelaku terlindungi lewat
// snapshot aktor_nama/aktor_email/aktor_peran.
func HitungHash(hashSebelumnya []byte, row AuditLog) []byte {
	var buf bytes.Buffer
	buf.WriteString(versiRantai)
	tulisBidang(&buf, hashSebelumnya, hashSebelumnya != nil)
	tulisTeks(&buf, new(strconv.FormatInt(int64(row.ID), 10)))
	tulisTeks(&buf, new(strconv.FormatInt(row.Waktu.UnixMicro(), 10)))
	for _, s := range []*string{
		row.Kode, row.AktorNama, row.AktorEmail, row.AktorPeran,
		&row.Aksi, &row.Entitas, &row.EntitasID, &row.Hasil,
		row.IP, row.UserAgent, row.CorrelationID,
		row.Sebelum, row.Sesudah, row.Metadata,
	} {
		tulisTeks(&buf, s)
	}
	sum := sha256.Sum256(buf.Bytes())
	return sum[:]
}

func tulisTeks(buf *bytes.Buffer, s *string) {
	if s == nil {
		tulisBidang(buf, nil, false)
		return
	}
	tulisBidang(buf, []byte(*s), true)
}

// tulisBidang: 0x00 untuk NULL, atau 0x01 + panjang (uint32 big-endian) + isi.
func tulisBidang(buf *bytes.Buffer, b []byte, ada bool) {
	if !ada {
		buf.WriteByte(0)
		return
	}
	buf.WriteByte(1)
	var panjang [4]byte
	binary.BigEndian.PutUint32(panjang[:], uint32(len(b)))
	buf.Write(panjang[:])
	buf.Write(b)
}

// Jenis masalah yang ditemukan verifikasi.
const (
	// MasalahIsiBerubah: isi baris tidak lagi cocok dengan hash yang tersimpan.
	MasalahIsiBerubah = "ISI_BERUBAH"
	// MasalahRantaiPutus: hash_sebelumnya tidak sama dengan hash baris sebelumnya (baris disisipkan, dihapus, atau
	// hash-nya diganti).
	MasalahRantaiPutus = "RANTAI_PUTUS"
	// MasalahArsipTidakCocok: rantai tidak tersambung ke segmen yang sudah diarsipkan.
	MasalahArsipTidakCocok = "ARSIP_TIDAK_COCOK"
)

// Masalah adalah satu temuan verifikasi rantai.
type Masalah struct {
	ID         int64  `json:"id,string"`
	Jenis      string `json:"jenis"`
	Keterangan string `json:"keterangan"`
}

// SegmenArsip adalah rangkaian id berurutan yang sudah dipindah ke berkas arsip: HashMasuk adalah hash_sebelumnya
// baris pertamanya (nil bila segmen ini awal rantai), HashKeluar hash baris terakhirnya.
type SegmenArsip struct {
	ArsipID     int64  `gorm:"column:arsip_id" json:"arsip_id,string"`
	IDAwal      int64  `gorm:"column:id_awal" json:"id_awal,string"`
	IDAkhir     int64  `gorm:"column:id_akhir" json:"id_akhir,string"`
	JumlahBaris int64  `gorm:"column:jumlah_baris" json:"jumlah_baris"`
	HashMasuk   []byte `gorm:"column:hash_masuk" json:"-"`
	HashKeluar  []byte `gorm:"column:hash_keluar" json:"-"`
}

// MaksMasalahDicatat membatasi daftar masalah yang disimpan per verifikasi; jumlahnya tetap dihitung semua.
const MaksMasalahDicatat = 20

// Verifikator memeriksa rantai baris audit yang diserahkan berurutan menurut id naik, menyambung lewat segmen arsip
// yang id-nya berada di antara baris. Setelah sebuah masalah, verifikasi berlanjut dengan berpegang pada hash baris
// itu sendiri, sehingga satu perusakan dilaporkan sekali dan perusakan lain tetap terdeteksi.
type Verifikator struct {
	segmen     []SegmenArsip
	si         int
	sebelum    []byte
	idTerakhir int64
	mulai      bool

	JumlahBaris   int64
	JumlahSegmen  int64
	JumlahMasalah int64
	Masalah       []Masalah
	IDAwal        int64
	IDAkhir       int64
}

// NewVerifikator membuat verifikator untuk rantai yang berawal dari genesis; segmen harus urut menurut IDAwal.
func NewVerifikator(segmen []SegmenArsip) *Verifikator {
	return &Verifikator{segmen: segmen}
}

// Periksa memproses satu baris; baris harus diserahkan dengan id naik.
func (v *Verifikator) Periksa(row AuditLog) error {
	id := int64(row.ID)
	if v.mulai && id <= v.idTerakhir {
		return fmt.Errorf("baris audit tidak urut: id %d sesudah %d", id, v.idTerakhir)
	}
	v.lewatiSegmenSebelum(id)

	if !bytes.Equal(row.HashSebelumnya, v.sebelum) {
		v.catat(id, MasalahRantaiPutus, fmt.Sprintf("hash_sebelumnya %s, seharusnya %s",
			ringkasHex(row.HashSebelumnya), ringkasHex(v.sebelum)))
	}
	if !bytes.Equal(HitungHash(row.HashSebelumnya, row), row.Hash) {
		v.catat(id, MasalahIsiBerubah, "isi baris tidak cocok dengan hash yang tersimpan")
	}

	if !v.mulai {
		v.IDAwal = id
	}
	v.mulai = true
	v.sebelum, v.idTerakhir, v.IDAkhir = row.Hash, id, id
	v.JumlahBaris++
	return nil
}

// Selesai memproses segmen arsip yang tersisa sesudah baris terakhir (normalnya tidak ada: kepala rantai tidak
// pernah diarsipkan) dan mengembalikan hash kepala rantai.
func (v *Verifikator) Selesai() []byte {
	v.lewatiSegmenSebelum(-1)
	return v.sebelum
}

// HashTerakhir adalah hash baris atau segmen terakhir yang sudah diproses.
func (v *Verifikator) HashTerakhir() []byte { return v.sebelum }

// lewatiSegmenSebelum menyambung rantai melewati segmen arsip yang seluruhnya sebelum id (semua bila id < 0).
func (v *Verifikator) lewatiSegmenSebelum(id int64) {
	for v.si < len(v.segmen) && (id < 0 || v.segmen[v.si].IDAkhir < id) {
		s := v.segmen[v.si]
		if v.mulai && s.IDAwal <= v.idTerakhir {
			v.catat(s.IDAwal, MasalahArsipTidakCocok, fmt.Sprintf("segmen arsip %d-%d bertumpang tindih dengan baris %d",
				s.IDAwal, s.IDAkhir, v.idTerakhir))
		} else if !bytes.Equal(s.HashMasuk, v.sebelum) {
			v.catat(s.IDAwal, MasalahArsipTidakCocok, fmt.Sprintf("segmen arsip %d-%d berawal dari %s, seharusnya %s",
				s.IDAwal, s.IDAkhir, ringkasHex(s.HashMasuk), ringkasHex(v.sebelum)))
		}
		v.sebelum = s.HashKeluar
		if s.IDAkhir > v.idTerakhir {
			v.idTerakhir = s.IDAkhir
		}
		v.mulai = true
		v.JumlahSegmen++
		v.si++
	}
}

func (v *Verifikator) catat(id int64, jenis, keterangan string) {
	v.JumlahMasalah++
	if len(v.Masalah) < MaksMasalahDicatat {
		v.Masalah = append(v.Masalah, Masalah{ID: id, Jenis: jenis, Keterangan: keterangan})
	}
}

// ringkasHex menampilkan 12 karakter awal hash (atau "kosong").
func ringkasHex(b []byte) string {
	if b == nil {
		return "kosong"
	}
	s := hex.EncodeToString(b)
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
