package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	intervensiRepo "github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OPS_KOORDINATOR kabupaten 1171 (Banda Aceh): peran bercakupan wilayah, tanpa izin intervensi bawaan selain lihat.
const emailKoordinator = "koordinator.lapangan@gardainflasi.id"

// bekalIzinLangsung memberi pengguna izin langsung (02-C) supaya peran bercakupan wilayah bisa mencoba aksi
// yang hanya dipegang peran tingkat provinsi.
func (s *suite) bekalIzinLangsung(t *testing.T, email string, izin ...string) {
	t.Helper()
	for _, nama := range izin {
		require.NoError(t, s.db.Exec(`INSERT INTO user_permissions (pengguna_id, permission_nama, alasan)
			SELECT id, ?, 'uji cakupan wilayah' FROM pengguna WHERE email = ? ON CONFLICT DO NOTHING`, nama, email).Error)
	}
}

// Kartu seed: INT-301 di kabupaten 1171 (kabupaten tugas koordinator), INT-302 di 1174.
func TestCakupan_KoordinatorHanyaMenjangkauKartuKabupatennya(t *testing.T) {
	s := newSuite(t, "intervensi_cakupan")
	s.bekalIzinLangsung(t, emailKoordinator, "intervensi.kelola", "intervensi.setujui")
	require.Equal(t, "1171", card(t, s, "INT-301").KabupatenKode)
	require.Equal(t, "1174", card(t, s, "INT-302").KabupatenKode)

	t.Run("ajukan persetujuan: kabupaten lain 404, kabupaten sendiri 201", func(t *testing.T) {
		code, _, _ := s.ajukan(t, emailKoordinator, "INT-302", "t-petakan")
		assert.Equal(t, http.StatusNotFound, code)
		assert.Zero(t, s.hitung(t, `SELECT count(*) FROM intervensi_persetujuan WHERE intervensi_id = ?::uuid`, kid("INT-302")))

		code, _, body := s.ajukan(t, emailKoordinator, "INT-301", "t-petakan")
		assert.Equal(t, http.StatusCreated, code, body)
	})

	t.Run("pindah tahap: kabupaten lain 404 dan kartu tidak berubah", func(t *testing.T) {
		kartu := card(t, s, "INT-302")

		got := s.pindah(t, emailKoordinator, "INT-302", "t-verifikasi", 0, kartu.Versi)

		assert.Equal(t, http.StatusNotFound, got.Code, got.Body)
		assert.Equal(t, kartu.Tahap, card(t, s, "INT-302").Tahap)
	})

	t.Run("aksi massal: kartu luar cakupan dilaporkan tidak ditemukan dan tidak berubah", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailKoordinator,
			map[string]any{"action": "tugaskan", "ids": []string{kid("INT-302"), kid("INT-303")}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		got := data[struct {
			Berhasil int `json:"berhasil"`
			Gagal    int `json:"gagal"`
			Hasil    []struct {
				ID       string `json:"id"`
				Berhasil bool   `json:"berhasil"`
				Alasan   string `json:"alasan"`
			} `json:"hasil"`
		}](t, w)
		require.Len(t, got.Hasil, 2)
		assert.False(t, got.Hasil[0].Berhasil, "INT-302 (1174) di luar cakupan")
		assert.Equal(t, "Kartu tidak ditemukan", got.Hasil[0].Alasan)
		assert.Zero(t, s.hitung(t, `SELECT count(*) FROM intervensi_pj WHERE intervensi_id = ?::uuid AND pengguna_id = garda_uuid_stabil('pengguna', 'koordinator_lapangan')`, kid("INT-302")))
	})

	t.Run("memutus persetujuan kartu kabupaten lain dijawab 404", func(t *testing.T) {
		code, p, body := s.ajukan(t, emailKelola, "INT-302", "t-petakan")
		require.Equal(t, http.StatusCreated, code, body)

		code, _, _ = s.putus(t, emailKoordinator, p.ID, "SETUJU", "")

		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "MENUNGGU", s.statusPersetujuan(t, p.ID))

		code, _, body = s.putus(t, emailSetuju, p.ID, "SETUJU", "")
		assert.Equal(t, http.StatusOK, code, "peran tingkat provinsi tetap bisa memutus: "+body)
	})
}

