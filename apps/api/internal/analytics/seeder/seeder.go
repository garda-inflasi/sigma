package seeder

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/analytics/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SeedAll populates all MongoDB collections with sample data for development.
func SeedAll(ctx context.Context, client *mongo.Client, dbName string) error {
	log.Println("[seeder] Starting MongoDB seed...")

	if err := seedRegionMaster(ctx, client, dbName); err != nil {
		return fmt.Errorf("seed region_master: %w", err)
	}

	if err := seedCommodityDailyBuckets(ctx, client, dbName); err != nil {
		return fmt.Errorf("seed commodity_daily_buckets: %w", err)
	}

	if err := seedFiscalSpaceBuckets(ctx, client, dbName); err != nil {
		return fmt.Errorf("seed fiscal_space_buckets: %w", err)
	}

	if err := seedFieldEntries(ctx, client, dbName); err != nil {
		return fmt.Errorf("seed field_entries: %w", err)
	}

	if err := seedModelRegistry(ctx, client, dbName); err != nil {
		return fmt.Errorf("seed model_registry: %w", err)
	}

	if err := createIndexes(ctx, client, dbName); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	log.Println("[seeder] Seed completed successfully!")
	return nil
}

// ── Region Master ─────────────────────────────────────────────────────

var regions = []struct {
	RegionID  string
	Name      string
	Type      string
	Neighbors []string
	Long      float64
	Lat       float64
}{
	{RegionID: "aceh_province", Name: "Provinsi Aceh", Type: "Provinsi", Neighbors: []string{"aceh_besar", "aceh_jaya", "pidie"}, Long: 95.5, Lat: 4.5},
	{RegionID: "banda_aceh", Name: "Kota Banda Aceh", Type: "Kota", Neighbors: []string{"aceh_besar"}, Long: 95.32, Lat: 5.55},
	{RegionID: "aceh_besar", Name: "Kab. Aceh Besar", Type: "Kabupaten", Neighbors: []string{"banda_aceh", "aceh_jaya", "pidie"}, Long: 95.5, Lat: 5.3},
	{RegionID: "aceh_jaya", Name: "Kab. Aceh Jaya", Type: "Kabupaten", Neighbors: []string{"aceh_besar", "aceh_barat"}, Long: 95.2, Lat: 4.8},
	{RegionID: "aceh_barat", Name: "Kab. Aceh Barat", Type: "Kabupaten", Neighbors: []string{"aceh_jaya", "nagan_raya"}, Long: 96.0, Lat: 4.2},
	{RegionID: "nagan_raya", Name: "Kab. Nagan Raya", Type: "Kabupaten", Neighbors: []string{"aceh_barat", "aceh_barat_daya"}, Long: 96.3, Lat: 4.0},
	{RegionID: "aceh_barat_daya", Name: "Kab. Aceh Barat Daya", Type: "Kabupaten", Neighbors: []string{"nagan_raya", "aceh_selatan"}, Long: 96.5, Lat: 3.8},
	{RegionID: "aceh_selatan", Name: "Kab. Aceh Selatan", Type: "Kabupaten", Neighbors: []string{"aceh_barat_daya", "aceh_tenggara"}, Long: 97.0, Lat: 3.5},
	{RegionID: "aceh_tenggara", Name: "Kab. Aceh Tenggara", Type: "Kabupaten", Neighbors: []string{"aceh_selatan", "aceh_timur"}, Long: 97.5, Lat: 3.5},
	{RegionID: "aceh_timur", Name: "Kab. Aceh Timur", Type: "Kabupaten", Neighbors: []string{"aceh_tenggara", "aceh_utara"}, Long: 97.8, Lat: 4.5},
	{RegionID: "aceh_utara", Name: "Kab. Aceh Utara", Type: "Kabupaten", Neighbors: []string{"aceh_timur", "pidie"}, Long: 97.0, Lat: 5.0},
	{RegionID: "pidie", Name: "Kab. Pidie", Type: "Kabupaten", Neighbors: []string{"aceh_utara", "aceh_besar", "pidie_jaya"}, Long: 95.8, Lat: 5.2},
	{RegionID: "pidie_jaya", Name: "Kab. Pidie Jaya", Type: "Kabupaten", Neighbors: []string{"pidie", "bireuen"}, Long: 96.0, Lat: 5.1},
	{RegionID: "bireuen", Name: "Kab. Bireuen", Type: "Kabupaten", Neighbors: []string{"pidie_jaya", "aceh_tengah"}, Long: 96.5, Lat: 5.1},
	{RegionID: "aceh_tengah", Name: "Kab. Aceh Tengah", Type: "Kabupaten", Neighbors: []string{"bireuen", "aceh_timur", "gayo_lues"}, Long: 96.8, Lat: 4.5},
	{RegionID: "gayo_lues", Name: "Kab. Gayo Lues", Type: "Kabupaten", Neighbors: []string{"aceh_tengah", "aceh_tenggara"}, Long: 97.2, Lat: 4.0},
	{RegionID: "sabang", Name: "Kota Sabang", Type: "Kota", Neighbors: []string{"aceh_besar"}, Long: 95.3, Lat: 5.9},
	{RegionID: "langsa", Name: "Kota Langsa", Type: "Kota", Neighbors: []string{"aceh_timur"}, Long: 97.9, Lat: 4.5},
	{RegionID: "lhokseumawe", Name: "Kota Lhokseumawe", Type: "Kota", Neighbors: []string{"aceh_utara"}, Long: 97.1, Lat: 5.1},
	{RegionID: "subulussalam", Name: "Kota Subulussalam", Type: "Kota", Neighbors: []string{"aceh_tenggara"}, Long: 97.9, Lat: 3.0},
	{RegionID: "aceh_singkil", Name: "Kab. Aceh Singkil", Type: "Kabupaten", Neighbors: []string{"aceh_selatan", "subulussalam"}, Long: 97.5, Lat: 2.5},
	{RegionID: "aceh_tamiang", Name: "Kab. Aceh Tamiang", Type: "Kabupaten", Neighbors: []string{"aceh_timur", "langsa"}, Long: 98.0, Lat: 4.2},
	{RegionID: "bener_meriah", Name: "Kab. Bener Meriah", Type: "Kabupaten", Neighbors: []string{"aceh_tengah", "bireuen"}, Long: 96.8, Lat: 4.7},
}

