package model

import (
	"github.com/go-playground/validator/v10"
	"veverse-api/translation"
)

type IError struct {
	Field   string
	Tag     string
	Value   string
	Message string
}

type ErrorResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func GetErrors(err error) []*IError {
	var errors []*IError
	for _, err := range err.(validator.ValidationErrors) {
		var el IError
		el.Field = err.Field()
		el.Tag = err.Tag()
		el.Value = err.Param()
		el.Message = err.Translate(translation.Trans)

		errors = append(errors, &el)
	}

	return errors
}
