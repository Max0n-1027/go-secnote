package i18n

var ja = Messages{
	Usage:        "使い方: %s [オプション] [ファイル ...]\n\nオプション:\n",
	FlagLang:     "表示言語 (ja または en)。省略時は LANG などの環境から決める",
	FlagVersion:  "バージョンを表示して終了する",
	FlagTabWidth: "タブ幅 (1-16)",
	FlagTabs:     "インデントにスペースではなくタブ文字を使う",
	FlagNoWrap:   "右端で折り返さない",
	BadLang:      "-lang に指定できるのは ja か en です: %s",

	MenuFile: "ファイル(&F)",
	MenuEdit: "編集(&E)",
	MenuView: "表示(&V)",
	MenuHelp: "ヘルプ(&H)",

	New:    "新規(&N)",
	Open:   "開く(&O)...",
	Save:   "上書き保存(&S)",
	SaveAs: "名前を付けて保存(&A)...",
	Close:  "閉じる(&C)",
	Exit:   "終了(&X)",

	Undo:         "元に戻す(&U)",
	Redo:         "やり直し(&R)",
	Cut:          "切り取り(&T)",
	Copy:         "コピー(&C)",
	Paste:        "貼り付け(&P)",
	Find:         "検索(&F)...",
	Replace:      "置換(&E)...",
	FindNext:     "次を検索(&N)",
	GoToLine:     "行へ移動(&G)...",
	SelectAll:    "すべて選択(&A)",
	WordWrap:     "右端で折り返す(&W): %s",
	Indent:       "インデント(&I): %s",
	TabWidth:     "タブ幅(&T): %d",
	LineEndings:  "改行コード(&E): %s",
	DocumentList: "ドキュメント一覧(&D)...",
	KeyList:      "キー操作(&K)",
	AboutApp:     "バージョン情報(&A)",

	On:     "オン",
	Off:    "オフ",
	Spaces: "スペース",
	Tabs:   "タブ",

	StateNew:       "未保存",
	StateEncrypted: "暗号化",
	StatePlain:     "平文",
	Position:       "行 %d, 列 %d",
	Untitled:       "無題 %d",

	OK:         "OK",
	Cancel:     "キャンセル",
	SaveButton: "保存(&S)",
	DontSave:   "保存しない(&N)",
	Overwrite:  "上書き(&O)",

	ErrorTitle:  "エラー",
	CannotOpen:  "開けませんでした: %s",
	CannotSave:  "保存できませんでした: %s",
	NotUTF8:     "UTF-8 のテキストではありません。",
	IsFolder:    "フォルダーです",
	TooLarge:    "ファイルが大きすぎます (上限 %d MiB)",
	PlainNotice: "暗号化されていないファイルです。保存すると暗号化した別のファイルを作ります。",
	Decrypting:  "復号しています...",
	Saving:      "暗号化して保存しています...",
	Saved:       "保存しました: %s",

	PasswordTitle: "パスワード",
	PasswordNote:  "「%s」のパスワードを入力してください。",
	PasswordLabel: "パスワード",
	EnterPassword: "パスワードを入力してください",
	WrongPassword: "パスワードが違います",
	CannotDecrypt: "復号できません: %s",

	NewPasswordTitle: "パスワードの設定",
	NewPasswordNote:  "「%s」を暗号化するパスワードを決めてください。忘れるとファイルは開けなくなります。",
	NewPassword:      "新しいパスワード",
	ConfirmPassword:  "確認のためもう一度",
	PasswordsDiffer:  "2 つのパスワードが一致しません",

	SaveChangesTitle: "変更の保存",
	SaveChanges:      "「%s」への変更を保存しますか?",

	OpenTitle:        "開く",
	SaveAsTitle:      "名前を付けて保存",
	FileName:         "ファイル名: ",
	OpenHint:         "Enter: 開く  Esc: キャンセル",
	SaveHint:         "Enter: 保存  Esc: キャンセル",
	CannotOpenFolder: "フォルダーを開けません: %s",
	FileNotFound:     "ファイルが見つかりません",
	AlreadyExists:    "「%s」は既に存在します。上書きしますか?",
	NoSuchFolder:     "保存先のフォルダーがありません",

	FindLabel:    "検索:",
	ReplaceLabel: "置換:",
	MatchCase:    "大小区別(&C)",
	WholeWord:    "単語単位(&W)",
	Regex:        "正規表現(&R)",
	MatchCount:   "%d 件",
	NoMatches:    "なし",
	Replaced:     "%d 件置換しました",
	BadRegex:     "正規表現が正しくありません",
	NotFound:     "見つかりません: %s",
	FindHint:     "Enter/F3: 次  Shift+F3: 前  Tab: 欄の移動  Alt+A: すべて置換  Esc: 閉じる",

	NothingToUndo:   "元に戻す操作はありません",
	NothingToRedo:   "やり直す操作はありません",
	GoToLineTitle:   "行へ移動",
	LineNumber:      "行番号 (1 - %d)",
	EnterLineNumber: "行番号を数字で入力してください",
	DocumentsTitle:  "ドキュメント一覧",
	KeysTitle:       "キー操作",
	KeysText: "Ctrl+N 新規    Ctrl+O 開く    Ctrl+S 上書き保存\n" +
		"Ctrl+W 閉じる  Ctrl+Q 終了    Ctrl+P ドキュメント一覧\n" +
		"Ctrl+Z 元に戻す               Ctrl+Y やり直し\n" +
		"Ctrl+X 切り取り  Ctrl+C コピー  Ctrl+V 貼り付け\n" +
		"Ctrl+A すべて選択             Ctrl+G 行へ移動\n" +
		"Ctrl+F 検索    Ctrl+R 置換    F3 / Shift+F3 次・前を検索\n" +
		"Shift+矢印キー 選択           Ctrl+Left / Right 単語単位で移動\n" +
		"Ctrl+Home / End 文頭・文末    Ctrl+Up / Down スクロール\n" +
		"Tab / Shift+Tab インデントの増減\n" +
		"Alt+Z 右端で折り返す          F10 メニュー",
	AboutTitle: "バージョン情報",
	AboutText: "secnote %s\n" +
		"パスワードで暗号化して保存するテキストエディター\n\n" +
		"ファイル形式: age (scrypt パスフレーズ)",
}