func seedRegionMaster(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionRegionMaster)

	// Check if data already exists
	count, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[seeder] region_master already has %d documents, skipping\n", count)
		return nil
	}

	now := time.Now()
	var docs []interface{}
	for _, r := range regions {
		docs = append(docs, domain.RegionMaster{
			RegionID: r.RegionID,
			Name:     r.Name,
			Type:     r.Type,
			GeoJSON: bson.M{
				"type": "Polygon",
				"coordinates": []interface{}{
					[]interface{}{[]interface{}{r.Long - 0.3, r.Lat - 0.3}, []interface{}{r.Long + 0.3, r.Lat - 0.3}, []interface{}{r.Long + 0.3, r.Lat + 0.3}, []interface{}{r.Long - 0.3, r.Lat + 0.3}, []interface{}{r.Long - 0.3, r.Lat - 0.3}},
				},
			},
			Neighbors:  r.Neighbors,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	result, err := coll.InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	log.Printf("[seeder] region_master: inserted %d documents\n", len(result.InsertedIDs))
	return nil
}

// ── Commodity Daily Buckets ───────────────────────────────────────────

var commodities = []struct {
	ID       string
	Name     string
	Unit     string
	Category string
	BasePrice float64
}{
	{ID: "beras_premium", Name: "Beras Premium", Unit: "kg", Category: "Pangan Pokok", BasePrice: 14500},
	{ID: "beras_medium", Name: "Beras Medium", Unit: "kg", Category: "Pangan Pokok", BasePrice: 12500},
	{ID: "cabai_merah_keriting", Name: "Cabai Merah Keriting", Unit: "kg", Category: "Hortikultura", BasePrice: 65000},
	{ID: "cabai_rawit_merah", Name: "Cabai Rawit Merah", Unit: "kg", Category: "Hortikultura", BasePrice: 75000},
	{ID: "bawang_merah", Name: "Bawang Merah", Unit: "kg", Category: "Hortikultura", BasePrice: 35000},
	{ID: "bawang_putih", Name: "Bawang Putih", Unit: "kg", Category: "Hortikultura", BasePrice: 38000},
	{ID: "minyak_goreng", Name: "Minyak Goreng", Unit: "liter", Category: "Pangan Pokok", BasePrice: 18000},
	{ID: "gula_pasir", Name: "Gula Pasir", Unit: "kg", Category: "Pangan Pokok", BasePrice: 16000},
	{ID: "tepung_terigu", Name: "Tepung Terigu", Unit: "kg", Category: "Pangan Pokok", BasePrice: 12000},
	{ID: "telur_ayam", Name: "Telur Ayam Ras", Unit: "kg", Category: "Protein", BasePrice: 28000},
	{ID: "daging_sapi", Name: "Daging Sapi", Unit: "kg", Category: "Protein", BasePrice: 130000},
	{ID: "daging_ayam", Name: "Daging Ayam Ras", Unit: "kg", Category: "Protein", BasePrice: 35000},
	{ID: "ikan_kembung", Name: "Ikan Kembung", Unit: "kg", Category: "Protein", BasePrice: 35000},
	{ID: "ikan_tongkol", Name: "Ikan Tongkol", Unit: "kg", Category: "Protein", BasePrice: 32000},
	{ID: "jagung", Name: "Jagung Pipilan", Unit: "kg", Category: "Pangan Pokok", BasePrice: 8000},
	{ID: "kedelai", Name: "Kedelai Impor", Unit: "kg", Category: "Pangan Pokok", BasePrice: 12000},
}

func seedCommodityDailyBuckets(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionCommodityDailyBuckets)

	count, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[seeder] commodity_daily_buckets already has %d documents, skipping\n", count)
		return nil
	}

	now := time.Now().Truncate(24 * time.Hour)
	var allDocs []interface{}

	// Generate 45 days of data for each commodity
	for day := 0; day < 45; day++ {
		date := now.AddDate(0, 0, -day)

		for _, c := range commodities {
			// Add some randomness to price over time
			dayFactor := float64(day) * 0.001
			noise := (math.Sin(float64(day)*0.3) * 0.05) + (float64(day%7) * 0.01)
			avgPrice := c.BasePrice * (1 + dayFactor + noise)

			// Regional variations
			var regionalBreakdown []domain.RegionalBreakdownItem
			for _, r := range regions {
				if r.RegionID == "aceh_province" {
					continue
				}
				regionalNoise := 1.0 + (float64(day%len(regions))*0.02) + (math.Sin(float64(day)*0.5+float64(len(regionalBreakdown))*0.7) * 0.03)
				regionalPrice := avgPrice * regionalNoise
				priceChange := (regionalPrice - c.BasePrice) / c.BasePrice * 100

				status := "Normal"
				if priceChange > 10 {
					status = "Tekanan/defisit"
				} else if priceChange > 5 {
					status = "Waspada"
				} else if priceChange < -5 {
					status = "Surplus"
				}

				regionalBreakdown = append(regionalBreakdown, domain.RegionalBreakdownItem{
					RegionID:       r.RegionID,
					RegionName:     r.Name,
					Price:          math.Round(regionalPrice),
					PriceChangePct: math.Round(priceChange*10) / 10,
					Status:         status,
				})
			}

			// Provincial metrics
			inflationMom := (avgPrice - c.BasePrice) / c.BasePrice * 100
			inflationYoy := inflationMom + 2.0 + (float64(day%10) * 0.1)

			provStatus := "Normal"
			if inflationMom > 8 {
				provStatus = "Defisit"
			} else if inflationMom > 4 {
				provStatus = "Waspada"
			} else if inflationMom < -3 {
				provStatus = "Surplus"
			}

			// Alerts (only for some days/commodities)
			var alerts []domain.AlertItem
			if day < 3 && c.ID == "cabai_merah_keriting" {
				alerts = append(alerts, domain.AlertItem{
					AlertID:        fmt.Sprintf("ALT-%s-%s-%02d", date.Format("20060102"), c.ID, day),
					RegionID:       "aceh_tengah",
					Type:           "Lonjakan Harga",
					Severity:       "Kritis",
					PriceChangePct: 18.3,
					Timestamp:      date.Add(8*time.Hour + time.Duration(day)*time.Hour),
				})
			}
			if day < 2 && c.ID == "bawang_merah" {
				alerts = append(alerts, domain.AlertItem{
					AlertID:        fmt.Sprintf("ALT-%s-%s-%02d", date.Format("20060102"), c.ID, day),
					RegionID:       "aceh_besar",
					Type:           "Stok Menipis",
					Severity:       "Tinggi",
					PriceChangePct: 12.5,
					Timestamp:      date.Add(9*time.Hour + time.Duration(day)*time.Hour),
				})
			}

			// AI Predictions (only for latest 14 days)
			var aiPredictions *domain.AIPredictions
			if day < 14 {
				forecastChange := 5.0 + (float64(day) * 0.5) + (math.Sin(float64(day)*0.7) * 2.0)
				forecastPrice := avgPrice * (1 + forecastChange/100)
				aiPredictions = &domain.AIPredictions{
					ModelVersion:      "v2.3.1",
					Forecast14dPrice:  math.Round(forecastPrice),
					ForecastChangePct: math.Round(forecastChange*10) / 10,
					ConfidenceInterval80: domain.ConfidenceInterval80{
						Lower: math.Round(forecastPrice * 0.85),
						Upper: math.Round(forecastPrice * 1.15),
					},
					Mape: 6.8,
					FeatureImportance: []domain.FeatureImportanceItem{
						{Feature: "Curah hujan sentra Gayo", ContributionScore: 9.3},
						{Feature: "Stok cold storage", ContributionScore: 5.3},
						{Feature: "Momentum harga 7 hari", ContributionScore: 4.7},
						{Feature: "Permintaan musiman", ContributionScore: 2.9},
						{Feature: "Pasokan dari Sumatera Utara", ContributionScore: -1.8},
					},
				}
			}

			// Operational metrics
			opMetrics := domain.OperationalMetrics{
				TotalFieldEntries:          400 + day*5 + (day % 20),
				ValidationRatePct:          85.0 + float64(day%15),
				ColdStorageUtilizationPct:  75.0 + float64(day%20),
				CommoditiesAboveNormal:     2 + (day % 4),
			}

			doc := domain.CommodityDailyBucket{
				Date:        date,
				CommodityID: c.ID,
				CommodityMetadata: domain.CommodityMetadata{
					Name:     c.Name,
					Unit:     c.Unit,
					Category: c.Category,
				},
				ProvincialMetrics: domain.ProvincialMetrics{
					AvgPrice:     math.Round(avgPrice),
					InflationMom: math.Round(inflationMom*10) / 10,
					InflationYoy: math.Round(inflationYoy*10) / 10,
					Status:       provStatus,
				},
				RegionalBreakdown:  regionalBreakdown,
				OperationalMetrics: opMetrics,
				Alerts:             alerts,
				AIPredictions:      aiPredictions,
				SchemaVersion:      1,
				CreatedAt:          date.Add(23*time.Hour + 59*time.Minute),
				UpdatedAt:          date.Add(23*time.Hour + 59*time.Minute),
			}
			allDocs = append(allDocs, doc)
		}
	}

	// Insert in batches of 100
	batchSize := 100
	for i := 0; i < len(allDocs); i += batchSize {
		end := i + batchSize
		if end > len(allDocs) {
			end = len(allDocs)
		}
		result, err := coll.InsertMany(ctx, allDocs[i:end])
		if err != nil {
			return fmt.Errorf("insert batch %d: %w", i/batchSize, err)
		}
		log.Printf("[seeder] commodity_daily_buckets: inserted batch %d (%d docs)\n", i/batchSize, len(result.InsertedIDs))
	}

	log.Printf("[seeder] commodity_daily_buckets: total inserted %d documents\n", len(allDocs))
	return nil
}

