package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Alur persetujuan perubahan master data (04-B): usulan TAMBAH/UBAH/TINJAU diajukan pemegang master.ajukan,
// lalu disetujui atau ditolak pemegang master.setujui yang bukan pengajunya. Satu usulan menunggu per entitas.

// Entitas master yang perubahannya melewati persetujuan.
const (
	EntitasKomoditas = "komoditas"
	EntitasPasar     = "pasar"
)

// Jenis usulan.
const (
	JenisTambah = "TAMBAH"
	JenisUbah   = "UBAH"
	// JenisTinjau: status saja (aksi massal "ajukan perubahan" tanpa isi).
	JenisTinjau = "TINJAU"
)

// Status usulan (enum status_persetujuan).
const (
	UsulanMenunggu  = "MENUNGGU"
	UsulanDisetujui = "DISETUJUI"
	UsulanDitolak   = "DITOLAK"
)

// Keputusan atas usulan.
const (
	KeputusanSetujui  = "SETUJUI"
	KeputusanTolak    = "TOLAK"
	KeputusanBatalkan = "BATALKAN"
)

// Batas panjang alasan.
const (
	MinAlasan = 3
	MaxAlasan = 1000
)

// Galat domain alur perubahan; delivery memetakannya ke kode HTTP dan kode amplop.
var (
	ErrValidasi          = errors.New("usulan tidak valid")
	ErrTanpaPerubahan    = errors.New("usulan tidak mengubah apa pun")
	ErrEntitasTidakAda   = errors.New("data master tidak ditemukan")
	ErrUsulanTidakAda    = errors.New("usulan tidak ditemukan")
	ErrUsulanMenunggu    = errors.New("data ini sudah punya usulan yang menunggu persetujuan")
	ErrKodeSudahAda      = errors.New("kode sudah dipakai")
	ErrStatusTidakSesuai = errors.New("status data tidak mengizinkan perubahan")
	ErrBukanMenunggu     = errors.New("usulan sudah diputuskan")
	ErrPemisahanTugas    = errors.New("pengaju tidak boleh memutuskan usulannya sendiri")
	ErrBukanPengaju      = errors.New("hanya pengaju yang boleh membatalkan usulan")
	ErrKedaluwarsa       = errors.New("data sudah berubah sejak usulan diajukan")
)

