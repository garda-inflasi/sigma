package integration

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/internal/prediksi/repository/postgres"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func modelUjiRepo(kode, versi string) *domain.ModelRegistry {
	return &domain.ModelRegistry{
		Kode: &kode, KomoditasKode: "CBM-KRT", Nama: "uji", Versi: versi, Mape: 2, Status: domain.StatusKandidat,
		DilatihPada: fixedNow, Metrics: "{}", FeatureImportance: "[]", Sumber: domain.SumberModel,
	}
}

func proyeksiUjiRepo(kab string, horizon, nilai int) domain.PrediksiHarga {
	return domain.PrediksiHarga{
		KomoditasKode: "CBM-KRT", KabupatenKode: kab, TanggalTarget: fixedNow.AddDate(0, 0, horizon).Truncate(24 * time.Hour), HorizonHari: horizon,
		Nilai: nilai, Bawah: nilai - 500, Atas: nilai + 500, HargaDasar: 50000, Faktor: "[]", Sumber: domain.SumberModel, DibuatPada: fixedNow,
	}
}

// TestRepo_SimpanModel_IDDariDBProyeksiTertaut: ID model BIGINT diberikan basis data, proyeksi ditautkan lewat
// model_id (versi ditulis konsisten) tanpa ID atau kode buatan aplikasi, dan menulis ulang kunci alami yang sama
// memperbarui baris yang ada (ID tetap), bukan menambah.
func TestRepo_SimpanModel_IDDariDBProyeksiTertaut(t *testing.T) {
	s := newMLSuite(t, "prediksi_repo_simpan", newFakeML(t).client())
	repo := postgres.New(s.db)
	m := modelUjiRepo("MDL-CBM-KRT-ml-uji", "ml-uji")

	require.NoError(t, repo.SimpanModel(t.Context(), m, []domain.PrediksiHarga{proyeksiUjiRepo("1171", 7, 52000), proyeksiUjiRepo("1171", 14, 53000)}))

	assert.Positive(t, int64(m.ID), "ID model diisi dari RETURNING")
	assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE model_id = ? AND model_versi = 'ml-uji' AND kode IS NULL`, m.ID))

	var idAwal []int64
	require.NoError(t, s.db.Raw(`SELECT id FROM prediksi_harga WHERE model_id = ? ORDER BY horizon_hari`, m.ID).Scan(&idAwal).Error)
	require.Len(t, idAwal, 2)

	ulang := proyeksiUjiRepo("1171", 7, 54000)
	ulang.ModelID, ulang.ModelVersi = m.ID, m.Versi
	require.NoError(t, repo.SimpanProyeksi(t.Context(), []domain.PrediksiHarga{ulang}))

	assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE model_id = ?`, m.ID), "kunci alami yang sama diperbarui, bukan ditambah")
	assert.Equal(t, 54000, s.hitung(t, `SELECT nilai FROM prediksi_harga WHERE id = ?`, idAwal[0]), "baris yang sama diperbarui")
	var idSesudah []int64
	require.NoError(t, s.db.Raw(`SELECT id FROM prediksi_harga WHERE model_id = ? ORDER BY horizon_hari`, m.ID).Scan(&idSesudah).Error)
	assert.Equal(t, idAwal, idSesudah, "ID BIGINT tidak berubah oleh upsert")
}

func TestRepo_SimpanModel_DuplikatDitolakDanTransaksiDibatalkan(t *testing.T) {
	s := newMLSuite(t, "prediksi_repo_duplikat", newFakeML(t).client())
	repo := postgres.New(s.db)
	require.NoError(t, repo.SimpanModel(t.Context(), modelUjiRepo("MDL-CBM-KRT-ml-uji", "ml-uji"), nil))
	sebelum := s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga`)

	// Versi yang sama (komoditas, versi) dengan kode berbeda, dan kode yang sama dengan versi berbeda.
	for nama, m := range map[string]*domain.ModelRegistry{
		"versi sama": modelUjiRepo("MDL-CBM-KRT-lain", "ml-uji"),
		"kode sama":  modelUjiRepo("MDL-CBM-KRT-ml-uji", "ml-lain"),
	} {
		err := repo.SimpanModel(t.Context(), m, []domain.PrediksiHarga{proyeksiUjiRepo("1171", 7, 52000)})
		assert.ErrorIs(t, err, domain.ErrModelSudahAda, nama)
	}
	assert.Equal(t, sebelum, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga`), "proyeksi dari percobaan yang gagal tidak tersimpan")
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM model_registry WHERE sumber = 'MODEL'`))
}

func TestRepo_SimpanProyeksi_TanpaModelDitolak(t *testing.T) {
	s := newMLSuite(t, "prediksi_repo_tanpamodel", newFakeML(t).client())
	repo := postgres.New(s.db)

	err := repo.SimpanProyeksi(t.Context(), []domain.PrediksiHarga{proyeksiUjiRepo("1171", 7, 52000)})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tanpa model")
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE sumber = 'MODEL'`))
}

func TestRepo_GetModel_IDAtauKode(t *testing.T) {
	s := newMLSuite(t, "prediksi_repo_getmodel", newFakeML(t).client())
	repo := postgres.New(s.db)
	byKode, err := repo.GetModel(t.Context(), idx.RujukanID{Kode: "MDL-CBM-KRT-v2.3.1"})
	require.NoError(t, err)
	require.NotNil(t, byKode.Kode)
	assert.Equal(t, "MDL-CBM-KRT-v2.3.1", *byKode.Kode)

	byID, err := repo.GetModel(t.Context(), idx.RujukanID{ID: byKode.ID})
	require.NoError(t, err)
	assert.Equal(t, byKode.ID, byID.ID)

	_, err = repo.GetModel(t.Context(), idx.RujukanID{Kode: "MDL-TIDAK-ADA"})
	assert.ErrorIs(t, err, domain.ErrModelTidakDitemukan)
	_, err = repo.GetModel(t.Context(), idx.RujukanID{ID: 999999999})
	assert.ErrorIs(t, err, domain.ErrModelTidakDitemukan)
}