// ── Fiscal Space Buckets ──────────────────────────────────────────────

func seedFiscalSpaceBuckets(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionFiscalSpaceBuckets)

	count, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[seeder] fiscal_space_buckets already has %d documents, skipping\n", count)
		return nil
	}

	now := time.Now()
	var docs []interface{}

	// Generate 6 months of fiscal data
	for month := 0; month < 6; month++ {
		period := now.AddDate(0, -month, 0).Format("2006-01")
		realizationPct := 15.0 + float64(month)*5.0 + float64(month%3)*2.0

		doc := domain.FiscalSpaceBucket{
			Period:   period,
			RegionID: "aceh_province",
			BudgetSummary: domain.BudgetSummary{
				TotalPagu:         186000000000,
				RealizedAmount:    186000000000 * realizationPct / 100,
				RealizationPct:    realizationPct,
				InterventionScore: 65.0 + float64(month)*2.0,
			},
			Programs: []domain.ProgramItem{
				{ProgramName: "Belanja tidak terduga (BTT)", Allocated: 120000000000, Used: 31200000000 + float64(month)*5000000000},
				{ProgramName: "Operasi pasar murah", Allocated: 24000000000, Used: 11200000000 + float64(month)*2000000000},
				{ProgramName: "Subsidi ongkos angkut", Allocated: 18000000000, Used: 9400000000 + float64(month)*1500000000},
				{ProgramName: "Gerakan tanam pangan", Allocated: 24000000000, Used: 2400000000 + float64(month)*1000000000},
			},
			SchemaVersion: 1,
			CreatedAt:     now.AddDate(0, -month, 0),
			UpdatedAt:     now.AddDate(0, -month, 0),
		}
		docs = append(docs, doc)
	}

	result, err := coll.InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	log.Printf("[seeder] fiscal_space_buckets: inserted %d documents\n", len(result.InsertedIDs))
	return nil
}

