package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pengguna.id adalah UUID (migrasi 000110): perilaku API terhadap bentuk ID, pembuatan, dan penghapusan.

func TestPenggunaUUID_RuteIDBukanUUIDDijawab404DanHurufBesarDiterima(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	for _, id := range []string{"bukan-uuid", "test-user-001", "USR-2024-0001", "123"} {
		for _, rute := range []struct{ metode, sufiks string }{
			{http.MethodGet, ""}, {http.MethodGet, "/riwayat-peran"}, {http.MethodGet, "/permissions"},
			{http.MethodPost, "/reset-mfa"}, {http.MethodPost, "/buka-kunci"}, {http.MethodDelete, ""},
		} {
			w := s.panggil(t, rute.metode, "/v1/users/"+id+rute.sufiks, nil, admin.access)
			assert.Equal(t, http.StatusNotFound, w.Code, "%s /v1/users/%s%s: %s", rute.metode, id, rute.sufiks, w.Body.String())
		}
	}

	w := s.panggil(t, http.MethodGet, "/v1/users/"+strings.ToUpper(userBiasa), nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var data struct {
		ID string `json:"id"`
	}
	s.decodeData(t, w.Body.Bytes(), &data)
	assert.Equal(t, userBiasa, data.ID, "id dikembalikan dalam bentuk kanonik huruf kecil")
}

func TestPenggunaUUID_BuatPenggunaIDOpsionalDanHarusUUID(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	badan := func(email string, id any) map[string]any {
		m := map[string]any{
			"nama": "Baru", "email": email, "password": "Kuat#12345",
			"peran": "OPS_MAHASISWA", "lapis": "OPERASIONAL", "lembaga": "BPPSDMP",
		}
		if id != nil {
			m["id"] = id
		}
		return m
	}

	t.Run("tanpa id: PostgreSQL atau server membuat UUID acak", func(t *testing.T) {
		require.Equal(t, http.StatusCreated, s.panggil(t, http.MethodPost, "/v1/users", badan("tanpa.id@gardainflasi.id", nil), admin.access).Code)
		var id string
		require.NoError(t, s.db.Raw("SELECT id::text FROM pengguna WHERE email = 'tanpa.id@gardainflasi.id'").Scan(&id).Error)
		assert.True(t, idx.ValidUUID(id), id)
	})
	t.Run("id UUID huruf besar disimpan huruf kecil", func(t *testing.T) {
		id := uid("buat-besar")
		require.Equal(t, http.StatusCreated, s.panggil(t, http.MethodPost, "/v1/users", badan("besar@gardainflasi.id", strings.ToUpper(id)), admin.access).Code)
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ?::uuid AND email = 'besar@gardainflasi.id'", id))
	})
	t.Run("id bukan UUID: 400 dan tidak ada yang tersimpan", func(t *testing.T) {
		for i, id := range []string{"u-baru", "USR-2024-0001", "superadmin", "123", " "} {
			w := s.panggil(t, http.MethodPost, "/v1/users", badan("salah"+string(rune('a'+i))+"@gardainflasi.id", id), admin.access)
			assert.Equal(t, http.StatusBadRequest, w.Code, "%q: %s", id, w.Body.String())
		}
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE email LIKE 'salah%'"))
	})
	t.Run("id yang sudah dipakai: 409, bukan 500", func(t *testing.T) {
		w := s.panggil(t, http.MethodPost, "/v1/users", badan("kembar.id@gardainflasi.id", userBiasa), admin.access)
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "ID pengguna")
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE email = 'kembar.id@gardainflasi.id'"))
	})
}

func TestPenggunaUUID_HapusAkunYangMasihBuktiAsalDataDijawab409LaluNonaktifkan(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.NoError(t, s.db.Exec(`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('CBM-KRT', 'Cabai', 'kg', 'cabai', 50000, 7)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES ('PSR-1', 'Pasar', '1171', 5.5, 95.3)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada, petugas_id)
		VALUES ('EH-HAPUS', 'CBM-KRT', 'PSR-1', 'KONSUMEN', 52000, 'kg', now(), ?::uuid)`, userBiasa).Error)

	w := s.panggil(t, http.MethodDelete, "/v1/users/"+userBiasa, nil, admin.access)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Nonaktifkan akun")
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ?::uuid", userBiasa), "akun tetap ada")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'DELETE_USER'"), "penghapusan yang ditolak tidak diaudit sebagai berhasil")

	// Jalan keluar yang disarankan: menonaktifkan akun lewat PATCH /v1/users/:id.
	p := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, map[string]any{"status": "NONAKTIF"}, admin.access)
	assert.Equal(t, http.StatusOK, p.Code, p.Body.String())
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ?::uuid AND status = 'NONAKTIF'", userBiasa))
}

func TestPenggunaUUID_HapusAkunHanyaDenganJejakDanMilikAkunBerhasil(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	s.mulaiSesi(t, emailBiasa, uaMac) // refresh token (CASCADE), percobaan masuk (SET NULL), audit LOGIN (SET NULL)
	require.NoError(t, s.db.Exec("INSERT INTO mfa_kode_cadangan (user_id, kode_hash) VALUES (?::uuid, 'h-hapus')", userBiasa).Error)
	require.NoError(t, s.db.Exec("INSERT INTO preferensi_notifikasi (pengguna_id, kanal) VALUES (?::uuid, 'EMAIL')", userBiasa).Error)

	w := s.panggil(t, http.MethodDelete, "/v1/users/"+userBiasa, nil, admin.access)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ?::uuid", userBiasa))
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM refresh_tokens WHERE user_id = ?::uuid", userBiasa), "milik akun ikut terhapus (CASCADE)")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = ?::uuid", userBiasa))
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM preferensi_notifikasi WHERE pengguna_id = ?::uuid", userBiasa))
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM login_attempts WHERE user_id = ?::uuid", userBiasa), "percobaan masuk dilepas dari akun (SET NULL)")
	assert.Positive(t, s.hitung(t, "SELECT count(*) FROM login_attempts WHERE user_id IS NULL AND success"), "barisnya tetap ada")
	assert.Positive(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND aktor_id IS NULL AND aktor_email = ?", emailBiasa), "audit tetap ada, aktornya NULL")
}