// PerubahanMaster memetakan tabel perubahan_master.
type PerubahanMaster struct {
	ID        idx.ID  `gorm:"column:id;primaryKey;autoIncrement"`
	Kode      *string `gorm:"column:kode"`
	Entitas   string  `gorm:"column:entitas"`
	EntitasID string  `gorm:"column:entitas_id"`
	Jenis     string  `gorm:"column:jenis"`
	// Versi adalah versi entitas yang berlaku bila usulan disetujui.
	Versi int `gorm:"column:versi"`
	// Perubahan dan Sebelum adalah objek JSON bidang → nilai.
	Perubahan       string     `gorm:"column:perubahan;type:jsonb"`
	Sebelum         string     `gorm:"column:sebelum;type:jsonb"`
	Status          string     `gorm:"column:status;default:MENUNGGU"`
	Alasan          *string    `gorm:"column:alasan"`
	AlasanPenolakan *string    `gorm:"column:alasan_penolakan"`
	DiajukanOlehID  string     `gorm:"column:diajukan_oleh_id;type:uuid"`
	DisetujuiOlehID *string    `gorm:"column:disetujui_oleh_id;type:uuid"`
	DisetujuiPada   *time.Time `gorm:"column:disetujui_pada"`
	DitolakOlehID   *string    `gorm:"column:ditolak_oleh_id;type:uuid"`
	DitolakPada     *time.Time `gorm:"column:ditolak_pada"`
	DibuatPada      time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (PerubahanMaster) TableName() string { return "perubahan_master" }

// PerubahanView adalah usulan beserta nama entitas dan para pelakunya.
type PerubahanView struct {
	PerubahanMaster
	EntitasNama   *string `gorm:"column:entitas_nama"`
	DiajukanNama  *string `gorm:"column:diajukan_nama"`
	DisetujuiNama *string `gorm:"column:disetujui_nama"`
	DitolakNama   *string `gorm:"column:ditolak_nama"`
}

// Diff adalah satu bidang yang diusulkan berubah.
type Diff struct {
	Bidang  string
	Sebelum json.RawMessage // nil bila bidang belum ada (TAMBAH)
	Sesudah json.RawMessage
}

// DiffUsulan menyusun diff dari kolom perubahan dan sebelum, urut menurut urutan bidang entitas.
func DiffUsulan(p PerubahanMaster) ([]Diff, error) {
	var sesudah, sebelum map[string]json.RawMessage
	if err := json.Unmarshal([]byte(orObjek(p.Perubahan)), &sesudah); err != nil {
		return nil, fmt.Errorf("baca perubahan usulan %s: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(orObjek(p.Sebelum)), &sebelum); err != nil {
		return nil, fmt.Errorf("baca keadaan sebelum usulan %s: %w", p.ID, err)
	}
	keys := make([]string, 0, len(sesudah))
	for k := range sesudah {
		keys = append(keys, k)
	}
	urutkanBidang(p.Entitas, keys)
	out := make([]Diff, len(keys))
	for i, k := range keys {
		out[i] = Diff{Bidang: k, Sebelum: sebelum[k], Sesudah: sesudah[k]}
	}
	return out, nil
}

func orObjek(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

// UsulanBaru adalah permintaan pengajuan usulan.
type UsulanBaru struct {
	Entitas   string
	Jenis     string
	EntitasID string
	// Perubahan sudah divalidasi dan dinormalkan (lihat ValidasiUsulan).
	Perubahan map[string]any
	Alasan    string
	AktorID   string
}

// Keputusan adalah tindakan atas satu usulan menunggu.
type Keputusan struct {
	Aksi    string // KeputusanSetujui, KeputusanTolak, KeputusanBatalkan
	AktorID string
	Alasan  string
}

// Putusan adalah hasil keputusan: usulan setelah diputuskan dan keadaan entitas sebelum/sesudah (untuk audit).
type Putusan struct {
	Usulan           PerubahanMaster
	Sebelum, Sesudah map[string]any
}

// FilterPerubahan menyaring daftar usulan.
type FilterPerubahan struct {
	Status    string
	Entitas   string
	EntitasID string
	Page      int
	Limit     int
}

// RepositoryPerubahan menjalankan alur usulan di dalam transaksi berkunci baris.
type RepositoryPerubahan interface {
	// Ajukan menyimpan usulan MENUNGGU: memeriksa keberadaan entitas dan rujukannya, membuang bidang yang tidak
	// berubah (ErrTanpaPerubahan bila tidak tersisa), mencatat keadaan sebelum, dan menandai komoditas menunggu.
	Ajukan(ctx context.Context, u UsulanBaru) (PerubahanMaster, error)
	// Putuskan menyetujui, menolak, atau membatalkan usulan menunggu.
	Putuskan(ctx context.Context, id idx.ID, k Keputusan) (Putusan, error)
	// PutuskanMenunggu memutuskan usulan menunggu milik satu entitas; ErrUsulanTidakAda bila tidak ada.
	PutuskanMenunggu(ctx context.Context, entitas, entitasID string, k Keputusan) (Putusan, error)
	// ArsipkanKomoditas mengarsipkan komoditas Aktif/menunggu dan menolak usulan menunggunya; false bila status
	// sudah berubah (balapan).
	ArsipkanKomoditas(ctx context.Context, kode, aktorID string) (bool, error)
	GetPerubahan(ctx context.Context, id idx.ID) (PerubahanView, error)
	ListPerubahan(ctx context.Context, f FilterPerubahan) ([]PerubahanView, int64, error)
}

// ── Validasi isi usulan ───────────────────────────────────────────────

type jenisNilai int

const (
	nilaiTeks jenisNilai = iota
	nilaiBulat
	nilaiDesimal
	nilaiBool
	nilaiKodeOpsional // teks berpola, boleh null
)

type aturanBidang struct {
	nama     string
	jenis    jenisNilai
	min, max float64 // panjang (teks) atau rentang (angka)
	pola     *regexp.Regexp
	// wajibTambah: harus ada saat TAMBAH.
	wajibTambah bool
	// hanyaTambah: hanya boleh diisi saat TAMBAH (mis. kabupaten pasar).
	hanyaTambah bool
}

var (
	polaKecamatan     = regexp.MustCompile(`^[0-9]{7}$`)
	polaKabupaten     = regexp.MustCompile(`^[0-9]{4}$`)
	polaKodeKomoditas = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,29}$`)
	polaKodePasar     = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,49}$`)
	aturanPerEntitas  = map[string][]aturanBidang{
		EntitasKomoditas: {
			{nama: "nama", jenis: nilaiTeks, min: 1, max: 255, wajibTambah: true},
			{nama: "kategori", jenis: nilaiTeks, min: 1, max: 100, wajibTambah: true},
			{nama: "satuan", jenis: nilaiTeks, min: 1, max: 50, wajibTambah: true},
			{nama: "harga_acuan", jenis: nilaiBulat, min: 1, max: math.MaxInt32, wajibTambah: true},
			{nama: "shelf_life_hari", jenis: nilaiBulat, min: 1, max: 3650, wajibTambah: true},
			{nama: "pilot", jenis: nilaiBool, wajibTambah: true},
		},
		EntitasPasar: {
			{nama: "nama", jenis: nilaiTeks, min: 1, max: 255, wajibTambah: true},
			{nama: "kabupaten_kode", jenis: nilaiTeks, min: 4, max: 4, pola: polaKabupaten, wajibTambah: true, hanyaTambah: true},
			{nama: "lat", jenis: nilaiDesimal, min: -90, max: 90, wajibTambah: true},
			{nama: "lng", jenis: nilaiDesimal, min: -180, max: 180, wajibTambah: true},
			{nama: "radius_meter", jenis: nilaiBulat, min: 1, max: 10000},
			{nama: "aktif", jenis: nilaiBool},
			{nama: "kecamatan_kode", jenis: nilaiKodeOpsional, pola: polaKecamatan},
		},
	}
)

// BidangEntitas mengembalikan nama bidang yang bisa diusulkan untuk entitas, menurut urutan tampil.
func BidangEntitas(entitas string) []string {
	aturan := aturanPerEntitas[entitas]
	out := make([]string, len(aturan))
	for i, a := range aturan {
		out[i] = a.nama
	}
	return out
}

func urutkanBidang(entitas string, keys []string) {
	urut := map[string]int{}
	for i, b := range BidangEntitas(entitas) {
		urut[b] = i
	}
	sort.SliceStable(keys, func(i, j int) bool {
		ui, oki := urut[keys[i]]
		uj, okj := urut[keys[j]]
		switch {
		case oki && okj:
			return ui < uj
		case oki != okj:
			return oki
		}
		return keys[i] < keys[j]
	})
}

// ValidasiUsulan memeriksa entitas, jenis, kode, dan isi usulan, lalu mengembalikan isi yang dinormalkan
// (teks dirapikan, bilangan bulat sebagai int64, desimal sebagai float64, null hanya untuk bidang opsional).
// Rujukan ke data lain (satuan, kabupaten, kecamatan) diperiksa repository di dalam transaksi.
func ValidasiUsulan(entitas, jenis, entitasID string, isi map[string]json.RawMessage) (map[string]any, error) {
	aturan, ok := aturanPerEntitas[entitas]
	if !ok {
		return nil, fmt.Errorf("%w: entitas %q tidak didukung (komoditas atau pasar)", ErrValidasi, entitas)
	}
	switch jenis {
	case JenisUbah:
		if strings.TrimSpace(entitasID) == "" {
			return nil, fmt.Errorf("%w: kode data wajib diisi", ErrValidasi)
		}
	case JenisTambah:
		pola := polaKodeKomoditas
		if entitas == EntitasPasar {
			pola = polaKodePasar
		}
		if !pola.MatchString(entitasID) {
			maks := 30
			if entitas == EntitasPasar {
				maks = 50
			}
			return nil, fmt.Errorf("%w: kode baru %q harus huruf kapital, angka, atau tanda hubung (2–%d karakter)",
				ErrValidasi, entitasID, maks)
		}
	default:
		return nil, fmt.Errorf("%w: jenis %q tidak didukung (TAMBAH atau UBAH)", ErrValidasi, jenis)
	}

	perAturan := make(map[string]aturanBidang, len(aturan))
	for _, a := range aturan {
		perAturan[a.nama] = a
	}
	out := make(map[string]any, len(isi))
	for bidang, raw := range isi {
		a, ada := perAturan[bidang]
		if !ada {
			return nil, fmt.Errorf("%w: bidang %q tidak dapat diubah", ErrValidasi, bidang)
		}
		if a.hanyaTambah && jenis != JenisTambah {
			return nil, fmt.Errorf("%w: bidang %q hanya diisi saat menambah data baru", ErrValidasi, bidang)
		}
		nilai, err := bacaNilai(a, raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %s", ErrValidasi, bidang, err.Error())
		}
		out[bidang] = nilai
	}
	if jenis == JenisTambah {
		for _, a := range aturan {
			if _, ada := out[a.nama]; a.wajibTambah && !ada {
				return nil, fmt.Errorf("%w: bidang %q wajib diisi untuk data baru", ErrValidasi, a.nama)
			}
		}
		if entitas == EntitasPasar {
			if _, ada := out["radius_meter"]; !ada {
				out["radius_meter"] = int64(400)
			}
			if _, ada := out["aktif"]; !ada {
				out["aktif"] = true
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrTanpaPerubahan
	}
	return out, nil
}

func bacaNilai(a aturanBidang, raw json.RawMessage) (any, error) {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		if a.jenis == nilaiKodeOpsional {
			return nil, nil
		}
		return nil, errors.New("tidak boleh kosong")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, errors.New("bukan nilai JSON yang sah")
	}
	switch a.jenis {
	case nilaiTeks, nilaiKodeOpsional:
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("harus berupa teks")
		}
		s = strings.TrimSpace(s)
		if n := float64(len([]rune(s))); n < math.Max(a.min, 1) || (a.max > 0 && n > a.max) {
			return nil, fmt.Errorf("harus %d–%d karakter", int(math.Max(a.min, 1)), int(a.max))
		}
		if a.pola != nil && !a.pola.MatchString(s) {
			return nil, errors.New("formatnya tidak sesuai")
		}
		return s, nil
	case nilaiBulat:
		num, ok := v.(json.Number)
		if !ok {
			return nil, errors.New("harus berupa bilangan bulat")
		}
		n, err := num.Int64()
		if err != nil {
			return nil, errors.New("harus berupa bilangan bulat")
		}
		if float64(n) < a.min || float64(n) > a.max {
			return nil, fmt.Errorf("harus %s–%s", formatBatas(a.min), formatBatas(a.max))
		}
		return n, nil
	case nilaiDesimal:
		num, ok := v.(json.Number)
		if !ok {
			return nil, errors.New("harus berupa angka")
		}
		f, err := num.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, errors.New("harus berupa angka")
		}
		if f < a.min || f > a.max {
			return nil, fmt.Errorf("harus %s–%s", formatBatas(a.min), formatBatas(a.max))
		}
		return f, nil
	case nilaiBool:
		b, ok := v.(bool)
		if !ok {
			return nil, errors.New("harus true atau false")
		}
		return b, nil
	}
	return nil, errors.New("jenis bidang tidak dikenal")
}

