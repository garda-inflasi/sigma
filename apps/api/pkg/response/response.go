package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the standard API response format
// Always use this for all API responses
type Envelope struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	// ML adalah lampiran hasil pipeline ML (docs/PIPELINE-DATA-ML.md §5): hanya ada pada respons Success endpoint
	// yang diperkaya, berdampingan dengan data yang isinya tidak pernah diubah. Kosong: kunci tidak dikirim.
	ML interface{} `json:"ml,omitempty"`
	// DataEnv mencatat mode sumber data yang diminta ("staging" | "prod") bila dikirim pemanggil.
	DataEnv string `json:"data_env,omitempty"`
}

// kunciLampiranML adalah kunci gin.Context untuk penyusun lampiran ML (LampirkanML).
const (
	kunciLampiranML    = "response.lampiran_ml"
	kunciPenyaringData = "response.penyaring_data"
	kunciDataEnv       = "response.data_env"
)

// PasangDataEnv mencatat mode sumber data aktif pada gin.Context dan header respons.
func PasangDataEnv(c *gin.Context, env string) {
	if env == "" {
		return
	}
	c.Set(kunciDataEnv, env)
	c.Header("X-Garda-Data-Env", env)
}

func bacaDataEnv(c *gin.Context) string {
	v, _ := c.Get(kunciDataEnv)
	s, _ := v.(string)
	return s
}

// PasangPenyaringData mendaftarkan transformasi aman-panic untuk payload Data saat mode prod aktif (mis. membuang baris DEMO).
func PasangPenyaringData(c *gin.Context, fn func(interface{}) interface{}) {
	c.Set(kunciPenyaringData, fn)
}

func saringData(c *gin.Context, data interface{}) (hasil interface{}) {
	v, ok := c.Get(kunciPenyaringData)
	if !ok {
		return data
	}
	fn, ok := v.(func(interface{}) interface{})
	if !ok || fn == nil {
		return data
	}
	defer func() {
		if recover() != nil {
			hasil = data
		}
	}()
	return fn(data)
}

// LampirkanML memasang penyusun lampiran `ml` untuk respons Success request ini. Penyusun baru dipanggil saat respons
// sukses ditulis (galat tidak memicu bacaan ML), dan kegagalan atau panic di dalamnya tidak pernah menggagalkan
// respons: lampiran dibuang dan data tetap terkirim seperti sebelumnya.
func LampirkanML(c *gin.Context, penyusun func() interface{}) {
	c.Set(kunciLampiranML, penyusun)
}

func lampiranML(c *gin.Context) (hasil interface{}) {
	v, ok := c.Get(kunciLampiranML)
	if !ok {
		return nil
	}
	penyusun, ok := v.(func() interface{})
	if !ok || penyusun == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			hasil = nil
		}
	}()
	return penyusun()
}

// Success sends a 200 OK response
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{
		Code:    "200",
		Message: "OK",
		Data:    saringData(c, data),
		ML:      lampiranML(c),
		DataEnv: bacaDataEnv(c),
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{
		Code:    "201",
		Message: "Created",
		Data:    data,
	})
}

// Accepted sends a 202 Accepted response: the work continues in the background.
func Accepted(c *gin.Context, data interface{}) {
	c.JSON(http.StatusAccepted, Envelope{
		Code:    "202",
		Message: "Accepted",
		Data:    data,
	})
}

// NoContent sends a 204 No Content response
func NoContent(c *gin.Context) {
	c.JSON(http.StatusNoContent, Envelope{
		Code:    "204",
		Message: "No Content",
	})
}

// Error sends an error response with the given HTTP status
func Error(c *gin.Context, httpStatus int, code, message string) {
	c.JSON(httpStatus, Envelope{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData sends an error response that carries a data payload.
func ErrorWithData(c *gin.Context, httpStatus int, code, message string, data interface{}) {
	c.JSON(httpStatus, Envelope{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "400", message)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "401", message)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "403", message)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "404", message)
}

// Conflict sends a 409 Conflict response
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, "409", message)
}

// InternalError sends a 500 Internal Server Error response
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "500", message)
}
