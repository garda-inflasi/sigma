package domain

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rantai membuat n baris berhash yang tersambung, seperti hasil pemicu basis data.
func rantai(n int, awal []byte, idMulai int64) []AuditLog {
	rows := make([]AuditLog, n)
	sebelum := awal
	for i := range rows {
		row := AuditLog{
			ID: idx.ID(idMulai + int64(i)), Waktu: time.Date(2026, 11, 1, 0, 0, i, 1000, time.UTC),
			Aksi: "UJI", Entitas: "uji", EntitasID: "1", Hasil: HasilBerhasil,
			Metadata: new(`{"i": 1}`), HashSebelumnya: sebelum,
		}
		row.Hash = HitungHash(sebelum, row)
		rows[i], sebelum = row, row.Hash
	}
	return rows
}

func periksaSemua(t *testing.T, v *Verifikator, rows []AuditLog) {
	t.Helper()
	for _, row := range rows {
		require.NoError(t, v.Periksa(row))
	}
}

func TestHitungHash_DeterministikDanPekaIsi(t *testing.T) {
	row := AuditLog{ID: 7, Waktu: time.Date(2026, 1, 2, 3, 4, 5, 6000, time.UTC), Aksi: "A", Entitas: "e", EntitasID: "1", Hasil: "BERHASIL"}
	h := HitungHash(nil, row)
	require.Len(t, h, UkuranHash)
	assert.Equal(t, h, HitungHash(nil, row))

	ubah := row
	ubah.Aksi = "B"
	assert.NotEqual(t, h, HitungHash(nil, ubah), "isi berubah => hash berubah")

	// NULL dan teks kosong dibedakan (penanda NULL), begitu pula pergeseran batas bidang.
	kosong := row
	kosong.IP = new("")
	assert.NotEqual(t, h, HitungHash(nil, kosong))
	geser1, geser2 := row, row
	geser1.Aksi, geser1.Entitas = "ab", "c"
	geser2.Aksi, geser2.Entitas = "a", "bc"
	assert.NotEqual(t, HitungHash(nil, geser1), HitungHash(nil, geser2))

	// aktor_id tidak ikut (boleh dikosongkan ON DELETE SET NULL).
	aktor := row
	aktor.AktorID = new("00000000-0000-0000-0000-000000000001")
	assert.Equal(t, h, HitungHash(nil, aktor))

	// Waktu dibaca dalam mikrodetik, tidak bergantung zona.
	zona := row
	zona.Waktu = row.Waktu.In(time.FixedZone("WIB", 7*3600))
	assert.Equal(t, h, HitungHash(nil, zona))

	assert.NotEqual(t, h, HitungHash(make([]byte, UkuranHash), row), "hash sebelumnya ikut")
}

// Vektor tetap mengunci format garda-audit-v1: mengubah HitungHash tanpa mengubah fungsi SQL (atau sebaliknya)
// memutus semua rantai yang ada. Nilainya juga dicocokkan dengan fungsi SQL di tes integrasi.
func TestHitungHash_VektorTetap(t *testing.T) {
	row := AuditLog{
		ID: 1, Waktu: time.Date(2026, 11, 7, 1, 2, 3, 456789000, time.UTC), Kode: new("demo-audit-0001"),
		AktorNama: new("Nama"), AktorEmail: new("a@b.id"), AktorPeran: new("ADMIN_SISTEM"),
		Aksi: "LOGIN", Entitas: "pengguna", EntitasID: "x", Hasil: "BERHASIL", IP: new("10.0.0.1"),
		Metadata: new(`{"a": "é"}`),
	}
	assert.Equal(t, "db4e2baac784965d5035632c1b521cc7b676127b16c66cd71174107bbd282e00", hex.EncodeToString(HitungHash(nil, row)))
}

func TestVerifikator_RantaiUtuh(t *testing.T) {
	rows := rantai(5, nil, 1)
	v := NewVerifikator(nil)
	periksaSemua(t, v, rows)

	assert.Equal(t, rows[4].Hash, v.Selesai())
	assert.EqualValues(t, 5, v.JumlahBaris)
	assert.Zero(t, v.JumlahMasalah)
	assert.EqualValues(t, 1, v.IDAwal)
	assert.EqualValues(t, 5, v.IDAkhir)
}

func TestVerifikator_IsiBerubah(t *testing.T) {
	rows := rantai(5, nil, 1)
	rows[2].Hasil = HasilGagal // diubah sesudah hash dihitung

	v := NewVerifikator(nil)
	periksaSemua(t, v, rows)

	require.EqualValues(t, 1, v.JumlahMasalah, "baris sesudahnya tetap menyambung ke hash tersimpan")
	assert.Equal(t, Masalah{ID: 3, Jenis: MasalahIsiBerubah, Keterangan: v.Masalah[0].Keterangan}, v.Masalah[0])
}

