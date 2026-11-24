// Package mongodb adalah klien MongoDB opsional untuk membaca hasil pipeline data dan ML (docs/PIPELINE-DATA-ML.md):
// database `dataunseen` (hasil crawl) dan `datasafe` (hasil training). API hanya membaca keduanya.
//
// Opsional berarti: tanpa MONGO_URI/MONGO_URL klien tidak dibuat (Buka mengembalikan nil tanpa galat), dan saat Mongo
// mati atau lambat proses tetap berjalan. Koneksi dibuka malas oleh driver; setiap operasi dibatasi Timeout pendek,
// sehingga Mongo yang lambat tidak menahan request lebih lama dari itu.
package mongodb

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Nilai bawaan konfigurasi.
const (
	DefaultDBUnseen = "dataunseen"
	DefaultDBSafe   = "datasafe"
	// DefaultTimeout membatasi pemilihan server, koneksi, dan setiap operasi (CSOT).
	DefaultTimeout = 1500 * time.Millisecond
)

// ErrURITidakValid dikembalikan Buka bila URI tidak dapat diurai. Pesannya sengaja tidak memuat URI (berisi sandi).
var ErrURITidakValid = errors.New("MONGO_URI/MONGO_URL tidak valid")

// Config adalah konfigurasi klien.
type Config struct {
	// URI dari MONGO_URI, atau MONGO_URL bila MONGO_URI kosong. Kosong berarti Mongo tidak dikonfigurasi.
	URI      string
	DBUnseen string
	DBSafe   string
	Timeout  time.Duration
}

// ConfigDariEnv membaca MONGO_URI (fallback MONGO_URL), MONGO_DB_UNSEEN (bawaan dataunseen), dan MONGO_DB_SAFE
// (bawaan datasafe).
func ConfigDariEnv(getenv func(string) string) Config {
	uri := strings.TrimSpace(getenv("MONGO_URI"))
	if uri == "" {
		uri = strings.TrimSpace(getenv("MONGO_URL"))
	}
	cfg := Config{
		URI:      uri,
		DBUnseen: strings.TrimSpace(getenv("MONGO_DB_UNSEEN")),
		DBSafe:   strings.TrimSpace(getenv("MONGO_DB_SAFE")),
		Timeout:  DefaultTimeout,
	}
	if cfg.DBUnseen == "" {
		cfg.DBUnseen = DefaultDBUnseen
	}
	if cfg.DBSafe == "" {
		cfg.DBSafe = DefaultDBSafe
	}
	return cfg
}

// Dikonfigurasi melaporkan apakah URI diisi.
func (c Config) Dikonfigurasi() bool { return c.URI != "" }

// Klien membungkus mongo.Client beserta dua database pipeline.
type Klien struct {
	client  *mongo.Client
	unseen  *mongo.Database
	safe    *mongo.Database
	timeout time.Duration
}

// Buka membuat klien tanpa menunggu server (driver menyambung saat operasi pertama). Mengembalikan (nil, nil) bila
// URI kosong. Galat hanya untuk konfigurasi yang tidak valid; Mongo yang mati tidak menggagalkan Buka.
func Buka(cfg Config) (*Klien, error) {
	if !cfg.Dikonfigurasi() {
		return nil, nil
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	opts := options.Client().ApplyURI(cfg.URI).
		SetAppName("garda-api").
		SetServerSelectionTimeout(timeout).
		SetConnectTimeout(timeout).
		SetTimeout(timeout).
		SetMaxPoolSize(20).
		SetRetryWrites(false).
		SetReadPreference(readpref.PrimaryPreferred())
	if err := opts.Validate(); err != nil {
		return nil, ErrURITidakValid
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, ErrURITidakValid
	}
	return &Klien{
		client:  client,
		unseen:  client.Database(cfg.DBUnseen),
		safe:    client.Database(cfg.DBSafe),
		timeout: timeout,
	}, nil
}

// Unseen adalah database hasil crawl (dataunseen).
func (k *Klien) Unseen() *mongo.Database { return k.unseen }

// Safe adalah database hasil training (datasafe).
func (k *Klien) Safe() *mongo.Database { return k.safe }

// Timeout adalah batas satu operasi.
func (k *Klien) Timeout() time.Duration { return k.timeout }

// Ping memeriksa server dalam batas Timeout.
func (k *Klien) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, k.timeout)
	defer cancel()
	return k.client.Ping(ctx, readpref.PrimaryPreferred())
}

// Tutup melepas pool koneksi.
func (k *Klien) Tutup(ctx context.Context) error {
	if k == nil {
		return nil
	}
	return k.client.Disconnect(ctx)
}