func (s *suite) statusPersetujuan(t *testing.T, id string) string {
	t.Helper()
	var status string
	require.NoError(t, s.db.Raw(`SELECT status FROM intervensi_persetujuan WHERE id = ?`, id).Scan(&status).Error)
	return status
}

// Penolakan karena pemisahan tugas tetap meninggalkan jejak: audit DITOLAK dengan sebabnya.
func TestPersetujuan_SeparationOfDutiesViolationIsAudited(t *testing.T) {
	s := newSuite(t, "intervensi_sod_audit")
	s.bekalIzinLangsung(t, emailKelola, "intervensi.setujui")
	code, p, body := s.ajukan(t, emailKelola, "INT-301", "t-petakan")
	require.Equal(t, http.StatusCreated, code, body)

	// Pengaju yang juga memegang izin setujui tetap tidak boleh memutus permintaannya sendiri.
	code, _, body = s.putus(t, emailKelola, p.ID, "SETUJU", "")

	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, body, "Pemisahan tugas")
	assert.Equal(t, "MENUNGGU", s.statusPersetujuan(t, p.ID))
	assert.Equal(t, "t-ews", card(t, s, "INT-301").Tahap)
	assert.Positive(t, s.hitung(t, `SELECT count(*) FROM audit_log WHERE aksi = 'PUTUS_PERSETUJUAN' AND hasil = 'DITOLAK'
		AND aktor_id = garda_uuid_stabil('pengguna', 'manajer_program') AND metadata->>'sebab' = 'pemisahan_tugas'`))
}

// Pemeriksaan di repository berdiri sendiri: walau usecase dilewati (potret basi, pemanggil lain), keputusan
// oleh pengaju atau oleh pemutus langkah sebelumnya tidak tersimpan.
func TestRepository_PutusMenegakkanPemisahanTugasDiBawahKunci(t *testing.T) {
	s := newSuite(t, "intervensi_sod_repo")
	_, p, _ := s.ajukan(t, emailKelola, "INT-301", "t-fiskal")
	require.Len(t, p.Langkah, 3, "rantai tiga langkah")
	repo := intervensiRepo.New(s.db)
	ctx := context.Background()
	pengaju := idx.UUIDStabil("pengguna", "manajer_program")
	penyetuju := idx.UUIDStabil("pengguna", "strategis_bi")
	id := idx.ID(mustInt(t, p.ID))

	_, err := repo.Putus(ctx, domain.PutusInput{PersetujuanID: id, LangkahKini: 1, Keputusan: domain.KeputusanSetuju, Oleh: pengaju})
	assert.ErrorIs(t, err, domain.ErrPemisahanTugas, "pengaju")
	assert.Equal(t, 1, s.hitung(t, `SELECT langkah_kini FROM intervensi_persetujuan WHERE id = ?`, p.ID), "tidak bergerak")

	_, err = repo.Putus(ctx, domain.PutusInput{PersetujuanID: id, LangkahKini: 1, Keputusan: domain.KeputusanSetuju, Oleh: penyetuju})
	require.NoError(t, err)

	_, err = repo.Putus(ctx, domain.PutusInput{PersetujuanID: id, LangkahKini: 2, Keputusan: domain.KeputusanSetuju, Oleh: penyetuju})
	assert.ErrorIs(t, err, domain.ErrPemisahanTugas, "pemutus langkah 1 memutus langkah 2")
	assert.Equal(t, "MENUNGGU", s.statusPersetujuan(t, p.ID))
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	require.NoError(t, err)
	return n
}
