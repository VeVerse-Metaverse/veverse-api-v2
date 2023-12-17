package translation

import (
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
)

var (
	uni   *ut.UniversalTranslator
	Trans ut.Translator
)

func InitTranslation() {
	english := en.New()
	uni = ut.New(english, english)
	Trans, _ = uni.GetTranslator("en")
}
