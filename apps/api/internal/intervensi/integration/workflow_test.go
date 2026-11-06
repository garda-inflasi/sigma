package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	"github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/internal/intervensi/workflow"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWorkflow memasang workflow pada rule engine EWS milik suite, seperti di cmd/api.
func (s *suite) newWorkflow(t *testing.T, cfg workflow.Config) (*workflow.Service, *ewsjobs.Jobs) {
	t.Helper()
	log := logger.NewDefault()
	wf := workflow.New(postgres.New(s.db), audit.NewWriter(s.db), cfg, log)
	ews := ewsjobs.Shared(s.db, log)
	ews.Engine.AddObserver(wf)
	return wf, ews
}

func (s *suite) hargaNaik(t *testing.T, id, komoditas, pasar string, dasar, kini int) {
	t.Helper()
	now := time.Now()
	for i, row := range []struct {
		harga int
		kapan time.Time
	}{{dasar, now.Add(-72 * time.Hour)}, {kini, now.Add(-2 * time.Hour)}} {
		require.NoError(t, s.db.Exec(`INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
			VALUES (?, ?, ?, 'KONSUMEN', ?, 'kg', 'DIVALIDASI', ?)`, id+string(rune('a'+i)), komoditas, pasar, row.harga, row.kapan).Error)
	}
}

func TestWorkflow_AlertBecomesAutomaticCardOnce(t *testing.T) {
	s := newSuite(t, "intervensi_wf_alert")
	_, ews := s.newWorkflow(t, workflow.Config{KartuOtomatisMin: ewsdomain.SeveritySiaga})
	s.hargaNaik(t, "HC", "CBM-KRT", "PSR-1171-01", 50000, 60000) // +20% = KRITIS
	s.hargaNaik(t, "HB", "BWM", "PSR-1107-01", 50000, 54500)     // +9% = WASPADA, di bawah minimum

	ringkasan, err := ews.Engine.Evaluasi(t.Context())
	require.NoError(t, err)
	require.Equal(t, 2, ringkasan.Dibuat)

	cards := listCards(t, s, "")
	require.Len(t, cards, 20, "19 kartu demo + 1 kartu otomatis")
	var auto kartuRow
	for _, k := range cards {
		if k.ID == "INT-1000" {
			auto = k
		}
	}
	require.NotEmpty(t, auto.ID)
	assert.Equal(t, "TEKANAN", auto.Siklus)
	assert.Equal(t, "t-ews", auto.Tahap)
	assert.Equal(t, "TINGGI", auto.Prioritas)
	assert.Equal(t, "CBM-KRT", auto.KomoditasKode)
	assert.Equal(t, "1171", auto.KabupatenKode)
	assert.False(t, auto.RekomendasiAI, "dibuat aturan, bukan model AI")
	require.NotNil(t, auto.EwsAlertID)
	assert.Contains(t, *auto.EwsAlertID, "EWS-2")
	require.Len(t, auto.PenanggungJawab, 1, "ditugasi otomatis ke unit manajemen program")
	assert.Equal(t, uid("manajer_program"), auto.PenanggungJawab[0].ID)

	var row struct {
		DibuatOlehID *string
		Judul        string
		Tenggat      time.Time
	}
	require.NoError(t, s.db.Raw(`SELECT dibuat_oleh_id, judul, tenggat FROM intervensi WHERE id = 'INT-1000'`).Scan(&row).Error)
	assert.Nil(t, row.DibuatOlehID, "kartu buatan sistem")
	assert.Equal(t, "Lonjakan harga cabai merah keriting — Kota Banda Aceh", row.Judul)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), row.Tenggat, time.Minute, "tenggat = SLA tahap pertama (1 hari)")

	rows := riwayat(t, s, "INT-1000")
	require.Len(t, rows, 1)
	assert.Equal(t, "BUAT", rows[0].Aksi)
	assert.Nil(t, rows[0].OlehID, "pelaku sistem")

	var audited int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'BUAT_OTOMATIS' AND entitas = 'intervensi'`).Scan(&audited).Error)
	assert.Equal(t, 1, audited)

	t.Run("evaluasi ulang tidak menggandakan kartu", func(t *testing.T) {
		_, err := ews.Engine.Evaluasi(t.Context())
		require.NoError(t, err)
		assert.Len(t, listCards(t, s, ""), 20)
	})

	t.Run("alert yang naik ke tingkat minimum memicu kartu", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 56500 WHERE id = 'HBb'`).Error) // +13% = SIAGA
		ringkasan, err := ews.Engine.Evaluasi(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, ringkasan.Dinaikkan)

		cards := listCards(t, s, "")
		require.Len(t, cards, 21)
		var bawang kartuRow
		for _, k := range cards {
			if k.KomoditasKode == "BWM" && k.KabupatenKode == "1107" && k.ID >= "INT-1000" {
				bawang = k
			}
		}
		assert.Equal(t, "SEDANG", bawang.Prioritas, "SIAGA = prioritas sedang")
	})
}

