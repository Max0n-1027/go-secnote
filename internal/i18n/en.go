package i18n

var en = Messages{
	Usage:        "Usage: %s [options] [file ...]\n\nOptions:\n",
	FlagLang:     "display language, ja or en (default: taken from LANG and the like)",
	FlagVersion:  "print the version and exit",
	FlagTabWidth: "tab width (1-16)",
	FlagTabs:     "indent with tab characters instead of spaces",
	FlagNoWrap:   "do not wrap long lines",
	BadLang:      "-lang must be ja or en, not %s",

	MenuFile: "&File",
	MenuEdit: "&Edit",
	MenuView: "&View",
	MenuHelp: "&Help",

	New:    "&New",
	Open:   "&Open...",
	Save:   "&Save",
	SaveAs: "Save &As...",
	Close:  "&Close",
	Exit:   "E&xit",

	Undo:         "&Undo",
	Redo:         "&Redo",
	Cut:          "Cu&t",
	Copy:         "&Copy",
	Paste:        "&Paste",
	Find:         "&Find...",
	Replace:      "R&eplace...",
	FindNext:     "Find &Next",
	GoToLine:     "&Go to Line...",
	SelectAll:    "Select &All",
	WordWrap:     "&Word Wrap: %s",
	Indent:       "&Indent: %s",
	TabWidth:     "&Tab Width: %d",
	LineEndings:  "Line &Endings: %s",
	DocumentList: "&Documents...",
	KeyList:      "&Keyboard Shortcuts",
	AboutApp:     "&About",

	On:     "On",
	Off:    "Off",
	Spaces: "Spaces",
	Tabs:   "Tabs",

	StateNew:       "Not saved",
	StateEncrypted: "Encrypted",
	StatePlain:     "Plain text",
	Position:       "Ln %d, Col %d",
	Untitled:       "Untitled %d",

	OK:         "OK",
	Cancel:     "Cancel",
	SaveButton: "&Save",
	DontSave:   "Do&n't Save",
	Overwrite:  "&Overwrite",

	ErrorTitle:  "Error",
	CannotOpen:  "Could not open %s",
	CannotSave:  "Could not save %s",
	NotUTF8:     "It is not UTF-8 text.",
	IsFolder:    "it is a folder",
	TooLarge:    "the file is too large (the limit is %d MiB)",
	PlainNotice: "This file is not encrypted. Saving it writes an encrypted copy alongside.",
	Decrypting:  "Decrypting...",
	Saving:      "Encrypting and saving...",
	Saved:       "Saved %s",

	PasswordTitle: "Password",
	PasswordNote:  "Enter the password for %s.",
	PasswordLabel: "Password",
	EnterPassword: "Enter a password",
	WrongPassword: "Wrong password",
	CannotDecrypt: "Cannot decrypt: %s",

	NewPasswordTitle: "Set Password",
	NewPasswordNote:  "Choose a password to encrypt %s. If you forget it, the file cannot be opened.",
	NewPassword:      "New password",
	ConfirmPassword:  "Type it again",
	PasswordsDiffer:  "The passwords do not match",

	SaveChangesTitle: "Unsaved Changes",
	SaveChanges:      "Save changes to %s?",

	OpenTitle:        "Open",
	SaveAsTitle:      "Save As",
	FileName:         "File name: ",
	OpenHint:         "Enter: Open  Esc: Cancel",
	SaveHint:         "Enter: Save  Esc: Cancel",
	CannotOpenFolder: "Cannot open the folder: %s",
	FileNotFound:     "File not found",
	AlreadyExists:    "%s already exists. Overwrite it?",
	NoSuchFolder:     "The folder does not exist",

	FindLabel:    "Find:",
	ReplaceLabel: "Replace:",
	MatchCase:    "Match &Case",
	WholeWord:    "&Whole Word",
	Regex:        "&Regex",
	MatchCount:   "%d found",
	NoMatches:    "No matches",
	Replaced:     "%d replaced",
	BadRegex:     "Invalid regular expression",
	NotFound:     "Not found: %s",
	FindHint:     "Enter/F3: next  Shift+F3: prev  Tab: field  Alt+A: replace all  Esc: close",

	NothingToUndo:   "Nothing to undo",
	NothingToRedo:   "Nothing to redo",
	GoToLineTitle:   "Go to Line",
	LineNumber:      "Line number (1 - %d)",
	EnterLineNumber: "Enter a line number",
	DocumentsTitle:  "Documents",
	KeysTitle:       "Keyboard Shortcuts",
	KeysText: "Ctrl+N  New            Ctrl+O  Open         Ctrl+S  Save\n" +
		"Ctrl+W  Close          Ctrl+Q  Exit         Ctrl+P  Documents\n" +
		"Ctrl+Z  Undo           Ctrl+Y  Redo\n" +
		"Ctrl+X  Cut            Ctrl+C  Copy         Ctrl+V  Paste\n" +
		"Ctrl+A  Select all     Ctrl+G  Go to line\n" +
		"Ctrl+F  Find           Ctrl+R  Replace\n" +
		"F3 / Shift+F3          Next / previous match\n" +
		"Shift+arrow keys       Select\n" +
		"Ctrl+Left / Right      Move by word\n" +
		"Ctrl+Home / End        Start / end of the text\n" +
		"Ctrl+Up / Down         Scroll\n" +
		"Tab / Shift+Tab        Indent / unindent\n" +
		"Alt+Z  Word wrap       F10  Menu",
	AboutTitle: "About",
	AboutText: "secnote %s\n" +
		"A text editor that saves its files encrypted with a password\n\n" +
		"File format: age (scrypt passphrase)",
}
