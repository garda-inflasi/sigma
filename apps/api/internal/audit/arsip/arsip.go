// Package arsip menulis dan memverifikasi berkas arsip partisi audit_log (penyimpanan dingin, 03-C).
//
// Satu partisi bulanan menjadi dua berkas di direktori arsip:
//
//	audit_log_pYYYYMM.jsonl.gz         satu baris JSON per baris audit, urut id naik (gzip)
//	audit_log_pYYYYMM.manifest.json    ringkasan: jumlah baris, SHA-256 berkas, ringkasan hash, dan segmen rantai
//
// Kolom JSONB disimpan sebagai teks persis seperti dikeluarkan Postgres, dan waktu dalam mikrodetik, sehingga hash
// setiap baris bisa dihitung ulang tanpa basis data (Verifikasi). Direktori arsip sebaiknya disalin ke penyimpanan
// objek atau media lain di luar VM; berkas tidak pernah diubah setelah manifest-nya ditulis.
package arsip

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// FormatManifest adalah versi format arsip.
const FormatManifest = "garda-audit-arsip-v1"

// NamaBerkasData dan NamaBerkasManifest adalah nama berkas arsip untuk satu partisi.
func NamaBerkasData(partisi string) string     { return partisi + ".jsonl.gz" }
func NamaBerkasManifest(partisi string) string { return partisi + ".manifest.json" }

// Baris adalah satu baris audit dalam berkas arsip.
type Baris struct {
	ID             string  `json:"id"`
	WaktuUS        int64   `json:"waktu_us"`
	Waktu          string  `json:"waktu"`
	Kode           *string `json:"kode,omitempty"`
	AktorID        *string `json:"aktor_id,omitempty"`
	AktorNama      *string `json:"aktor_nama,omitempty"`
	AktorEmail     *string `json:"aktor_email,omitempty"`
	AktorPeran     *string `json:"aktor_peran,omitempty"`
	Aksi           string  `json:"aksi"`
	Entitas        string  `json:"entitas"`
	EntitasID      string  `json:"entitas_id"`
	Hasil          string  `json:"hasil"`
	IP             *string `json:"ip,omitempty"`
	UserAgent      *string `json:"user_agent,omitempty"`
	CorrelationID  *string `json:"correlation_id,omitempty"`
	Sebelum        *string `json:"sebelum,omitempty"`
	Sesudah        *string `json:"sesudah,omitempty"`
	Metadata       *string `json:"metadata,omitempty"`
	HashSebelumnya string  `json:"hash_sebelumnya,omitempty"`
	Hash           string  `json:"hash"`
}

// DariAudit memetakan baris basis data ke baris arsip.
func DariAudit(row domain.AuditLog) Baris {
	b := Baris{
		ID: row.ID.String(), WaktuUS: row.Waktu.UnixMicro(), Waktu: row.Waktu.UTC().Format(time.RFC3339Nano),
		Kode: row.Kode, AktorID: row.AktorID, AktorNama: row.AktorNama, AktorEmail: row.AktorEmail,
		AktorPeran: row.AktorPeran, Aksi: row.Aksi, Entitas: row.Entitas, EntitasID: row.EntitasID, Hasil: row.Hasil,
		IP: row.IP, UserAgent: row.UserAgent, CorrelationID: row.CorrelationID,
		Sebelum: row.Sebelum, Sesudah: row.Sesudah, Metadata: row.Metadata,
		Hash: hex.EncodeToString(row.Hash),
	}
	if row.HashSebelumnya != nil {
		b.HashSebelumnya = hex.EncodeToString(row.HashSebelumnya)
	}
	return b
}

