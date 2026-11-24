package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intervalJob(t *testing.T, kv map[string]string, nama string) (string, bool) {
	t.Helper()
	s, err := buildScheduler(deps{log: quiet(), getenv: env(kv)})
	require.NoError(t, err)
	for _, st := range s.Statuses() {
		if st.Name == nama {
			return st.Interval, true
		}
	}
	return "", false
}

// Jembatan EWS pipeline ML hanya ada bila Mongo dikonfigurasi; bawaan 5 menit, "off" mematikan.
func TestMLJembatanEWS_HanyaDenganMongo(t *testing.T) {
	_, ada := intervalJob(t, nil, "ml_jembatan_ews")
	assert.False(t, ada, "tanpa MONGO_URI/MONGO_URL job tidak didaftarkan")

	mongo := map[string]string{"MONGO_URI": "mongodb://127.0.0.1:1/?connect=direct"}
	interval, ada := intervalJob(t, mongo, "ml_jembatan_ews")
	require.True(t, ada)
	assert.Equal(t, "5m0s", interval)

	interval, ada = intervalJob(t, map[string]string{"MONGO_URL": "mongodb://127.0.0.1:1/", "JOB_ML_JEMBATAN_EWS_INTERVAL": "off"}, "ml_jembatan_ews")
	require.True(t, ada, "MONGO_URL adalah fallback MONGO_URI")
	assert.Equal(t, "0s", interval)

	_, err := buildScheduler(deps{log: quiet(), getenv: env(map[string]string{"MONGO_URI": "mongodb://x", "JOB_ML_JEMBATAN_EWS_INTERVAL": "sebentar"})})
	assert.Error(t, err, "interval tak valid ditolak seperti job lain")
}
