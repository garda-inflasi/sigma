package schematest

// Pendaftaran bersifat per modul agar modul baru cukup menambah berkas:
//
//	models_<modul>_test.go  init() memanggil registerModels("<modul>", &domain.X{}, ...)
//	pending_<modul>_test.go init() memanggil markPending("<modul>", "<alasan>")
//
// Menyelaraskan modul berarti menghapus berkas pending-nya; tes gagal bila modul
// yang masih ditandai pending ternyata sudah cocok.

var (
	registeredModels = map[string][]any{}
	pendingModules   = map[string]string{}
)

// registerModels mendaftarkan entity GORM (pointer ke struct ber-TableName) milik modul.
func registerModels(module string, entities ...any) {
	if _, exists := registeredModels[module]; exists {
		panic("schematest: models modul " + module + " terdaftar dua kali")
	}
	registeredModels[module] = entities
}

// markPending menandai modul yang entity-nya belum selaras dengan migrasi,
// beserta alasannya.
func markPending(module, reason string) {
	if _, exists := pendingModules[module]; exists {
		panic("schematest: pending modul " + module + " terdaftar dua kali")
	}
	pendingModules[module] = reason
}
