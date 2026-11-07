package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/internal/prediksi/mlclient"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeMLRepo struct {
	seri      []domain.SeriHarian
	produksi  *domain.ModelRegistry
	err       error
	errSimpan error
	evaluasi  int64
	disimpan  *domain.ModelRegistry
	proyeksi  []domain.PrediksiHarga
	seriDari  time.Time
	hariIniEv time.Time

	// models adalah registri palsu, berkunci pada kode model.
	models       map[string]*domain.ModelRegistry
	digantikan   *domain.ModelRegistry
	dipromosikan []string // kode model
	diarsipkan   []string // kode model
	oleh         string
	waktu        time.Time
}

func (r *fakeMLRepo) SeriKonsumen(_ context.Context, _ string, dari time.Time) ([]domain.SeriHarian, error) {
	r.seriDari = dari
	return r.seri, r.err
}

// idModelBaru adalah ID BIGINT yang "diberikan basis data" kepada model hasil pelatihan di fake.
const idModelBaru idx.ID = 5

func (r *fakeMLRepo) SimpanModel(_ context.Context, m *domain.ModelRegistry, p []domain.PrediksiHarga) error {
	if r.errSimpan != nil {
		return r.errSimpan
	}
	m.ID = idModelBaru
	r.disimpan, r.proyeksi = m, p
	return nil
}

func (r *fakeMLRepo) SimpanProyeksi(_ context.Context, p []domain.PrediksiHarga) error {
	r.proyeksi = p
	return r.err
}

func (r *fakeMLRepo) ModelProduksi(context.Context, string) (*domain.ModelRegistry, error) {
	if r.produksi == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.produksi, nil
}

func (r *fakeMLRepo) GetModelView(_ context.Context, id idx.ID) (*domain.ModelView, error) {
	return &domain.ModelView{ID: id}, r.err
}

// cari mencari model di `models` menurut ID atau kode, seperti repositori sungguhan.
func (r *fakeMLRepo) cari(ref idx.RujukanID) (*domain.ModelRegistry, bool) {
	for _, m := range r.models {
		if (ref.ID != 0 && m.ID == ref.ID) || (ref.Kode != "" && m.Kode != nil && *m.Kode == ref.Kode) {
			return m, true
		}
	}
	return nil, false
}

// GetModel mengembalikan model menurut ID atau kode dari `models`; ErrModelTidakDitemukan bila tak ada.
func (r *fakeMLRepo) GetModel(_ context.Context, ref idx.RujukanID) (*domain.ModelRegistry, error) {
	if m, ok := r.cari(ref); ok {
		return m, nil
	}
	return nil, domain.ErrModelTidakDitemukan
}

// Promosikan meniru aturan repositori: model PRODUKSI tidak dapat dipromosikan lagi.
func (r *fakeMLRepo) Promosikan(_ context.Context, ref idx.RujukanID, oleh string, waktu time.Time) (*domain.HasilPromosi, error) {
	m, ok := r.cari(ref)
	if !ok {
		return nil, domain.ErrModelTidakDitemukan
	}
	if m.Status == domain.StatusProduksi {
		return nil, domain.ErrStatusModelTidakValid
	}
	hasil := &domain.HasilPromosi{DariStatus: m.Status, Digantikan: r.digantikan}
	m.Status = domain.StatusProduksi
	hasil.Model = *m
	r.dipromosikan = append(r.dipromosikan, *m.Kode)
	r.oleh, r.waktu = oleh, waktu
	return hasil, nil
}

func (r *fakeMLRepo) Arsipkan(_ context.Context, ref idx.RujukanID, oleh string, waktu time.Time) (*domain.ModelRegistry, error) {
	m, ok := r.cari(ref)
	if !ok {
		return nil, domain.ErrModelTidakDitemukan
	}
	if m.Status != domain.StatusKandidat {
		return nil, domain.ErrStatusModelTidakValid
	}
	m.Status = domain.StatusArsip
	r.diarsipkan = append(r.diarsipkan, *m.Kode)
	r.oleh, r.waktu = oleh, waktu
	return m, nil
}

