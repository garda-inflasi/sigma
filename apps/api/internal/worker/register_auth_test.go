package worker

import (
	"context"
	"testing"
	"time"

	authuc "github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKonfigAuthPembersihanDefault(t *testing.T) {
	cfg, err := konfigAuthPembersihan(env(nil))
	require.NoError(t, err)
	assert.Equal(t, authuc.DefaultKonfigPembersihan(), cfg)
	assert.Equal(t, 90*24*time.Hour, cfg.RetensiLoginAttempt)
	assert.Equal(t, 7*24*time.Hour, cfg.RetensiResetToken)
	assert.Equal(t, 1000, cfg.UkuranBatch)
}

func TestKonfigAuthPembersihanDariEnv(t *testing.T) {
	cfg, err := konfigAuthPembersihan(env(map[string]string{
		"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": " 30 ",
		"JOB_AUTH_PEMBERSIHAN_RESET_HARI": "2",
		"JOB_AUTH_PEMBERSIHAN_BATCH":      "250",
	}))
	require.NoError(t, err)
	assert.Equal(t, 30*24*time.Hour, cfg.RetensiLoginAttempt)
	assert.Equal(t, 2*24*time.Hour, cfg.RetensiResetToken)
	assert.Equal(t, 250, cfg.UkuranBatch)
}

func TestKonfigAuthPembersihanBatasDiterima(t *testing.T) {
	for nama, kv := range map[string]map[string]string{
		"login 1 hari (lantai)":  {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "1"},
		"login maksimum":         {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "3650"},
		"reset nol = langsung":   {"JOB_AUTH_PEMBERSIHAN_RESET_HARI": "0"},
		"reset maksimum":         {"JOB_AUTH_PEMBERSIHAN_RESET_HARI": "365"},
		"batch minimum":          {"JOB_AUTH_PEMBERSIHAN_BATCH": "1"},
		"batch maksimum":         {"JOB_AUTH_PEMBERSIHAN_BATCH": "100000"},
		"spasi di sekitar nilai": {"JOB_AUTH_PEMBERSIHAN_BATCH": "  5\t"},
	} {
		_, err := konfigAuthPembersihan(env(kv))
		assert.NoError(t, err, nama)
	}
}

func TestKonfigAuthPembersihanLoginOffTidakMenghapusLoginAttempts(t *testing.T) {
	for _, nilai := range []string{"off", "OFF", "false", "disabled"} {
		cfg, err := konfigAuthPembersihan(env(map[string]string{"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": nilai}))
		require.NoError(t, err, nilai)
		assert.Zero(t, cfg.RetensiLoginAttempt, nilai)
	}
}

func TestKonfigAuthPembersihanMenolakNilaiTakValid(t *testing.T) {
	for nama, kv := range map[string]map[string]string{
		"login nol (bukan off)":       {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "0"},
		"login di bawah lantai":       {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "-1"},
		"login terlalu besar":         {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "3651"},
		"login bukan angka":           {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "sembilan puluh"},
		"login durasi Go":             {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "24h"},
		"login pecahan":               {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "1.5"},
		"reset negatif":               {"JOB_AUTH_PEMBERSIHAN_RESET_HARI": "-1"},
		"reset terlalu besar":         {"JOB_AUTH_PEMBERSIHAN_RESET_HARI": "366"},
		"reset bukan angka":           {"JOB_AUTH_PEMBERSIHAN_RESET_HARI": "seminggu"},
		"batch nol":                   {"JOB_AUTH_PEMBERSIHAN_BATCH": "0"},
		"batch terlalu besar":         {"JOB_AUTH_PEMBERSIHAN_BATCH": "100001"},
		"batch bukan angka":           {"JOB_AUTH_PEMBERSIHAN_BATCH": "banyak"},
		"interval tidak valid":        {"JOB_AUTH_PEMBERSIHAN_INTERVAL": "sering"},
		"interval di bawah 1 detik":   {"JOB_AUTH_PEMBERSIHAN_INTERVAL": "10ms"},
		"salah satu valid, lainnya x": {"JOB_AUTH_PEMBERSIHAN_LOGIN_HARI": "30", "JOB_AUTH_PEMBERSIHAN_BATCH": "x"},
	} {
		_, err := buildScheduler(deps{log: quiet(), getenv: env(kv)})
		assert.Error(t, err, nama)
	}
}

