package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func performRequest(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	handler(c)
	return w
}

func TestSuccess(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Success(c, map[string]string{"key": "value"})
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Envelope
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "200", resp.Code)
	assert.Equal(t, "OK", resp.Message)
	assert.NotNil(t, resp.Data)
}

func TestSuccess_NilData(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Success(c, nil)
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Envelope
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "200", resp.Code)
}

func TestCreated(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Created(c, map[string]string{"id": "1"})
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp Envelope
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "201", resp.Code)
}

func TestNoContent(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		NoContent(c)
	})

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestError(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "gagal")
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp Envelope
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "BAD_REQUEST", resp.Code)
	assert.Equal(t, "gagal", resp.Message)
}

func TestBadRequest(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		BadRequest(c, "input salah")
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUnauthorized(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Unauthorized(c, "token tidak valid")
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestForbidden(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Forbidden(c, "tidak punya akses")
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestNotFound(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		NotFound(c, "tidak ditemukan")
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestConflict(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Conflict(c, "konflik")
	})

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInternalError(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		InternalError(c, "kesalahan server")
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
