package httpapi

import (
	"errors"
	"reflect"
	"strings"

	"github.com/PabloPavan/sniply_api/internal/snippets"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		field := fl.Field()
		if field.Kind() != reflect.String {
			return false
		}
		return strings.TrimSpace(field.String()) != ""
	})
	validate.RegisterValidation("trimmedemail", func(fl validator.FieldLevel) bool {
		field := fl.Field()
		if field.Kind() != reflect.String {
			return false
		}
		email := strings.TrimSpace(field.String())
		if email == "" {
			return false
		}
		if len(email) > 254 {
			return false
		}
		return validate.Var(email, "email") == nil
	})
	validate.RegisterValidation("maxlines", func(fl validator.FieldLevel) bool {
		field := fl.Field()
		if field.Kind() != reflect.String {
			return false
		}
		raw := field.String()
		lines := strings.Count(raw, "\n") + 1
		return lines <= maxLinesFromParam(fl.Param())
	})
}

type UserCreateDTO struct {
	Email    string `json:"email" validate:"required,notblank,trimmedemail"`
	Password string `json:"password" validate:"required,notblank,max=72"`
}

func (r *UserCreateDTO) Validate() error {
	if err := validate.Struct(r); err != nil {
		return validationMessage(err, map[string]map[string]string{
			"Email": {
				"required":     "email and password are required",
				"notblank":     "email and password are required",
				"trimmedemail": "invalid email",
			},
			"Password": {
				"required": "email and password are required",
				"notblank": "email and password are required",
			},
		}, "invalid request")
	}
	return nil
}

type UserUpdateDTO struct {
	Email    *string `json:"email,omitempty" validate:"omitempty,trimmedemail"`
	Password *string `json:"password,omitempty" validate:"omitempty,notblank,max=72"`
	Role     *string `json:"role,omitempty"`
}

func (r *UserUpdateDTO) Validate() error {
	if err := validate.Struct(r); err != nil {
		return validationMessage(err, map[string]map[string]string{
			"Email": {
				"trimmedemail": "invalid email",
			},
			"Password": {
				"notblank": "invalid password",
			},
		}, "invalid request")
	}
	return nil
}

type APIKeyCreateDTO struct {
	Name  string `json:"name"`
	Scope string `json:"scope" validate:"omitempty,oneof=read write read_write"`
}

func (r *APIKeyCreateDTO) Validate() error {
	if err := validate.Struct(r); err != nil {
		return validationMessage(err, map[string]map[string]string{
			"Scope": {
				"oneof": "invalid scope",
			},
		}, "invalid request")
	}
	return nil
}

type SnippetCreateDTO struct {
	Name       string              `json:"name" validate:"required,notblank,max=200"`
	Content    string              `json:"content" validate:"required,notblank,max=250000,maxlines=5000"`
	Language   string              `json:"language" validate:"omitempty,notblank,max=32"`
	Tags       []string            `json:"tags" validate:"max=20,dive,max=32"`
	Visibility snippets.Visibility `json:"visibility" validate:"omitempty,oneof=public private"`
}

func (r *SnippetCreateDTO) Validate() error {
	if err := validate.Struct(r); err != nil {
		return validationMessage(err, map[string]map[string]string{
			"Name": {
				"required": "name and content are required",
				"notblank": "name and content are required",
				"max":      "name is too long",
			},
			"Content": {
				"required": "name and content are required",
				"notblank": "name and content are required",
				"max":      "content is too long",
				"maxlines": "content has too many lines",
			},
			"Language": {
				"notblank": "invalid language",
				"max":      "invalid language",
			},
			"Tags": {
				"max": "too many tags",
			},
		}, "invalid request")
	}
	return nil
}

func validationMessage(err error, messages map[string]map[string]string, fallback string) error {
	var valErrs validator.ValidationErrors
	if !errors.As(err, &valErrs) {
		return errors.New(fallback)
	}
	for _, valErr := range valErrs {
		if fieldMessages, ok := messages[valErr.Field()]; ok {
			if msg, ok := fieldMessages[valErr.Tag()]; ok {
				return errors.New(msg)
			}
			if msg, ok := fieldMessages["*"]; ok {
				return errors.New(msg)
			}
		}
	}
	return errors.New(fallback)
}

func maxLinesFromParam(param string) int {
	n := 0
	for _, r := range param {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