func TestAuthPembersihanTerdaftarDenganIntervalSatuJam(t *testing.T) {
	s, err := buildScheduler(deps{log: quiet(), getenv: env(nil)})
	require.NoError(t, err)
	assert.Contains(t, s.Names(), "auth_pembersihan")
	got := map[string]string{}
	for _, st := range s.Statuses() {
		got[st.Name] = st.Interval
	}
	assert.Equal(t, "1h0m0s", got["auth_pembersihan"])
}

func TestAuthPembersihanIntervalDariEnv(t *testing.T) {
	for nilai, want := range map[string]string{"15m": "15m0s", "off": "0s", "0": "0s"} {
		s, err := buildScheduler(deps{log: quiet(), getenv: env(map[string]string{"JOB_AUTH_PEMBERSIHAN_INTERVAL": nilai})})
		require.NoError(t, err, nilai)
		got := map[string]string{}
		for _, st := range s.Statuses() {
			got[st.Name] = st.Interval
		}
		assert.Equal(t, want, got["auth_pembersihan"], nilai)
	}
}

// Job terdaftar menghapus baris mati sungguhan lewat scheduler (advisory lock, `worker run auth_pembersihan`)
// dan membiarkan baris hidup.
func TestRunAuthPembersihanAgainstMigratedDatabase(t *testing.T) {
	db := pg.NewDatabase(t, "worker_auth_pembersihan")
	pgtest.MigrateUp(t, db)
	require.NoError(t, db.Exec(`
		INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, status)
		VALUES (garda_uuid_stabil('pengguna', 'worker-u'), 'Worker U', 'worker-u@gardainflasi.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'AKTIF')`).Error)
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO jwt_blacklist (jti, expires_at) VALUES ('mati', ?), ('hidup', ?)`,
		now.Add(-time.Minute), now.Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, revoked_at) VALUES
		(garda_uuid_stabil('pengguna', 'worker-u'), 'rt-mati', gen_random_uuid(), ?, NULL),
		(garda_uuid_stabil('pengguna', 'worker-u'), 'rt-dicabut-berlaku', gen_random_uuid(), ?, ?),
		(garda_uuid_stabil('pengguna', 'worker-u'), 'rt-hidup', gen_random_uuid(), ?, NULL)`,
		now.Add(-time.Minute), now.Add(time.Hour), now.Add(-time.Second), now.Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, used_at) VALUES
		(garda_uuid_stabil('pengguna', 'worker-u'), 'pr-mati-lama', ?, NULL),
		(garda_uuid_stabil('pengguna', 'worker-u'), 'pr-mati-baru', ?, NULL),
		(garda_uuid_stabil('pengguna', 'worker-u'), 'pr-hidup', ?, NULL)`,
		now.Add(-30*24*time.Hour), now.Add(-time.Hour), now.Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO login_attempts (user_id, success, failure_reason, attempted_at) VALUES
		(garda_uuid_stabil('pengguna', 'worker-u'), false, 'lama', ?), (garda_uuid_stabil('pengguna', 'worker-u'), false, 'baru', ?)`,
		now.Add(-100*24*time.Hour), now.Add(-time.Hour)).Error)

	s, err := buildScheduler(deps{db: db, log: quiet(), getenv: env(nil)})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ran, err := s.RunOnce(ctx, "auth_pembersihan")
	require.NoError(t, err)
	require.True(t, ran)

	kolom := func(query string) []string {
		var out []string
		require.NoError(t, db.Raw(query).Scan(&out).Error)
		return out
	}
	assert.Equal(t, []string{"hidup"}, kolom("SELECT jti FROM jwt_blacklist"))
	assert.ElementsMatch(t, []string{"rt-dicabut-berlaku", "rt-hidup"}, kolom("SELECT token_hash FROM refresh_tokens"))
	assert.ElementsMatch(t, []string{"pr-mati-baru", "pr-hidup"}, kolom("SELECT token_hash FROM password_reset_tokens"))
	assert.Equal(t, []string{"baru"}, kolom("SELECT failure_reason FROM login_attempts"))

	for _, st := range s.Statuses() {
		if st.Name == "auth_pembersihan" {
			assert.Empty(t, st.LastError)
			assert.Equal(t, 1, st.Runs)
		}
	}
}
