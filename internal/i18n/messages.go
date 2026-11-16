package i18n

// Messages is every piece of text the editor shows, in one language.
//
// Menu items and buttons mark their accelerator key with "&", as in "&File"
// or "ファイル(&F)"; the "&" is not shown. Fields that are format strings say
// what their verbs stand for.
type Messages struct {
	// Command line
	Usage        string // %s: program name
	FlagLang     string
	FlagVersion  string
	FlagTabWidth string
	FlagTabs     string
	FlagNoWrap   string
	BadLang      string // %s: the value given to -lang

	// Menu bar and menus
	MenuFile, MenuEdit, MenuView, MenuHelp string

	New, Open, Save, SaveAs, Close, Exit string

	Undo, Redo, Cut, Copy, Paste    string
	Find, Replace, FindNext         string
	GoToLine, SelectAll             string
	WordWrap                        string // %s: On or Off
	Indent                          string // %s: Spaces or Tabs
	TabWidth                        string // %d: width
	LineEndings                     string // %s: LF or CRLF
	DocumentList, KeyList, AboutApp string

	On, Off      string
	Spaces, Tabs string

	// Status bar
	StateNew, StateEncrypted, StatePlain string
	Position                             string // %d: line, %d: column
	Untitled                             string // %d: number

	// Buttons
	OK, Cancel, SaveButton, DontSave, Overwrite string

	// Opening and saving
	ErrorTitle  string
	CannotOpen  string // %s: file name
	CannotSave  string // %s: file name
	NotUTF8     string
	IsFolder    string
	TooLarge    string // %d: limit in MiB
	PlainNotice string
	Decrypting  string
	Saving      string
	Saved       string // %s: file name

	PasswordTitle string
	PasswordNote  string // %s: file name
	PasswordLabel string
	EnterPassword string
	WrongPassword string
	CannotDecrypt string // %s: reason

	NewPasswordTitle string
	NewPasswordNote  string // %s: file name
	NewPassword      string
	ConfirmPassword  string
	PasswordsDiffer  string

	SaveChangesTitle string
	SaveChanges      string // %s: file name

	// File dialog
	OpenTitle        string
	SaveAsTitle      string
	FileName         string
	OpenHint         string
	SaveHint         string
	CannotOpenFolder string // %s: reason
	FileNotFound     string
	AlreadyExists    string // %s: file name
	NoSuchFolder     string

	// Find bar
	FindLabel    string
	ReplaceLabel string
	MatchCase    string
	WholeWord    string
	Regex        string
	MatchCount   string // %d: number of matches
	NoMatches    string
	Replaced     string // %d: number of replacements
	BadRegex     string
	NotFound     string // %s: search term
	FindHint     string

	// Other dialogs and notices
	NothingToUndo   string
	NothingToRedo   string
	GoToLineTitle   string
	LineNumber      string // %d: last line
	EnterLineNumber string
	DocumentsTitle  string
	KeysTitle       string
	KeysText        string
	AboutTitle      string
	AboutText       string // %s: version
}