func TestVerifikator_BarisDihapusAtauHashDiganti(t *testing.T) {
	rows := rantai(5, nil, 1)
	dihapus := append(append([]AuditLog{}, rows[:2]...), rows[3:]...)

	v := NewVerifikator(nil)
	periksaSemua(t, v, dihapus)
	require.EqualValues(t, 1, v.JumlahMasalah)
	assert.Equal(t, MasalahRantaiPutus, v.Masalah[0].Jenis)
	assert.EqualValues(t, 4, v.Masalah[0].ID)

	// Penyerang menghitung ulang hash satu baris: isinya cocok, tetapi baris berikutnya tidak lagi menyambung.
	rows = rantai(4, nil, 1)
	rows[1].Aksi = "PALSU"
	rows[1].Hash = HitungHash(rows[1].HashSebelumnya, rows[1])
	v = NewVerifikator(nil)
	periksaSemua(t, v, rows)
	require.EqualValues(t, 1, v.JumlahMasalah)
	assert.Equal(t, MasalahRantaiPutus, v.Masalah[0].Jenis)
	assert.EqualValues(t, 3, v.Masalah[0].ID)
}

func TestVerifikator_GenesisHarusKosong(t *testing.T) {
	rows := rantai(2, make([]byte, UkuranHash), 10)
	v := NewVerifikator(nil)
	periksaSemua(t, v, rows)
	require.EqualValues(t, 1, v.JumlahMasalah, "baris pertama tanpa segmen arsip harus berawal dari genesis")
	assert.Equal(t, MasalahRantaiPutus, v.Masalah[0].Jenis)
}

func TestVerifikator_MenyambungLewatSegmenArsip(t *testing.T) {
	semua := rantai(10, nil, 1)
	// id 1-3 dan 6 diarsipkan (6 menyelang di antara baris hidup, mis. baris bertanggal mundur di batas bulan).
	segmen := []SegmenArsip{
		{IDAwal: 1, IDAkhir: 3, JumlahBaris: 3, HashMasuk: nil, HashKeluar: semua[2].Hash},
		{IDAwal: 6, IDAkhir: 6, JumlahBaris: 1, HashMasuk: semua[4].Hash, HashKeluar: semua[5].Hash},
	}
	hidup := append(append([]AuditLog{}, semua[3:5]...), semua[6:]...)

	v := NewVerifikator(segmen)
	periksaSemua(t, v, hidup)
	assert.Equal(t, semua[9].Hash, v.Selesai())
	assert.Zero(t, v.JumlahMasalah, "%+v", v.Masalah)
	assert.EqualValues(t, 2, v.JumlahSegmen)
	assert.EqualValues(t, 6, v.JumlahBaris)

	// Segmen yang hash masuknya tidak menyambung (catatan arsip dipalsukan).
	segmen[1].HashMasuk = semua[0].Hash
	v = NewVerifikator(segmen)
	periksaSemua(t, v, hidup)
	require.EqualValues(t, 1, v.JumlahMasalah)
	assert.Equal(t, Masalah{ID: 6, Jenis: MasalahArsipTidakCocok, Keterangan: v.Masalah[0].Keterangan}, v.Masalah[0])
}

func TestVerifikator_SegmenTumpangTindihDanUrutan(t *testing.T) {
	semua := rantai(4, nil, 1)
	segmen := []SegmenArsip{{IDAwal: 2, IDAkhir: 5, HashMasuk: semua[0].Hash, HashKeluar: semua[3].Hash}}
	v := NewVerifikator(segmen)
	require.NoError(t, v.Periksa(semua[0]))
	require.NoError(t, v.Periksa(semua[2]))
	v.Selesai()
	assert.Positive(t, v.JumlahMasalah)

	v = NewVerifikator(nil)
	require.NoError(t, v.Periksa(semua[1]))
	assert.Error(t, v.Periksa(semua[0]), "baris harus urut id naik")
}

func TestRentangPartisi(t *testing.T) {
	dari, sampai, ok := RentangPartisi("audit_log_p202612")
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), dari)
	assert.Equal(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), sampai)
	for _, nama := range []string{"audit_log_default", "audit_log_p2026", "audit_log_p202613", "x_p202601"} {
		_, _, ok := RentangPartisi(nama)
		assert.False(t, ok, nama)
	}
	assert.Equal(t, "audit_log_p202612", NamaPartisi(time.Date(2026, 11, 30, 23, 0, 0, 0, time.FixedZone("x", -3600))))
}
