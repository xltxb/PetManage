package apperr

import (
	"errors"
	"net/http"
	"testing"

	"pawprint/backend/internal/pkg/errcode"
)

func TestNew(t *testing.T) {
	ae := New(errcode.InsufficientStock)
	if ae.Code != errcode.InsufficientStock {
		t.Errorf("Code = %d, want %d", ae.Code, errcode.InsufficientStock)
	}
	if ae.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus = %d, want %d", ae.HTTPStatus, http.StatusUnprocessableEntity)
	}
	if ae.Message != "库存不足" {
		t.Errorf("Message = %q, want %q", ae.Message, "库存不足")
	}
}

func TestNewCustomMessage(t *testing.T) {
	ae := New(errcode.InsufficientStock, "皇家幼犬粮 库存不足")
	if ae.Message != "皇家幼犬粮 库存不足" {
		t.Errorf("Message = %q", ae.Message)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("db connection refused")
	ae := Wrap(cause, errcode.InternalError)
	if ae.Code != errcode.InternalError {
		t.Errorf("Code = %d", ae.Code)
	}
	if !errors.Is(ae, cause) {
		t.Error("errors.Is should find the wrapped error")
	}
	if ae.Error() != "[5000] 服务器内部错误: db connection refused" {
		t.Errorf("Error() = %q", ae.Error())
	}
}

func TestHelperConstructors(t *testing.T) {
	tests := []struct {
		name     string
		ae       *AppError
		wantCode int
		wantHTTP int
	}{
		{"BadRequest", BadRequest("field required"), errcode.BadRequest, http.StatusBadRequest},
		{"NotFound", NotFound("user 99"), errcode.NotFound, http.StatusNotFound},
		{"Unauthorized", Unauthorized(), errcode.Unauthenticated, http.StatusUnauthorized},
		{"Forbidden", Forbidden(), errcode.Forbidden, http.StatusForbidden},
		{"Internal", Internal(errors.New("boom")), errcode.InternalError, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ae.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", tt.ae.Code, tt.wantCode)
			}
			if tt.ae.HTTPStatus != tt.wantHTTP {
				t.Errorf("HTTPStatus = %d, want %d", tt.ae.HTTPStatus, tt.wantHTTP)
			}
		})
	}
}