func (r *fakeMLRepo) EvaluasiJatuhTempo(_ context.Context, hariIni time.Time) (int64, error) {
	r.hariIniEv = hariIni
	return r.evaluasi, r.err
}

type fakeML struct {
	train    *mlclient.TrainResponse
	predict  *mlclient.PredictResponse
	err      error
	health   *mlclient.Health
	gotTrain mlclient.TrainRequest
	gotPred  mlclient.PredictRequest
}

func (f *fakeML) Train(_ context.Context, req mlclient.TrainRequest) (*mlclient.TrainResponse, error) {
	f.gotTrain = req
	return f.train, f.err
}

func (f *fakeML) Predict(_ context.Context, req mlclient.PredictRequest) (*mlclient.PredictResponse, error) {
	f.gotPred = req
	return f.predict, f.err
}

func (f *fakeML) Health(context.Context) (*mlclient.Health, error) { return f.health, f.err }

var sekarang = time.Date(2026, 11, 5, 14, 3, 12, 0, time.UTC)

func uc(repo *fakeMLRepo, ml mlclient.Client) (*MLOps, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	u := NewMLOps(repo, ml, rec, logger.NewDefault())
	u.now = func() time.Time { return sekarang }
	return u, rec
}

func seriContoh(kab string) domain.SeriHarian {
	return domain.SeriHarian{KabupatenKode: kab, Titik: []domain.TitikHarga{
		{Tanggal: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), Harga: 50000},
		{Tanggal: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), Harga: 51000},
	}}
}

func hasilOK(id string) mlclient.HasilSeri {
	return mlclient.HasilSeri{
		ID: id, Status: "OK", Algoritma: "holt",
		Params:     map[string]float64{"alpha": 0.5, "beta": 0.2, "gamma": 0, "phi": 0.95},
		GalatRMSE:  map[string]float64{"7": 400, "14": 600},
		Metrik:     mlclient.Metrik{Mape: 2.5, Rmse: 1200, BaselineMape: 3.0, BaselineRmse: 1500},
		HargaDasar: 51000, TanggalDasar: "2026-11-04", NHari: 90,
		Ramalan: []mlclient.Ramalan{
			{HorizonHari: 7, TanggalTarget: "2026-11-11", Nilai: 52000, Bawah: 51000, Atas: 53000, Faktor: []mlclient.Faktor{{Nama: "Tren harga", Kontribusi: 2.0, Detail: "d"}}},
			{HorizonHari: 14, TanggalTarget: "2026-11-18", Nilai: 53000, Bawah: 51500, Atas: 54500, Faktor: []mlclient.Faktor{{Nama: "Tren harga", Kontribusi: 3.9, Detail: "d"}, {Nama: "Pola musiman mingguan", Kontribusi: 0.1, Detail: "m"}}},
		},
	}
}

func jawabanLatih(seri ...mlclient.HasilSeri) *mlclient.TrainResponse {
	return &mlclient.TrainResponse{
		VersiLayanan: "0.1.0", KomoditasKode: "CBM-KRT", Seri: seri,
		Ringkasan: mlclient.Ringkasan{NSeriOK: 1, Mape: 2.5, Rmse: 1200, BaselineMape: 3.0, BaselineRmse: 1500, Algoritma: map[string]int{"holt": 1}},
	}
}

