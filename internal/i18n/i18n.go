// Package i18n holds the text the editor shows, in each language it
// supports, and works out which of them the user wants.
package i18n

import (
	"os"
	"strings"
)

// Lang is a supported language, named by its ISO 639-1 code.
type Lang string

const (
	English  Lang = "en"
	Japanese Lang = "ja"
)

// Langs lists every supported language.
var Langs = []Lang{English, Japanese}

// Parse maps a language tag or locale name, such as "ja", "ja-JP" or
// "ja_JP.UTF-8", to a supported language. The C and POSIX locales count as
// English. For anything else it returns English and false.
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(s)
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexAny(s, "_-"); i >= 0 {
		s = s[:i]
	}
	switch s {
	case "ja":
		return Japanese, true
	case "en", "c", "posix":
		return English, true
	}
	return English, false
}

// Detect returns the language of the user's environment. The first of the
// locale variables LC_ALL, LC_MESSAGES and LANG that is set decides, as it
// does for other programs; without them it is the display language of the
// operating system where there is a way to ask. Unsupported languages fall
// back to English.
func Detect() Lang {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(name); v != "" {
			l, _ := Parse(v)
			return l
		}
	}
	for _, tag := range systemLangs() {
		if l, ok := Parse(tag); ok {
			return l
		}
	}
	return English
}

// For returns the messages in l, or the English ones when l is not
// supported.
func For(l Lang) *Messages {
	if l == Japanese {
		return &ja
	}
	return &en
}