// ── Field Entries ─────────────────────────────────────────────────────

func seedFieldEntries(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionFieldEntries)

	count, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[seeder] field_entries already has %d documents, skipping\n", count)
		return nil
	}

	now := time.Now().Truncate(24 * time.Hour)
	var docs []interface{}

	// Generate field entries for today
	entryNum := 0
	for _, c := range commodities {
		for _, r := range regions {
			if r.RegionID == "aceh_province" {
				continue
			}
			entryNum++
			if entryNum > 200 {
				break
			}

			regionalNoise := 1.0 + (float64(entryNum%10) * 0.02)
			price := c.BasePrice * regionalNoise
			deviation := (price - c.BasePrice) / c.BasePrice * 100

			status := domain.FieldEntryPending
			if entryNum%5 == 0 {
				status = domain.FieldEntryApproved
			} else if entryNum%7 == 0 {
				status = domain.FieldEntryRejected
			}

			recordedAt := now.Add(time.Duration(entryNum%12) * time.Hour)

			doc := domain.FieldEntry{
				EntryID:           fmt.Sprintf("ENT-%s-%03d", now.Format("20060102"), entryNum),
				CommodityID:       c.ID,
				MarketID:          fmt.Sprintf("pasar_%s", r.RegionID),
				RegionID:          r.RegionID,
				Price:             math.Round(price),
				PriceDeviationPct: math.Round(deviation*10) / 10,
				RecordedBy:        fmt.Sprintf("user_surveyor_%d", 1+(entryNum%10)),
				RecordedAt:        recordedAt,
				Status:            status,
				CreatedAt:         recordedAt,
				UpdatedAt:         recordedAt,
			}

			if status == domain.FieldEntryApproved {
				validatedBy := "user_validator_1"
				validatedAt := recordedAt.Add(30 * time.Minute)
				doc.ValidatedBy = &validatedBy
				doc.ValidatedAt = &validatedAt
			}

			docs = append(docs, doc)
		}
		if entryNum > 200 {
			break
		}
	}

	result, err := coll.InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	log.Printf("[seeder] field_entries: inserted %d documents\n", len(result.InsertedIDs))
	return nil
}

