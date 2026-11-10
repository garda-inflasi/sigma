package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoPalsu mencatat panggilan data dan menjawab cakupan pengguna sesuai pengaturan.
type repoPalsu struct {
	mu            sync.Mutex
	pengguna      domain.PenggunaCakupan
	penggunaErr   error
	dataDipanggil []string
	qKpi          domain.DeretKpiQuery
	qHeatmap      domain.HeatmapQuery
	qCakupan      domain.Cakupan
	qPetugas      domain.AktivitasPetugasQuery
	qStok         domain.StokQuery
	qProdusen     domain.ProdusenQuery
	qLogistik     domain.LogistikQuery
}

func (r *repoPalsu) catat(nama string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dataDipanggil = append(r.dataDipanggil, nama)
}

func (r *repoPalsu) PenggunaCakupan(context.Context, string) (domain.PenggunaCakupan, error) {
	return r.pengguna, r.penggunaErr
}
func (r *repoPalsu) DeretKpi(_ context.Context, q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
	r.catat("kpi")
	r.qKpi = q
	return nil, nil
}
func (r *repoPalsu) TargetKpi(context.Context, time.Time) ([]domain.TargetKpi, error) {
	r.catat("target")
	return nil, nil
}
func (r *repoPalsu) Heatmap(_ context.Context, q domain.HeatmapQuery) (*domain.HeatmapData, error) {
	r.catat("heatmap")
	r.qHeatmap = q
	return &domain.HeatmapData{}, nil
}
func (r *repoPalsu) PembandingHarga(_ context.Context, _ time.Time, c domain.Cakupan) ([]domain.PembandingHarga, error) {
	r.catat("pembanding")
	r.qCakupan = c
	return nil, nil
}
func (r *repoPalsu) CuacaTerbaru(_ context.Context, _, _ time.Time, c domain.Cakupan) ([]domain.CuacaKabupaten, error) {
	r.catat("cuaca")
	r.qCakupan = c
	return nil, nil
}
func (r *repoPalsu) AktivitasLapangan(_ context.Context, _, _ time.Time, c domain.Cakupan) ([]domain.AktivitasKabupaten, error) {
	r.catat("aktivitas")
	r.qCakupan = c
	return nil, nil
}
func (r *repoPalsu) AktivitasPetugas(_ context.Context, q domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
	r.catat("petugas")
	r.qPetugas = q
	return nil, nil
}
func (r *repoPalsu) FaktaStok(_ context.Context, q domain.StokQuery) (domain.FaktaStok, error) {
	r.catat("stok")
	r.qStok = q
	return domain.FaktaStok{}, nil
}
func (r *repoPalsu) Produsen(_ context.Context, q domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
	r.catat("produsen")
	r.qProdusen = q
	return nil, nil
}
func (r *repoPalsu) Logistik(_ context.Context, q domain.LogistikQuery) (domain.LogistikFakta, error) {
	r.catat("logistik")
	r.qLogistik = q
	return domain.LogistikFakta{}, nil
}

var semuaJalur = []string{
	"/kpi", "/heatmap", "/aktivitas-lapangan", "/pembanding-harga", "/cuaca",
	"/aktivitas-petugas", "/stok", "/produsen", "/logistik",
}

// router merakit handler asli di atas repository palsu; middleware palsu menggantikan JWTAuth dan mengisi user_id.
func router(repo *repoPalsu) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := usecase.New(repo, logger.NewDefault(), usecase.TanpaCache())
	h := New(uc)
	r := gin.New()
	g := r.Group("/dashboard/operasional", func(c *gin.Context) { c.Set("user_id", "uuid-pengguna") })
	g.GET("/kpi", h.GetKpiOperasional)
	g.GET("/heatmap", h.GetHeatmap)
	g.GET("/aktivitas-lapangan", h.GetAktivitasLapangan)
	g.GET("/pembanding-harga", h.GetPembandingHarga)
	g.GET("/cuaca", h.GetCuaca)
	g.GET("/aktivitas-petugas", h.GetAktivitasPetugas)
	g.GET("/stok", h.GetStok)
	g.GET("/produsen", h.GetProdusen)
	g.GET("/logistik", h.GetLogistik)
	return r
}

