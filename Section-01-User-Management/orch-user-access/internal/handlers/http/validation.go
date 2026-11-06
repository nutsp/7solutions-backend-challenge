package httphandler

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	playground "github.com/go-playground/validator/v10"

	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/validator"
)

// Orch owns REST field rules. Complete registration before serving requests.
func newRequestValidator() *playground.Validate {
	engine := validator.New(validator.Config{})
	rules := map[string]playground.Func{
		"validutf8": func(f playground.FieldLevel) bool {

			return utf8.ValidString(f.Field().String())
		},
		"nocontrol": func(f playground.FieldLevel) bool {
			return !strings.ContainsFunc(f.Field().String(), unicode.IsControl)
		},
		"minbytes": func(f playground.FieldLevel) bool {
			n, err := strconv.Atoi(f.Param())
			return err == nil && len(f.Field().String()) >= n
		},
		"maxbytes": func(f playground.FieldLevel) bool {
			n, err := strconv.Atoi(f.Param())
			return err == nil && len(f.Field().String()) <= n
		},
		"barehex": func(f playground.FieldLevel) bool {

			_, err := hex.DecodeString(f.Field().String())
			return err == nil
		},
	}
	for tag, rule := range rules {
		if err := engine.RegisterValidation(tag, rule); err != nil {
			panic(err)
		}
	}
	return engine
}

func validateRequest(engine *playground.Validate, input any) error {
	switch value := input.(type) {
	case *registerJSON:
		value.Name = strings.TrimSpace(value.Name)
		value.Email = strings.ToLower(strings.TrimSpace(value.Email))
	case *loginJSON:
		value.Email = strings.ToLower(strings.TrimSpace(value.Email))
	case *patchJSON:
		if value.Name == nil && value.Email == nil {
			return apperrors.ErrInvalid
		}
		if value.Name != nil {
			normalized := strings.TrimSpace(*value.Name)
			value.Name = &normalized
		}
		if value.Email != nil {
			normalized := strings.ToLower(strings.TrimSpace(*value.Email))
			value.Email = &normalized
		}
	}
	if err := engine.Struct(input); err != nil {
		return apperrors.ErrInvalid
	}
	return nil
}

func validUserID(engine *playground.Validate, id string) bool {
	return engine.Var(id, "required,len=32,hexadecimal,barehex") == nil
}
