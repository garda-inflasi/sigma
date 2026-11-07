package arsip

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func barisBerantai(ids []int64, awal []byte) []domain.AuditLog {
	rows := make([]domain.AuditLog, len(ids))
	sebelum := awal
	for i, id := range ids {
		row := domain.AuditLog{
			ID: idx.ID(id), Waktu: time.Date(2024, 3, 5, 1, 2, i, 123456000, time.FixedZone("WIB", 7*3600)),
			Aksi: "UJI", Entitas: "uji", EntitasID: "1", Hasil: domain.HasilBerhasil,
			Sebelum: new(`{"x": [1, 2]}`), UserAgent: new("ü-agent"), HashSebelumnya: sebelum,
		}
		row.Hash = domain.HitungHash(sebelum, row)
		rows[i], sebelum = row, row.Hash
	}
	return rows
}

// tulisArsip menulis baris dan manifest dengan satu segmen per rangkaian berurutan yang diberikan.
func tulisArsip(t *testing.T, dir string, rows []domain.AuditLog, segmen []domain.SegmenArsip) string {
	t.Helper()
	p, err := BuatPenulis(filepath.Join(dir, NamaBerkasData("audit_log_p202403")))
	require.NoError(t, err)
	for _, row := range rows {
		require.NoError(t, p.Tulis(row))
	}
	sha, err := p.Tutup()
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, NamaBerkasData("audit_log_p202403")+".tmp"))
	assert.True(t, os.IsNotExist(err), "berkas sementara tidak tersisa")

	pertama, terakhir := int64(rows[0].ID), int64(rows[len(rows)-1].ID)
	m := ManifestDari(domain.Arsip{
		ID: 3, Partisi: "audit_log_p202403", Dari: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		Sampai: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), JumlahBaris: p.JumlahBaris(),
		IDPertama: &pertama, IDTerakhir: &terakhir, Ringkasan: p.Ringkasan(),
		Berkas: NamaBerkasData("audit_log_p202403"), SHA256Berkas: sha, RetensiBulan: 12,
	}, segmen)
	path := filepath.Join(dir, NamaBerkasManifest("audit_log_p202403"))
	require.NoError(t, TulisManifest(path, m))
	return path
}

func TestArsip_TulisDanVerifikasi(t *testing.T) {
	// id 1-3 dan 5-6 di partisi ini; id 4 hidup di partisi lain (segmen terpisah).
	semua := barisBerantai([]int64{1, 2, 3, 4, 5, 6}, nil)
	rows := append(append([]domain.AuditLog{}, semua[:3]...), semua[4:]...)
	segmen := []domain.SegmenArsip{
		{IDAwal: 1, IDAkhir: 3, JumlahBaris: 3, HashKeluar: semua[2].Hash},
		{IDAwal: 5, IDAkhir: 6, JumlahBaris: 2, HashMasuk: semua[3].Hash, HashKeluar: semua[5].Hash},
	}
	path := tulisArsip(t, t.TempDir(), rows, segmen)

	h, err := Verifikasi(path)
	require.NoError(t, err)
	assert.True(t, h.Valid(), "%+v", h.Masalah)
	assert.EqualValues(t, 5, h.JumlahBaris)
	assert.Equal(t, 2, h.JumlahSegmen)
	assert.Equal(t, "audit_log_p202403", h.Partisi)
}

func TestArsip_BarisKembaliUtuh(t *testing.T) {
	row := barisBerantai([]int64{42}, nil)[0]
	row.Kode, row.AktorID = new("demo-audit-0001"), new("00000000-0000-0000-0000-000000000001")
	row.Hash = domain.HitungHash(row.HashSebelumnya, row)
	kembali, err := DariAudit(row).KeAudit()
	require.NoError(t, err)
	assert.Equal(t, row.Waktu.UnixMicro(), kembali.Waktu.UnixMicro())
	assert.Equal(t, row.Hash, domain.HitungHash(kembali.HashSebelumnya, kembali), "hash dihitung ulang dari baris arsip")
	assert.Equal(t, *row.Kode, *kembali.Kode)
}

func TestArsip_PerusakanTerdeteksi(t *testing.T) {
	rows := barisBerantai([]int64{1, 2, 3}, nil)
	segmen := []domain.SegmenArsip{{IDAwal: 1, IDAkhir: 3, JumlahBaris: 3, HashKeluar: rows[2].Hash}}

	t.Run("isi berkas diganti", func(t *testing.T) {
		dir := t.TempDir()
		path := tulisArsip(t, dir, rows, segmen)
		// Tulis ulang berkas data dengan satu baris diubah (gzip baru, jadi SHA-256 juga berubah).
		data := filepath.Join(dir, NamaBerkasData("audit_log_p202403"))
		baru := append([]domain.AuditLog{}, rows...)
		baru[1].Hasil = domain.HasilGagal
		f, err := os.Create(data)
		require.NoError(t, err)
		gz := gzip.NewWriter(f)
		for _, row := range baru {
			b := DariAudit(row)
			raw := `{"id":"` + b.ID + `","waktu_us":` + strconv.FormatInt(b.WaktuUS, 10) + `,"waktu":"` + b.Waktu +
				`","aksi":"UJI","entitas":"uji","entitas_id":"1","hasil":"` + b.Hasil + `","sebelum":"{\"x\": [1, 2]}","user_agent":"ü-agent","hash":"` + b.Hash + `"`
			if b.HashSebelumnya != "" {
				raw += `,"hash_sebelumnya":"` + b.HashSebelumnya + `"`
			}
			_, err := gz.Write([]byte(raw + "}\n"))
			require.NoError(t, err)
		}
		require.NoError(t, gz.Close())
		require.NoError(t, f.Close())

		h, err := Verifikasi(path)
		require.NoError(t, err)
		require.False(t, h.Valid())
		jenis := map[string]bool{}
		for _, m := range h.Masalah {
			jenis[m.Jenis] = true
		}
		assert.True(t, jenis[domain.MasalahIsiBerubah], "%+v", h.Masalah)
	})

	t.Run("manifest menyatakan segmen lain", func(t *testing.T) {
		salah := []domain.SegmenArsip{{IDAwal: 1, IDAkhir: 3, JumlahBaris: 3, HashMasuk: rows[0].Hash, HashKeluar: rows[2].Hash}}
		h, err := Verifikasi(tulisArsip(t, t.TempDir(), rows, salah))
		require.NoError(t, err)
		assert.False(t, h.Valid())
	})

	t.Run("baris di luar segmen", func(t *testing.T) {
		pendek := []domain.SegmenArsip{{IDAwal: 1, IDAkhir: 2, JumlahBaris: 2, HashKeluar: rows[1].Hash}}
		_, err := Verifikasi(tulisArsip(t, t.TempDir(), rows, pendek))
		assert.Error(t, err)
	})
}