func get(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(nethttp.MethodGet, "/dashboard/operasional"+path, nil))
	return w
}

type amplop struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func bacaAmplop(t *testing.T, w *httptest.ResponseRecorder) amplop {
	t.Helper()
	var a amplop
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &a), w.Body.String())
	return a
}

// Galat saat membaca cakupan dijawab 500 untuk SEMUA endpoint, dan tidak ada data yang disentuh: gagal-tutup, bukan
// diteruskan dengan cakupan yang lebih luas.
func TestCakupanGagalDibaca_Semua500TanpaMenyentuhData(t *testing.T) {
	repo := &repoPalsu{
		// Peran yang membuka seluruh Aceh: bila galat diabaikan, inilah yang akan bocor.
		pengguna:    domain.PenggunaCakupan{Peran: "MANAJER_DATA"},
		penggunaErr: errors.New("koneksi basis data putus"),
	}
	r := router(repo)

	for _, jalur := range semuaJalur {
		w := get(r, jalur)
		assert.Equal(t, nethttp.StatusInternalServerError, w.Code, jalur)
		assert.Equal(t, "Gagal menentukan cakupan data", bacaAmplop(t, w).Message, jalur)
		assert.NotContains(t, w.Body.String(), "koneksi basis data", "rincian galat internal tidak bocor ke klien: "+jalur)
	}
	assert.Empty(t, repo.dataDipanggil, "tidak satu pun query data dijalankan tanpa cakupan yang pasti")
}

func TestParameterTidakValidDijawab400DenganPesanJelas(t *testing.T) {
	repo := &repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}
	r := router(repo)

	cases := []struct {
		query, pesan string
	}{
		{"?kabupaten_kode=abc", "kabupaten_kode harus 4 digit"},
		{"?komoditas_kode=cabai", "komoditas_kode harus berupa huruf besar"},
		{"?dari=kemarin", "dari harus berformat YYYY-MM-DD"},
		{"?sampai=2026-02-30", "sampai harus berformat YYYY-MM-DD"},
		{"?dari=2999-01-01", "dari tidak boleh setelah hari ini"},
		{"?sampai=2999-01-01", "sampai tidak boleh setelah hari ini"},
		{"?dari=2026-11-06&sampai=2026-11-05", "dari tidak boleh setelah sampai"},
		{"?dari=2000-01-01&sampai=2026-11-05", "paling lama 90 hari"},
	}
	for _, jalur := range []string{"/kpi", "/heatmap"} {
		for _, c := range cases {
			w := get(r, jalur+c.query)
			require.Equal(t, nethttp.StatusBadRequest, w.Code, jalur+c.query)
			a := bacaAmplop(t, w)
			assert.Equal(t, "400", a.Code)
			assert.Contains(t, a.Message, c.pesan, jalur+c.query)
		}
	}
	assert.Empty(t, repo.dataDipanggil, "parameter tidak valid ditolak sebelum menyentuh data")
}

func TestKpiDanHeatmapMenerimaFilterDariQuery(t *testing.T) {
	repo := &repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}
	r := router(repo)

	w := get(r, "/kpi?kabupaten_kode=1171&komoditas_kode=CBM-KRT")
	require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "1171", repo.qKpi.KabupatenKode)
	assert.Equal(t, "CBM-KRT", repo.qKpi.KomoditasKode)
	assert.Equal(t, domain.CakupanSemua(), repo.qKpi.Cakupan)

	w = get(r, "/heatmap?kabupaten_kode=1108&komoditas_kode=TMT&sampai=2026-11-01")
	require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "1108", repo.qHeatmap.KabupatenKode)
	assert.Equal(t, "TMT", repo.qHeatmap.KomoditasKode)
	assert.True(t, repo.qHeatmap.Langsung)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), repo.qHeatmap.Acuan)
}

