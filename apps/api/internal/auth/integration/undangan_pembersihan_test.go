package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/usecase"
)

// Pembersihan undangan_pengguna oleh job auth_pembersihan: yang hidup tidak disentuh; yang kedaluwarsa, terpakai,
// atau dicabut dihapus sesudah masa simpan (RetensiResetToken, bawaan 7 hari), tepat di batasnya.

func undanganBaris(t *testing.T, db *gorm.DB, nama, userID string, expiresAt time.Time, usedAt, dicabutPada *time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		"INSERT INTO undangan_pengguna (user_id, token_hash, email, expires_at, used_at, dicabut_pada) VALUES (?, ?, ?, ?, ?, ?)",
		userID, nama, userID+"@gardainflasi.id", expiresAt, usedAt, dicabutPada).Error)
}

func TestPembersihan_UndanganPengguna(t *testing.T) {
	db := pbrDB(t)
	require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, status)
		VALUES (garda_uuid_stabil('pengguna', 'pembersihan-user-c'), 'C', 'pembersihan-user-c@gardainflasi.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'MENUNGGU')`).Error)
	const retensi = 7 * 24 * time.Hour
	lama := pbrAcuan.Add(-retensi - time.Hour) // sudah lewat masa simpan
	baru := pbrAcuan.Add(-retensi + time.Hour) // masih dalam masa simpan
	depan := pbrAcuan.Add(48 * time.Hour)

	// Satu undangan hidup per akun (indeks parsial uq_undangan_pengguna_aktif).
	undanganBaris(t, db, "hidup", pbrUserA, depan, nil, nil)
	undanganBaris(t, db, "kedaluwarsa-lama", pbrUserB, lama, nil, nil)
	undanganBaris(t, db, "kedaluwarsa-baru", uid("pembersihan-user-c"), baru, nil, nil)
	for _, k := range []struct {
		nama  string
		used  *time.Time
		cabut *time.Time
	}{
		{"terpakai-lama", pbrWaktu(lama), nil},
		{"terpakai-baru", pbrWaktu(baru), nil},
		{"dicabut-lama", nil, pbrWaktu(lama)},
		{"dicabut-baru", nil, pbrWaktu(baru)},
	} {
		undanganBaris(t, db, k.nama, pbrUserA, depan, k.used, k.cabut)
	}

	hasil, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.EqualValues(t, 3, langkahDihapus(hasil, "undangan_pengguna"))
	assert.ElementsMatch(t,
		[]string{"hidup", "kedaluwarsa-baru", "terpakai-baru", "dicabut-baru"},
		pbrSisa(t, db, "SELECT token_hash FROM undangan_pengguna"))
}

func TestPembersihan_UndanganPenggunaTepatDiBatas(t *testing.T) {
	db := pbrDB(t)
	const retensi = 7 * 24 * time.Hour
	batas := pbrAcuan.Add(-retensi)
	undanganBaris(t, db, "tepat-batas", pbrUserA, batas, nil, nil)
	undanganBaris(t, db, "terpakai-tepat-batas", pbrUserA, pbrAcuan.Add(time.Hour), pbrWaktu(batas), nil)
	undanganBaris(t, db, "satu-detik-lewat", pbrUserB, batas.Add(-time.Second), nil, nil)

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"tepat-batas", "terpakai-tepat-batas"}, pbrSisa(t, db, "SELECT token_hash FROM undangan_pengguna"),
		"perbandingan ketat: yang waktunya tepat di batas dipertahankan")
}

func TestPembersihan_UndanganPenggunaBatchDanRetensiNol(t *testing.T) {
	db := pbrDB(t)
	for i := range 5 {
		undanganBaris(t, db, "terpakai-"+string(rune('a'+i)), pbrUserA, pbrAcuan.Add(time.Hour), pbrWaktu(pbrAcuan.Add(-time.Hour)), nil)
	}
	cfg := usecase.DefaultKonfigPembersihan()
	cfg.RetensiResetToken = 0 // langsung dihapus begitu mati
	cfg.UkuranBatch = 2

	hasil, err := pbrPembersih(db, cfg).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.EqualValues(t, 5, langkahDihapus(hasil, "undangan_pengguna"))
	assert.Empty(t, pbrSisa(t, db, "SELECT token_hash FROM undangan_pengguna"))
}

func langkahDihapus(h usecase.HasilPembersihan, nama string) int64 {
	for _, l := range h.Langkah {
		if l.Nama == nama {
			return l.Dihapus
		}
	}
	return -1
}
