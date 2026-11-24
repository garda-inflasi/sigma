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

func (s *suite) ubahKanalAturan(t *testing.T, kode string, kanal []string) {
	t.Helper()
	list := data[[]aturanRow](t, s.call(t, http.MethodGet, "/v1/ews/aturan", emailLihat, nil))
	for _, a := range list {
		if a.Kode != nil && *a.Kode == kode {
			body := bodyAturan(a.Versi, a.AmbangWaspada, a.AmbangSiaga, a.AmbangKritis)
			body["kanal"] = kanal
			body["cooldown_jam"] = a.CooldownJam
			w := s.call(t, http.MethodPut, "/v1/ews/aturan/"+a.ID, emailAtur, body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			return
		}
	}
	t.Fatalf("aturan %s tidak ada", kode)
}

func (s *suite) lonjakanCabai(t *testing.T) {
	t.Helper()
	now := time.Now()
	s.entriHarga(t, "H1", "CBM-KRT", "PSR-1171-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "CBM-KRT", "PSR-1171-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))
}

// aturPreferensiUji: manajer_risiko mematikan push; koordinator_lapangan mematikan push dan email hanya KRITIS.
func (s *suite) aturPreferensiUji(t *testing.T) {
	t.Helper()
	put := func(email string, prefs ...map[string]any) {
		w := s.call(t, http.MethodPut, "/v1/ews/notifikasi/preferensi", email, map[string]any{"preferensi": prefs})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	put(emailPenuh, map[string]any{"kanal": "PUSH", "severity_min": "SIAGA", "aktif": false})
	put(emailAkui, map[string]any{"kanal": "PUSH", "severity_min": "SIAGA", "aktif": false}, map[string]any{"kanal": "EMAIL", "severity_min": "KRITIS", "aktif": true})
}

func TestNotifikasi_EngineAlertEnqueuesPerRecipientAndPreference(t *testing.T) {
	s := newSuite(t, "ews_notif_antrean")
	s.ubahKanalAturan(t, "ATR-LH-CBM-KRT", []string{"DASHBOARD", "PUSH", "EMAIL"})
	s.aturPreferensiUji(t)

	now := time.Now()
	s.entriHarga(t, "H1", "CBM-KRT", "PSR-1171-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "CBM-KRT", "PSR-1171-01", 56500, "DIVALIDASI", now.Add(-2*time.Hour)) // +13% = SIAGA
	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	alert := s.alertMesin(t, "LONJAKAN_HARGA|CBM-KRT|1171")[0]
	require.Equal(t, "SIAGA", alert.Severity)

	got := s.outbox(t, "")
	assert.ElementsMatch(t, []baris{
		{alert.ID, uid("manajer_data"), "EMAIL", "manajer.data@gardainflasi.id", "MENUNGGU", 0},
		{alert.ID, uid("manajer_data"), "PUSH", uid("manajer_data"), "MENUNGGU", 0},
		{alert.ID, uid("manajer_program"), "EMAIL", "manajer.program@gardainflasi.id", "MENUNGGU", 0},
		{alert.ID, uid("manajer_program"), "PUSH", uid("manajer_program"), "MENUNGGU", 0},
		{alert.ID, uid("manajer_risiko"), "EMAIL", "manajer.risiko@gardainflasi.id", "MENUNGGU", 0},
		{alert.ID, uid("user_mfa"), "EMAIL", "mfa@gardainflasi.id", "MENUNGGU", 0},
		{alert.ID, uid("user_mfa"), "PUSH", uid("user_mfa"), "MENUNGGU", 0},
		{alert.ID, uid("superadmin"), "EMAIL", "superadmin@gardainflasi.id", "MENUNGGU", 0},
		{alert.ID, uid("superadmin"), "PUSH", uid("superadmin"), "MENUNGGU", 0},
	}, got, "di bawah KRITIS preferensi berlaku: risiko tanpa push, koordinator tidak menerima; peran tanpa ews.akui tidak menerima")

	assert.Zero(t, s.evaluasi(t).Dibuat)
	assert.Len(t, s.outbox(t, ""), 9, "evaluasi ulang tidak menggandakan antrean")
}

func TestNotifikasi_KritisMengabaikanPreferensiTingkatDanKanalNonaktif(t *testing.T) {
	s := newSuite(t, "ews_notif_kritis_pref")
	s.ubahKanalAturan(t, "ATR-LH-CBM-KRT", []string{"DASHBOARD", "PUSH", "EMAIL"})
	s.aturPreferensiUji(t)
	// Kanal email juga dimatikan: tidak boleh menahan KRITIS.
	w := s.call(t, http.MethodPut, "/v1/ews/notifikasi/preferensi", emailPenuh, map[string]any{"preferensi": []map[string]any{
		{"kanal": "PUSH", "severity_min": "KRITIS", "aktif": false}, {"kanal": "EMAIL", "severity_min": "KRITIS", "aktif": false},
	}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	s.lonjakanCabai(t) // +20% = KRITIS
	require.Equal(t, 1, s.evaluasi(t).Dibuat)

	got := s.outbox(t, "")
	assert.Len(t, got, 12, "enam penerima, dua kanal aturan")
	for _, penerima := range []string{uid("manajer_risiko"), uid("koordinator_lapangan")} {
		var kanal []string
		for _, b := range got {
			if b.Pengguna == penerima {
				kanal = append(kanal, b.Kanal)
			}
		}
		assert.ElementsMatch(t, []string{"EMAIL", "PUSH"}, kanal, penerima)
	}
}

func TestNotifikasi_PenyimpananYangMengecualikanKritisDitolak(t *testing.T) {
	s := newSuite(t, "ews_notif_tolak")
	for _, nilai := range []string{"NONE", "TIDAK_ADA", "DI_ATAS_KRITIS", ""} {
		w := s.call(t, http.MethodPut, "/v1/ews/notifikasi/preferensi", emailPenuh, map[string]any{"preferensi": []map[string]any{
			{"kanal": "EMAIL", "severity_min": nilai, "aktif": true},
		}})
		assert.Equal(t, http.StatusBadRequest, w.Code, "%q: %s", nilai, w.Body.String())
	}
	var n int64
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM preferensi_notifikasi`).Scan(&n).Error)
	assert.Zero(t, n)
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
	assert.Len(t, s.outbox(t, "WHERE kanal = 'PUSH'"), 6, "enam pengguna berperan penindak menerima begitu alert mencapai SIAGA")

	// Notifikasi SIAGA sudah terkirim; kenaikan ke KRITIS wajib sampai juga, tanpa baris ganda.
	require.NoError(t, s.db.Exec(`UPDATE ews_notifikasi SET status = 'TERKIRIM', percobaan = 1, dikirim_pada = now()`).Error)
	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 60000 WHERE kode = 'H2'`).Error) // +20% = KRITIS
	require.Equal(t, 1, s.evaluasi(t).Dinaikkan)
	got := s.outbox(t, "")
	assert.Len(t, got, 6, "tidak ada baris ganda")
	for _, b := range got {
		assert.Equal(t, "MENUNGGU", b.Status, "dijadwalkan ulang sebagai KRITIS")
		assert.Zero(t, b.Percobaan)
	}

	// Evaluasi berikutnya tidak mengirim ulang lagi.
	require.NoError(t, s.db.Exec(`UPDATE ews_notifikasi SET status = 'TERKIRIM', percobaan = 1`).Error)
	assert.Zero(t, s.evaluasi(t).Dinaikkan)
	assert.Empty(t, s.outbox(t, "WHERE status = 'MENUNGGU'"))
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
	require.Len(t, s.outbox(t, ""), 12)

	t.Run("hanya kanal terkonfigurasi yang dikirim nyata; sisanya dicatat", func(t *testing.T) {
		email := &pengirimUji{kanal: domain.KanalEmail}
		svc := s.layanan(t, notifikasi.Config{}, email) // PUSH tanpa adaptor

		r, err := svc.Kirim(t.Context())

		require.NoError(t, err)
		assert.Equal(t, notifikasi.RingkasanKirim{Terkirim: 6, Dicatat: 6}, r)
		require.Len(t, email.got, 6)
		assert.Contains(t, email.got[0].Judul, "Lonjakan harga cabai")
		assert.Equal(t, "KRITIS", email.got[0].Severity)
		assert.Equal(t, "/ews?id="+email.got[0].AlertID.String(), email.got[0].TautanPath, "tautan memakai ID desimal alert")

		assert.Len(t, s.outbox(t, "WHERE kanal = 'EMAIL' AND status = 'TERKIRIM'"), 6)
		assert.Len(t, s.outbox(t, "WHERE kanal = 'PUSH' AND status = 'DICATAT'"), 6)
		var sent int
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM ews_notifikasi WHERE status = 'TERKIRIM' AND dikirim_pada IS NOT NULL AND percobaan = 1`).Scan(&sent).Error)
		assert.Equal(t, 6, sent)
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
	assert.Equal(t, notifikasi.RingkasanKirim{Diulang: 6}, r)
	rows := s.outbox(t, "")
	for _, b := range rows {
		assert.Equal(t, "MENUNGGU", b.Status)
		assert.Equal(t, 1, b.Percobaan)
	}
	var tertunda int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM ews_notifikasi WHERE jadwal_berikut > NOW() + INTERVAL '30 seconds'`).Scan(&tertunda).Error)
	assert.Equal(t, 6, tertunda, "percobaan berikutnya dijadwalkan nanti (backoff)")

	r, err = svc.Kirim(t.Context())
	require.NoError(t, err)
	assert.Equal(t, notifikasi.RingkasanKirim{}, r, "belum jatuh tempo")

	require.NoError(t, s.db.Exec(`UPDATE ews_notifikasi SET jadwal_berikut = NOW() - INTERVAL '1 second'`).Error)
	r, err = svc.Kirim(t.Context())
	require.NoError(t, err)
	assert.Equal(t, notifikasi.RingkasanKirim{Gagal: 6}, r, "percobaan kedua habis")
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
	require.Len(t, first, 6)
	second, err := repo.Klaim(t.Context(), time.Now(), 10, 5*time.Minute)
	require.NoError(t, err)
	assert.Empty(t, second, "baris yang sedang dikirim tidak diklaim lagi sebelum lease habis")

	crashed, err := repo.Klaim(t.Context(), time.Now().Add(6*time.Minute), 10, 5*time.Minute)
	require.NoError(t, err)
	assert.Len(t, crashed, 6, "pekerja yang mati tidak menghilangkan notifikasi: lease kedaluwarsa lalu diklaim ulang")
}

func TestNotifikasi_AutoEscalationOfUnacknowledgedAlerts(t *testing.T) {
	s := newSuite(t, "ews_notif_eskalasi")
	svc := s.layanan(t, notifikasi.Config{EskalasiSetelah: 10 * time.Minute})

	ids, err := svc.Eskalasi(t.Context())
	require.NoError(t, err)
	// Alert demo BARU >= SIAGA yang lebih tua dari 10 menit: EWS-1001 (27m), EWS-1002 (50m), EWS-1003 (73m).
	// EWS-1000 baru 4 menit; EWS-0902 WASPADA; surplus INFO.
	var kode []string
	require.NoError(t, s.db.Raw(`SELECT kode FROM ews_alert WHERE id IN ? ORDER BY id`, ids).Scan(&kode).Error)
	assert.Equal(t, []string{"EWS-1001", "EWS-1002", "EWS-1003"}, kode)

	var rows []struct {
		Kode, Status, Catatan string
		DieskalasiOlehID      *string
		Oleh                  *string
	}
	require.NoError(t, s.db.Raw(`SELECT a.kode, a.status, r.catatan, a.dieskalasi_oleh_id, r.oleh
		FROM ews_alert a JOIN ews_riwayat r ON r.alert_id = a.id AND r.catatan LIKE 'Eskalasi otomatis%'
		WHERE a.kode IN ('EWS-1001','EWS-1002','EWS-1003') ORDER BY a.id`).Scan(&rows).Error)
	require.Len(t, rows, 3)
	// EWS-1001 KRITIS: ditandai dan tercatat, tetapi tetap BARU sampai ditindaklanjuti manusia.
	assert.Equal(t, "BARU", rows[0].Status)
	assert.Equal(t, "Eskalasi otomatis: tidak diakui dalam 10 menit; tetap menunggu tindak lanjut", rows[0].Catatan)
	for _, r := range rows[1:] {
		assert.Equal(t, "DIESKALASI", r.Status)
		assert.Equal(t, "Eskalasi otomatis: tidak diakui dalam 10 menit", r.Catatan)
	}
	for _, r := range rows {
		assert.Nil(t, r.Oleh, "dilakukan sistem")
		assert.Nil(t, r.DieskalasiOlehID)
	}
	var ditandai *time.Time
	require.NoError(t, s.db.Raw(`SELECT dieskalasi_pada FROM ews_alert WHERE kode = 'EWS-1001'`).Scan(&ditandai).Error)
	assert.NotNil(t, ditandai, "penanda eskalasi tetap ada")
	var status string
	require.NoError(t, s.db.Raw(`SELECT status FROM ews_alert WHERE kode = 'EWS-1000'`).Scan(&status).Error)
	assert.Equal(t, "BARU", status)

	var audited []string
	require.NoError(t, s.db.Raw(`SELECT entitas_id FROM audit_log WHERE aksi = 'ESKALASI_OTOMATIS' ORDER BY entitas_id::bigint`).Scan(&audited).Error)
	assert.Len(t, audited, 3)
	for i, id := range ids {
		assert.Equal(t, id.String(), audited[i], "audit mencatat ID alert desimal")
	}

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