func TestLatih_MenyimpanKandidatDanProyeksiBersumberModel(t *testing.T) {
	repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}
	ml := &fakeML{train: jawabanLatih(hasilOK("1171"))}
	u, rec := uc(repo, ml)

	view, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

	require.NoError(t, err)
	assert.Equal(t, idModelBaru, view.ID, "ID model BIGINT dari basis data, bukan buatan aplikasi")

	// Permintaan ke ML: seri dari entri harga, horizon tetap, riwayat 180 hari.
	assert.Equal(t, "CBM-KRT", ml.gotTrain.KomoditasKode)
	assert.Equal(t, []int{7, 14, 30}, ml.gotTrain.Horizon)
	assert.Equal(t, []mlclient.Titik{{Tanggal: "2026-11-03", Nilai: 50000}, {Tanggal: "2026-11-04", Nilai: 51000}}, ml.gotTrain.Seri[0].Titik)
	assert.Equal(t, sekarang.AddDate(0, 0, -180), repo.seriDari)

	m := repo.disimpan
	require.NotNil(t, m)
	require.NotNil(t, m.Kode)
	assert.Equal(t, "MDL-CBM-KRT-ml-20261105-140312", *m.Kode, "kode tampilan model tetap MDL-<komoditas>-<versi>")
	assert.Equal(t, domain.StatusKandidat, m.Status, "model baru tidak langsung dipakai")
	assert.Equal(t, domain.SumberModel, m.Sumber)
	assert.Equal(t, "ml-20261105-140312", m.Versi)
	assert.Equal(t, 2.5, m.Mape)
	assert.Equal(t, "ml://garda-ml/0.1.0", *m.ArtefakURI)
	assert.Equal(t, "holt×1", *m.Algorithm)
	assert.Contains(t, m.Catatan, "backtest walk-forward")

	metrik, err := domain.BacaMetrik(m.Metrics)
	require.NoError(t, err)
	assert.Equal(t, 3.0, metrik.BaselineMape)
	assert.Equal(t, []int{7, 14, 30}, metrik.Horizon)
	assert.Equal(t, 0.5, metrik.Seri["1171"].Params["alpha"])
	assert.Equal(t, 600.0, metrik.Seri["1171"].GalatRMSE["14"])

	require.Len(t, repo.proyeksi, 2)
	p := repo.proyeksi[1]
	assert.Zero(t, p.ID, "ID proyeksi diberikan basis data")
	assert.Nil(t, p.Kode, "proyeksi hasil ML di-upsert menurut kunci alaminya, tanpa kode")
	assert.Equal(t, "1171", p.KabupatenKode)
	assert.Equal(t, 14, p.HorizonHari)
	assert.Equal(t, "2026-11-18", p.TanggalTarget.Format(time.DateOnly))
	assert.Equal(t, domain.SumberModel, p.Sumber)
	assert.Equal(t, "ml-20261105-140312", p.ModelVersi)
	assert.Equal(t, [3]int{51500, 53000, 54500}, [3]int{p.Bawah, p.Nilai, p.Atas})
	assert.Equal(t, 51000, p.HargaDasar)
	assert.Contains(t, p.Faktor, `"kontribusi":3.9`)

	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "LATIH_MODEL", e.Aksi)
	assert.Equal(t, uid("u-1"), e.AktorID)
	assert.Equal(t, audit.HasilBerhasil, e.Hasil)
	assert.Equal(t, "5", e.EntitasID, "audit memakai ID BIGINT model")
	assert.Equal(t, "model_registry", e.Entitas)
}

func TestLatih_ImportansiFaktorDinormalkan(t *testing.T) {
	got := importansiFaktor([]mlclient.HasilSeri{hasilOK("1171")})

	require.Len(t, got, 2)
	assert.Equal(t, "Tren harga", got[0].Nama)
	assert.InDelta(t, 5.9/6.0, got[0].Bobot, 0.001)
	assert.InDelta(t, 1.0, got[0].Bobot+got[1].Bobot, 0.002)
}

func TestImportansiFaktor_NolSemuaMenjadiKosong(t *testing.T) {
	s := hasilOK("1171")
	for i := range s.Ramalan {
		s.Ramalan[i].Faktor = []mlclient.Faktor{{Nama: "Harga terakhir dipertahankan", Kontribusi: 0}}
	}
	assert.Empty(t, importansiFaktor([]mlclient.HasilSeri{s}))
}

func TestLatih_TanpaLayananML(t *testing.T) {
	u, rec := uc(&fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}, nil)

	_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

	assert.ErrorIs(t, err, ErrLayananMLMati)
	assert.Empty(t, rec.Entries(), "tidak ada yang berubah, tidak ada audit")
}

func TestLatih_GalatLayanan(t *testing.T) {
	kasus := []struct {
		nama string
		err  error
		want error
	}{
		{"layanan mati", mlclient.ErrTidakTersedia, ErrLayananMLMati},
		{"layanan menolak", mlclient.ErrDitolak, ErrMLGagal},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}
			u, _ := uc(repo, &fakeML{err: k.err})

			_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

			assert.ErrorIs(t, err, k.want)
			assert.Nil(t, repo.disimpan, "tidak ada model yang tersimpan")
		})
	}
}

