package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sseClient membaca satu koneksi SSE nyata dan mengantarkan frame ke channel.
type sseClient struct {
	resp   *http.Response
	frames chan map[string]string
	cancel context.CancelFunc
}

// openStream membuka /v1/ews/stream lewat cookie garda_access (seperti browser).
func (s *suite) openStream(t *testing.T, srv *httptest.Server, email, lastEventID string) *sseClient {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/ews/stream", nil)
	require.NoError(t, err)
	if email != "" {
		req.AddCookie(&http.Cookie{Name: "garda_access", Value: s.login(t, email)})
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	c := &sseClient{resp: resp, frames: make(chan map[string]string, 32), cancel: cancel}
	t.Cleanup(c.close)
	if resp.StatusCode != http.StatusOK {
		close(c.frames)
		return c
	}

	go func() {
		defer close(c.frames)
		sc := bufio.NewScanner(resp.Body)
		cur := map[string]string{}
		for sc.Scan() {
			line := sc.Text()
			if line == "" {
				if len(cur) > 0 {
					c.frames <- cur
				}
				cur = map[string]string{}
				continue
			}
			if k, v, ok := strings.Cut(line, ": "); ok {
				cur[k] = v
			}
		}
	}()
	return c
}

func (c *sseClient) close() {
	c.cancel()
	_ = c.resp.Body.Close()
}

// next menunggu frame berikutnya; ok false bila tidak ada dalam d.
func (c *sseClient) next(d time.Duration) (map[string]string, bool) {
	select {
	case f, ok := <-c.frames:
		return f, ok
	case <-time.After(d):
		return nil, false
	}
}

type alertEvent struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Jenis    string `json:"jenis"`
	Wilayah  string `json:"wilayah"`
	Status   string `json:"status"`
}

func eventData(t *testing.T, f map[string]string) alertEvent {
	t.Helper()
	var a alertEvent
	require.NoError(t, json.Unmarshal([]byte(f["data"]), &a))
	return a
}

func TestStream_RequiresAuthenticationAndPermission(t *testing.T) {
	s := sharedSuite(t)
	srv := httptest.NewServer(s.router)
	t.Cleanup(srv.Close) // dipasang sebelum klien, jadi berjalan sesudah koneksi klien ditutup

	assert.Equal(t, http.StatusUnauthorized, s.openStream(t, srv, "", "").resp.StatusCode)
	assert.Equal(t, http.StatusForbidden, s.openStream(t, srv, emailTanpaIzin, "").resp.StatusCode)
}

func TestStream_DeliversRuleEngineAlertsWithResumeAndNoDuplicates(t *testing.T) {
	s := newSuite(t, "ews_stream")
	srv := httptest.NewServer(s.router)
	t.Cleanup(srv.Close) // dipasang sebelum klien, jadi berjalan sesudah koneksi klien ditutup
	now := time.Now()

	// Dua kondisi yang akan terdeteksi: bawang merah Meulaboh (+9%) dan Banda Aceh (+20%).
	s.entriHarga(t, "H1", "BWM", "PSR-1107-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "BWM", "PSR-1107-01", 54500, "DIVALIDASI", now.Add(-2*time.Hour))

	live := s.openStream(t, srv, emailLihat, "")
	assert.Equal(t, "text/event-stream; charset=utf-8", live.resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache, no-transform", live.resp.Header.Get("Cache-Control"))
	assert.Equal(t, "no", live.resp.Header.Get("X-Accel-Buffering"))
	retry, ok := live.next(3 * time.Second)
	require.True(t, ok)
	assert.Equal(t, "5000", retry["retry"])
	_, ok = live.next(300 * time.Millisecond)
	assert.False(t, ok, "klien baru tidak menerima riwayat peristiwa lama")

	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	f1, ok := live.next(5 * time.Second)
	require.True(t, ok, "alert baru sampai tanpa menunggu polling")
	assert.Equal(t, "alert", f1["event"])
	first := eventData(t, f1)
	assert.Equal(t, "WASPADA", first.Severity)
	assert.Equal(t, "LONJAKAN_HARGA", first.Jenis)
	assert.Equal(t, "KABUPATEN ACEH BARAT", first.Wilayah)
	assert.Equal(t, "BARU", first.Status)

	// Kondisi memburuk: alert yang sama naik tingkat dan dikirim lagi sebagai peristiwa baru.
	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 56500 WHERE kode = 'H2'`).Error)
	require.Equal(t, 1, s.evaluasi(t).Dinaikkan)
	f2, ok := live.next(5 * time.Second)
	require.True(t, ok)
	second := eventData(t, f2)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, "SIAGA", second.Severity)
	assert.NotEqual(t, f1["id"], f2["id"])

	// Alert ketiga muncul saat klien terputus; menyambung ulang dengan Last-Event-ID
	// hanya menerima yang terlewat, tanpa mengulang peristiwa yang sudah diterima.
	live.close()
	s.entriHarga(t, "H3", "CBM-KRT", "PSR-1171-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H4", "CBM-KRT", "PSR-1171-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))
	require.Equal(t, 1, s.evaluasi(t).Dibuat)

	resumed := s.openStream(t, srv, emailLihat, f2["id"])
	_, ok = resumed.next(3 * time.Second)
	require.True(t, ok, "retry")
	f3, ok := resumed.next(5 * time.Second)
	require.True(t, ok)
	third := eventData(t, f3)
	assert.Equal(t, "KRITIS", third.Severity)
	assert.Equal(t, "KOTA BANDA ACEH", third.Wilayah)
	_, ok = resumed.next(300 * time.Millisecond)
	assert.False(t, ok, "tidak ada duplikat setelah sambung ulang")

	// Pengguna lain (tanpa Last-Event-ID) mulai dari sekarang.
	other := s.openStream(t, srv, emailPenuh, "")
	_, _ = other.next(3 * time.Second)
	_, ok = other.next(300 * time.Millisecond)
	assert.False(t, ok)
}

func TestStream_StaleOrInvalidLastEventIDStartsFromNow(t *testing.T) {
	s := newSuite(t, "ews_stream_header")
	srv := httptest.NewServer(s.router)
	t.Cleanup(srv.Close) // dipasang sebelum klien, jadi berjalan sesudah koneksi klien ditutup

	c := s.openStream(t, srv, emailLihat, "bukan-angka")
	require.Equal(t, http.StatusOK, c.resp.StatusCode)
	_, ok := c.next(3 * time.Second)
	assert.True(t, ok, "retry tetap dikirim")
}

// Server produksi memakai WriteTimeout 15 detik; aliran harus tetap hidup melewatinya.
func TestStream_OutlivesServerWriteTimeout(t *testing.T) {
	s := newSuite(t, "ews_stream_timeout")
	srv := httptest.NewUnstartedServer(s.router)
	srv.Config.WriteTimeout = 300 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)
	now := time.Now()
	s.entriHarga(t, "H1", "BWM", "PSR-1107-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "BWM", "PSR-1107-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))

	c := s.openStream(t, srv, emailLihat, "")
	_, ok := c.next(3 * time.Second)
	require.True(t, ok)
	time.Sleep(900 * time.Millisecond) // tiga kali WriteTimeout

	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	f, ok := c.next(5 * time.Second)
	require.True(t, ok, "koneksi masih terbuka setelah melewati WriteTimeout")
	assert.Equal(t, "KRITIS", eventData(t, f).Severity)
}
