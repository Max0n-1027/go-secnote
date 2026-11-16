package i18n

import "golang.org/x/sys/windows"

// systemLangs returns the user's preferred display languages, most
// preferred first. Console programs on Windows rarely see LANG, so this is
// what usually decides there.
func systemLangs() []string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return nil
	}
	return langs
}