func TestLatih_DataTidakCukup(t *testing.T) {
	t.Run("tanpa entri harga", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{}, &fakeML{})
		_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")
		assert.ErrorIs(t, err, ErrDataTidakCukup)
	})

	t.Run("semua seri dilewati ML", func(t *testing.T) {
		repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}
		ml := &fakeML{train: &mlclient.TrainResponse{Seri: []mlclient.HasilSeri{{ID: "1171", Status: "DILEWATI", Alasan: "riwayat kurang"}}}}
		u, _ := uc(repo, ml)

		_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

		assert.ErrorIs(t, err, ErrDataTidakCukup)
		assert.ErrorContains(t, err, "riwayat kurang")
		assert.Nil(t, repo.disimpan)
	})
}

func TestLatih_SeriDilewatiDicatatDiMetrik(t *testing.T) {
	repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171"), seriContoh("1101")}}
	ml := &fakeML{train: jawabanLatih(hasilOK("1171"), mlclient.HasilSeri{ID: "1101", Status: "DILEWATI", Alasan: "cakupan rendah"})}
	ml.train.Ringkasan.NSeriDilewati = 1
	u, _ := uc(repo, ml)

	_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

	require.NoError(t, err)
	metrik, err := domain.BacaMetrik(repo.disimpan.Metrics)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"1101": "cakupan rendah"}, metrik.Dilewati)
	assert.Contains(t, repo.disimpan.Catatan, "1 kabupaten dilewati")
	assert.Len(t, repo.proyeksi, 2, "hanya seri OK yang punya proyeksi")
}

func TestLatih_PelanggaranKontrakDitolak(t *testing.T) {
	kasus := map[string]func(*mlclient.HasilSeri){
		"seri tidak diminta":       func(s *mlclient.HasilSeri) { s.ID = "9999" },
		"interval terbalik":        func(s *mlclient.HasilSeri) { s.Ramalan[0].Bawah = s.Ramalan[0].Nilai + 1 },
		"nilai di atas batas atas": func(s *mlclient.HasilSeri) { s.Ramalan[0].Atas = s.Ramalan[0].Nilai - 1 },
		"tanggal rusak":            func(s *mlclient.HasilSeri) { s.Ramalan[0].TanggalTarget = "kemarin" },
		"harga dasar nol":          func(s *mlclient.HasilSeri) { s.HargaDasar = 0 },
	}
	for nama, ubah := range kasus {
		t.Run(nama, func(t *testing.T) {
			s := hasilOK("1171")
			ubah(&s)
			repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}
			u, _ := uc(repo, &fakeML{train: jawabanLatih(s)})

			_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

			assert.ErrorIs(t, err, ErrMLGagal)
			assert.Nil(t, repo.disimpan)
		})
	}
}

func TestLatih_GalatAuditTidakMenggagalkanPelatihan(t *testing.T) {
	repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}
	u, rec := uc(repo, &fakeML{train: jawabanLatih(hasilOK("1171"))})
	rec.Err = errors.New("audit down")

	_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

	require.NoError(t, err)
}

func modelProduksiHasilLatih(t *testing.T) *domain.ModelRegistry {
	t.Helper()
	repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}}
	u, _ := uc(repo, &fakeML{train: jawabanLatih(hasilOK("1171"))})
	_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")
	require.NoError(t, err)
	repo.disimpan.Status = domain.StatusProduksi
	return repo.disimpan
}

