package dto

import "github.com/garda-inflasi/api/pkg/idx"

// uid memberi UUID akun uji dari slug lamanya (rumus yang sama dengan seed dan migrasi 000110), supaya ID pengguna
// di tes berbentuk UUID seperti di basis data. Slug tetap terbaca di tempat pemakaian.
func uid(slug string) string { return idx.UUIDStabil("pengguna", slug) }
