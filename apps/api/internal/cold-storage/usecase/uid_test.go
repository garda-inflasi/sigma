package usecase

import "github.com/garda-inflasi/api/pkg/idx"

// uid memberi UUID akun uji dari slug lamanya (rumus yang sama dengan seed dan migrasi 000110), supaya ID pengguna
// di tes berbentuk UUID seperti di basis data. Slug tetap terbaca di tempat pemakaian.
func uid(slug string) string { return idx.UUIDStabil("pengguna", slug) }

// fas memberi UUID fasilitas uji dari kodenya (rumus yang sama dengan seed dan migrasi 000113).
func fas(kode string) string { return idx.UUIDStabil("cold_storage", kode) }