// KeAudit memetakan baris arsip kembali ke baris audit (untuk menghitung ulang hash).
func (b Baris) KeAudit() (domain.AuditLog, error) {
	id, err := strconv.ParseInt(b.ID, 10, 64)
	if err != nil {
		return domain.AuditLog{}, fmt.Errorf("id %q tidak valid", b.ID)
	}
	hashBaris, err := hex.DecodeString(b.Hash)
	if err != nil || len(hashBaris) != domain.UkuranHash {
		return domain.AuditLog{}, fmt.Errorf("baris %s: hash tidak valid", b.ID)
	}
	var sebelumnya []byte
	if b.HashSebelumnya != "" {
		if sebelumnya, err = hex.DecodeString(b.HashSebelumnya); err != nil || len(sebelumnya) != domain.UkuranHash {
			return domain.AuditLog{}, fmt.Errorf("baris %s: hash_sebelumnya tidak valid", b.ID)
		}
	}
	return domain.AuditLog{
		ID: idx.ID(id), Kode: b.Kode, Waktu: time.UnixMicro(b.WaktuUS).UTC(),
		AktorID: b.AktorID, AktorNama: b.AktorNama, AktorEmail: b.AktorEmail, AktorPeran: b.AktorPeran,
		Aksi: b.Aksi, Entitas: b.Entitas, EntitasID: b.EntitasID, Hasil: b.Hasil,
		IP: b.IP, UserAgent: b.UserAgent, CorrelationID: b.CorrelationID,
		Sebelum: b.Sebelum, Sesudah: b.Sesudah, Metadata: b.Metadata,
		HashSebelumnya: sebelumnya, Hash: hashBaris,
	}, nil
}

// Segmen adalah segmen rantai dalam manifest (hash dalam hex; hash_masuk kosong = awal rantai).
type Segmen struct {
	IDAwal      string `json:"id_awal"`
	IDAkhir     string `json:"id_akhir"`
	JumlahBaris int64  `json:"jumlah_baris"`
	HashMasuk   string `json:"hash_masuk,omitempty"`
	HashKeluar  string `json:"hash_keluar"`
}

// Manifest menjelaskan satu berkas arsip.
type Manifest struct {
	Format         string    `json:"format"`
	ArsipID        string    `json:"arsip_id"`
	Partisi        string    `json:"partisi"`
	Dari           time.Time `json:"dari"`
	Sampai         time.Time `json:"sampai"`
	JumlahBaris    int64     `json:"jumlah_baris"`
	IDPertama      string    `json:"id_pertama,omitempty"`
	IDTerakhir     string    `json:"id_terakhir,omitempty"`
	Berkas         string    `json:"berkas"`
	SHA256Berkas   string    `json:"sha256_berkas"`
	Ringkasan      string    `json:"ringkasan"`
	RetensiBulan   int       `json:"retensi_bulan"`
	DiarsipkanPada time.Time `json:"diarsipkan_pada"`
	Segmen         []Segmen  `json:"segmen"`
}

// ManifestDari menyusun manifest dari catatan arsip dan segmennya di basis data.
func ManifestDari(a domain.Arsip, segmen []domain.SegmenArsip) Manifest {
	m := Manifest{
		Format: FormatManifest, ArsipID: a.ID.String(), Partisi: a.Partisi, Dari: a.Dari.UTC(), Sampai: a.Sampai.UTC(),
		JumlahBaris: a.JumlahBaris, Berkas: a.Berkas, SHA256Berkas: a.SHA256Berkas,
		Ringkasan: hex.EncodeToString(a.Ringkasan), RetensiBulan: a.RetensiBulan, DiarsipkanPada: a.DiarsipkanPada.UTC(),
		Segmen: make([]Segmen, len(segmen)),
	}
	if a.IDPertama != nil {
		m.IDPertama = strconv.FormatInt(*a.IDPertama, 10)
	}
	if a.IDTerakhir != nil {
		m.IDTerakhir = strconv.FormatInt(*a.IDTerakhir, 10)
	}
	for i, s := range segmen {
		m.Segmen[i] = Segmen{
			IDAwal: strconv.FormatInt(s.IDAwal, 10), IDAkhir: strconv.FormatInt(s.IDAkhir, 10),
			JumlahBaris: s.JumlahBaris, HashKeluar: hex.EncodeToString(s.HashKeluar),
		}
		if s.HashMasuk != nil {
			m.Segmen[i].HashMasuk = hex.EncodeToString(s.HashMasuk)
		}
	}
	return m
}

// Penulis menulis satu berkas data arsip. Berkas ditulis ke nama sementara dan baru mendapat nama akhirnya saat
// Tutup berhasil, sehingga berkas setengah jadi tidak pernah tampak sebagai arsip.
type Penulis struct {
	path, tmp string
	f         *os.File
	buf       *bufio.Writer
	gz        *gzip.Writer
	enc       *json.Encoder
	ringkasan hash.Hash
	n         int64
}