func TestSemuaEndpointMemakaiCakupanPeranDariBasisData(t *testing.T) {
	repo := &repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "OPS_KOORDINATOR", KabupatenKode: "1171"}}
	r := router(repo)

	for _, jalur := range semuaJalur {
		require.Equal(t, nethttp.StatusOK, get(r, jalur).Code, jalur)
	}

	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qKpi.Cakupan)
	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qHeatmap.Cakupan)
	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qCakupan, "aktivitas, pembanding, dan cuaca")
	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qPetugas.Cakupan)
	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qStok.Cakupan)
	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qProdusen.Cakupan)
	assert.Equal(t, domain.CakupanKabupaten("1171"), repo.qLogistik.Cakupan)
	assert.ElementsMatch(t, []string{"kpi", "heatmap", "aktivitas", "pembanding", "cuaca", "petugas", "stok", "produsen", "logistik"}, repo.dataDipanggil,
		"Koordinator tidak membaca target provinsi")
}

func TestCakupanTertutupDijawab200Kosong(t *testing.T) {
	// Koordinator tanpa kabupaten tugas, akun yang tidak ada, dan peran yang belum dikenal: hasil kosong, bukan seluruh Aceh.
	cases := map[string]*repoPalsu{
		"koordinator tanpa kabupaten": {pengguna: domain.PenggunaCakupan{Peran: "OPS_KOORDINATOR"}},
		"akun tidak ada":              {penggunaErr: domain.ErrPenggunaTidakAda},
		"peran asing":                 {pengguna: domain.PenggunaCakupan{Peran: "PERAN_BARU", KabupatenKode: "1171"}},
	}
	for nama, repo := range cases {
		t.Run(nama, func(t *testing.T) {
			r := router(repo)
			for _, jalur := range semuaJalur {
				w := get(r, jalur)
				require.Equal(t, nethttp.StatusOK, w.Code, jalur)
				assert.NotEmpty(t, bacaAmplop(t, w).Data, jalur)
			}
			assert.Empty(t, repo.dataDipanggil, "tidak ada query data untuk cakupan tertutup")
		})
	}

	t.Run("bentuk kosong tetap valid", func(t *testing.T) {
		r := router(&repoPalsu{penggunaErr: domain.ErrPenggunaTidakAda})
		var kpis []map[string]any
		require.NoError(t, json.Unmarshal(bacaAmplop(t, get(r, "/kpi")).Data, &kpis))
		assert.Len(t, kpis, 4)
		assert.JSONEq(t, `{"kabupaten":[],"komoditas":[],"sel":[]}`, string(bacaAmplop(t, get(r, "/heatmap")).Data))
		assert.JSONEq(t, `[]`, string(bacaAmplop(t, get(r, "/aktivitas-lapangan")).Data))
		assert.JSONEq(t, `[]`, string(bacaAmplop(t, get(r, "/cuaca")).Data))
		assert.JSONEq(t, `{"pasangan":0,"sepakat":0,"batas_pct":10,"items":[]}`, string(bacaAmplop(t, get(r, "/pembanding-harga")).Data))

		// Ringkasan 08-B: larik selalu [] dan angka nol, bukan null. Hasilnya sama dengan basis data kosong.
		var petugas struct {
			Tanggal     string          `json:"tanggal"`
			JendelaHari int             `json:"jendela_hari"`
			Petugas     json.RawMessage `json:"petugas"`
		}
		require.NoError(t, json.Unmarshal(bacaAmplop(t, get(r, "/aktivitas-petugas")).Data, &petugas))
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, petugas.Tanggal)
		assert.Equal(t, 7, petugas.JendelaHari)
		assert.JSONEq(t, `[]`, string(petugas.Petugas))

		var stok map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(bacaAmplop(t, get(r, "/stok")).Data, &stok))
		assert.JSONEq(t, `[]`, string(stok["komoditas"]))
		assert.JSONEq(t, `{"total_ton":0,"jumlah_lot":0,"jumlah_fasilitas":0,"kapasitas_ton":0,"fasilitas_operasional":0,
			"utilisasi_pct":0,"mendekati_kedaluwarsa_ton":0,"risiko_tinggi_ton":0,"kedaluwarsa_lot":0,"kedaluwarsa_ton":0,
			"di_luar_operasional_ton":0}`, string(stok["ringkasan"]))
		assert.JSONEq(t, `7`, string(stok["ambang_kedaluwarsa_hari"]))
		assert.JSONEq(t, `0.6`, string(stok["ambang_risiko"]))

		var produsen map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(bacaAmplop(t, get(r, "/produsen")).Data, &produsen))
		assert.JSONEq(t, `[]`, string(produsen["komoditas"]))

		var logistik map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(bacaAmplop(t, get(r, "/logistik")).Data, &logistik))
		assert.JSONEq(t, `[]`, string(logistik["komoditas"]))
		assert.JSONEq(t, `[]`, string(logistik["terbaru"]))
		assert.JSONEq(t, `{"masuk":{"jumlah":0,"volume_ton":0},"keluar":{"jumlah":0,"volume_ton":0},"pindah":{"jumlah":0,"volume_ton":0}}`, string(logistik["ringkasan"]))
		assert.JSONEq(t, `{"jumlah":0,"volume_ton":0,"biaya":0,"items":[]}`, string(logistik["rute"]))
	})
}

