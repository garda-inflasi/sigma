package idx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPtrAtauNil(t *testing.T) {
	assert.Nil(t, PtrAtauNil(""), "pelaku sistem ditulis NULL")
	id := uuid.NewString()
	got := PtrAtauNil(id)
	require.NotNil(t, got)
	assert.Equal(t, id, *got)
}

func TestParse(t *testing.T) {
	sah := map[string]ID{"1": 1, "42": 42, "9223372036854775807": 9223372036854775807}
	for teks, mau := range sah {
		got, err := Parse(teks)
		require.NoError(t, err, teks)
		assert.Equal(t, mau, got, teks)
	}
	for _, teks := range []string{"", "0", "-1", "+1", "01", " 1", "1 ", "1.0", "1e3", "abc", "HRG-1", "9223372036854775808", "99999999999999999999"} {
		_, err := Parse(teks)
		assert.ErrorIs(t, err, ErrIDTidakSah, "%q harus ditolak", teks)
	}
}

func TestParseDaftar(t *testing.T) {
	got, err := ParseDaftar([]string{" 3", "1", "", "3", "2 "})
	require.NoError(t, err)
	assert.Equal(t, []ID{3, 1, 2}, got, "kosong dibuang, ganda dirapatkan, urutan pertama dipertahankan")

	_, err = ParseDaftar([]string{"1", "x"})
	assert.ErrorIs(t, err, ErrIDTidakSah)

	kosong, err := ParseDaftar(nil)
	require.NoError(t, err)
	assert.Empty(t, kosong)
}

func TestJSON_DitulisSebagaiStringDanMenerimaAngka(t *testing.T) {
	type wadah struct {
		ID     ID   `json:"id"`
		Induk  *ID  `json:"induk"`
		Daftar []ID `json:"daftar"`
	}
	induk := ID(7)
	b, err := json.Marshal(wadah{ID: 9007199254740993, Induk: &induk, Daftar: []ID{1, 2}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"9007199254740993","induk":"7","daftar":["1","2"]}`, string(b),
		"ID di atas 2^53 tetap utuh karena ditulis sebagai string")

	var w wadah
	require.NoError(t, json.Unmarshal([]byte(`{"id":12,"induk":null,"daftar":["3",4]}`), &w))
	assert.Equal(t, ID(12), w.ID)
	assert.Nil(t, w.Induk)
	assert.Equal(t, []ID{3, 4}, w.Daftar)

	for _, salah := range []string{`{"id":"0"}`, `{"id":-1}`, `{"id":"x"}`, `{"id":1.5}`, `{"id":true}`} {
		assert.Error(t, json.Unmarshal([]byte(salah), &w), salah)
	}
}

func TestScanDanValue(t *testing.T) {
	var id ID
	for _, src := range []any{int64(5), int32(5), []byte("5"), "5"} {
		require.NoError(t, id.Scan(src))
		assert.Equal(t, ID(5), id)
	}
	require.NoError(t, id.Scan(nil))
	assert.Equal(t, ID(0), id)
	assert.Error(t, id.Scan(3.5))

	v, err := ID(8).Value()
	require.NoError(t, err)
	assert.Equal(t, int64(8), v)
}

func TestValidUUID(t *testing.T) {
	baku := uuid.NewString()
	assert.True(t, ValidUUID(baku))
	n, ok := NormalUUID("5B2F8F3E-6A1D-4C7E-9A52-3F0D1C8E7B14")
	assert.True(t, ok)
	assert.Equal(t, "5b2f8f3e-6a1d-4c7e-9a52-3f0d1c8e7b14", n)
	for _, salah := range []string{"", "superadmin", "INT-1000", "urn:uuid:" + baku, "{" + baku + "}", "5b2f8f3e6a1d4c7e9a523f0d1c8e7b14", baku + " ", "zzzzzzzz-6a1d-4c7e-9a52-3f0d1c8e7b14"} {
		assert.False(t, ValidUUID(salah), "%q", salah)
	}
}

func TestUUIDStabil_DeterministikDanVersi3(t *testing.T) {
	a := UUIDStabil("pengguna", "superadmin")
	assert.Equal(t, a, UUIDStabil("pengguna", "superadmin"), "deterministik")
	assert.NotEqual(t, a, UUIDStabil("intervensi", "superadmin"), "ruang memisahkan nilai")
	u := uuid.MustParse(a)
	assert.Equal(t, uuid.Version(3), u.Version())
	assert.Equal(t, uuid.RFC4122, u.Variant())
	// Nilai acuan: fungsi SQL garda_uuid_stabil (migrasi 000109) harus menghasilkan nilai yang sama; diuji
	// terhadap PostgreSQL di internal/platform/schematest.
	assert.Equal(t, uuid.NewMD5(NamespaceGarda, []byte("pengguna:superadmin")).String(), a)
}

func TestParamHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jalankan := func(path string, h gin.HandlerFunc) *httptest.ResponseRecorder {
		r := gin.New()
		r.GET("/x/:id", h)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	t.Run("ParamID", func(t *testing.T) {
		var dapat ID
		h := func(c *gin.Context) {
			id, ok := ParamID(c, "id", "Entri tidak ditemukan")
			if !ok {
				return
			}
			dapat = id
			c.Status(http.StatusOK)
		}
		assert.Equal(t, http.StatusOK, jalankan("/x/15", h).Code)
		assert.Equal(t, ID(15), dapat)
		w := jalankan("/x/HRG-1", h)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Entri tidak ditemukan")
	})

	t.Run("ParamUUID", func(t *testing.T) {
		var dapat string
		h := func(c *gin.Context) {
			id, ok := ParamUUID(c, "id", "Pengguna tidak ditemukan")
			if !ok {
				return
			}
			dapat = id
			c.Status(http.StatusOK)
		}
		assert.Equal(t, http.StatusOK, jalankan("/x/5B2F8F3E-6A1D-4C7E-9A52-3F0D1C8E7B14", h).Code)
		assert.Equal(t, "5b2f8f3e-6a1d-4c7e-9a52-3f0d1c8e7b14", dapat)
		assert.Equal(t, http.StatusNotFound, jalankan("/x/superadmin", h).Code)
	})

	t.Run("ParamUUIDAtauKode", func(t *testing.T) {
		var dapat RujukanUUID
		h := func(c *gin.Context) {
			r, ok := ParamUUIDAtauKode(c, "id", 20, "Intervensi tidak ditemukan")
			if !ok {
				return
			}
			dapat = r
			c.Status(http.StatusOK)
		}
		assert.Equal(t, http.StatusOK, jalankan("/x/INT-1000", h).Code)
		assert.Equal(t, RujukanUUID{Kode: "INT-1000"}, dapat)
		u := uuid.NewString()
		assert.Equal(t, http.StatusOK, jalankan("/x/"+u, h).Code)
		assert.Equal(t, RujukanUUID{ID: u}, dapat)
		assert.Equal(t, http.StatusNotFound, jalankan("/x/kode-yang-terlalu-panjang-sekali", h).Code)
	})
}
