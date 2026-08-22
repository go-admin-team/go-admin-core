package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type payload struct {
	Name string `json:"name" binding:"required"`
}

// bindWith runs Bind against a request carrying the given Accept-Language and
// an empty body, which fails the required tag.
func bindWith(t *testing.T, header string) error {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	if header != "" {
		req.Header.Set("Accept-Language", header)
	}
	c.Request = req

	e := &Api{Context: c}
	e.Bind(&payload{}, binding.JSON)
	return e.Errors
}

// The two halves of this — reading the header and building the translator —
// were both written and neither was called, so a validation failure came back
// in the validator's default form whatever the client asked for.
func TestValidationErrorsFollowAcceptLanguage(t *testing.T) {
	zh := bindWith(t, "zh-CN")
	if zh == nil {
		t.Fatal("an empty body passed a required field")
	}

	en := bindWith(t, "en")
	if en == nil {
		t.Fatal("an empty body passed a required field")
	}

	if !hasHan(zh.Error()) {
		t.Errorf("a zh-CN request got %q, which carries no Chinese", zh)
	}
	if !strings.Contains(en.Error(), "required") {
		t.Errorf("an en request got %q, which is not the English message", en)
	}
	if zh.Error() == en.Error() {
		t.Error("both languages produced the same message")
	}
}

// The underscore form is what #126 fixed in the parser; this is the path that
// carries it, so the two are only useful together.
func TestUnderscoreLocaleReachesTheTranslator(t *testing.T) {
	got := bindWith(t, "zh_CN")
	if got == nil {
		t.Fatal("an empty body passed a required field")
	}
	if !hasHan(got.Error()) {
		t.Errorf("zh_CN got %q, which carries no Chinese", got)
	}
}

// No header at all must still produce something a caller can read.
func TestNoAcceptLanguageStillTranslates(t *testing.T) {
	got := bindWith(t, "")
	if got == nil {
		t.Fatal("an empty body passed a required field")
	}
	if strings.Contains(got.Error(), "Key: ") {
		t.Errorf("got the untranslated validator form: %q", got)
	}
}

// hasHan reports whether s carries a Han character. Asked this way rather than
// by comparing against a literal message: the assertion is that the reply is
// in Chinese, not that the library words it one particular way — and the
// literal would be Chinese in a repository that keeps its source in English.
func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// gin lets an application turn validation off with binding.Validator = nil,
// and Engine on a nil interface panics. Nothing here has validation errors to
// translate in that state, so building no translators is the right answer —
// the alternative was taking the caller's process down from a helper.
func TestBuildTranslatorsSurvivesADisabledValidator(t *testing.T) {
	saved := binding.Validator
	binding.Validator = nil
	t.Cleanup(func() { binding.Validator = saved })

	// Called directly: buildTranslators runs once through sync.Once, and by
	// this point another test has already spent it.
	buildTranslators()
}