func TestSegarkan_MemakaiParameterTerlatihTanpaMelatihUlang(t *testing.T) {
	model := modelProduksiHasilLatih(t)
	repo := &fakeMLRepo{produksi: model, seri: []domain.SeriHarian{seriContoh("1171"), seriContoh("1101")}}
	ml := &fakeML{predict: &mlclient.PredictResponse{Seri: []mlclient.HasilSeri{hasilOK("1171")}}}
	u, rec := uc(repo, ml)

	n, err := u.Segarkan(t.Context(), uid("u-1"), "CBM-KRT")

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	require.Len(t, ml.gotPred.Seri, 1, "kabupaten yang tak ikut dilatih tidak diramal")
	assert.Equal(t, "holt", ml.gotPred.Seri[0].Algoritma)
	assert.Equal(t, 0.5, ml.gotPred.Seri[0].Params["alpha"])
	assert.Equal(t, 400.0, ml.gotPred.Seri[0].GalatRMSE["7"])
	assert.Equal(t, []int{7, 14, 30}, ml.gotPred.Horizon)
	assert.Equal(t, model.Versi, repo.proyeksi[0].ModelVersi)
	for _, p := range repo.proyeksi {
		assert.Equal(t, model.ID, p.ModelID, "proyeksi ditautkan ke model produksi lewat ID")
		assert.Nil(t, p.Kode)
	}
	e, _ := rec.Last()
	assert.Equal(t, "SEGARKAN_PROYEKSI", e.Aksi)
	assert.Equal(t, model.ID.String(), e.EntitasID)
}

func TestSegarkan_Penolakan(t *testing.T) {
	t.Run("tanpa layanan ML", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{}, nil)
		_, err := u.Segarkan(t.Context(), uid("u-1"), "CBM-KRT")
		assert.ErrorIs(t, err, ErrLayananMLMati)
	})
	t.Run("tanpa model produksi", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{}, &fakeML{})
		_, err := u.Segarkan(t.Context(), uid("u-1"), "CBM-KRT")
		assert.ErrorIs(t, err, ErrModelTidakAda)
	})
	t.Run("model produksi adalah data demo", func(t *testing.T) {
		demo := &domain.ModelRegistry{Sumber: domain.SumberDemo, Metrics: "{}"}
		u, _ := uc(&fakeMLRepo{produksi: demo}, &fakeML{})
		_, err := u.Segarkan(t.Context(), uid("u-1"), "CBM-KRT")
		assert.ErrorIs(t, err, ErrBukanHasilLatih)
	})
	t.Run("tanpa data harga", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{produksi: modelProduksiHasilLatih(t)}, &fakeML{})
		_, err := u.Segarkan(t.Context(), uid("u-1"), "CBM-KRT")
		assert.ErrorIs(t, err, ErrDataTidakCukup)
	})
	t.Run("layanan mati", func(t *testing.T) {
		repo := &fakeMLRepo{produksi: modelProduksiHasilLatih(t), seri: []domain.SeriHarian{seriContoh("1171")}}
		u, _ := uc(repo, &fakeML{err: mlclient.ErrTidakTersedia})
		_, err := u.Segarkan(t.Context(), uid("u-1"), "CBM-KRT")
		assert.ErrorIs(t, err, ErrLayananMLMati)
	})
}

func TestEvaluasi_MencatatJumlahDanAudit(t *testing.T) {
	repo := &fakeMLRepo{evaluasi: 7}
	u, rec := uc(repo, nil)

	n, err := u.Evaluasi(t.Context(), uid("u-1"))

	require.NoError(t, err)
	assert.Equal(t, int64(7), n)
	assert.Equal(t, sekarang, repo.hariIniEv)
	e, _ := rec.Last()
	assert.Equal(t, "EVALUASI_PREDIKSI", e.Aksi)
}

func TestEvaluasi_GalatRepositori(t *testing.T) {
	boom := errors.New("db down")
	u, rec := uc(&fakeMLRepo{err: boom}, nil)

	_, err := u.Evaluasi(t.Context(), uid("u-1"))

	assert.ErrorIs(t, err, boom)
	assert.Empty(t, rec.Entries())
}

func TestStatus(t *testing.T) {
	t.Run("tidak dikonfigurasi", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{}, nil)
		assert.Equal(t, StatusML{}, u.Status(t.Context()))
	})
	t.Run("tersedia", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{}, &fakeML{health: &mlclient.Health{Status: "ok", Versi: "0.1.0"}})
		assert.Equal(t, StatusML{Dikonfigurasi: true, Tersedia: true, Versi: "0.1.0"}, u.Status(t.Context()))
	})
	t.Run("mati", func(t *testing.T) {
		u, _ := uc(&fakeMLRepo{}, &fakeML{err: mlclient.ErrTidakTersedia})
		assert.Equal(t, StatusML{Dikonfigurasi: true}, u.Status(t.Context()))
	})
}

