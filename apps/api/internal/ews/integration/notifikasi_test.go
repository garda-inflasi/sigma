package integration

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	authusecase "github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ews/notifikasi"
	"github.com/garda-inflasi/api/internal/ews/repository/postgres"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type baris struct {
	Alert, Pengguna, Kanal, Tujuan, Status string
	Percobaan                              int
}

func (s *suite) outbox(t *testing.T, where string, args ...any) []baris {
	t.Helper()
	var rows []baris
	require.NoError(t, s.db.Raw(`SELECT alert_id AS alert, pengguna_id AS pengguna, kanal, tujuan, status, percobaan
		FROM ews_notifikasi `+where+` ORDER BY alert_id, pengguna_id, kanal`, args...).Scan(&rows).Error)
	return rows
}

func (s *suite) ubahKanalAturan(t *testing.T, id string, kanal []string) {
	t.Helper()
	list := data[[]aturanRow](t, s.call(t, http.MethodGet, "/v1/ews/aturan", emailLihat, nil))
	for _, a := range list {
		if a.ID == id {
			body := bodyAturan(a.Versi, a.AmbangWaspada, a.AmbangSiaga, a.AmbangKritis)
			body["kanal"] = kanal
			body["cooldown_jam"] = a.CooldownJam
			w := s.call(t, http.MethodPut, "/v1/ews/aturan/"+id, emailAtur, body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			return
		}
	}
	t.Fatalf("aturan %s tidak ada", id)
}

func (s *suite) lonjakanCabai(t *testing.T) {
	t.Helper()
	now := time.Now()
	s.entriHarga(t, "H1", "CBM-KRT", "PSR-1171-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "CBM-KRT", "PSR-1171-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))
}

func TestNotifikasi_EngineAlertEnqueuesPerRecipientAndPreference(t *testing.T) {
	s := newSuite(t, "ews_notif_antrean")
	s.ubahKanalAturan(t, "ATR-LH-CBM-KRT", []string{"DASHBOARD", "PUSH", "EMAIL"})

	// manajer_risiko mematikan push; koordinator_lapangan hanya mau email, dan hanya KRITIS.
	put := func(email string, prefs ...map[string]any) {
		w := s.call(t, http.MethodPut, "/v1/ews/notifikasi/preferensi", email, map[string]any{"preferensi": prefs})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	put(emailPenuh, map[string]any{"kanal": "PUSH", "severity_min": "SIAGA", "aktif": false})
	put(emailAkui, map[string]any{"kanal": "PUSH", "severity_min": "SIAGA", "aktif": false}, map[string]any{"kanal": "EMAIL", "severity_min": "KRITIS", "aktif": true})

	s.lonjakanCabai(t)
	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	alert := s.alertMesin(t, "LONJAKAN_HARGA|CBM-KRT|1171")[0].ID

	got := s.outbox(t, "")
	assert.ElementsMatch(t, []baris{
		{alert, uid("manajer_data"), "EMAIL", "manajer.data@gardainflasi.id", "MENUNGGU", 0},
		{alert, uid("manajer_data"), "PUSH", uid("manajer_data"), "MENUNGGU", 0},
		{alert, uid("manajer_program"), "EMAIL", "manajer.program@gardainflasi.id", "MENUNGGU", 0},
		{alert, uid("manajer_program"), "PUSH", uid("manajer_program"), "MENUNGGU", 0},
		{alert, uid("manajer_risiko"), "EMAIL", "manajer.risiko@gardainflasi.id", "MENUNGGU", 0},
		{alert, uid("koordinator_lapangan"), "EMAIL", "koordinator.lapangan@gardainflasi.id", "MENUNGGU", 0},
		{alert, uid("user_mfa"), "EMAIL", "mfa@gardainflasi.id", "MENUNGGU", 0},
		{alert, uid("user_mfa"), "PUSH", uid("user_mfa"), "MENUNGGU", 0},
	}, got, "risiko tanpa push; koordinator hanya email; peran tanpa ews.akui (strategis, mahasiswa) tidak menerima")

	assert.Zero(t, s.evaluasi(t).Dibuat)
	assert.Len(t, s.outbox(t, ""), 8, "evaluasi ulang tidak menggandakan antrean")
}

func TestNotifikasi_SeverityBelowMinimumIsNotQueuedUntilItRises(t *testing.T) {
	s := newSuite(t, "ews_notif_naik")
	now := time.Now()
	s.entriHarga(t, "H1", "BWM", "PSR-1107-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "BWM", "PSR-1107-01", 54500, "DIVALIDASI", now.Add(-2*time.Hour)) // +9% = WASPADA

	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	assert.Empty(t, s.outbox(t, ""), "WASPADA di bawah tingkat minimum bawaan SIAGA")

	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 56500 WHERE kode = 'H2'`).Error) // +13% = SIAGA
	require.Equal(t, 1, s.evaluasi(t).Dinaikkan)
	assert.Len(t, s.outbox(t, "WHERE kanal = 'PUSH'"), 5, "lima pengguna berperan penindak menerima begitu alert mencapai SIAGA")

	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 60000 WHERE kode = 'H2'`).Error) // +20% = KRITIS
	require.Equal(t, 1, s.evaluasi(t).Dinaikkan)
	assert.Len(t, s.outbox(t, ""), 5, "yang sudah dinotifikasi tidak dinotifikasi ulang")
}

