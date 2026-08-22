/*
 * @Author: lwnmengjing
 * @Date: 2021/6/9 10:39 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/9 10:39 上午
 */

package api

import (
	"errors"
	"strings"
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	chTranslations "github.com/go-playground/validator/v10/translations/zh"
)

// translators are built once. transInit rebuilt them on every call and
// registered them onto the process-wide validator each time, which is both
// wasted work and a write to shared state from a request goroutine.
var (
	transOnce   sync.Once
	translators map[string]ut.Translator
)

func buildTranslators() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	zhT, enT := zh.New(), en.New()
	uni := ut.New(enT, zhT, enT)

	translators = make(map[string]ut.Translator, 2)
	for tag, register := range map[string]func(*validator.Validate, ut.Translator) error{
		"zh": chTranslations.RegisterDefaultTranslations,
		"en": enTranslations.RegisterDefaultTranslations,
	} {
		t, found := uni.GetTranslator(tag)
		if !found {
			continue
		}
		if err := register(v, t); err != nil {
			continue
		}
		translators[tag] = t
	}
}

// translatorFor returns the translator for a locale tag such as zh-cn, falling
// back to English. A nil result means the process validator is not the one
// these translations were registered against, in which case the caller keeps
// the untranslated error rather than losing it.
func translatorFor(locale string) ut.Translator {
	transOnce.Do(buildTranslators)

	if t, ok := translators[locale]; ok {
		return t
	}
	if base, _, found := strings.Cut(locale, "-"); found {
		if t, ok := translators[base]; ok {
			return t
		}
	}
	return translators["en"]
}

// TranslateValidationErrors renders validation failures in the language the
// request asked for. Anything that is not a validation error is returned
// unchanged.
func TranslateValidationErrors(err error, locale string) error {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err
	}

	t := translatorFor(locale)
	if t == nil {
		return err
	}

	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		msgs = append(msgs, fe.Translate(t))
	}
	return errors.New(strings.Join(msgs, "; "))
}
