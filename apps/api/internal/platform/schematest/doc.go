// Package schematest memastikan entity GORM cocok dengan skema hasil migrasi.
//
// Tes menerapkan seluruh db/migrations dengan goose ke PostgreSQL sementara
// (dockertest) lalu membandingkan tabel, kolom, dan tipe tiap entity dengan
// information_schema. Isinya hanya berkas tes; lihat apps/api/README.md bagian
// "Menambah modul" untuk cara mendaftarkan entity modul baru.
package schematest