func TestWorkflow_AlertAlreadyHavingAManualCardGetsNoSecondOne(t *testing.T) {
	s := newSuite(t, "intervensi_wf_kartu_manual")
	ews := ewsjobs.Shared(s.db, logger.NewDefault())
	s.hargaNaik(t, "HB", "BWM", "PSR-1107-01", 50000, 54500) // +9% = WASPADA: alert tanpa kartu otomatis
	_, err := ews.Engine.Evaluasi(t.Context())
	require.NoError(t, err)
	var alertID string
	require.NoError(t, s.db.Raw(`SELECT id FROM ews_alert WHERE dedup_key = 'LONJAKAN_HARGA|BWM|1107'`).Scan(&alertID).Error)

	// Pengguna membuat kartu sendiri untuk alert itu.
	w := s.call(t, http.MethodPost, "/v1/intervensi", emailKelola, map[string]any{
		"judul": "Tindak lanjut manual", "siklus": "TEKANAN", "prioritas": "SEDANG", "komoditas_kode": "BWM", "kabupaten_kode": "1107",
		"tenggat": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339), "ews_alert_id": alertID,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Workflow baru dipasang; alert naik ke SIAGA, tetapi kartunya sudah ada.
	s.newWorkflow(t, workflow.Config{KartuOtomatisMin: ewsdomain.SeveritySiaga})
	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 56500 WHERE id = 'HBb'`).Error)
	ringkasan, err := ews.Engine.Evaluasi(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ringkasan.Dinaikkan)

	assert.Len(t, listCards(t, s, ""), 20, "19 demo + kartu manual; tidak ada kartu otomatis kedua")
}

func TestWorkflow_DisabledByConfigCreatesNoCards(t *testing.T) {
	s := newSuite(t, "intervensi_wf_mati")
	_, ews := s.newWorkflow(t, workflow.Config{})
	s.hargaNaik(t, "HC", "CBM-KRT", "PSR-1171-01", 50000, 60000)

	_, err := ews.Engine.Evaluasi(t.Context())

	require.NoError(t, err)
	assert.Len(t, listCards(t, s, ""), 19)
}

func TestWorkflow_SLAEscalatesOverdueCardsOnce(t *testing.T) {
	s := newSuite(t, "intervensi_wf_sla")
	wf, _ := s.newWorkflow(t, workflow.Config{SLAInterval: time.Hour})

	// Kartu yang baru berpindah tahap tidak melewati SLA; pemindahan menyetel ulang jam tahap.
	require.Equal(t, http.StatusOK, s.pindah(t, emailKelola, "INT-302", "t-verifikasi", 0, 1).Code)
	var sejak time.Time
	require.NoError(t, s.db.Raw(`SELECT tahap_sejak FROM intervensi WHERE id = 'INT-302'`).Scan(&sejak).Error)
	assert.WithinDuration(t, time.Now(), sejak, time.Minute)
	require.NoError(t, s.db.Exec(`UPDATE intervensi SET tahap_sejak = NOW() WHERE id = 'INT-303'`).Error)

	n, err := wf.CekSLA(t.Context())
	require.NoError(t, err)
	// 19 kartu demo: 2 kartu selesai dikecualikan, INT-302 dan INT-303 baru pindah/disetel ulang.
	assert.Equal(t, 15, n)

	var status struct {
		DieskalasiPada *time.Time
		DieskalasiOleh *string
		Prioritas      string
	}
	require.NoError(t, s.db.Raw(`SELECT dieskalasi_pada, dieskalasi_oleh_id AS dieskalasi_oleh, prioritas FROM intervensi WHERE id = 'INT-301'`).Scan(&status).Error)
	assert.NotNil(t, status.DieskalasiPada)
	assert.Nil(t, status.DieskalasiOleh, "dieskalasi oleh sistem")
	assert.Equal(t, "TINGGI", status.Prioritas)
	assert.Nil(t, card(t, s, "INT-302").DieskalasiPada)
	assert.Nil(t, card(t, s, "INT-303").DieskalasiPada)
	assert.Nil(t, card(t, s, "INT-319").DieskalasiPada, "kartu selesai tidak dieskalasi")

	rows := riwayat(t, s, "INT-301")
	last := rows[len(rows)-1]
	assert.Equal(t, "AJUKAN", last.Aksi, "eskalasi membuka telaah Dewan Pengarah")
	assert.Equal(t, "ESKALASI", rows[len(rows)-2].Aksi)

	var alasan string
	require.NoError(t, s.db.Raw(`SELECT alasan FROM intervensi_riwayat WHERE intervensi_id = 'INT-301' AND aksi = 'ESKALASI'`).Scan(&alasan).Error)
	assert.Equal(t, `Melewati SLA tahap "EWS terpicu" (1 hari)`, alasan)

	// Telaah eskalasi muncul di antrean persetujuan; INT-304 sudah punya permintaan terbuka, jadi tidak ditambah.
	var eskalasi int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM intervensi_persetujuan WHERE jenis = 'ESKALASI' AND status = 'MENUNGGU'`).Scan(&eskalasi).Error)
	assert.Equal(t, 14, eskalasi)

	again, err := wf.CekSLA(t.Context())
	require.NoError(t, err)
	assert.Zero(t, again, "idempoten: kartu yang sudah dieskalasi tidak dieskalasi lagi")

	var audited int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'ESKALASI_SLA'`).Scan(&audited).Error)
	assert.Equal(t, 15, audited)
}
