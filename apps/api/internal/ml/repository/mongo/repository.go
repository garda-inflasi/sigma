// Package mongo adalah repository BACA hasil pipeline di MongoDB (dataunseen dan datasafe). Paket ini sengaja hanya
// memakai Find, FindOne, Aggregate, CountDocuments, dan Ping (dijaga tes): API tidak pernah menulis ke Mongo.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/pkg/platform/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	drv "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/sync/errgroup"
)

// Nama koleksi (docs/PIPELINE-DATA-ML.md §2-3).
const (
	KoleksiSiklus    = "ml_training_cycles"
	KoleksiInflasi   = "datasafe_inflasi_signals"
	KoleksiForecast  = "datasafe_forecasts"
	KoleksiXai       = "datasafe_xai_explanations"
	KoleksiEws       = "datasafe_ews_signals"
	KoleksiMakro     = "datasafe_macro_indicators"
	KoleksiKlaster   = "datasafe_region_clusters"
	KoleksiCrawlRuns = "crawl_runs"
)

// maksDokumen membatasi satu bacaan koleksi hasil per siklus (pagar memori; satu siklus Aceh jauh di bawahnya).
const maksDokumen = 20000

// proyeksiSiklus membuang bagian metrik yang besar dan tidak dipakai pembaca (parameter model, rincian kegagalan).
var proyeksiSiklus = bson.D{
	{Key: "model_metrics.regresi.params", Value: 0},
	{Key: "model_metrics.kegagalan", Value: 0},
	{Key: "model_metrics.durasi_tahap_ms", Value: 0},
}

type Repository struct {
	unseen *drv.Database
	safe   *drv.Database
}

var _ domain.Repository = (*Repository)(nil)

// New membuat repository dari klien platform.
func New(k *mongodb.Klien) *Repository {
	return &Repository{unseen: k.Unseen(), safe: k.Safe()}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.safe.Client().Ping(ctx, nil)
}

func (r *Repository) SiklusTerbaru(ctx context.Context) (*domain.Siklus, error) {
	var s domain.Siklus
	err := r.safe.Collection(KoleksiSiklus).FindOne(ctx,
		bson.D{{Key: "status", Value: bson.D{{Key: "$in", Value: domain.StatusFinal}}}},
		options.FindOne().SetSort(bson.D{{Key: "cycle_timestamp", Value: -1}}).SetProjection(proyeksiSiklus),
	).Decode(&s)
	if errors.Is(err, drv.ErrNoDocuments) {
		return nil, domain.ErrTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("siklus terbaru: %w", err)
	}
	return &s, nil
}

func (r *Repository) Siklus(ctx context.Context, cycleID string) (*domain.Siklus, error) {
	var s domain.Siklus
	err := r.safe.Collection(KoleksiSiklus).FindOne(ctx, bson.D{{Key: "_id", Value: cycleID}},
		options.FindOne().SetProjection(proyeksiSiklus)).Decode(&s)
	if errors.Is(err, drv.ErrNoDocuments) {
		return nil, domain.ErrTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("siklus %s: %w", cycleID, err)
	}
	return &s, nil
}

func (r *Repository) ListSiklus(ctx context.Context, q domain.ListSiklusQuery) ([]domain.Siklus, int64, error) {
	filter := bson.D{}
	if len(q.Status) > 0 {
		filter = append(filter, bson.E{Key: "status", Value: bson.D{{Key: "$in", Value: q.Status}}})
	}
	rentang := bson.D{}
	if q.Dari != nil {
		rentang = append(rentang, bson.E{Key: "$gte", Value: *q.Dari})
	}
	if q.Sampai != nil {
		rentang = append(rentang, bson.E{Key: "$lt", Value: *q.Sampai})
	}
	if len(rentang) > 0 {
		filter = append(filter, bson.E{Key: "cycle_timestamp", Value: rentang})
	}
	koleksi := r.safe.Collection(KoleksiSiklus)
	total, err := koleksi.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("hitung siklus: %w", err)
	}
	cur, err := koleksi.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "cycle_timestamp", Value: -1}}).
		SetSkip(int64((q.Page-1)*q.Limit)).SetLimit(int64(q.Limit)).
		SetProjection(append(bson.D{{Key: "source_freshness", Value: 0}}, proyeksiSiklus...)))
	if err != nil {
		return nil, 0, fmt.Errorf("daftar siklus: %w", err)
	}
	items := []domain.Siklus{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, fmt.Errorf("baca daftar siklus: %w", err)
	}
	return items, total, nil
}

// semua membaca seluruh dokumen satu siklus dari koleksi ke dst.
func (r *Repository) semua(ctx context.Context, koleksi, cycleID string, proyeksi bson.D, dst any) error {
	opts := options.Find().SetLimit(maksDokumen)
	if proyeksi != nil {
		opts.SetProjection(proyeksi)
	}
	cur, err := r.safe.Collection(koleksi).Find(ctx, bson.D{{Key: "cycle_id", Value: cycleID}}, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", koleksi, err)
	}
	if err := cur.All(ctx, dst); err != nil {
		return fmt.Errorf("baca %s: %w", koleksi, err)
	}
	return nil
}