func TestGalatDataDijawab500GenerikTanpaBocorRincian(t *testing.T) {
	repo := &repoGalatData{repoPalsu: repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}}
	gin.SetMode(gin.TestMode)
	h := New(usecase.New(repo, logger.NewDefault(), usecase.TanpaCache()))
	r := gin.New()
	g := r.Group("/dashboard/operasional", func(c *gin.Context) { c.Set("user_id", "uuid-pengguna") })
	g.GET("/kpi", h.GetKpiOperasional)
	g.GET("/cuaca", h.GetCuaca)
	g.GET("/aktivitas-petugas", h.GetAktivitasPetugas)
	g.GET("/stok", h.GetStok)
	g.GET("/produsen", h.GetProdusen)
	g.GET("/logistik", h.GetLogistik)

	for _, jalur := range []string{"/kpi", "/cuaca", "/aktivitas-petugas", "/stok", "/produsen", "/logistik"} {
		w := get(r, jalur)
		assert.Equal(t, nethttp.StatusInternalServerError, w.Code, jalur)
		assert.NotContains(t, w.Body.String(), "relasi", jalur)
		assert.NotContains(t, w.Body.String(), "tidak ada", jalur)
	}
}

// repoGalatData selalu gagal pada query data.
type repoGalatData struct{ repoPalsu }

func (r *repoGalatData) DeretKpi(context.Context, domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
	return nil, errors.New("relasi mv_kpi_operasional tidak ada")
}
func (r *repoGalatData) CuacaTerbaru(context.Context, time.Time, time.Time, domain.Cakupan) ([]domain.CuacaKabupaten, error) {
	return nil, errors.New("relasi cuaca_harian tidak ada")
}
func (r *repoGalatData) AktivitasPetugas(context.Context, domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
	return nil, errors.New("relasi pengguna tidak ada")
}
func (r *repoGalatData) FaktaStok(context.Context, domain.StokQuery) (domain.FaktaStok, error) {
	return domain.FaktaStok{}, errors.New("relasi stok_cold_storage tidak ada")
}
func (r *repoGalatData) Produsen(context.Context, domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
	return nil, errors.New("relasi agregat_harga_harian tidak ada")
}
func (r *repoGalatData) Logistik(context.Context, domain.LogistikQuery) (domain.LogistikFakta, error) {
	return domain.LogistikFakta{}, errors.New("relasi transaksi_stok tidak ada")
}

