// Package idx menyeragamkan kunci primer (PK) di seluruh modul. Ketentuannya ada di docs/SKEMA-DATABASE.md.
//
//   - ID untuk tabel transaksional, time-series, dan log: BIGINT GENERATED ALWAYS AS IDENTITY. Di Go bertipe
//     ID (int64). Di JSON ditulis sebagai string desimal ("42"), sama dengan pemetaan JSON proto3 untuk int64.
//     Alasannya: aman untuk Number JavaScript, dan klien memperlakukan semua ID sebagai string tak bermakna.
//   - UUID untuk entitas bisnis dan identitas yang terlihat klien (pengguna, intervensi, policy brief, sumber
//     data, fasilitas cold storage): kolom `uuid DEFAULT gen_random_uuid()`, di Go bertipe string kanonik
//     huruf kecil.
//   - UUIDStabil memberi UUID deterministik (versi 3, RFC 9562) untuk baris seed dan migrasi data dari ID lama,
//     sehingga basis data yang dimigrasi dan basis data baru yang di-seed memakai UUID yang sama. Fungsi SQL
//     garda_uuid_stabil (migrasi 000109) menghitung nilai yang identik.
package idx

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ID adalah kunci surrogate BIGINT (GENERATED ALWAYS AS IDENTITY). Nilai nol berarti "belum ada": GORM tidak
// mengirim kolom identity bernilai nol saat INSERT dan mengisinya dari RETURNING.
type ID int64

// ErrIDTidakSah: teks bukan bilangan bulat positif dalam rentang BIGINT.
var ErrIDTidakSah = errors.New("id harus bilangan bulat positif")

// Parse membaca ID dari teks (parameter rute, kueri). Hanya angka desimal tanpa tanda, tanpa spasi, tanpa nol
// di depan, dan bernilai 1..2^63-1; selain itu ErrIDTidakSah. Bentuk ketat ini membuat satu baris hanya punya
// satu ejaan URL.
func Parse(s string) (ID, error) {
	if s == "" || len(s) > 19 || s[0] == '0' {
		return 0, ErrIDTidakSah
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, ErrIDTidakSah
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, ErrIDTidakSah
	}
	return ID(n), nil
}

// ParseDaftar membaca beberapa ID sekaligus (mis. ids=1,2,3). Entri kosong dibuang dan ID ganda dirapatkan,
// dengan urutan pertama dipertahankan. Satu entri yang salah membuat seluruh daftar ditolak.
func ParseDaftar(teks []string) ([]ID, error) {
	out := make([]ID, 0, len(teks))
	terlihat := make(map[ID]struct{}, len(teks))
	for _, t := range teks {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		id, err := Parse(t)
		if err != nil {
			return nil, fmt.Errorf("%w: %q", err, t)
		}
		if _, ada := terlihat[id]; ada {
			continue
		}
		terlihat[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// String menulis ID sebagai angka desimal.
func (id ID) String() string { return strconv.FormatInt(int64(id), 10) }

// Int64 mengembalikan nilai mentah (untuk kueri SQL mentah).
func (id ID) Int64() int64 { return int64(id) }

// MarshalJSON menulis ID sebagai string JSON ("42").
func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON menerima string desimal ("42") maupun angka JSON (42). Nilai di luar bilangan bulat positif
// ditolak supaya ID salah ketik tidak diam-diam menjadi nol.
func (id *ID) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*id = 0
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var teks string
		if err := json.Unmarshal(b, &teks); err != nil {
			return err
		}
		s = teks
	}
	v, err := Parse(s)
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(b))
	}
	*id = v
	return nil
}

// Value membuat ID dapat dipakai sebagai argumen kueri.
func (id ID) Value() (driver.Value, error) { return int64(id), nil }

// Scan membaca ID dari kolom BIGINT.
func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*id = ID(v)
	case int32:
		*id = ID(v)
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return fmt.Errorf("idx.ID: %w", err)
		}
		*id = ID(n)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("idx.ID: %w", err)
		}
		*id = ID(n)
	case nil:
		*id = 0
	default:
		return fmt.Errorf("idx.ID: tipe sumber %T tidak didukung", src)
	}
	return nil
}

// ── UUID ──────────────────────────────────────────────────────────────

// ValidUUID melaporkan apakah s adalah UUID dalam bentuk kanonik 36 karakter (8-4-4-4-12, heksadesimal).
// Bentuk lain yang diterima uuid.Parse (urn:uuid:, kurung kurawal, tanpa tanda hubung) ditolak supaya satu baris
// hanya punya satu ejaan URL dan teks bebas tidak pernah sampai ke kolom uuid (galat 22P02 di PostgreSQL).
func ValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	_, err := uuid.Parse(s)
	return err == nil
}

// NormalUUID mengembalikan bentuk kanonik huruf kecil bila s UUID yang sah.
func NormalUUID(s string) (string, bool) {
	if !ValidUUID(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

// NamespaceGarda adalah namespace UUID versi 3 milik Garda Inflasi. Tidak boleh diubah: seed dan migrasi data
// bergantung pada nilai ini, dan fungsi SQL garda_uuid_stabil menyalinnya.
var NamespaceGarda = uuid.MustParse("5b2f8f3e-6a1d-4c7e-9a52-3f0d1c8e7b14")

// UUIDStabil adalah UUID versi 3 dari "<ruang>:<kunci>" di NamespaceGarda, mis.
// UUIDStabil("pengguna", "superadmin"). Hanya untuk baris seed dan migrasi data ID lama; baris yang dibuat
// aplikasi memakai gen_random_uuid().
func UUIDStabil(ruang, kunci string) string {
	return uuid.NewMD5(NamespaceGarda, []byte(ruang+":"+kunci)).String()
}
