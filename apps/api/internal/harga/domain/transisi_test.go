package domain_test

import (
	"slices"
	"testing"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/stretchr/testify/assert"
)

func TestEvaluasi_AllStatusActionCombinations(t *testing.T) {
	semuaStatus := []string{domain.StatusDraf, domain.StatusDiajukan, domain.StatusDivalidasi, domain.StatusDipublikasi, domain.StatusDitolak}
	sah := map[domain.BulkAction][]string{
		domain.BulkAjukan:     {domain.StatusDraf},
		domain.BulkValidasi:   {domain.StatusDiajukan},
		domain.BulkPublikasi:  {domain.StatusDivalidasi},
		domain.BulkTolak:      {domain.StatusDiajukan},
		domain.BulkKembalikan: {domain.StatusDiajukan, domain.StatusDivalidasi},
	}
	pencatat, aktor := uid("mahasiswa_1"), uid("mahasiswa_1")

	for aksi, dari := range sah {
		for _, status := range semuaStatus {
			// Aktor = pencatat hanya relevan bagi ajukan; aksi lain diuji dengan aktor berbeda.
			pelaku := "koordinator"
			if aksi == domain.BulkAjukan {
				pelaku = aktor
			}
			sebab := domain.Evaluasi(aksi, status, &pencatat, pelaku)
			if slices.Contains(dari, status) {
				assert.Empty(t, sebab, "%s dari %s harus sah", aksi, status)
			} else {
				assert.Equal(t, domain.SebabStatus, sebab, "%s dari %s harus ditolak", aksi, status)
			}
		}
	}
}

func TestEvaluasi_SeparationOfDuties(t *testing.T) {
	sendiri := "koordinator"

	assert.Equal(t, domain.SebabPemisahanTugas, domain.Evaluasi(domain.BulkValidasi, domain.StatusDiajukan, &sendiri, sendiri))
	assert.Equal(t, domain.SebabPemisahanTugas, domain.Evaluasi(domain.BulkPublikasi, domain.StatusDivalidasi, &sendiri, sendiri))
	assert.Empty(t, domain.Evaluasi(domain.BulkValidasi, domain.StatusDiajukan, nil, sendiri), "entri tanpa pencatat tidak melanggar pemisahan tugas")
}

func TestEvaluasi_AjukanOnlyByOwner(t *testing.T) {
	pemilik, lain := uid("mahasiswa_1"), uid("mahasiswa_2")

	assert.Empty(t, domain.Evaluasi(domain.BulkAjukan, domain.StatusDraf, &pemilik, pemilik))
	assert.Equal(t, domain.SebabBukanPemilik, domain.Evaluasi(domain.BulkAjukan, domain.StatusDraf, &pemilik, lain))
	assert.Equal(t, domain.SebabBukanPemilik, domain.Evaluasi(domain.BulkAjukan, domain.StatusDraf, nil, lain))
}

func TestBulkAction_PermissionAndTarget(t *testing.T) {
	assert.Equal(t, "harga.input", domain.BulkAjukan.Izin())
	assert.Equal(t, "harga.validasi", domain.BulkValidasi.Izin())
	assert.Equal(t, "harga.publikasi", domain.BulkPublikasi.Izin())
	assert.Equal(t, "harga.validasi", domain.BulkTolak.Izin())
	assert.Equal(t, "harga.validasi", domain.BulkKembalikan.Izin())
	assert.Empty(t, domain.BulkAction("hapus").Izin())
	assert.Equal(t, domain.StatusDitolak, domain.BulkTolak.StatusTujuan())
	assert.True(t, domain.BulkTolak.AlasanWajib())
	assert.False(t, domain.BulkKembalikan.AlasanWajib())
}
