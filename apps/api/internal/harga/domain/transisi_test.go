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
			sebab := domain.Evaluasi(aksi, status, domain.Jejak{PetugasID: &pencatat}, pelaku)
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

	assert.Equal(t, domain.SebabPemisahanTugas, domain.Evaluasi(domain.BulkValidasi, domain.StatusDiajukan, domain.Jejak{PetugasID: &sendiri}, sendiri),
		"divalidasi_oleh tidak boleh sama dengan petugas_id")
	assert.Equal(t, domain.SebabPemisahanTugas, domain.Evaluasi(domain.BulkPublikasi, domain.StatusDivalidasi, domain.Jejak{PetugasID: &sendiri}, sendiri))
	assert.Empty(t, domain.Evaluasi(domain.BulkValidasi, domain.StatusDiajukan, domain.Jejak{}, sendiri), "entri tanpa pencatat tidak melanggar pemisahan tugas")
}

// Pencatat, validator, dan penerbit adalah tiga orang berbeda: penerbit tidak boleh validator entri itu.
func TestEvaluasi_PublisherMustDifferFromValidator(t *testing.T) {
	pencatat, validator, penerbit := "mahasiswa", "koordinator", "manajer_data"
	jejak := domain.Jejak{PetugasID: &pencatat, ValidatorID: &validator}

	assert.Equal(t, domain.SebabPemisahanTugas, domain.Evaluasi(domain.BulkPublikasi, domain.StatusDivalidasi, jejak, validator), "validator mempublikasikan entri yang divalidasinya")
	assert.Equal(t, domain.SebabPemisahanTugas, domain.Evaluasi(domain.BulkPublikasi, domain.StatusDivalidasi, jejak, pencatat), "pencatat mempublikasikan entrinya sendiri")
	assert.Empty(t, domain.Evaluasi(domain.BulkPublikasi, domain.StatusDivalidasi, jejak, penerbit), "orang ketiga boleh")

	// Aksi lain tidak terkena larangan validator: validator boleh mengembalikan atau menolak.
	diajukan := domain.Jejak{PetugasID: &pencatat, ValidatorID: &validator}
	assert.Empty(t, domain.Evaluasi(domain.BulkKembalikan, domain.StatusDivalidasi, diajukan, validator))
	assert.Empty(t, domain.Evaluasi(domain.BulkTolak, domain.StatusDiajukan, diajukan, validator))
}

func TestEvaluasi_AjukanOnlyByOwner(t *testing.T) {
	pemilik, lain := uid("mahasiswa_1"), uid("mahasiswa_2")

	assert.Empty(t, domain.Evaluasi(domain.BulkAjukan, domain.StatusDraf, domain.Jejak{PetugasID: &pemilik}, pemilik))
	assert.Equal(t, domain.SebabBukanPemilik, domain.Evaluasi(domain.BulkAjukan, domain.StatusDraf, domain.Jejak{PetugasID: &pemilik}, lain))
	assert.Equal(t, domain.SebabBukanPemilik, domain.Evaluasi(domain.BulkAjukan, domain.StatusDraf, domain.Jejak{}, lain))
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
