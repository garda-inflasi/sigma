package seeder

import (
	"testing"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Setiap tahap yang izinnya bukan intervensi.kelola adalah tahap persetujuan dan harus punya alur;
// sebaliknya, alur pemindahan hanya ada untuk tahap persetujuan.
func TestBuildAlur_CoversEveryApprovalStageAndNothingElse(t *testing.T) {
	alur, langkah := BuildAlur()

	gated := map[string]string{} // siklus|tahap -> izin
	for _, tahap := range BuildTahap() {
		if tahap.IzinDibutuhkan != domain.IzinBawaan {
			gated[tahap.Siklus+"|"+tahap.KodeTahap] = tahap.IzinDibutuhkan
		}
	}
	require.NotEmpty(t, gated)

	langkahPerAlur := map[string][]domain.AlurLangkah{}
	for _, l := range langkah {
		langkahPerAlur[l.AlurKode] = append(langkahPerAlur[l.AlurKode], l)
	}

	covered := map[string]bool{}
	kode := map[string]bool{}
	var eskalasi int
	for _, a := range alur {
		assert.False(t, kode[a.Kode], "kode ganda %s", a.Kode)
		kode[a.Kode] = true
		assert.True(t, a.Aktif, a.Kode)
		require.NotEmpty(t, langkahPerAlur[a.Kode], "%s harus punya langkah", a.Kode)
		for i, l := range langkahPerAlur[a.Kode] {
			assert.Equal(t, i+1, l.Urutan, "%s: urutan langkah berurutan dari 1", a.Kode)
			assert.NotEmpty(t, l.Label)
		}

		if a.Jenis == domain.JenisEskalasi {
			eskalasi++
			assert.Nil(t, a.Siklus)
			assert.Nil(t, a.TahapTujuan)
			continue
		}
		require.NotNil(t, a.Siklus, a.Kode)
		require.NotNil(t, a.TahapTujuan, a.Kode)
		key := *a.Siklus + "|" + *a.TahapTujuan
		izin, ok := gated[key]
		require.True(t, ok, "%s menunjuk tahap yang bukan tahap persetujuan (%s)", a.Kode, key)
		covered[key] = true
		// Langkah terakhir alur adalah izin yang disyaratkan tahapnya, jadi alur dan izin tahap tidak saling bertentangan.
		last := langkahPerAlur[a.Kode][len(langkahPerAlur[a.Kode])-1]
		assert.Equal(t, izin, last.IzinDibutuhkan, "%s: langkah akhir harus izin tahap tujuan", a.Kode)
	}
	assert.Equal(t, 1, eskalasi)
	for key := range gated {
		assert.True(t, covered[key], "tahap persetujuan %s belum punya alur", key)
	}
}

func TestSeedReference_AddsAlurButNeverOverwritesEditedOnes(t *testing.T) {
	db := seededDB(t, "intervensi_alur_seed")
	ctx := t.Context()

	for range 2 {
		ref, err := SeedReference(ctx, db)
		require.NoError(t, err)
		assert.Equal(t, 5, ref.Alur)
		assert.Equal(t, 6, ref.LangkahAlur)
	}
	assert.EqualValues(t, 5, count(t, db, "alur_persetujuan"))
	assert.EqualValues(t, 6, count(t, db, "alur_persetujuan_langkah"))

	// Admin mengubah alur lewat data: langkah dihapus, label diganti, satu alur dimatikan.
	require.NoError(t, db.Exec(`UPDATE alur_persetujuan_langkah SET izin_dibutuhkan = 'intervensi.setujui', label = 'Diubah admin' WHERE alur_kode = 'pindah-tekanan-fiskal' AND urutan = 2`).Error)
	require.NoError(t, db.Exec(`UPDATE alur_persetujuan SET aktif = false WHERE kode = 'pindah-surplus-selesai'`).Error)

	_, err := SeedReference(ctx, db)
	require.NoError(t, err)

	var label string
	require.NoError(t, db.Raw(`SELECT label FROM alur_persetujuan_langkah WHERE alur_kode = 'pindah-tekanan-fiskal' AND urutan = 2`).Scan(&label).Error)
	assert.Equal(t, "Diubah admin", label)
	var aktif bool
	require.NoError(t, db.Raw(`SELECT aktif FROM alur_persetujuan WHERE kode = 'pindah-surplus-selesai'`).Scan(&aktif).Error)
	assert.False(t, aktif, "seed tidak mengaktifkan ulang alur yang dimatikan")
}

func TestSeedDemo_OpenRequestIsRebuiltWithoutDuplicates(t *testing.T) {
	db := seededDB(t, "intervensi_permintaan_demo")
	ctx := t.Context()
	_, err := SeedReference(ctx, db)
	require.NoError(t, err)

	for range 2 {
		demo, err := SeedDemo(ctx, db, seedNow)
		require.NoError(t, err)
		assert.Equal(t, 1, demo.Persetujuan)
	}

	assert.EqualValues(t, 1, count(t, db, "intervensi_persetujuan"))
	assert.EqualValues(t, 2, count(t, db, "intervensi_persetujuan_langkah"), "rantai disalin dari alur fiskal (2 langkah)")
	var p struct {
		IntervensiID string
		Status       string
		TahapTujuan  string
		DiajukanOleh string
	}
	require.NoError(t, db.Raw(`SELECT intervensi_id, status, tahap_tujuan, diajukan_oleh FROM intervensi_persetujuan`).Scan(&p).Error)
	assert.Equal(t, "INT-304", p.IntervensiID)
	assert.Equal(t, "MENUNGGU", p.Status)
	assert.Equal(t, "t-fiskal", p.TahapTujuan)
	assert.Equal(t, "manajer_program", p.DiajukanOleh)
}

func TestBuildKartu_TahapSejakFollowsTheWalkToTheCurrentTahap(t *testing.T) {
	kartu, _ := BuildKartu(seedNow)
	byID := map[string]domain.Intervensi{}
	for _, k := range kartu {
		byID[k.ID] = k
	}
	first := byID["INT-301"] // t-ews: tahap pertama
	assert.Equal(t, first.DibuatPada, first.TahapSejak)
	fiskal := byID["INT-305"] // t-fiskal: tahap kelima
	assert.Equal(t, fiskal.DibuatPada.Add(4*selangTahap), fiskal.TahapSejak)
}
