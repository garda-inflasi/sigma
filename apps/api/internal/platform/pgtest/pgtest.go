// Package pgtest menyediakan PostgreSQL sementara (container dockertest) dan
// migrasi goose sungguhan untuk tes integrasi. Satu container dipakai bersama
// oleh seluruh tes dalam satu paket; tiap tes membuat basis data sendiri.
//
// Memerlukan Docker. Tes yang memakainya dilewati dengan `go test -short`.
package pgtest

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/pkg/database"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	user     = "garda"
	password = "garda_secret"
	// Basis data bawaan container, dipakai hanya untuk membuat basis data tes.
	adminDB = "postgres"
	// Batas hidup container bila proses uji mati sebelum sempat Purge. Harus jauh di atas durasi paket
	// terlama (auth/integration bisa lebih dari 10 menit saat Docker dipakai banyak proses sekaligus), karena
	// container yang kedaluwarsa di tengah paket membuat sisa tesnya gagal beruntun.
	containerTTLSeconds = 1800
)

// Server adalah container PostgreSQL yang sedang berjalan.
type Server struct {
	pool       *dockertest.Pool
	resource   *dockertest.Resource
	host, port string
}

// Main menjalankan seluruh tes paket dengan satu container bersama, lalu
// membuangnya. Isi *dst dengan server tersebut (nil bila -short). Kembalian
// adalah kode keluar untuk os.Exit.
func Main(m *testing.M, dst **Server) int {
	flag.Parse() // testing.Short() membutuhkan flag yang sudah di-parse
	if !testing.Short() {
		srv, err := Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pgtest: %v\n", err)
			return 1
		}
		*dst = srv
	}

	code := m.Run()

	if *dst != nil {
		if err := (*dst).Purge(); err != nil {
			fmt.Fprintf(os.Stderr, "pgtest: gagal membuang container: %v\n", err)
		}
	}
	return code
}

// Start menyalakan container PostgreSQL dan menunggu sampai siap.
func Start() (*Server, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("tidak bisa terhubung ke Docker (gunakan -short untuk melewati): %w", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + adminDB,
		},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("gagal menyalakan container PostgreSQL: %w", err)
	}
	if err := resource.Expire(containerTTLSeconds); err != nil {
		_ = pool.Purge(resource)
		return nil, fmt.Errorf("atur batas hidup container: %w", err)
	}

	host, port, _ := strings.Cut(resource.GetHostPort("5432/tcp"), ":")
	srv := &Server{pool: pool, resource: resource, host: host, port: port}

	pool.MaxWait = 90 * time.Second
	if err := pool.Retry(srv.ping); err != nil {
		_ = pool.Purge(resource)
		return nil, fmt.Errorf("PostgreSQL tidak siap: %w", err)
	}
	return srv, nil
}

func (s *Server) ping() error {
	db, err := s.open(adminDB)
	if err != nil {
		return err
	}
	defer closeDB(db)
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Purge membuang container.
func (s *Server) Purge() error {
	return s.pool.Purge(s.resource)
}

func (s *Server) open(dbName string) (*gorm.DB, error) {
	return database.New(database.Config{
		Host: s.host, Port: s.port, User: user, Password: password, DBName: dbName, SSLMode: "disable",
	})
}

// BukaSebagai membuka basis data dbName sebagai pengguna lain (mis. login aplikasi berhak minimum); koneksinya ditutup
// saat tes selesai.
func (s *Server) BukaSebagai(t *testing.T, dbName, namaPengguna, sandi string) *gorm.DB {
	t.Helper()
	db, err := database.New(database.Config{
		Host: s.host, Port: s.port, User: namaPengguna, Password: sandi, DBName: dbName, SSLMode: "disable",
	})
	require.NoError(t, err)
	t.Cleanup(func() { closeDB(db) })
	return db
}

// DSN mengembalikan DATABASE_URL untuk basis data bernama dbName; dipakai tes
// yang menjalankan biner (mis. cmd/seed) terhadap container ini.
func (s *Server) DSN(dbName string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     s.host + ":" + s.port,
		Path:     dbName,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// NewDatabase membuat basis data kosong bernama name; saat tes selesai koneksinya ditutup dan basis datanya
// dibuang. Tanpa itu setiap tes meninggalkan satu salinan skema lengkap (puluhan MB) di container sampai
// paketnya selesai, dan paket besar menumpuk beberapa GB di disk Docker. Tes dilewati bila server nil (mode
// -short).
func (s *Server) NewDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := s.CreateDatabase(t, name)
	t.Cleanup(func() {
		closeDB(db)
		s.dropDatabase(t, name)
	})
	return db
}

// dropDatabase membuang basis data tes. Kegagalan hanya dicatat: basis data yang tersisa ikut hilang bersama
// container, jadi tidak boleh menggagalkan tes yang sudah lulus.
func (s *Server) dropDatabase(t *testing.T, name string) {
	admin, err := s.open(adminDB)
	if err != nil {
		t.Logf("pgtest: buang basis data %s: %v", name, err)
		return
	}
	defer closeDB(admin)
	// FORCE memutus koneksi yang masih terbuka (mis. pool yang lupa ditutup kode yang diuji).
	if err := admin.Exec(`DROP DATABASE IF EXISTS "` + name + `" WITH (FORCE)`).Error; err != nil {
		t.Logf("pgtest: buang basis data %s: %v", name, err)
	}
}

// CreateDatabase seperti NewDatabase, tetapi koneksinya tidak ditutup otomatis
// sehingga bisa hidup lebih lama dari tes yang membuatnya (suite bersama).
func (s *Server) CreateDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	if s == nil {
		t.Skip("tes integrasi dilewati (-short)")
	}

	admin, err := s.open(adminDB)
	require.NoError(t, err)
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	closeDB(admin)

	db, err := s.open(name)
	require.NoError(t, err)
	return db
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// MigrationsDir adalah lokasi mutlak db/migrations, tidak bergantung pada
// direktori kerja tes.
func MigrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
	return filepath.Join(root, "db", "migrations")
}

// Migrator membuat Migrator goose untuk db.
func Migrator(t *testing.T, db *gorm.DB) *migrate.Migrator {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	m, err := migrate.New(sqlDB, MigrationsDir())
	require.NoError(t, err)
	return m
}

// MigrateUp menerapkan semua migrasi memakai goose.
func MigrateUp(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, Migrator(t, db).Up(context.Background()), "goose up")
}
