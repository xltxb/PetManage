package validator

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Register adds custom validators to the Gin binding engine.
func Register() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("phone", validatePhone)
		_ = v.RegisterValidation("storeid", validateStoreID)
	}
}

// validatePhone checks that a field looks like a Chinese mobile number.
func validatePhone(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if len(s) != 11 {
		return false
	}
	if s[0] != '1' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validateStoreID checks that a store ID field is non-empty.
func validateStoreID(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	return s != ""
}
