package idx

// PtrAtauNil memetakan ID pelaku yang boleh kosong ke kolom uuid nullable: teks kosong menjadi nil (NULL),
// selain itu pointer ke salinannya. PostgreSQL menolak string kosong untuk kolom uuid (22P02), jadi pelaku
// sistem atau anonim (job terjadwal, login gagal, ingestion CLI) harus ditulis sebagai NULL.
func PtrAtauNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