func (r *Repository) Snapshot(ctx context.Context, s domain.Siklus) (*domain.Snapshot, error) {
	snap := &domain.Snapshot{
		Siklus: s, Inflasi: []domain.SinyalInflasi{}, Forecast: []domain.Forecast{}, Xai: []domain.Xai{},
		Ews: []domain.SinyalEws{}, Makro: []domain.Makro{}, Klaster: []domain.KlasterWilayah{},
	}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return r.semua(gctx, KoleksiInflasi, s.CycleID, nil, &snap.Inflasi) })
	g.Go(func() error {
		return r.semua(gctx, KoleksiForecast, s.CycleID, bson.D{{Key: "riwayat", Value: 0}}, &snap.Forecast)
	})
	g.Go(func() error { return r.semua(gctx, KoleksiXai, s.CycleID, nil, &snap.Xai) })
	g.Go(func() error { return r.semua(gctx, KoleksiEws, s.CycleID, nil, &snap.Ews) })
	g.Go(func() error { return r.semua(gctx, KoleksiMakro, s.CycleID, nil, &snap.Makro) })
	g.Go(func() error { return r.semua(gctx, KoleksiKlaster, s.CycleID, nil, &snap.Klaster) })
	if err := g.Wait(); err != nil {
		return nil, err
	}
	urutkan(snap)
	return snap, nil
}

// urutkan memberi urutan deterministik (Mongo tidak menjamin urutan alami).
func urutkan(s *domain.Snapshot) {
	sort.SliceStable(s.Inflasi, func(i, j int) bool { return s.Inflasi[i].WilayahKode < s.Inflasi[j].WilayahKode })
	sort.SliceStable(s.Forecast, func(i, j int) bool {
		a, b := s.Forecast[i], s.Forecast[j]
		if a.KabupatenKode != b.KabupatenKode {
			return a.KabupatenKode < b.KabupatenKode
		}
		return a.KomoditasKode < b.KomoditasKode
	})
	sort.SliceStable(s.Ews, func(i, j int) bool { return s.Ews[i].SinyalKunci < s.Ews[j].SinyalKunci })
	sort.SliceStable(s.Makro, func(i, j int) bool {
		a, b := s.Makro[i], s.Makro[j]
		if a.Domain != b.Domain {
			return a.Domain < b.Domain
		}
		return a.Indikator < b.Indikator
	})
	sort.SliceStable(s.Klaster, func(i, j int) bool { return s.Klaster[i].RegencyCode < s.Klaster[j].RegencyCode })
}

func (r *Repository) Forecast(ctx context.Context, cycleID, komoditasKode, kabupatenKode string) (*domain.Forecast, error) {
	var f domain.Forecast
	err := r.safe.Collection(KoleksiForecast).FindOne(ctx, bson.D{
		{Key: "cycle_id", Value: cycleID}, {Key: "komoditas_kode", Value: komoditasKode}, {Key: "kabupaten_kode", Value: kabupatenKode},
	}).Decode(&f)
	if errors.Is(err, drv.ErrNoDocuments) {
		return nil, domain.ErrTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("forecast %s %s %s: %w", cycleID, komoditasKode, kabupatenKode, err)
	}
	return &f, nil
}

// pipaRunSumber mengambil run terakhir per (sumber, dataset, backfill?) menurut waktu mulai.
var pipaRunSumber = drv.Pipeline{
	{{Key: "$sort", Value: bson.D{{Key: "mulai", Value: -1}}}},
	{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: bson.D{
			{Key: "sumber", Value: "$sumber"}, {Key: "dataset", Value: "$dataset"},
			{Key: "backfill", Value: bson.D{{Key: "$eq", Value: bson.A{"$pemicu", domain.PemicuBackfill}}}},
		}},
		{Key: "run", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
	}}},
}

func (r *Repository) RunSumber(ctx context.Context) ([]domain.RunSumber, error) {
	type runRow struct {
		ID struct {
			Sumber   string `bson:"sumber"`
			Dataset  string `bson:"dataset"`
			Backfill bool   `bson:"backfill"`
		} `bson:"_id"`
		Run domain.CrawlRun `bson:"run"`
	}
	var rows []runRow
	if r.safe != nil {
		cur, err := r.safe.Collection(KoleksiCrawlRuns).Aggregate(ctx, pipaRunSumber)
		if err != nil {
			return nil, fmt.Errorf("run crawl terakhir: %w", err)
		}
		if err := cur.All(ctx, &rows); err != nil {
			return nil, fmt.Errorf("baca run crawl terakhir: %w", err)
		}
	}
	if len(rows) == 0 && r.unseen != nil {
		cur, err := r.unseen.Collection(KoleksiCrawlRuns).Aggregate(ctx, pipaRunSumber)
		if err != nil {
			return nil, fmt.Errorf("run crawl terakhir: %w", err)
		}
		if err := cur.All(ctx, &rows); err != nil {
			return nil, fmt.Errorf("baca run crawl terakhir: %w", err)
		}
	}
	per := map[[2]string]*domain.RunSumber{}
	for i := range rows {
		row := rows[i]
		k := [2]string{row.ID.Sumber, row.ID.Dataset}
		rs, ok := per[k]
		if !ok {
			rs = &domain.RunSumber{Sumber: row.ID.Sumber, Dataset: row.ID.Dataset}
			per[k] = rs
		}
		run := row.Run
		if row.ID.Backfill {
			rs.Backfill = &run
		} else {
			rs.Terjadwal = &run
		}
	}
	out := make([]domain.RunSumber, 0, len(per))
	for _, rs := range per {
		out = append(out, *rs)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sumber != out[j].Sumber {
			return out[i].Sumber < out[j].Sumber
		}
		return out[i].Dataset < out[j].Dataset
	})
	return out, nil
}
