package apperrors

import (
	"context"
	"encoding/json"
	"net/http"
)

// Standard error codes.
const (
	CodeInvalidParams     = "INVALID_PARAMS"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeNotFound          = "NOT_FOUND"
	CodeConflict          = "CONFLICT"
	CodeInternalError     = "INTERNAL_ERROR"
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeTokenExpired      = "TOKEN_EXPIRED"
	CodeDuplicateLicense = "DUPLICATE_LICENSE"
	CodeMerchantFrozen   = "MERCHANT_FROZEN"
	CodeMerchantClosed   = "MERCHANT_CLOSED"
	CodeAccountLocked     = "ACCOUNT_LOCKED"
	CodeSignatureInvalid  = "SIGNATURE_INVALID"
	CodeSignatureMissing  = "SIGNATURE_MISSING"
	CodeAppKeyInvalid     = "APPKEY_INVALID"
)

var codeToStatus = map[string]int{
	CodeInvalidParams:     http.StatusBadRequest,
	CodeUnauthorized:      http.StatusUnauthorized,
	CodeForbidden:         http.StatusForbidden,
	CodeNotFound:          http.StatusNotFound,
	CodeConflict:          http.StatusConflict,
	CodeInternalError:     http.StatusInternalServerError,
	CodeInvalidCredentials: http.StatusUnauthorized,
	CodeTokenExpired:      http.StatusUnauthorized,
	CodeDuplicateLicense: http.StatusConflict,
	CodeMerchantFrozen:   http.StatusForbidden,
	CodeMerchantClosed:   http.StatusForbidden,
	CodeAccountLocked:     http.StatusTooManyRequests,
	CodeSignatureInvalid:  http.StatusUnauthorized,
	CodeSignatureMissing:  http.StatusBadRequest,
	CodeAppKeyInvalid:     http.StatusUnauthorized,
}

// AppError is a structured application error.
type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + " (" + e.Err.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

// HTTPStatus returns the HTTP status code for this error.
func (e *AppError) HTTPStatus() int {
	if s, ok := codeToStatus[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// ErrorResponse is the standard JSON error response format.
type ErrorResponse struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id"`
}

// WriteError writes a standardized JSON error response.
func WriteError(w http.ResponseWriter, r *http.Request, appErr *AppError) {
	requestID := RequestIDFromContext(r.Context())
	if requestID == "" {
		requestID = w.Header().Get("X-Request-ID")
	}

	resp := ErrorResponse{
		Code:      appErr.Code,
		Message:   appErr.Message,
		Data:      nil,
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus())
	json.NewEncoder(w).Encode(resp)
}

// Helper constructors.

func NewValidationError(msg string) *AppError {
	return &AppError{Code: CodeInvalidParams, Message: msg}
}

func NewUnauthorizedError(msg string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: msg}
}

func NewForbiddenError(msg string) *AppError {
	return &AppError{Code: CodeForbidden, Message: msg}
}

func NewNotFoundError(msg string) *AppError {
	return &AppError{Code: CodeNotFound, Message: msg}
}

func NewConflictError(msg string) *AppError {
	return &AppError{Code: CodeConflict, Message: msg}
}

func NewDuplicateLicenseError(msg string) *AppError {
	return &AppError{Code: CodeDuplicateLicense, Message: msg}
}

func NewInternalError(msg string, err error) *AppError {
	return &AppError{Code: CodeInternalError, Message: msg, Err: err}
}

func NewSignatureInvalidError(msg string) *AppError {
	return &AppError{Code: CodeSignatureInvalid, Message: msg}
}

func NewSignatureMissingError(msg string) *AppError {
	return &AppError{Code: CodeSignatureMissing, Message: msg}
}

func NewAppKeyInvalidError(msg string) *AppError {
	return &AppError{Code: CodeAppKeyInvalid, Message: msg}
}

// NewAppError creates an AppError with an explicit code.
func NewAppError(code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// Context helpers for request ID.

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext retrieves the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
