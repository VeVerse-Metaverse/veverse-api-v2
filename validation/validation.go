package validation

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/helper"
	"veverse-api/translation"
)

var (
	Validator *validator.Validate
)

var translationsMessages = map[string]map[string]string{}

func RegisterValidations() {
	Validator = validator.New()

	// Register custom validation
	// For uuid.UUID fields.
	err := Validator.RegisterValidation("uuid", checkUUID)
	if err != nil {
		logrus.Errorf("Err uuid validation registration %v", err)
	}

	err = Validator.RegisterValidation("hasUpper", hasUpper)
	if err != nil {
		logrus.Errorf("Err hasUpper validation registration %v", err)
	}

	err = Validator.RegisterValidation("hasLower", hasLower)
	if err != nil {
		logrus.Errorf("Err hasLower validation registration %v", err)
	}

	err = Validator.RegisterValidation("hasNumber", hasNumber)
	if err != nil {
		logrus.Errorf("Err hasNumber validation registration %v", err)
	}

	err = enTranslations.RegisterDefaultTranslations(Validator, translation.Trans)
	if err != nil {
		logrus.Errorf("Err default translations registration %v", err)
	}

	translationsMessages["en"] = make(map[string]string)
	translationsMessages["en"]["hasLower"] = "{0} should contain at least one lower case letter"
	translationsMessages["en"]["hasUpper"] = "{0} should contain at least one upper case letter"
	translationsMessages["en"]["hasNumber"] = "{0} should contain at least one digit"
	translationsMessages["en"]["uuid"] = "{0} is not a valid UUID"

	for v := range translationsMessages["en"] {
		addTranslation(v, translationsMessages["en"][v])
	}
}

func addTranslation(tag string, errMessage string) {
	registerFn := func(ut ut.Translator) error {
		return ut.Add(tag, errMessage, false)
	}

	transFn := func(ut ut.Translator, fe validator.FieldError) string {
		param := fe.Param()
		tag := fe.Tag()

		t, err := ut.T(tag, fe.Field(), param)
		if err != nil {
			return fe.(error).Error()
		}

		return t
	}

	_ = Validator.RegisterTranslation(tag, translation.Trans, registerFn, transFn)
}

func hasUpper(fl validator.FieldLevel) bool {
	var str = fl.Field().String()
	return helper.HasUppercase(str)
}

func hasLower(fl validator.FieldLevel) bool {
	var str = fl.Field().String()
	return helper.HasLowercase(str)
}

func hasNumber(fl validator.FieldLevel) bool {
	var str = fl.Field().String()
	return helper.HasNumber(str)
}

func checkUUID(fl validator.FieldLevel) bool {
	field := fl.Field().String()
	_, err := uuid.Parse(field)
	if err != nil {
		return false
	}

	return true
}