// ── Model Registry ────────────────────────────────────────────────────

func seedModelRegistry(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionModelRegistry)

	count, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[seeder] model_registry already has %d documents, skipping\n", count)
		return nil
	}

	now := time.Now()
	var docs []interface{}

	models := []struct {
		CommodityID string
		Version     string
		Algorithm   string
		Mape        float64
		DriftPSI    float64
		Status      domain.ModelStatus
		TrainedDays int
		DeployedDays *int
	}{
		{CommodityID: "cabai_merah_keriting", Version: "v2.3.1", Algorithm: "Ensemble LightGBM + SARIMAX", Mape: 7.6, DriftPSI: 0.06, Status: domain.ModelStatusProduction, TrainedDays: 18, DeployedDays: intPtr(17)},
		{CommodityID: "cabai_merah_keriting", Version: "v2.3.0", Algorithm: "LightGBM", Mape: 8.2, DriftPSI: 0.12, Status: domain.ModelStatusArchived, TrainedDays: 45, DeployedDays: intPtr(44)},
		{CommodityID: "cabai_rawit_merah", Version: "v1.2.0", Algorithm: "XGBoost + ARIMA", Mape: 9.1, DriftPSI: 0.08, Status: domain.ModelStatusProduction, TrainedDays: 20, DeployedDays: intPtr(19)},
		{CommodityID: "bawang_merah", Version: "v1.1.0", Algorithm: "Random Forest + SARIMA", Mape: 8.5, DriftPSI: 0.04, Status: domain.ModelStatusProduction, TrainedDays: 25, DeployedDays: intPtr(24)},
		{CommodityID: "beras_premium", Version: "v3.0.0", Algorithm: "Prophet + LSTM", Mape: 5.2, DriftPSI: 0.03, Status: domain.ModelStatusProduction, TrainedDays: 10, DeployedDays: intPtr(9)},
		{CommodityID: "beras_premium", Version: "v3.1.0", Algorithm: "Prophet + LSTM + Attention", Mape: 4.8, DriftPSI: 0.02, Status: domain.ModelStatusCandidate, TrainedDays: 3, DeployedDays: nil},
		{CommodityID: "minyak_goreng", Version: "v1.0.0", Algorithm: "SARIMAX", Mape: 10.3, DriftPSI: 0.15, Status: domain.ModelStatusProduction, TrainedDays: 30, DeployedDays: intPtr(29)},
		{CommodityID: "daging_sapi", Version: "v2.0.0", Algorithm: "Ensemble Gradient Boosting", Mape: 6.7, DriftPSI: 0.05, Status: domain.ModelStatusProduction, TrainedDays: 15, DeployedDays: intPtr(14)},
		{CommodityID: "telur_ayam", Version: "v1.3.0", Algorithm: "LightGBM + SARIMAX", Mape: 7.1, DriftPSI: 0.07, Status: domain.ModelStatusProduction, TrainedDays: 22, DeployedDays: intPtr(21)},
		{CommodityID: "gula_pasir", Version: "v1.0.0", Algorithm: "ARIMA", Mape: 11.2, DriftPSI: 0.20, Status: domain.ModelStatusArchived, TrainedDays: 60, DeployedDays: intPtr(59)},
	}

	for _, m := range models {
		trainedAt := now.AddDate(0, 0, -m.TrainedDays)
		var deployedAt *time.Time
		if m.DeployedDays != nil {
			d := now.AddDate(0, 0, -*m.DeployedDays)
			deployedAt = &d
		}

		doc := domain.ModelRegistry{
			CommodityID: m.CommodityID,
			Version:     m.Version,
			Algorithm:   m.Algorithm,
			Metrics: domain.ModelMetrics{
				Mape:    m.Mape,
				DriftPSI: m.DriftPSI,
			},
			Status:     m.Status,
			TrainedAt:  trainedAt,
			DeployedAt: deployedAt,
			CreatedAt:  trainedAt,
			UpdatedAt:  trainedAt,
		}
		docs = append(docs, doc)
	}

	result, err := coll.InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	log.Printf("[seeder] model_registry: inserted %d documents\n", len(result.InsertedIDs))
	return nil
}

