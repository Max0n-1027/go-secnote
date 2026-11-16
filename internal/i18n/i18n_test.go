package i18n

import (
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"ja", Japanese, true},
		{"ja_JP.UTF-8", Japanese, true},
		{"ja-JP", Japanese, true},
		{"JA_jp.eucJP@cjk", Japanese, true},
		{"en", English, true},
		{"en_US.UTF-8", English, true},
		{"en-GB", English, true},
		{"C", English, true},
		{"C.UTF-8", English, true},
		{"POSIX", English, true},
		{"de_DE.UTF-8", English, false},
		{"", English, false},
	}
	for _, tt := range tests {
		if got, ok := Parse(tt.in); got != tt.want || ok != tt.ok {
			t.Errorf("Parse(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDetectFollowsLocaleVariables(t *testing.T) {
	set := func(all, messages, lang string) {
		t.Setenv("LC_ALL", all)
		t.Setenv("LC_MESSAGES", messages)
		t.Setenv("LANG", lang)
	}
	tests := []struct {
		all, messages, lang string
		want                Lang
	}{
		{"", "", "ja_JP.UTF-8", Japanese},
		{"", "", "en_US.UTF-8", English},
		{"", "ja_JP.UTF-8", "en_US.UTF-8", Japanese},
		{"C", "ja_JP.UTF-8", "ja_JP.UTF-8", English},
		{"ja_JP.UTF-8", "C", "C", Japanese},
		// The first variable that is set decides, even when its language
		// is not supported.
		{"", "", "fr_FR.UTF-8", English},
	}
	for _, tt := range tests {
		set(tt.all, tt.messages, tt.lang)
		if got := Detect(); got != tt.want {
			t.Errorf("LC_ALL=%q LC_MESSAGES=%q LANG=%q: Detect() = %q, want %q", tt.all, tt.messages, tt.lang, got, tt.want)
		}
	}

	if runtime.GOOS != "windows" {
		set("", "", "")
		if got := Detect(); got != English {
			t.Errorf("no locale variables: Detect() = %q, want English", got)
		}
	}
}

func TestFor(t *testing.T) {
	if For(Japanese) != &ja || For(English) != &en || For("fr") != &en {
		t.Fatal("For returned the wrong catalog")
	}
}

var verb = regexp.MustCompile(`%[-+# 0]*[0-9]*(\.[0-9]+)?[a-zA-Z%]`)

// Every language must say everything, with the same format verbs in the
// same order, so that a translation can never print %!d(MISSING) or drop
// an argument.
func TestCatalogsAreComplete(t *testing.T) {
	base := reflect.ValueOf(en)
	typ := base.Type()
	for _, l := range Langs {
		m := reflect.ValueOf(*For(l))
		for i := range typ.NumField() {
			name := typ.Field(i).Name
			got := m.Field(i).String()
			if got == "" {
				t.Errorf("%s: %s is empty", l, name)
				continue
			}
			want := verb.FindAllString(base.Field(i).String(), -1)
			if have := verb.FindAllString(got, -1); !reflect.DeepEqual(have, want) {
				t.Errorf("%s: %s has verbs %q, English has %q", l, name, have, want)
			}
		}
	}
}