// Ringkasan 08-B: parameter salah bentuk dijawab 400 dengan pesan yang jelas dan tanpa memantulkan isian, sebelum
// cakupan dibaca dan sebelum data disentuh.
func TestRingkasanOperasional_ParameterTidakValidDijawab400(t *testing.T) {
	repo := &repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}
	r := router(repo)

	cases := []struct {
		jalur, query, pesan string
	}{
		{"/aktivitas-petugas", "?kabupaten_kode=abc", "kabupaten_kode harus 4 digit"},
		{"/aktivitas-petugas", "?kabupaten_kode=117", "kabupaten_kode harus 4 digit"},
		{"/stok", "?kabupaten_kode=abc", "kabupaten_kode harus 4 digit"},
		{"/stok", "?komoditas_kode=cabai", "komoditas_kode harus berupa huruf besar"},
		{"/produsen", "?kabupaten_kode=11711", "kabupaten_kode harus 4 digit"},
		{"/produsen", "?komoditas_kode=A", "komoditas_kode harus berupa huruf besar"},
		{"/logistik", "?komoditas_kode=drop%20table", "komoditas_kode harus berupa huruf besar"},
	}
	for _, c := range cases {
		w := get(r, c.jalur+c.query)
		require.Equal(t, nethttp.StatusBadRequest, w.Code, c.jalur+c.query)
		a := bacaAmplop(t, w)
		assert.Equal(t, "400", a.Code)
		assert.Contains(t, a.Message, c.pesan, c.jalur+c.query)
		assert.NotContains(t, w.Body.String(), "drop", "isian tidak dipantulkan: "+c.jalur+c.query)
	}
	assert.Empty(t, repo.dataDipanggil, "parameter tidak valid ditolak sebelum menyentuh data")

	// Isian kosong berarti tanpa filter, bukan galat.
	for _, jalur := range []string{"/aktivitas-petugas", "/stok", "/produsen", "/logistik"} {
		assert.Equal(t, nethttp.StatusOK, get(r, jalur+"?kabupaten_kode=&komoditas_kode=").Code, jalur)
	}
}

// Aktivitas per petugas hanya membaca kabupaten_kode: komoditas_kode (bahkan yang salah bentuk) diabaikan dan tidak
// membentuk kunci cache.
func TestAktivitasPetugas_KomoditasKodeDiabaikan(t *testing.T) {
	repo := &repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}

	w := get(router(repo), "/aktivitas-petugas?kabupaten_kode=1108&komoditas_kode=bukan%20kode")

	require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "1108", repo.qPetugas.KabupatenKode)
}

func TestRingkasanOperasional_FilterDanCakupanSampaiKeRepository(t *testing.T) {
	repo := &repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}
	r := router(repo)

	require.Equal(t, nethttp.StatusOK, get(r, "/stok?kabupaten_kode=1108&komoditas_kode=TMT").Code)
	assert.Equal(t, domain.CakupanSemua(), repo.qStok.Cakupan)
	assert.Equal(t, "1108", repo.qStok.KabupatenKode)
	assert.Equal(t, "TMT", repo.qStok.KomoditasKode)

	require.Equal(t, nethttp.StatusOK, get(r, "/produsen?kabupaten_kode=1171&komoditas_kode=BWM").Code)
	assert.Equal(t, "1171", repo.qProdusen.KabupatenKode)
	assert.Equal(t, "BWM", repo.qProdusen.KomoditasKode)

	require.Equal(t, nethttp.StatusOK, get(r, "/logistik?kabupaten_kode=1106&komoditas_kode=CBM-KRT").Code)
	assert.Equal(t, "1106", repo.qLogistik.KabupatenKode)
	assert.Equal(t, "CBM-KRT", repo.qLogistik.KomoditasKode)
}

// repoIsi menjawab keempat ringkasan dengan data supaya nama bidang JSON (kontrak ke web) terkunci.
type repoIsi struct{ repoPalsu }