// BuatPenulis membuat berkas data arsip di path (direktori induknya dibuat bila belum ada).
func BuatPenulis(path string) (*Penulis, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("buat direktori arsip: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("buat berkas arsip: %w", err)
	}
	buf := bufio.NewWriterSize(f, 256<<10)
	gz := gzip.NewWriter(buf)
	return &Penulis{path: path, tmp: tmp, f: f, buf: buf, gz: gz, enc: json.NewEncoder(gz), ringkasan: sha256.New()}, nil
}

// Tulis menambahkan satu baris; baris harus diserahkan urut id naik.
func (p *Penulis) Tulis(row domain.AuditLog) error {
	if err := p.enc.Encode(DariAudit(row)); err != nil {
		return fmt.Errorf("tulis baris arsip %s: %w", row.ID, err)
	}
	p.ringkasan.Write(row.Hash)
	p.n++
	return nil
}

// JumlahBaris adalah banyaknya baris yang sudah ditulis.
func (p *Penulis) JumlahBaris() int64 { return p.n }

// Ringkasan adalah SHA-256 atas gabungan hash baris yang sudah ditulis (sama dengan audit_log_arsip.ringkasan).
func (p *Penulis) Ringkasan() []byte { return p.ringkasan.Sum(nil) }

// Tutup menuntaskan gzip, menyinkronkan berkas ke disk, memberinya nama akhir, dan mengembalikan SHA-256 berkas.
func (p *Penulis) Tutup() (string, error) {
	err := errors.Join(p.gz.Close(), p.buf.Flush(), p.f.Sync(), p.f.Close())
	if err != nil {
		_ = os.Remove(p.tmp)
		return "", fmt.Errorf("tutup berkas arsip: %w", err)
	}
	if err := os.Rename(p.tmp, p.path); err != nil {
		_ = os.Remove(p.tmp)
		return "", fmt.Errorf("simpan berkas arsip: %w", err)
	}
	return SHA256Berkas(p.path)
}

// Batal membuang berkas yang belum selesai ditulis.
func (p *Penulis) Batal() {
	_ = p.gz.Close()
	_ = p.f.Close()
	_ = os.Remove(p.tmp)
}

