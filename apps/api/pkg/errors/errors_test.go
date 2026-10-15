package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(http.StatusBadRequest, "BAD_REQUEST", "something went wrong")
	assert.NotNil(t, err)
	assert.Equal(t, "BAD_REQUEST", err.Code)
	assert.Equal(t, "something went wrong", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.HTTPStatus)
}

func TestNew_WithCause(t *testing.T) {
	cause := errors.New("inner error")
	err := New(http.StatusBadRequest, "BAD_REQUEST", "something went wrong", cause)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "inner error")
	assert.True(t, errors.Is(err, cause))
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("invalid input")
	assert.Equal(t, http.StatusBadRequest, err.HTTPStatus)
	assert.Equal(t, "400", err.Code)
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("not allowed")
	assert.Equal(t, http.StatusUnauthorized, err.HTTPStatus)
}

func TestForbidden(t *testing.T) {
	err := Forbidden("no access")
	assert.Equal(t, http.StatusForbidden, err.HTTPStatus)
}

func TestNotFound(t *testing.T) {
	err := NotFound("not found")
	assert.Equal(t, http.StatusNotFound, err.HTTPStatus)
}

func TestConflict(t *testing.T) {
	err := Conflict("conflict")
	assert.Equal(t, http.StatusConflict, err.HTTPStatus)
}

func TestInternal(t *testing.T) {
	err := Internal("server error")
	assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
}

func TestError_NoCause(t *testing.T) {
	err := New(http.StatusBadRequest, "BAD", "message")
	assert.Equal(t, "message", err.Error())
}

func TestError_WithCause(t *testing.T) {
	cause := errors.New("cause")
	err := New(http.StatusBadRequest, "BAD", "message", cause)
	assert.Equal(t, "message: cause", err.Error())
}
