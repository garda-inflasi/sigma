// Package mongotest menyediakan MongoDB sementara (container dockertest `mongo:7`) untuk tes integrasi pembaca
// pipeline. Satu container dipakai bersama oleh satu paket; tiap tes memakai nama database sendiri dan membuangnya.
// Memerlukan Docker; paket yang memakainya melewatinya dengan `go test -short`.
package mongotest

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// containerTTLSeconds membatasi umur container bila proses uji mati sebelum Purge.
const containerTTLSeconds = 1800

// Server adalah container MongoDB yang sedang berjalan.
type Server struct {
	pool     *dockertest.Pool
	resource *dockertest.Resource
	uri      string
	client   *mongo.Client
}

// Start menyalakan container mongo:7 dan menunggu sampai menjawab ping.
func Start() (*Server, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("tidak bisa terhubung ke Docker (gunakan -short untuk melewati): %w", err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{Repository: "mongo", Tag: "7"},
		func(hc *docker.HostConfig) {
			hc.AutoRemove = true
			hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
		})
	if err != nil {
		return nil, fmt.Errorf("gagal menyalakan container MongoDB: %w", err)
	}
	if err := resource.Expire(containerTTLSeconds); err != nil {
		_ = pool.Purge(resource)
		return nil, fmt.Errorf("atur batas hidup container: %w", err)
	}
	srv := &Server{pool: pool, resource: resource,
		uri: "mongodb://" + resource.GetHostPort("27017/tcp") + "/?directConnection=true"}
	pool.MaxWait = 90 * time.Second
	if err := pool.Retry(func() error {
		c, err := mongo.Connect(options.Client().ApplyURI(srv.uri).SetServerSelectionTimeout(2 * time.Second))
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := c.Ping(ctx, nil); err != nil {
			_ = c.Disconnect(context.Background())
			return err
		}
		srv.client = c
		return nil
	}); err != nil {
		_ = pool.Purge(resource)
		return nil, fmt.Errorf("MongoDB tidak siap: %w", err)
	}
	return srv, nil
}

// URI adalah alamat server untuk MONGO_URI.
func (s *Server) URI() string { return s.uri }

// Client adalah klien admin tes (boleh menulis data fixture).
func (s *Server) Client() *mongo.Client { return s.client }

var bukanHuruf = regexp.MustCompile(`[^a-z0-9_]+`)

// NamaDB membuat nama database unik untuk tes dan membuangnya saat tes selesai.
func (s *Server) NamaDB(t *testing.T, akhiran string) string {
	t.Helper()
	nama := bukanHuruf.ReplaceAllString(strings.ToLower(t.Name()), "_")
	if len(nama) > 40 {
		nama = nama[len(nama)-40:]
	}
	nama = fmt.Sprintf("t_%s_%s_%d", nama, akhiran, time.Now().UnixNano()%1_000_000)
	t.Cleanup(func() { _ = s.client.Database(nama).Drop(context.Background()) })
	return nama
}

// Purge membuang container.
func (s *Server) Purge() error {
	if s.client != nil {
		_ = s.client.Disconnect(context.Background())
	}
	return s.pool.Purge(s.resource)
}