// ── Indexes ───────────────────────────────────────────────────────────

func createIndexes(ctx context.Context, client *mongo.Client, dbName string) error {
	// commodity_daily_buckets: compound index on commodity_id + date
	_, err := client.Database(dbName).Collection(domain.CollectionCommodityDailyBuckets).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "commodity_id", Value: 1}, {Key: "date", Value: -1}},
		Options: options.Index().SetName("idx_commodity_date"),
	})
	if err != nil {
		return fmt.Errorf("create idx_commodity_date: %w", err)
	}
	log.Println("[seeder] created index idx_commodity_date on commodity_daily_buckets")

	// field_entries: index on status + recorded_at
	_, err = client.Database(dbName).Collection(domain.CollectionFieldEntries).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}, {Key: "recorded_at", Value: -1}},
		Options: options.Index().SetName("idx_status_recorded_at"),
	})
	if err != nil {
		return fmt.Errorf("create idx_status_recorded_at: %w", err)
	}
	log.Println("[seeder] created index idx_status_recorded_at on field_entries")

	// field_entries: index on region_id + commodity_id + recorded_at
	_, err = client.Database(dbName).Collection(domain.CollectionFieldEntries).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "region_id", Value: 1}, {Key: "commodity_id", Value: 1}, {Key: "recorded_at", Value: -1}},
		Options: options.Index().SetName("idx_region_commodity_recorded"),
	})
	if err != nil {
		return fmt.Errorf("create idx_region_commodity_recorded: %w", err)
	}
	log.Println("[seeder] created index idx_region_commodity_recorded on field_entries")

	// region_master: unique index on region_id
	_, err = client.Database(dbName).Collection(domain.CollectionRegionMaster).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "region_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_region_id_unique"),
	})
	if err != nil {
		return fmt.Errorf("create idx_region_id_unique: %w", err)
	}
	log.Println("[seeder] created unique index idx_region_id_unique on region_master")

	// model_registry: index on commodity_id + version
	_, err = client.Database(dbName).Collection(domain.CollectionModelRegistry).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "commodity_id", Value: 1}, {Key: "version", Value: -1}},
		Options: options.Index().SetName("idx_commodity_version"),
	})
	if err != nil {
		return fmt.Errorf("create idx_commodity_version: %w", err)
	}
	log.Println("[seeder] created index idx_commodity_version on model_registry")

	return nil
}

func intPtr(i int) *int {
	return &i
}