// idUji adalah ID BIGINT model uji menurut kodenya (pegangan yang dibaca di tes).
var idUji = map[string]idx.ID{"kandidat": 11, "arsip": 12, "produksi": 13, "lama": 14, "buruk": 15, "baik": 16, "k": 17, "a": 18}

func modelUji(kode, status, sumber string, mape, baseline float64) *domain.ModelRegistry {
	metrik := `{}`
	if sumber == domain.SumberModel {
		metrik = fmt.Sprintf(`{"mape":%v,"baseline_mape":%v}`, mape, baseline)
	}
	return &domain.ModelRegistry{
		ID: idUji[kode], Kode: &kode, KomoditasKode: "CBM-KRT", Versi: "v-" + kode, Status: status, Sumber: sumber, Mape: mape, Metrics: metrik,
	}
}

func TestCekBaseline(t *testing.T) {
	assert.NoError(t, cekBaseline(modelUji("a", domain.StatusKandidat, domain.SumberModel, 2.5, 3.0)))
	assert.NoError(t, cekBaseline(modelUji("a", domain.StatusKandidat, domain.SumberModel, 3.0, 3.0)), "sama dengan baseline lolos")
	assert.NoError(t, cekBaseline(modelUji("a", domain.StatusKandidat, domain.SumberDemo, 9, 0)), "model demo tak punya metrik pelatihan")

	err := cekBaseline(modelUji("a", domain.StatusKandidat, domain.SumberModel, 3.5, 3.0))
	assert.ErrorIs(t, err, ErrLebihBurukDariBaseline)
	assert.ErrorContains(t, err, "3,50% > baseline naif 3,00%")

	rusak := modelUji("a", domain.StatusKandidat, domain.SumberModel, 1, 2)
	rusak.Metrics = "bukan json"
	assert.Error(t, cekBaseline(rusak))
}

func TestAksiMassal_PromosiMemanggilRepoDanMencatatAudit(t *testing.T) {
	lama := modelUji("lama", domain.StatusProduksi, domain.SumberDemo, 1, 0)
	repo := &fakeMLRepo{
		digantikan: lama,
		models: map[string]*domain.ModelRegistry{
			"kandidat": modelUji("kandidat", domain.StatusKandidat, domain.SumberModel, 2.5, 3.0),
			"arsip":    modelUji("arsip", domain.StatusArsip, domain.SumberDemo, 4, 0),
		},
	}
	u, rec := uc(repo, nil)

	hasil, err := u.AksiMassal(t.Context(), AksiPromosi, []string{"kandidat", "arsip"}, uid("u-1"))

	require.NoError(t, err)
	assert.Equal(t, 2, hasil.Berhasil)
	assert.Empty(t, hasil.Gagal)
	assert.Equal(t, []string{"kandidat", "arsip"}, repo.dipromosikan)
	assert.Equal(t, uid("u-1"), repo.oleh)
	assert.Equal(t, sekarang, repo.waktu)

	entries := rec.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, "PROMOSI_MODEL", entries[0].Aksi)
	assert.Equal(t, idUji["kandidat"].String(), entries[0].EntitasID, "audit memakai ID BIGINT, bukan kode")
	assert.Equal(t, idUji["lama"].String(), entries[0].Sesudah.(map[string]any)["menggantikan"])
	assert.Equal(t, "kandidat", *entries[0].Metadata["kode"].(*string), "kode tampilan ikut di metadata")
	assert.Equal(t, false, entries[0].Metadata["rollback"])
	assert.Equal(t, true, entries[1].Metadata["rollback"], "promosi dari ARSIP adalah rollback")
}

