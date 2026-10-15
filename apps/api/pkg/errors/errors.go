package errors

import "net/http"

// AppError represents a structured application error
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError
func New(httpStatus int, code, message string, errs ...error) *AppError {
	e := &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
	if len(errs) > 0 {
		e.Err = errs[0]
	}
	return e
}

// BadRequest creates a 400 error
func BadRequest(message string, errs ...error) *AppError {
	return New(http.StatusBadRequest, "400", message, errs...)
}

// Unauthorized creates a 401 error
func Unauthorized(message string, errs ...error) *AppError {
	return New(http.StatusUnauthorized, "401", message, errs...)
}

// Forbidden creates a 403 error
func Forbidden(message string, errs ...error) *AppError {
	return New(http.StatusForbidden, "403", message, errs...)
}

// NotFound creates a 404 error
func NotFound(message string, errs ...error) *AppError {
	return New(http.StatusNotFound, "404", message, errs...)
}

// Conflict creates a 409 error
func Conflict(message string, errs ...error) *AppError {
	return New(http.StatusConflict, "409", message, errs...)
}

// Internal creates a 500 error
func Internal(message string, errs ...error) *AppError {
	return New(http.StatusInternalServerError, "500", message, errs...)
}