var (
	waktuUji = time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	cs01     = "CS-01"
	idCS     = "aef12e36-fce3-3252-8c8a-ccdf9bd9dfcd"
	namaCS   = "Cold Storage Lambaro"
	kabCS    = "1108"
	kabNama  = "KABUPATEN ACEH BESAR"
)

func (r *repoIsi) AktivitasPetugas(context.Context, domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
	terakhir := waktuUji
	return []domain.AktivitasPetugas{{
		ID: "03dc79aa-484b-322e-a0ae-3937fb4f3a23", Nama: "Aulia Rahman", KabupatenKode: "1171", KabupatenNama: "KOTA BANDA ACEH",
		EntriHariIni: 10, TargetHarian: 14, MenungguValidasi: 19, DitolakJendela: 7, EntriTerakhirPada: &terakhir,
		TugasAktif: 3, TugasSelesai: 1, TugasTerlambat: 1,
	}, {
		ID: "29a832f8-99f5-38a2-a350-5536c6be397d", Nama: "Rizky Pratama", KabupatenKode: "1174", KabupatenNama: "KOTA LHOKSEUMAWE", TargetHarian: 14,
	}}, nil
}

func (r *repoIsi) FaktaStok(context.Context, domain.StokQuery) (domain.FaktaStok, error) {
	return domain.FaktaStok{
		Baris: []domain.BarisStok{
			{KomoditasKode: "TMT", KomoditasNama: "Tomat", FasilitasID: idCS, Kelas: domain.KelasStokBerlaku, JumlahLot: 3, VolumeTon: 79.5, MendekatiTon: 79.5, RisikoTinggiTon: 31},
			{KomoditasKode: "BWM", KomoditasNama: "Bawang merah", FasilitasID: "f2", Kelas: domain.KelasStokKedaluwarsa, JumlahLot: 1, VolumeTon: 5},
		},
		Kapasitas: domain.KapasitasStok{Fasilitas: 3, Ton: 950},
	}, nil
}

func (r *repoIsi) Produsen(context.Context, domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
	harga, hargaK, ton, periode, sumber := 41682.0, 57471.0, 3100.0, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "DEMO"
	return []domain.ProdusenFakta{
		{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", Satuan: "kg", Pilot: true,
			HargaProdusen: &harga, EntriProdusen: 50, KabProdusen: 10, HargaKonsumen: &hargaK, EntriKonsumen: 47, KabKonsumen: 9,
			ProduksiTon: &ton, ProduksiPeriode: &periode, ProduksiSumber: &sumber},
		{KomoditasKode: "BRS-MED", KomoditasNama: "Beras medium", Satuan: "kg", Pilot: true, ProduksiTon: &ton, ProduksiPeriode: &periode, ProduksiSumber: &sumber},
	}, nil
}

func (r *repoIsi) Logistik(context.Context, domain.LogistikQuery) (domain.LogistikFakta, error) {
	return domain.LogistikFakta{
		Agregat: []domain.GerakanAgregat{{KomoditasKode: "TMT", KomoditasNama: "Tomat", Jenis: domain.JenisMasuk, Jumlah: 2, VolumeTon: 40.04}},
		Terbaru: []domain.Pergerakan{{
			ID: 6, Waktu: waktuUji, Jenis: domain.JenisMasuk, KomoditasKode: "TMT", KomoditasNama: "Tomat", VolumeTon: 36,
			TujuanID: &idCS, TujuanKode: &cs01, TujuanNama: &namaCS, TujuanKabupatenKode: &kabCS, TujuanKabupatenNama: &kabNama,
		}},
		Rute: domain.RuteRingkas{Jumlah: 5, VolumeTon: 166, Biaya: 137905543, Items: []domain.RuteAktif{{
			ID: 1, AsalKode: "1106", Asal: "KABUPATEN ACEH TENGAH", TujuanKode: "1171", Tujuan: "KOTA BANDA ACEH",
			KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", VolumeTon: 42, JarakKm: 268, JamTempuh: 6,
			Biaya: 47275123, Status: "USULAN", Sumber: "DEMO", DibuatPada: waktuUji,
		}}},
	}, nil
}

