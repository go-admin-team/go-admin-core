/*
 * @Author: lwnmengjing
 * @Date: 2021/6/9 10:59 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/9 10:59 上午
 */

package language

import (
	"sort"
	"strconv"
	"strings"
)

type language struct {
	name    string
	quality float64
}

type languageSlice []language

func (e languageSlice) SortByQuality() {
	sort.Sort(e)
}

func (e languageSlice) Len() int {
	return len(e)
}

func (e languageSlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e languageSlice) Less(i, j int) bool {
	return e[i].quality > e[j].quality
}

// ParseAcceptLanguage returns RFC1766 language codes parsed and sorted from
// languages.
//
// If supportedLanguages is not empty, the returned codes will be filtered
// by its contents.
func ParseAcceptLanguage(languages string, supportedLanguages []string) []string {
	preferredLanguages := strings.Split(languages, ",")
	preferredLanguagesLen := len(preferredLanguages)

	// Preallocate processed languages, as we know the maximum possible.
	langCap := preferredLanguagesLen
	if len(supportedLanguages) > 0 {
		langCap = len(supportedLanguages)
	}
	langs := make(languageSlice, 0, langCap)

	for i, rawPreferredLanguage := range preferredLanguages {
		// Format strings.
		preferredLanguage := strings.Replace(strings.ToLower(strings.TrimSpace(rawPreferredLanguage)), "_", "-", 0)

		if preferredLanguage == "" {
			continue
		}

		// Split out quality factor.
		parts := strings.SplitN(preferredLanguage, ";", 2)

		// If supported languages are given, return only the langs that fit.
		supported := len(supportedLanguages) == 0
		for _, supportedLanguage := range supportedLanguages {
			if supported = supportedLanguage == parts[0]; supported {
				break
			}
		}

		if !supported {
			continue
		}

		lang := language{parts[0], 0}
		if len(parts) == 2 {
			q := parts[1]

			if strings.HasPrefix(q, "q=") {
				q = strings.SplitN(q, "=", 2)[1]
				var err error
				if lang.quality, err = strconv.ParseFloat(q, 64); err != nil {
					// Default value (1) if quality is empty.
					lang.quality = 1
				}
			}
		}

		// Use order of items if no quality is given.
		if lang.quality == 0 {
			lang.quality = float64(preferredLanguagesLen - i)
		}

		langs = append(langs, lang)

	}

	langs.SortByQuality()

	// Filter quality string.
	langString := make([]string, 0, len(langs))
	for _, lang := range langs {
		langString = append(langString, lang.name)
	}

	return langString

}
