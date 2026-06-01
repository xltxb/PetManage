package apperr

import (
	"fmt"
	"net/http"

	"pawprint/backend/internal/pkg/errcode"
)

// AppError is an application-level error with code and HTTP status.
type AppError struct {
	Code       int    `json:"code"`
	HTTPStatus int    `json:"-"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// HTTPStatusForCode maps an application error code to HTTP status.
func HTTPStatusForCode(code int) int {
	switch code {
	case errcode.Unauthenticated:
		return http.StatusUnauthorized
	case errcode.Forbidden, errcode.StoreForbidden:
		return http.StatusForbidden
	case errcode.BadRequest:
		return http.StatusBadRequest
	case errcode.NotFound:
		return http.StatusNotFound
	case errcode.StateTransitionInvalid:
		return http.StatusConflict
	case errcode.ResourceConflict, errcode.InsufficientStock, errcode.InsufficientWallet:
		return http.StatusUnprocessableEntity
	case errcode.PaymentNotEnabled:
		return http.StatusNotImplemented
	case errcode.InternalError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// New creates an AppError from a code and optional message.
func New(code int, msg ...string) *AppError {
	message := errcode.Message(code)
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	return &AppError{
		Code:       code,
		HTTPStatus: HTTPStatusForCode(code),
		Message:    message,
	}
}

// Wrap wraps an error with an error code.
func Wrap(err error, code int, msg ...string) *AppError {
	ae := New(code, msg...)
	ae.Err = err
	return ae
}

// BadRequest creates a 400/2001 validation error.
func BadRequest(msg string) *AppError {
	return New(errcode.BadRequest, msg)
}

// NotFound creates a 404/2002 not found error.
func NotFound(msg string) *AppError {
	return New(errcode.NotFound, msg)
}

// Unauthorized creates a 401/1001 auth error.
func Unauthorized(msg ...string) *AppError {
	return New(errcode.Unauthenticated, msg...)
}

// Forbidden creates a 403/1002 forbidden error.
func Forbidden(msg ...string) *AppError {
	return New(errcode.Forbidden, msg...)
}

// Internal creates a 500/5000 internal error.
func Internal(err error) *AppError {
	return Wrap(err, errcode.InternalError)
}