// SHA256Berkas menghitung SHA-256 isi berkas (hex).
func SHA256Berkas(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// TulisManifest menulis manifest secara atomik (berkas sementara lalu rename).
func TulisManifest(path string, m Manifest) error {
	isi, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(isi, '\n'), 0o640); err != nil {
		return fmt.Errorf("tulis manifest arsip: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("simpan manifest arsip: %w", err)
	}
	return nil
}

// HasilVerifikasi adalah hasil memeriksa satu berkas arsip tanpa basis data.
type HasilVerifikasi struct {
	Partisi       string
	JumlahBaris   int64
	JumlahSegmen  int
	JumlahMasalah int64
	Masalah       []domain.Masalah
}

// Valid melaporkan apakah berkas utuh.
func (h HasilVerifikasi) Valid() bool { return h.JumlahMasalah == 0 }

// Verifikasi memeriksa satu arsip dari manifest-nya: SHA-256 berkas, jumlah baris, ringkasan hash, hash setiap baris,
// dan sambungan rantai di dalam setiap segmen. Galat dikembalikan bila berkas tidak bisa dibaca sama sekali;
// ketidakcocokan isi dilaporkan di HasilVerifikasi.
func Verifikasi(pathManifest string) (HasilVerifikasi, error) {
	isi, err := os.ReadFile(pathManifest)
	if err != nil {
		return HasilVerifikasi{}, fmt.Errorf("baca manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(isi, &m); err != nil {
		return HasilVerifikasi{}, fmt.Errorf("manifest tidak valid: %w", err)
	}
	if m.Format != FormatManifest {
		return HasilVerifikasi{}, fmt.Errorf("format manifest %q tidak dikenal", m.Format)
	}
	h := HasilVerifikasi{Partisi: m.Partisi, JumlahSegmen: len(m.Segmen)}
	catat := func(id int64, jenis, ket string) {
		h.JumlahMasalah++
		if len(h.Masalah) < domain.MaksMasalahDicatat {
			h.Masalah = append(h.Masalah, domain.Masalah{ID: id, Jenis: jenis, Keterangan: ket})
		}
	}

	pathData := filepath.Join(filepath.Dir(pathManifest), filepath.Base(m.Berkas))
	sha, err := SHA256Berkas(pathData)
	if err != nil {
		return h, fmt.Errorf("baca berkas arsip: %w", err)
	}
	if sha != m.SHA256Berkas {
		catat(0, domain.MasalahIsiBerubah, "SHA-256 berkas tidak sama dengan manifest")
	}

	segmen, err := segmenManifest(m.Segmen)
	if err != nil {
		return h, err
	}
	f, err := os.Open(pathData)
	if err != nil {
		return h, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return h, fmt.Errorf("berkas arsip bukan gzip: %w", err)
	}
	defer gz.Close()

	ringkasan := sha256.New()
	dec := json.NewDecoder(gz)
	si := -1
	var sebelum []byte
	var idTerakhir int64
	for {
		var b Baris
		if err := dec.Decode(&b); err == io.EOF {
			break
		} else if err != nil {
			return h, fmt.Errorf("baris arsip ke-%d tidak terbaca: %w", h.JumlahBaris+1, err)
		}
		row, err := b.KeAudit()
		if err != nil {
			return h, err
		}
		id := int64(row.ID)
		if h.JumlahBaris > 0 && id <= idTerakhir {
			return h, fmt.Errorf("baris arsip tidak urut: id %d sesudah %d", id, idTerakhir)
		}
		h.JumlahBaris++
		ringkasan.Write(row.Hash)

		// Masuk ke segmen berikutnya bila id melewati akhir segmen sekarang.
		if si < 0 || id > segmen[si].IDAkhir {
			if si >= 0 && !bytes.Equal(sebelum, segmen[si].HashKeluar) {
				catat(segmen[si].IDAkhir, domain.MasalahArsipTidakCocok, "hash keluar segmen tidak cocok")
			}
			si++
			if si >= len(segmen) || id < segmen[si].IDAwal || id > segmen[si].IDAkhir {
				return h, fmt.Errorf("baris %d tidak termasuk segmen mana pun di manifest", id)
			}
			sebelum = segmen[si].HashMasuk
		}
		if !bytes.Equal(row.HashSebelumnya, sebelum) {
			catat(id, domain.MasalahRantaiPutus, "hash_sebelumnya tidak menyambung")
		}
		if !bytes.Equal(domain.HitungHash(row.HashSebelumnya, row), row.Hash) {
			catat(id, domain.MasalahIsiBerubah, "isi baris tidak cocok dengan hash yang tersimpan")
		}
		sebelum, idTerakhir = row.Hash, id
	}
	if si >= 0 && !bytes.Equal(sebelum, segmen[si].HashKeluar) {
		catat(segmen[si].IDAkhir, domain.MasalahArsipTidakCocok, "hash keluar segmen tidak cocok")
	}
	if si != len(segmen)-1 {
		catat(0, domain.MasalahArsipTidakCocok, fmt.Sprintf("manifest mencatat %d segmen, berkas memuat %d", len(segmen), si+1))
	}
	if h.JumlahBaris != m.JumlahBaris {
		catat(0, domain.MasalahArsipTidakCocok, fmt.Sprintf("manifest mencatat %d baris, berkas memuat %d", m.JumlahBaris, h.JumlahBaris))
	}
	if hex.EncodeToString(ringkasan.Sum(nil)) != m.Ringkasan {
		catat(0, domain.MasalahArsipTidakCocok, "ringkasan hash berkas tidak sama dengan manifest")
	}
	return h, nil
}

func segmenManifest(in []Segmen) ([]domain.SegmenArsip, error) {
	out := make([]domain.SegmenArsip, len(in))
	for i, s := range in {
		awal, err1 := strconv.ParseInt(s.IDAwal, 10, 64)
		akhir, err2 := strconv.ParseInt(s.IDAkhir, 10, 64)
		keluar, err3 := hex.DecodeString(s.HashKeluar)
		if err := errors.Join(err1, err2, err3); err != nil {
			return nil, fmt.Errorf("segmen manifest ke-%d tidak valid: %w", i+1, err)
		}
		out[i] = domain.SegmenArsip{IDAwal: awal, IDAkhir: akhir, JumlahBaris: s.JumlahBaris, HashKeluar: keluar}
		if s.HashMasuk != "" {
			if out[i].HashMasuk, err1 = hex.DecodeString(s.HashMasuk); err1 != nil {
				return nil, fmt.Errorf("segmen manifest ke-%d: hash_masuk tidak valid", i+1)
			}
		}
		if i > 0 && out[i].IDAwal <= out[i-1].IDAkhir {
			return nil, fmt.Errorf("segmen manifest tidak urut di segmen ke-%d", i+1)
		}
	}
	return out, nil
}
