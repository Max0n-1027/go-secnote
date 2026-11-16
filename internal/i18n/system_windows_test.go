package i18n

import "testing"

// Windows always has a display language, so the call must come back with
// at least one.
func TestSystemLangs(t *testing.T) {
	langs := systemLangs()
	if len(langs) == 0 {
		t.Fatal("no display language reported")
	}
	t.Logf("display languages: %q", langs)
	for _, tag := range langs {
		if tag == "" {
			t.Errorf("empty language tag in %q", langs)
		}
	}
}
