// Package orchestrator menjembatani API Go dengan worker Python crawler (:8091) dan ML pipeline (:8092)
// melalui HTTP internal serta menyediakan aliran status realtime (SSE) tanpa pernah menulis ke MongoDB dari Go.
package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/pkg/logger"
)

const (
	EnvCrawlerURL  = "CRAWLER_SERVICE_URL"
	EnvPipelineURL = "ML_PIPELINE_SERVICE_URL"

	timeoutHTTP    = 3 * time.Second
	intervalPoll   = 800 * time.Millisecond
	batasTungguJob = 20 * time.Minute
	maksLogChain   = 50
)

var (
	ErrTidakDikonfigurasi = errors.New("layanan kontrol worker belum dikonfigurasi")
	ErrSedangBerjalan     = errors.New("job worker sedang berjalan")
	ErrTidakTerjangkau    = errors.New("layanan worker tidak terjangkau")
)

type WorkerStatus struct {
	Configured     bool                   `json:"configured"`
	Reachable      bool                   `json:"reachable"`
	Berjalan       bool                   `json:"berjalan"`
	JobID          string                 `json:"job_id,omitempty"`
	CycleID        string                 `json:"cycle_id,omitempty"`
	Sumber         []string               `json:"sumber,omitempty"`
	GagalSaja      bool                   `json:"gagal_saja,omitempty"`
	Tahap          string                 `json:"tahap,omitempty"`
	ProgressPct    int                    `json:"progress_pct"`
	DatasetAktif   string                 `json:"dataset_aktif,omitempty"`
	SelesaiDataset int                    `json:"selesai_dataset,omitempty"`
	TotalDataset   int                    `json:"total_dataset,omitempty"`
	Mulai          string                 `json:"mulai,omitempty"`
	Selesai        string                 `json:"selesai,omitempty"`
	DurasiMs       *int64                 `json:"durasi_ms,omitempty"`
	Logs           []string               `json:"logs"`
	Hasil          interface{}            `json:"hasil,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Jadwal         map[string]interface{} `json:"jadwal,omitempty"`
	SlotTerakhir   map[string]interface{} `json:"slot_terakhir,omitempty"`
}

type ChainState struct {
	Aktif         bool     `json:"aktif"`
	ChainID       string   `json:"chain_id,omitempty"`
	Tahap         string   `json:"tahap"`
	ProgressPct   int      `json:"progress_pct"`
	Pesan         string   `json:"pesan,omitempty"`
	Mulai         string   `json:"mulai,omitempty"`
	Selesai       string   `json:"selesai,omitempty"`
	Error         string   `json:"error,omitempty"`
	CrawlerJobID  string   `json:"crawler_job_id,omitempty"`
	PipelineJobID string   `json:"pipeline_job_id,omitempty"`
	Logs          []string `json:"logs"`
}

type Snapshot struct {
	Timestamp string       `json:"timestamp"`
	Crawler   WorkerStatus `json:"crawler"`
	Pipeline  WorkerStatus `json:"pipeline"`
	Chain     ChainState   `json:"chain"`
}

type TriggerCrawlerRequest struct {
	Sumber    []string `json:"sumber,omitempty"`
	GagalSaja bool     `json:"gagal_saja,omitempty"`
	LanjutML  bool     `json:"lanjut_ml,omitempty"`
}

type Orchestrator struct {
	crawlerURL  string
	pipelineURL string
	client      *http.Client
	log         *logger.Logger

	mu    sync.RWMutex
	chain ChainState
	subs  map[chan Snapshot]struct{}

	watchMu      sync.Mutex
	watchRunning map[string]bool
}

var (
	sharedOnce sync.Once
	sharedOrch *Orchestrator
)

// Shared mengembalikan singleton Orchestrator per proses dari environment.
func Shared(log *logger.Logger) *Orchestrator {
	sharedOnce.Do(func() { sharedOrch = DariEnv(os.Getenv, log) })
	return sharedOrch
}

// DariEnv membuat Orchestrator baru dari fungsi lookup environment.
func DariEnv(getenv func(string) string, log *logger.Logger) *Orchestrator {
	cURL := ""
	pURL := ""
	if getenv != nil {
		cURL = strings.TrimSpace(getenv(EnvCrawlerURL))
		pURL = strings.TrimSpace(getenv(EnvPipelineURL))
	}
	return New(cURL, pURL, nil, log)
}

func New(crawlerURL, pipelineURL string, client *http.Client, log *logger.Logger) *Orchestrator {
	if client == nil {
		client = &http.Client{Timeout: timeoutHTTP}
	}
	return &Orchestrator{
		crawlerURL:   strings.TrimRight(strings.TrimSpace(crawlerURL), "/"),
		pipelineURL:  strings.TrimRight(strings.TrimSpace(pipelineURL), "/"),
		client:       client,
		log:          log,
		chain:        ChainState{Tahap: "SIAGA", Logs: []string{}},
		subs:         make(map[chan Snapshot]struct{}),
		watchRunning: make(map[string]bool),
	}
}

func (o *Orchestrator) CrawlerConfigured() bool  { return o.crawlerURL != "" }
func (o *Orchestrator) PipelineConfigured() bool { return o.pipelineURL != "" }

func (o *Orchestrator) Subscribe() (chan Snapshot, func()) {
	ch := make(chan Snapshot, 8)
	o.mu.Lock()
	o.subs[ch] = struct{}{}
	o.mu.Unlock()
	return ch, func() {
		o.mu.Lock()
		if _, ok := o.subs[ch]; ok {
			delete(o.subs, ch)
			close(ch)
		}
		o.mu.Unlock()
	}
}

func (o *Orchestrator) broadcast(s Snapshot) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for ch := range o.subs {
		select {
		case ch <- s:
		default:
			// Klien lambat dilewati tanpa memblokir ticker.
		}
	}
}

func (o *Orchestrator) getWorkerStatus(ctx context.Context, baseURL string) WorkerStatus {
	if baseURL == "" {
		return WorkerStatus{
			Configured: false,
			Reachable:  false,
			Tahap:      "TIDAK_TERKONFIGURASI",
			Logs:       []string{},
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeoutHTTP)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/status", nil)
	if err != nil {
		return WorkerStatus{Configured: true, Reachable: false, Tahap: "GALAT_URL", Error: err.Error(), Logs: []string{}}
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return WorkerStatus{Configured: true, Reachable: false, Tahap: "TIDAK_TERJANGKAU", Error: err.Error(), Logs: []string{}}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return WorkerStatus{
			Configured: true,
			Reachable:  false,
			Tahap:      "GALAT_HTTP",
			Error:      fmt.Sprintf("HTTP %d", resp.StatusCode),
			Logs:       []string{},
		}
	}
	var st WorkerStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&st); err != nil {
		return WorkerStatus{Configured: true, Reachable: false, Tahap: "GALAT_DECODE", Error: err.Error(), Logs: []string{}}
	}
	st.Configured = true
	st.Reachable = true
	if st.Logs == nil {
		st.Logs = []string{}
	}
	return st
}

// Status membaca status terkini worker crawler dan ML pipeline secara paralel serta menggabungkannya dengan state chain.
func (o *Orchestrator) Status(ctx context.Context) Snapshot {
	var wg sync.WaitGroup
	var cStat, pStat WorkerStatus
	wg.Add(2)
	go func() {
		defer wg.Done()
		cStat = o.getWorkerStatus(ctx, o.crawlerURL)
	}()
	go func() {
		defer wg.Done()
		pStat = o.getWorkerStatus(ctx, o.pipelineURL)
	}()
	wg.Wait()

	o.mu.RLock()
	ch := o.chain
	logsCopy := append([]string(nil), ch.Logs...)
	o.mu.RUnlock()
	ch.Logs = logsCopy

	return Snapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Crawler:   cStat,
		Pipeline:  pStat,
		Chain:     ch,
	}
}

func (o *Orchestrator) postTrigger(ctx context.Context, baseURL string, payload interface{}) (int, WorkerStatus, error) {
	if baseURL == "" {
		return http.StatusServiceUnavailable, WorkerStatus{}, ErrTidakDikonfigurasi
	}
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return http.StatusBadRequest, WorkerStatus{}, err
		}
		bodyReader = bytes.NewReader(raw)
	} else {
		bodyReader = bytes.NewReader([]byte("{}"))
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/run", bodyReader)
	if err != nil {
		return http.StatusBadGateway, WorkerStatus{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return http.StatusServiceUnavailable, WorkerStatus{}, fmt.Errorf("%w: %v", ErrTidakTerjangkau, err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var parsed struct {
		Status string       `json:"status"`
		Code   string       `json:"code"`
		Error  string       `json:"error"`
		State  WorkerStatus `json:"state"`
	}
	_ = json.Unmarshal(rawResp, &parsed)
	if parsed.State.Logs == nil {
		parsed.State.Logs = []string{}
	}
	parsed.State.Configured = true
	parsed.State.Reachable = true

	if resp.StatusCode == http.StatusConflict {
		return http.StatusConflict, parsed.State, ErrSedangBerjalan
	}
	if resp.StatusCode >= 400 {
		msg := parsed.Error
		if msg == "" {
			msg = fmt.Sprintf("worker menjawab HTTP %d", resp.StatusCode)
		}
		return resp.StatusCode, parsed.State, errors.New(msg)
	}
	return http.StatusAccepted, parsed.State, nil
}

// TriggerCrawler memicu eksekusi crawler (manual retry atau sumber tertentu) serta opsional merangkai eksekusi ML.
func (o *Orchestrator) TriggerCrawler(ctx context.Context, req TriggerCrawlerRequest) (Snapshot, int, error) {
	if !o.CrawlerConfigured() {
		return o.Status(ctx), http.StatusServiceUnavailable, ErrTidakDikonfigurasi
	}
	if req.LanjutML && !o.PipelineConfigured() {
		return o.Status(ctx), http.StatusServiceUnavailable, fmt.Errorf("%w: %s kosong", ErrTidakDikonfigurasi, EnvPipelineURL)
	}

	o.mu.RLock()
	chainAktif := o.chain.Aktif
	o.mu.RUnlock()
	if chainAktif {
		return o.Status(ctx), http.StatusConflict, ErrSedangBerjalan
	}

	code, cState, err := o.postTrigger(ctx, o.crawlerURL, map[string]interface{}{
		"sumber":     req.Sumber,
		"gagal_saja": req.GagalSaja,
	})
	if err != nil {
		return o.Status(ctx), code, err
	}

	if req.LanjutML {
		chainID := fmt.Sprintf("CHN-%d", time.Now().UTC().UnixNano())
		o.mu.Lock()
		o.chain = ChainState{
			Aktif:        true,
			ChainID:      chainID,
			Tahap:        "CRAWLER",
			ProgressPct:  5,
			Pesan:        "Menjalankan crawler terlebih dahulu sebelum training ML...",
			Mulai:        time.Now().UTC().Format(time.RFC3339),
			CrawlerJobID: cState.JobID,
			Logs:         []string{fmt.Sprintf("[%s UTC] Rantai Crawler -> ML dimulai (%s)", time.Now().UTC().Format("15:04:05"), cState.JobID)},
		}
		o.mu.Unlock()
		go o.jalankanPemantauRantai(chainID, cState.JobID)
	} else {
		go o.pantauWorkerTunggal("crawler")
	}

	snap := o.Status(ctx)
	o.broadcast(snap)
	return snap, http.StatusAccepted, nil
}

// TriggerPipeline memicu ulang training ML worker tanpa menjalankan crawler.
func (o *Orchestrator) TriggerPipeline(ctx context.Context) (Snapshot, int, error) {
	if !o.PipelineConfigured() {
		return o.Status(ctx), http.StatusServiceUnavailable, ErrTidakDikonfigurasi
	}
	code, _, err := o.postTrigger(ctx, o.pipelineURL, nil)
	if err != nil {
		return o.Status(ctx), code, err
	}
	go o.pantauWorkerTunggal("pipeline")
	snap := o.Status(ctx)
	o.broadcast(snap)
	return snap, http.StatusAccepted, nil
}

func (o *Orchestrator) pantauWorkerTunggal(nama string) {
	o.watchMu.Lock()
	if o.watchRunning[nama] {
		o.watchMu.Unlock()
		return
	}
	o.watchRunning[nama] = true
	o.watchMu.Unlock()
	defer func() {
		o.watchMu.Lock()
		delete(o.watchRunning, nama)
		o.watchMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), batasTungguJob)
	defer cancel()
	ticker := time.NewTicker(intervalPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := o.Status(ctx)
			o.broadcast(snap)
			aktif := (nama == "crawler" && snap.Crawler.Berjalan) || (nama == "pipeline" && snap.Pipeline.Berjalan)
			if !aktif {
				return
			}
		}
	}
}

func (o *Orchestrator) catatChain(tahap string, pct int, pesan string, selesai bool, errMsg, pipelineJobID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.chain.Tahap = tahap
	o.chain.ProgressPct = pct
	o.chain.Pesan = pesan
	if pipelineJobID != "" {
		o.chain.PipelineJobID = pipelineJobID
	}
	if errMsg != "" {
		o.chain.Error = errMsg
	}
	if selesai {
		o.chain.Aktif = false
		o.chain.Selesai = time.Now().UTC().Format(time.RFC3339)
	}
	baris := fmt.Sprintf("[%s UTC] %s", time.Now().UTC().Format("15:04:05"), pesan)
	o.chain.Logs = append(o.chain.Logs, baris)
	if len(o.chain.Logs) > maksLogChain {
		o.chain.Logs = o.chain.Logs[len(o.chain.Logs)-maksLogChain:]
	}
}

func (o *Orchestrator) jalankanPemantauRantai(chainID, crawlerJobID string) {
	ctx, cancel := context.WithTimeout(context.Background(), batasTungguJob)
	defer cancel()

	ticker := time.NewTicker(intervalPoll)
	defer ticker.Stop()

	// Fase 1: tunggu crawler selesai
	for {
		select {
		case <-ctx.Done():
			o.catatChain("GAGAL", 100, "Batas waktu pemantauan rantai Crawler -> ML terlampaui.", true, "timeout", "")
			o.broadcast(o.Status(context.Background()))
			return
		case <-ticker.C:
			cStat := o.getWorkerStatus(ctx, o.crawlerURL)
			if cStat.Berjalan {
				pct := 5 + int(float64(cStat.ProgressPct)*0.4)
				o.catatChain("CRAWLER", pct, fmt.Sprintf("Crawler berjalan (%s: %d%%)", cStat.Tahap, cStat.ProgressPct), false, "", "")
				o.broadcast(o.Status(ctx))
				continue
			}
			if cStat.Error != "" || cStat.Tahap == "GALAT" {
				msg := cStat.Error
				if msg == "" {
					msg = "Crawler berakhir dengan galat"
				}
				o.catatChain("GAGAL", 100, "Crawler gagal sehingga training ML dibatalkan: "+msg, true, msg, "")
				o.broadcast(o.Status(context.Background()))
				return
			}
			goto triggerML
		}
	}

triggerML:
	o.catatChain("MENUNGGU_ML", 48, "Crawler selesai ("+crawlerJobID+"). Memicu ML pipeline...", false, "", "")
	o.broadcast(o.Status(ctx))

	_, pState, err := o.postTrigger(ctx, o.pipelineURL, nil)
	if err != nil {
		o.catatChain("GAGAL", 100, "Gagal memicu ML pipeline: "+err.Error(), true, err.Error(), "")
		o.broadcast(o.Status(context.Background()))
		return
	}
	o.catatChain("ML_PIPELINE", 52, "ML pipeline dimulai ("+pState.JobID+").", false, "", pState.JobID)
	o.broadcast(o.Status(ctx))

	// Fase 2: tunggu ML pipeline selesai
	for {
		select {
		case <-ctx.Done():
			o.catatChain("GAGAL", 100, "Batas waktu pemantauan ML pipeline terlampaui.", true, "timeout", pState.JobID)
			o.broadcast(o.Status(context.Background()))
			return
		case <-ticker.C:
			pStat := o.getWorkerStatus(ctx, o.pipelineURL)
			if pStat.Berjalan {
				pct := 50 + int(float64(pStat.ProgressPct)*0.48)
				o.catatChain("ML_PIPELINE", pct, fmt.Sprintf("ML training berjalan (%s: %d%%)", pStat.Tahap, pStat.ProgressPct), false, "", pStat.JobID)
				o.broadcast(o.Status(ctx))
				continue
			}
			if pStat.Error != "" || pStat.Tahap == "GAGAL" {
				msg := pStat.Error
				if msg == "" {
					msg = "ML pipeline berakhir dengan galat"
				}
				o.catatChain("GAGAL", 100, "ML pipeline gagal: "+msg, true, msg, pStat.JobID)
				o.broadcast(o.Status(context.Background()))
				return
			}
			o.catatChain("SELESAI", 100, "Rantai Crawler -> ML selesai. Hasil datasafe diperbarui.", true, "", pStat.JobID)
			o.broadcast(o.Status(context.Background()))
			if o.log != nil {
				o.log.Info("ml: rantai crawler -> pipeline selesai", map[string]interface{}{"chain_id": chainID, "cycle_id": pStat.CycleID})
			}
			return
		}
	}
}
