package language

import (
	"reflect"
	"testing"
)

// A browser may send either form of a locale tag, and the parser normalises
// the underscore one so that it can match a supported tag. The normalisation
// asked strings.Replace for zero replacements, so it never happened and the
// underscore form matched nothing.
func TestParseAcceptLanguageNormalisesUnderscores(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		supported []string
		want      []string
	}{
		{
			name:      "an underscore tag matches its hyphen form",
			header:    "zh_CN",
			supported: []string{"zh-cn"},
			want:      []string{"zh-cn"},
		},
		{
			name:      "the hyphen form still matches",
			header:    "zh-CN",
			supported: []string{"zh-cn"},
			want:      []string{"zh-cn"},
		},
		{
			name:      "quality still orders the result",
			header:    "en;q=0.8,zh_CN;q=0.9",
			supported: []string{"zh-cn", "en"},
			want:      []string{"zh-cn", "en"},
		},
		{
			name:      "an unsupported tag is dropped",
			header:    "de_DE",
			supported: []string{"zh-cn"},
			want:      []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseAcceptLanguage(c.header, c.supported)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParseAcceptLanguage(%q, %v) = %v, want %v", c.header, c.supported, got, c.want)
			}
		})
	}
}
