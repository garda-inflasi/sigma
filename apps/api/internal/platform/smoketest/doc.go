// Package smoketest menjalankan tes asap seed: PostgreSQL sementara dimigrasi,
// diisi biner cmd/seed (lapis reference lalu demo), lalu router penuh (auth +
// modules.RegisterAll) dipanggil dengan akun demo sungguhan.
//
// Kasus didaftarkan per modul di smoke_<modul>_test.go; lihat apps/api/README.md
// bagian "Menambah modul".
package smoketest