// Nama bidang respons adalah kontrak ke web: tes ini gagal bila ada yang berubah nama tanpa disengaja.
func TestRingkasanOperasional_BentukJSONTerkunci(t *testing.T) {
	repo := &repoIsi{repoPalsu: repoPalsu{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}}}
	gin.SetMode(gin.TestMode)
	h := New(usecase.New(repo, logger.NewDefault(), usecase.TanpaCache()))
	r := gin.New()
	g := r.Group("/dashboard/operasional", func(c *gin.Context) { c.Set("user_id", "uuid-pengguna") })
	g.GET("/aktivitas-petugas", h.GetAktivitasPetugas)
	g.GET("/stok", h.GetStok)
	g.GET("/produsen", h.GetProdusen)
	g.GET("/logistik", h.GetLogistik)

	t.Run("aktivitas-petugas", func(t *testing.T) {
		w := get(r, "/aktivitas-petugas")
		require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
		var got struct {
			Tanggal     string            `json:"tanggal"`
			JendelaHari int               `json:"jendela_hari"`
			Petugas     []json.RawMessage `json:"petugas"`
		}
		require.NoError(t, json.Unmarshal(bacaAmplop(t, w).Data, &got))
		assert.Equal(t, 7, got.JendelaHari)
		require.Len(t, got.Petugas, 2)
		assert.JSONEq(t, `{"id":"03dc79aa-484b-322e-a0ae-3937fb4f3a23","nama":"Aulia Rahman","kabupaten_kode":"1171","kabupaten_nama":"KOTA BANDA ACEH",
			"entri_hari_ini":10,"target_harian":14,"menunggu_validasi":19,"ditolak_jendela":7,"entri_terakhir_pada":"2026-11-10T03:00:00Z",
			"tugas_aktif":3,"tugas_selesai":1,"tugas_terlambat":1}`, string(got.Petugas[0]))
		// Petugas yang belum pernah mencatat: entri_terakhir_pada null (bukan hilang, bukan waktu nol).
		assert.JSONEq(t, `{"id":"29a832f8-99f5-38a2-a350-5536c6be397d","nama":"Rizky Pratama","kabupaten_kode":"1174","kabupaten_nama":"KOTA LHOKSEUMAWE",
			"entri_hari_ini":0,"target_harian":14,"menunggu_validasi":0,"ditolak_jendela":0,"entri_terakhir_pada":null,
			"tugas_aktif":0,"tugas_selesai":0,"tugas_terlambat":0}`, string(got.Petugas[1]))
		// Hanya nama tampilan dan ID publik: tidak ada email, telepon, atau data pribadi lain.
		for _, larangan := range []string{"email", "telepon", "nik", "password", "lembaga", "mfa"} {
			assert.NotContains(t, w.Body.String(), larangan)
		}
	})

	t.Run("stok", func(t *testing.T) {
		w := get(r, "/stok")
		require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
		var got map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(bacaAmplop(t, w).Data, &got))
		assert.JSONEq(t, `{"total_ton":79.5,"jumlah_lot":3,"jumlah_fasilitas":1,"kapasitas_ton":950,"fasilitas_operasional":3,"utilisasi_pct":8.4,
			"mendekati_kedaluwarsa_ton":79.5,"risiko_tinggi_ton":31,"kedaluwarsa_lot":1,"kedaluwarsa_ton":5,"di_luar_operasional_ton":0}`, string(got["ringkasan"]))
		assert.JSONEq(t, `[{"komoditas_kode":"TMT","komoditas_nama":"Tomat","total_ton":79.5,"jumlah_lot":3,"jumlah_fasilitas":1,
			"mendekati_kedaluwarsa_ton":79.5,"risiko_tinggi_ton":31}]`, string(got["komoditas"]))
		assert.JSONEq(t, `7`, string(got["ambang_kedaluwarsa_hari"]))
		assert.JSONEq(t, `0.6`, string(got["ambang_risiko"]))
	})

	t.Run("produsen", func(t *testing.T) {
		w := get(r, "/produsen")
		require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
		var got struct {
			Komoditas []json.RawMessage `json:"komoditas"`
		}
		require.NoError(t, json.Unmarshal(bacaAmplop(t, w).Data, &got))
		require.Len(t, got.Komoditas, 2)
		assert.JSONEq(t, `{"komoditas_kode":"CBM-KRT","komoditas_nama":"Cabai merah keriting","satuan":"kg",
			"produsen":{"harga":41682,"jumlah_entri":50,"jumlah_kabupaten":10},
			"konsumen":{"harga":57471,"jumlah_entri":47,"jumlah_kabupaten":9},
			"margin":{"selisih":15789,"persen_produsen":37.9,"persen_konsumen":27.5},
			"produksi":{"ton":3100,"periode":"2026-10-01","sumber":"DEMO","demo":true}}`, string(got.Komoditas[1]))
		// Produksi tanpa harga: blok harga dan margin null, bukan nol. Komoditas pilot menurut nama: Beras sebelum Cabai.
		assert.JSONEq(t, `{"komoditas_kode":"BRS-MED","komoditas_nama":"Beras medium","satuan":"kg","produsen":null,"konsumen":null,"margin":null,
			"produksi":{"ton":3100,"periode":"2026-10-01","sumber":"DEMO","demo":true}}`, string(got.Komoditas[0]))
	})

	t.Run("logistik", func(t *testing.T) {
		w := get(r, "/logistik")
		require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
		var got map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(bacaAmplop(t, w).Data, &got))
		assert.JSONEq(t, `{"masuk":{"jumlah":2,"volume_ton":40},"keluar":{"jumlah":0,"volume_ton":0},"pindah":{"jumlah":0,"volume_ton":0}}`, string(got["ringkasan"]))
		assert.JSONEq(t, `[{"komoditas_kode":"TMT","komoditas_nama":"Tomat","masuk":{"jumlah":2,"volume_ton":40},"keluar":{"jumlah":0,"volume_ton":0},"pindah":{"jumlah":0,"volume_ton":0}}]`, string(got["komoditas"]))
		// ID transaksi BIGINT ditulis sebagai string; MASUK tidak punya asal; tidak ada pelaku maupun alasan.
		assert.JSONEq(t, `[{"id":"6","waktu":"2026-11-10T03:00:00Z","jenis":"MASUK","komoditas_kode":"TMT","komoditas_nama":"Tomat","volume_ton":36,
			"asal":null,"tujuan":{"id":"aef12e36-fce3-3252-8c8a-ccdf9bd9dfcd","kode":"CS-01","nama":"Cold Storage Lambaro","kabupaten_kode":"1108","kabupaten_nama":"KABUPATEN ACEH BESAR"}}]`, string(got["terbaru"]))
		for _, larangan := range []string{"oleh", "alasan", "pelaku"} {
			assert.NotContains(t, string(got["terbaru"]), larangan)
		}
		assert.JSONEq(t, `{"jumlah":5,"volume_ton":166,"biaya":137905543,"items":[{"id":"1","kode":null,"asal_kode":"1106","asal":"KABUPATEN ACEH TENGAH",
			"tujuan_kode":"1171","tujuan":"KOTA BANDA ACEH","komoditas_kode":"CBM-KRT","komoditas_nama":"Cabai merah keriting","volume_ton":42,"jarak_km":268,
			"jam_tempuh":6,"biaya":47275123,"status":"USULAN","sumber":"DEMO","dibuat_pada":"2026-11-10T03:00:00Z"}]}`, string(got["rute"]))
	})
}