// pengirimUji adalah adaptor palsu yang mencatat dan gagal sesuai program.
type pengirimUji struct {
	kanal string
	mu    sync.Mutex
	err   error
	got   []domain.Pesan
}

func (p *pengirimUji) Kanal() string { return p.kanal }
func (p *pengirimUji) Kirim(_ context.Context, m domain.Pesan) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.got = append(p.got, m)
	return p.err
}

func (s *suite) layanan(t *testing.T, cfg notifikasi.Config, pengirim ...domain.Pengirim) *notifikasi.Service {
	t.Helper()
	return notifikasi.New(postgres.New(s.db), authusecase.RolesWithPermission(notifikasi.PermissionPenerima), cfg,
		audit.NewWriter(s.db), logger.NewDefault(), pengirim...)
}

func TestNotifikasi_DispatchDeliversRetriesAndRecordsWithoutAdapter(t *testing.T) {
	s := newSuite(t, "ews_notif_kirim")
	s.ubahKanalAturan(t, "ATR-LH-CBM-KRT", []string{"DASHBOARD", "PUSH", "EMAIL"})
	s.lonjakanCabai(t)
	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	require.Len(t, s.outbox(t, ""), 10)

	t.Run("hanya kanal terkonfigurasi yang dikirim nyata; sisanya dicatat", func(t *testing.T) {
		email := &pengirimUji{kanal: domain.KanalEmail}
		svc := s.layanan(t, notifikasi.Config{}, email) // PUSH tanpa adaptor

		r, err := svc.Kirim(t.Context())

		require.NoError(t, err)
		assert.Equal(t, notifikasi.RingkasanKirim{Terkirim: 5, Dicatat: 5}, r)
		require.Len(t, email.got, 5)
		assert.Contains(t, email.got[0].Judul, "Lonjakan harga cabai")
		assert.Equal(t, "KRITIS", email.got[0].Severity)
		assert.Equal(t, "/ews?id="+email.got[0].AlertID, email.got[0].TautanPath)

		assert.Len(t, s.outbox(t, "WHERE kanal = 'EMAIL' AND status = 'TERKIRIM'"), 5)
		assert.Len(t, s.outbox(t, "WHERE kanal = 'PUSH' AND status = 'DICATAT'"), 5)
		var sent int
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM ews_notifikasi WHERE status = 'TERKIRIM' AND dikirim_pada IS NOT NULL AND percobaan = 1`).Scan(&sent).Error)
		assert.Equal(t, 5, sent)
		var galat string
		require.NoError(t, s.db.Raw(`SELECT galat FROM ews_notifikasi WHERE status = 'DICATAT' LIMIT 1`).Scan(&galat).Error)
		assert.Contains(t, galat, "belum dikonfigurasi")

		r, err = svc.Kirim(t.Context())
		require.NoError(t, err)
		assert.Equal(t, notifikasi.RingkasanKirim{}, r, "tidak ada yang dikirim dua kali")
	})
}

func TestNotifikasi_FailingAdapterIsRetriedWithBackoffThenMarkedFailed(t *testing.T) {
	s := newSuite(t, "ews_notif_retry")
	s.lonjakanCabai(t)
	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	push := &pengirimUji{kanal: domain.KanalPush, err: errors.New("layanan push mati")}
	svc := s.layanan(t, notifikasi.Config{MaxPercobaan: 2}, push)

	r, err := svc.Kirim(t.Context())
	require.NoError(t, err)
	assert.Equal(t, notifikasi.RingkasanKirim{Diulang: 5}, r)
	rows := s.outbox(t, "")
	for _, b := range rows {
		assert.Equal(t, "MENUNGGU", b.Status)
		assert.Equal(t, 1, b.Percobaan)
	}
	var tertunda int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM ews_notifikasi WHERE jadwal_berikut > NOW() + INTERVAL '30 seconds'`).Scan(&tertunda).Error)
	assert.Equal(t, 5, tertunda, "percobaan berikutnya dijadwalkan nanti (backoff)")

	r, err = svc.Kirim(t.Context())
	require.NoError(t, err)
	assert.Equal(t, notifikasi.RingkasanKirim{}, r, "belum jatuh tempo")

	require.NoError(t, s.db.Exec(`UPDATE ews_notifikasi SET jadwal_berikut = NOW() - INTERVAL '1 second'`).Error)
	r, err = svc.Kirim(t.Context())
	require.NoError(t, err)
	assert.Equal(t, notifikasi.RingkasanKirim{Gagal: 5}, r, "percobaan kedua habis")
	for _, b := range s.outbox(t, "") {
		assert.Equal(t, "GAGAL", b.Status)
		assert.Equal(t, 2, b.Percobaan)
	}
	var galat string
	require.NoError(t, s.db.Raw(`SELECT galat FROM ews_notifikasi LIMIT 1`).Scan(&galat).Error)
	assert.Equal(t, "layanan push mati", galat)
}

