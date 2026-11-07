package main

import (
	"fmt"
	"io"

	"github.com/garda-inflasi/api/internal/audit/arsip"
)

// verifikasiArsip memeriksa satu berkas arsip audit dari manifest-nya; galat bila berkas tidak utuh.
func verifikasiArsip(pathManifest string, out io.Writer) error {
	hasil, err := arsip.Verifikasi(pathManifest)
	if err != nil {
		return fmt.Errorf("verifikasi arsip: %w", err)
	}
	fmt.Fprintf(out, "%s: %d baris, %d segmen rantai\n", hasil.Partisi, hasil.JumlahBaris, hasil.JumlahSegmen)
	if hasil.Valid() {
		fmt.Fprintln(out, "arsip utuh")
		return nil
	}
	for _, m := range hasil.Masalah {
		fmt.Fprintf(out, "  %s id=%d: %s\n", m.Jenis, m.ID, m.Keterangan)
	}
	return fmt.Errorf("arsip %s rusak: %d masalah", hasil.Partisi, hasil.JumlahMasalah)
}
