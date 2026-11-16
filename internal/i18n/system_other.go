//go:build !windows

package i18n

// systemLangs returns nothing: outside Windows the locale variables are the
// way a terminal program learns the user's language.
func systemLangs() []string { return nil }
