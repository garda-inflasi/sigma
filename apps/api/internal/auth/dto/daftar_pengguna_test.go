package dto

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bind meniru Handler.ListUsers: ShouldBindQuery pada gin context dengan query string.
func bind(t *testing.T, query string) (ListUserRequest, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/users?"+query, nil)
	var req ListUserRequest
	return req, c.ShouldBindQuery(&req)
}

func TestListUserRequest_BindMengisiFilterYangDisematkan(t *testing.T) {
	req, err := bind(t, "search=ani&peran=ADMIN_SISTEM&status=AKTIF&lapis=SISTEM&mfa=true&terkunci=false&sort=nama&arah=desc&page=3&limit=25")
	require.NoError(t, err)

	q, err := req.Query()
	require.NoError(t, err)
	assert.Equal(t, "ani", q.Filter.Cari)
	assert.Equal(t, "ADMIN_SISTEM", q.Filter.Peran)
	assert.Equal(t, "AKTIF", q.Filter.Status)
	assert.Equal(t, "SISTEM", q.Filter.Lapis)
	require.NotNil(t, q.Filter.MFA)
	assert.True(t, *q.Filter.MFA)
	require.NotNil(t, q.Filter.Terkunci)
	assert.False(t, *q.Filter.Terkunci)
	assert.Equal(t, "nama", q.Urut)
	assert.Equal(t, "desc", q.Arah)
	assert.Equal(t, 3, q.Page)
	assert.Equal(t, 25, q.Limit)
}

func TestListUserRequest_TanpaParameterBerarti_TanpaFilter(t *testing.T) {
	req, err := bind(t, "")
	require.NoError(t, err)

	q, err := req.Query()
	require.NoError(t, err)
	assert.Equal(t, domain.DaftarPenggunaQuery{}, q)
}

func TestListUserRequest_ParameterKosongSamaDenganTidakAda(t *testing.T) {
	req, err := bind(t, "search=&peran=&status=&lapis=&mfa=&terkunci=&sort=&arah=")
	require.NoError(t, err)

	q, err := req.Query()
	require.NoError(t, err)
	assert.Equal(t, domain.DaftarPenggunaQuery{}, q, "mfa= kosong tidak boleh menjadi false")
}

func TestListUserRequest_BatasHalamanDanLimit(t *testing.T) {
	for _, ok := range []string{"limit=1", "limit=100", "page=1", "page=999999", "limit=0", "page=0"} {
		_, err := bind(t, ok)
		assert.NoError(t, err, ok)
	}
	for _, salah := range []string{"limit=101", "limit=-1", "page=-1", "limit=abc", "page=1.5"} {
		_, err := bind(t, salah)
		assert.Error(t, err, salah)
	}
}

// Batas limit di tag binding harus sama dengan konstanta domain yang dipakai dokumentasi dan klien.
func TestListUserRequest_TagLimitSamaDenganKonstantaDomain(t *testing.T) {
	f, ok := reflect.TypeOf(ListUserRequest{}).FieldByName("Limit")
	require.True(t, ok)
	assert.Contains(t, f.Tag.Get("binding"), "max="+strconv.Itoa(domain.MaksLimitPengguna))
}

func TestFilterPenggunaRequest_BooleanHarusPersisTrueAtauFalse(t *testing.T) {
	for _, nilai := range []string{"1", "0", "TRUE", "True", "ya", "yes", " true", "null"} {
		t.Run("mfa="+nilai, func(t *testing.T) {
			_, err := FilterPenggunaRequest{MFA: nilai}.Filter()

			var galat *domain.FilterPenggunaError
			require.True(t, errors.As(err, &galat))
			assert.Equal(t, "mfa", galat.Param)
		})
		t.Run("terkunci="+nilai, func(t *testing.T) {
			_, err := FilterPenggunaRequest{Terkunci: nilai}.Filter()

			var galat *domain.FilterPenggunaError
			require.True(t, errors.As(err, &galat))
			assert.Equal(t, "terkunci", galat.Param)
		})
	}
}

func TestFilterPenggunaRequest_BooleanTriState(t *testing.T) {
	f, err := FilterPenggunaRequest{}.Filter()
	require.NoError(t, err)
	assert.Nil(t, f.MFA)
	assert.Nil(t, f.Terkunci)

	f, err = FilterPenggunaRequest{MFA: "false", Terkunci: "true"}.Filter()
	require.NoError(t, err)
	require.NotNil(t, f.MFA)
	assert.False(t, *f.MFA)
	require.NotNil(t, f.Terkunci)
	assert.True(t, *f.Terkunci)
}

// FilterPenggunaRequest dipakai ekspor tanpa ListUserRequest: ia bisa disematkan sendiri-sendiri.
func TestFilterPenggunaRequest_DapatDisematkanDiRequestLain(t *testing.T) {
	type eksporRequest struct {
		FilterPenggunaRequest
		Format string `form:"format"`
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/users/ekspor?status=NONAKTIF&mfa=false&format=csv", nil)
	var req eksporRequest
	require.NoError(t, c.ShouldBindQuery(&req))

	f, err := req.Filter()
	require.NoError(t, err)
	assert.Equal(t, "NONAKTIF", f.Status)
	require.NotNil(t, f.MFA)
	assert.False(t, *f.MFA)
	assert.Equal(t, "csv", req.Format)
}

func TestToRiwayatPeranResponse(t *testing.T) {
	waktu := time.Date(2026, 11, 6, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	dari, id, nama := "OPS_MAHASISWA", uid("admin-1"), "Admin Satu"
	res := ToRiwayatPeranResponse([]domain.PerubahanPeran{
		{AuditID: "a-2", Waktu: waktu, Aksi: "ASSIGN_ROLE", Dari: &dari, Ke: "MANAJER_DATA", AktorID: &id, AktorNama: &nama},
		{AuditID: "a-1", Waktu: waktu, Aksi: "CREATE_USER", Ke: "OPS_MAHASISWA"},
		{AuditID: "a-0", Waktu: waktu, Aksi: "UPDATE_USER", Ke: "OPS_AGREGATOR", AktorNama: &nama},
	}, true)

	require.Len(t, res.Items, 3)
	assert.True(t, res.Terpotong)
	assert.Equal(t, time.UTC, res.Items[0].Waktu.Location(), "waktu dikirim dalam UTC")
	require.NotNil(t, res.Items[0].Aktor)
	assert.Equal(t, &id, res.Items[0].Aktor.ID)
	assert.Equal(t, &nama, res.Items[0].Aktor.Nama)
	assert.Nil(t, res.Items[1].Dari, "peran awal: dari kosong")
	assert.Nil(t, res.Items[1].Aktor, "tanpa pelaku: aktor null")
	require.NotNil(t, res.Items[2].Aktor, "akun aktor dihapus: nama snapshot tetap terbaca")
	assert.Nil(t, res.Items[2].Aktor.ID)

	assert.False(t, strings.Contains(string(mustJSON(t, ToRiwayatPeranResponse(nil, false))), "null"), "tanpa riwayat: items larik kosong, bukan null")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