func formatBatas(f float64) string {
	if f == math.MaxInt32 {
		return "2147483647"
	}
	return fmt.Sprintf("%g", f)
}

// KonversiTersimpan mengubah isi usulan yang dibaca ulang dari JSON (bilangan sebagai json.Number) ke tipe Go
// sesuai jenis bidangnya, sehingga dikirim ke basis data sebagai angka, bukan teks.
func KonversiTersimpan(entitas string, isi map[string]any) map[string]any {
	jenis := map[string]jenisNilai{}
	for _, a := range aturanPerEntitas[entitas] {
		jenis[a.nama] = a.jenis
	}
	out := make(map[string]any, len(isi))
	for b, v := range isi {
		if num, ok := v.(json.Number); ok {
			if jenis[b] == nilaiDesimal {
				f, _ := num.Float64()
				v = f
			} else if n, err := num.Int64(); err == nil {
				v = n
			} else {
				f, _ := num.Float64()
				v = f
			}
		}
		out[b] = v
	}
	return out
}

// SamaNilai membandingkan nilai usulan dengan nilai tersimpan menurut bentuk JSON-nya.
func SamaNilai(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}

// ValidasiAlasan merapikan alasan dan memeriksa panjangnya; wajib false mengizinkan kosong.
func ValidasiAlasan(alasan string, wajib bool) (string, error) {
	alasan = strings.TrimSpace(alasan)
	n := len([]rune(alasan))
	if n == 0 && !wajib {
		return "", nil
	}
	if n < MinAlasan || n > MaxAlasan {
		return "", fmt.Errorf("%w: alasan harus %d–%d karakter", ErrValidasi, MinAlasan, MaxAlasan)
	}
	return alasan, nil
}