func TestAksiMassal_PromosiDitolakBilaLebihBurukDariBaseline(t *testing.T) {
	repo := &fakeMLRepo{models: map[string]*domain.ModelRegistry{
		"buruk": modelUji("buruk", domain.StatusKandidat, domain.SumberModel, 4.0, 3.0),
		"baik":  modelUji("baik", domain.StatusKandidat, domain.SumberModel, 2.0, 3.0),
	}}
	u, rec := uc(repo, nil)

	hasil, err := u.AksiMassal(t.Context(), AksiPromosi, []string{"buruk", "baik"}, uid("u-1"))

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Berhasil)
	require.Len(t, hasil.Gagal, 1)
	assert.Equal(t, "buruk", hasil.Gagal[0].ID)
	assert.Contains(t, hasil.Gagal[0].Alasan, "lebih buruk dari baseline")
	assert.Equal(t, []string{"baik"}, repo.dipromosikan, "model yang lebih buruk tidak menyentuh produksi")
	assert.Len(t, rec.Entries(), 1)
}

func TestAksiMassal_StatusDanIdTidakValidDilewati(t *testing.T) {
	repo := &fakeMLRepo{models: map[string]*domain.ModelRegistry{
		"produksi": modelUji("produksi", domain.StatusProduksi, domain.SumberDemo, 2, 0),
		"arsip":    modelUji("arsip", domain.StatusArsip, domain.SumberDemo, 2, 0),
	}}
	u, rec := uc(repo, nil)

	promosi, err := u.AksiMassal(t.Context(), AksiPromosi, []string{"produksi", "hilang"}, uid("u-1"))
	require.NoError(t, err)
	assert.Zero(t, promosi.Berhasil)
	assert.Equal(t, []GagalModel{
		{ID: "produksi", Alasan: "status model tidak memungkinkan aksi ini"},
		{ID: "hilang", Alasan: "model tidak ditemukan"},
	}, promosi.Gagal)

	arsip, err := u.AksiMassal(t.Context(), AksiArsip, []string{"produksi", "arsip", "hilang"}, uid("u-1"))
	require.NoError(t, err)
	assert.Zero(t, arsip.Berhasil, "model produksi dan yang sudah arsip tidak dapat diarsipkan")
	assert.Len(t, arsip.Gagal, 3)
	assert.Empty(t, rec.Entries(), "tidak ada yang berubah, tidak ada audit")
}

func TestAksiMassal_ArsipkanKandidat(t *testing.T) {
	repo := &fakeMLRepo{models: map[string]*domain.ModelRegistry{"k": modelUji("k", domain.StatusKandidat, domain.SumberModel, 5, 3)}}
	u, rec := uc(repo, nil)

	hasil, err := u.AksiMassal(t.Context(), AksiArsip, []string{"k"}, uid("u-1"))

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Berhasil, "kandidat yang buruk tetap boleh diarsipkan")
	assert.Equal(t, []string{"k"}, repo.diarsipkan)
	e, _ := rec.Last()
	assert.Equal(t, "ARSIP_MODEL", e.Aksi)
	assert.Equal(t, domain.StatusArsip, e.Sesudah.(map[string]any)["status"])
}

func TestAksiMassal_AksiTidakDikenal(t *testing.T) {
	u, _ := uc(&fakeMLRepo{}, nil)

	_, err := u.AksiMassal(t.Context(), "hapus", []string{"x"}, uid("u-1"))

	assert.ErrorIs(t, err, ErrAksiModelTidakValid)
}

func TestAksiMassal_GalatBasisDataBukanAlasanMelewati(t *testing.T) {
	boom := errors.New("db down")
	dasar := &fakeMLRepo{models: map[string]*domain.ModelRegistry{"k": modelUji("k", domain.StatusKandidat, domain.SumberDemo, 1, 0)}}
	u, _ := uc(dasar, nil)
	u.repo = &repoGalatPromosi{fakeMLRepo: dasar, err: boom}

	_, err := u.AksiMassal(t.Context(), AksiPromosi, []string{"k"}, uid("u-1"))

	assert.ErrorIs(t, err, boom)
}

type repoGalatPromosi struct {
	*fakeMLRepo
	err error
}

func (r *repoGalatPromosi) Promosikan(context.Context, idx.RujukanID, string, time.Time) (*domain.HasilPromosi, error) {
	return nil, r.err
}