func TestNotifikasi_ClaimLeasesRowsSoWorkersDoNotDoubleSend(t *testing.T) {
	s := newSuite(t, "ews_notif_lease")
	s.lonjakanCabai(t)
	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	repo := postgres.New(s.db)

	first, err := repo.Klaim(t.Context(), time.Now(), 10, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 5)
	second, err := repo.Klaim(t.Context(), time.Now(), 10, 5*time.Minute)
	require.NoError(t, err)
	assert.Empty(t, second, "baris yang sedang dikirim tidak diklaim lagi sebelum lease habis")

	crashed, err := repo.Klaim(t.Context(), time.Now().Add(6*time.Minute), 10, 5*time.Minute)
	require.NoError(t, err)
	assert.Len(t, crashed, 5, "pekerja yang mati tidak menghilangkan notifikasi: lease kedaluwarsa lalu diklaim ulang")
}

func TestNotifikasi_AutoEscalationOfUnacknowledgedAlerts(t *testing.T) {
	s := newSuite(t, "ews_notif_eskalasi")
	svc := s.layanan(t, notifikasi.Config{EskalasiSetelah: 10 * time.Minute})

	ids, err := svc.Eskalasi(t.Context())
	require.NoError(t, err)
	// Alert demo BARU >= SIAGA yang lebih tua dari 10 menit: EWS-1001 (27m), EWS-1002 (50m), EWS-1003 (73m).
	// EWS-1000 baru 4 menit; EWS-0902 WASPADA; surplus INFO.
	assert.Equal(t, []string{"EWS-1001", "EWS-1002", "EWS-1003"}, ids)

	var rows []struct {
		ID, Status, Catatan string
		DieskalasiOlehID    *string
		Oleh                *string
	}
	require.NoError(t, s.db.Raw(`SELECT a.id, a.status, r.catatan, a.dieskalasi_oleh_id, r.oleh
		FROM ews_alert a JOIN ews_riwayat r ON r.alert_id = a.id AND r.ke = 'DIESKALASI'
		WHERE a.id IN ('EWS-1001','EWS-1002','EWS-1003') ORDER BY a.id`).Scan(&rows).Error)
	require.Len(t, rows, 3)
	for _, r := range rows {
		assert.Equal(t, "DIESKALASI", r.Status)
		assert.Equal(t, "Eskalasi otomatis: tidak diakui dalam 10 menit", r.Catatan)
		assert.Nil(t, r.Oleh, "dilakukan sistem")
		assert.Nil(t, r.DieskalasiOlehID)
	}
	var status string
	require.NoError(t, s.db.Raw(`SELECT status FROM ews_alert WHERE id = 'EWS-1000'`).Scan(&status).Error)
	assert.Equal(t, "BARU", status)

	var audited int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'ESKALASI_OTOMATIS'`).Scan(&audited).Error)
	assert.Equal(t, 3, audited)

	again, err := svc.Eskalasi(t.Context())
	require.NoError(t, err)
	assert.Empty(t, again, "idempoten")

	off := s.layanan(t, notifikasi.Config{})
	none, err := off.Eskalasi(t.Context())
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestNotifikasi_PreferencesAndPushSubscriptionEndpoints(t *testing.T) {
	s := newSuite(t, "ews_notif_api")
	const prefPath, pushPath = "/v1/ews/notifikasi/preferensi", "/v1/ews/notifikasi/langganan-push"
	langganan := map[string]any{"endpoint": "https://push.example.com/abc", "p256dh": "BKey", "auth": "aKey"}

	t.Run("hanya peran penindak (ews.akui)", func(t *testing.T) {
		for _, path := range []string{prefPath} {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzin, nil).Code)
		}
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPut, pushPath, emailLihat, langganan).Code)
	})

	t.Run("bawaan: aktif, minimum SIAGA, tanpa pengiriman nyata", func(t *testing.T) {
		w := s.call(t, http.MethodGet, prefPath, emailAkui, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := data[struct {
			Preferensi []struct {
				Kanal       string `json:"kanal"`
				SeverityMin string `json:"severity_min"`
				Aktif       bool   `json:"aktif"`
				Tersedia    bool   `json:"tersedia"`
			} `json:"preferensi"`
			VAPIDPublicKey string `json:"vapid_public_key"`
		}](t, w)
		require.Len(t, got.Preferensi, 2)
		assert.Equal(t, "EMAIL", got.Preferensi[0].Kanal)
		assert.Equal(t, "SIAGA", got.Preferensi[0].SeverityMin)
		assert.True(t, got.Preferensi[0].Aktif)
		assert.False(t, got.Preferensi[0].Tersedia, "SMTP tidak dikonfigurasi di tes")
		assert.Empty(t, got.VAPIDPublicKey)
	})

	t.Run("simpan, baca ulang, dan audit", func(t *testing.T) {
		w := s.call(t, http.MethodPut, prefPath, emailAkui, map[string]any{"preferensi": []map[string]any{{"kanal": "PUSH", "severity_min": "WASPADA", "aktif": false}}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var row struct {
			SeverityMin string
			Aktif       bool
		}
		require.NoError(t, s.db.Raw(`SELECT severity_min, aktif FROM preferensi_notifikasi WHERE pengguna_id = garda_uuid_stabil('pengguna', 'koordinator_lapangan') AND kanal = 'PUSH'`).Scan(&row).Error)
		assert.Equal(t, "WASPADA", row.SeverityMin)
		assert.False(t, row.Aktif)
		var audited int
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'UBAH_PREFERENSI_NOTIFIKASI' AND aktor_id = garda_uuid_stabil('pengguna', 'koordinator_lapangan')`).Scan(&audited).Error)
		assert.Equal(t, 1, audited)
	})

	t.Run("masukan tidak valid ditolak", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"kanal":       {"preferensi": []map[string]any{{"kanal": "SMS", "severity_min": "SIAGA", "aktif": true}}},
			"tingkat":     {"preferensi": []map[string]any{{"kanal": "PUSH", "severity_min": "GENTING", "aktif": true}}},
			"kosong":      {"preferensi": []map[string]any{}},
			"tanpa aktif": {"preferensi": []map[string]any{{"kanal": "PUSH", "severity_min": "SIAGA"}}},
		} {
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPut, prefPath, emailAkui, body).Code, name)
		}
	})

	t.Run("langganan push didaftarkan, diganti, dan dibatalkan", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.call(t, http.MethodPut, pushPath, emailAkui, langganan).Code)
		require.Equal(t, http.StatusOK, s.call(t, http.MethodPut, pushPath, emailPenuh, langganan).Code, "endpoint yang sama berpindah ke pemilik baru")
		var owner string
		require.NoError(t, s.db.Raw(`SELECT pengguna_id FROM langganan_push WHERE endpoint = ?`, langganan["endpoint"]).Scan(&owner).Error)
		assert.Equal(t, uid("manajer_risiko"), owner)

		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPut, pushPath, emailAkui, map[string]any{"endpoint": "bukan-url", "p256dh": "k", "auth": "a"}).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodDelete, pushPath, emailAkui, map[string]any{"endpoint": langganan["endpoint"]}).Code, "bukan pemiliknya")
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodDelete, pushPath, emailPenuh, map[string]any{"endpoint": langganan["endpoint"]}).Code)
		var n int
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM langganan_push`).Scan(&n).Error)
		assert.Zero(t, n)
	})
}