func TestAlasanDilewati(t *testing.T) {
	alasan, ok := alasanDilewati(fmt.Errorf("bungkus: %w", domain.ErrModelTanpaProyeksi))
	assert.True(t, ok)
	assert.Contains(t, alasan, "belum punya proyeksi")

	_, ok = alasanDilewati(errors.New("db down"))
	assert.False(t, ok)
	_, ok = alasanDilewati(nil)
	assert.False(t, ok)
}

func TestLatih_VersiSudahAdaDiteruskan(t *testing.T) {
	repo := &fakeMLRepo{seri: []domain.SeriHarian{seriContoh("1171")}, errSimpan: domain.ErrModelSudahAda}
	u, rec := uc(repo, &fakeML{train: jawabanLatih(hasilOK("1171"))})

	_, err := u.Latih(t.Context(), uid("u-1"), "CBM-KRT")

	assert.ErrorIs(t, err, domain.ErrModelSudahAda)
	assert.Empty(t, rec.Entries())
}

func TestRujukanModel_IDAtauKode(t *testing.T) {
	kasus := []struct {
		teks string
		want idx.RujukanID
		sah  bool
	}{
		{"42", idx.RujukanID{ID: 42}, true},
		{"MDL-CBM-KRT-v2.3.1", idx.RujukanID{Kode: "MDL-CBM-KRT-v2.3.1"}, true},
		{"0042", idx.RujukanID{Kode: "0042"}, true}, // bukan ID yang sah (nol di depan): dicari sebagai kode
		{"-5", idx.RujukanID{Kode: "-5"}, true},     // negatif: dicari sebagai kode, tidak akan ketemu
		{strings.Repeat("M", 64), idx.RujukanID{Kode: strings.Repeat("M", 64)}, true},
		{strings.Repeat("M", 65), idx.RujukanID{}, false}, // lebih panjang dari kolom kode
		{"", idx.RujukanID{}, false},
	}
	for _, k := range kasus {
		got, sah := RujukanModel(k.teks)
		assert.Equal(t, k.sah, sah, k.teks)
		assert.Equal(t, k.want, got, k.teks)
	}
}

func TestKodeModel_NilBilaTidakMuatDiKolomKode(t *testing.T) {
	require.NotNil(t, kodeModel("CBM-KRT", "ml-20261105-140312"))
	assert.Equal(t, "MDL-CBM-KRT-ml-20261105-140312", *kodeModel("CBM-KRT", "ml-20261105-140312"))
	assert.Nil(t, kodeModel(strings.Repeat("K", 50), "ml-20261105-140312"), "kode terlalu panjang tidak boleh membuat INSERT gagal")
}

func TestAksiMassal_MenerimaIDBigintDanKodeAlias(t *testing.T) {
	repo := &fakeMLRepo{models: map[string]*domain.ModelRegistry{
		"kandidat": modelUji("kandidat", domain.StatusKandidat, domain.SumberDemo, 2, 0),
		"arsip":    modelUji("arsip", domain.StatusArsip, domain.SumberDemo, 2, 0),
	}}
	u, rec := uc(repo, nil)

	// "11" = ID BIGINT kandidat; "arsip" = kode model arsip; ID yang tidak ada dan kode terlalu panjang dilewati.
	hasil, err := u.AksiMassal(t.Context(), AksiPromosi, []string{"11", "arsip", "999", strings.Repeat("x", 65)}, uid("u-1"))

	require.NoError(t, err)
	assert.Equal(t, 2, hasil.Berhasil)
	assert.ElementsMatch(t, []string{"kandidat", "arsip"}, repo.dipromosikan)
	require.Len(t, hasil.Gagal, 2)
	assert.Equal(t, GagalModel{ID: "999", Alasan: "model tidak ditemukan"}, hasil.Gagal[0], "ID yang tidak ada")
	assert.Equal(t, strings.Repeat("x", 65), hasil.Gagal[1].ID, "rujukan ditampilkan persis seperti dikirim")
	assert.Equal(t, "model tidak ditemukan", hasil.Gagal[1].Alasan)
	for _, e := range rec.Entries() {
		assert.Contains(t, []string{"11", "12"}, e.EntitasID, "audit selalu memakai ID BIGINT, juga bila rujukannya kode")
	}
}
