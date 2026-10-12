package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"secnote/internal/vault"
)

// maxFileSize is the largest file the editor will load. Everything is held
// in memory, so something far bigger than any note is refused outright.
const maxFileSize = 64 << 20

const utf8BOM = "\xEF\xBB\xBF"

func (a *App) newDoc() *document {
	a.untitled++
	d := newDocument(&a.env, "")
	d.name = fmt.Sprintf("無題 %d", a.untitled)
	a.addDoc(d)
	return d
}

// addDoc shows d, in place of the current document if that is still an
// untouched new one.
func (a *App) addDoc(d *document) {
	if len(a.docs) > 0 && a.doc().pristine() {
		a.docs[a.cur] = d
		return
	}
	a.docs = append(a.docs, d)
	a.cur = len(a.docs) - 1
}

// startDir is where file dialogs open: next to the current document, or in
// the working directory.
func (a *App) startDir() string {
	if p := a.doc().path; p != "" {
		return filepath.Dir(p)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func (a *App) cmdNew() { a.newDoc() }

func (a *App) cmdOpen() {
	if path, ok := a.pickFile(pickOpen, a.startDir(), ""); ok {
		a.openPath(path, false)
	}
}

// openPath opens the file at path in a new document, asking for its
// password. With create set, a path that does not exist yet gives an empty
// document that will be saved there.
func (a *App) openPath(path string, create bool) {
	path, err := filepath.Abs(path)
	if err != nil {
		a.errorBox("開けませんでした: "+path, err)
		return
	}
	for i, d := range a.docs {
		if d.path == path {
			a.cur = i
			return
		}
	}

	data, err := readFile(path)
	if create && errors.Is(err, fs.ErrNotExist) {
		d := newDocument(&a.env, "")
		d.path = path
		a.addDoc(d)
		return
	}
	if err != nil {
		a.errorBox("開けませんでした: "+filepath.Base(path), err)
		return
	}

	password := ""
	encrypted := vault.IsEncrypted(data)
	if encrypted {
		_, ok := a.prompt(promptSpec{
			title:  "パスワード",
			note:   fmt.Sprintf("「%s」のパスワードを入力してください。", filepath.Base(path)),
			labels: []string{"パスワード"},
			masked: true,
			check: func(v []string) string {
				if v[0] == "" {
					return "パスワードを入力してください"
				}
				a.flash("復号しています...")
				defer func() { a.status = "" }()
				plaintext, err := vault.Decrypt(v[0], data)
				switch {
				case errors.Is(err, vault.ErrWrongPassword):
					return "パスワードが違います"
				case err != nil:
					return "復号できません: " + err.Error()
				}
				data, password = plaintext, v[0]
				return ""
			},
		})
		if !ok {
			return
		}
	}

	if !utf8.Valid(data) {
		a.errorBox("開けませんでした: "+filepath.Base(path)+"\nUTF-8 のテキストではありません。", nil)
		return
	}
	text := strings.TrimPrefix(string(data), utf8BOM)
	d := newDocument(&a.env, normalizeEOL(text))
	d.path, d.password, d.plain = path, password, !encrypted
	d.crlf = strings.Contains(text, "\r\n")
	d.savedCRLF = d.crlf
	a.addDoc(d)
	if d.plain {
		a.status = "暗号化されていないファイルです。保存すると暗号化した別のファイルを作ります。"
	}
}

func readFile(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, errors.New("フォルダーです")
	}
	if fi.Size() > maxFileSize {
		return nil, fmt.Errorf("ファイルが大きすぎます (上限 %d MiB)", maxFileSize>>20)
	}
	return os.ReadFile(path)
}

func (a *App) cmdSave()   { a.save(a.doc()) }
func (a *App) cmdSaveAs() { a.saveAs(a.doc()) }

// save writes d back to its file under the password it was opened with. A
// document that has neither yet goes through saveAs. It reports whether the
// document was saved.
func (a *App) save(d *document) bool {
	if d.path == "" || d.password == "" {
		return a.saveAs(d)
	}
	return a.write(d, d.path, d.password)
}

// saveAs asks for a file name and a new password and writes d there.
func (a *App) saveAs(d *document) bool {
	dir, name := a.startDir(), ""
	if d.path != "" {
		name = filepath.Base(d.path)
		if d.plain {
			// Keep the unencrypted original; the encrypted copy goes
			// next to it.
			name += ".age"
		}
	}
	path, ok := a.pickFile(pickSave, dir, name)
	if !ok {
		return false
	}
	v, ok := a.prompt(promptSpec{
		title:  "パスワードの設定",
		note:   fmt.Sprintf("「%s」を暗号化するパスワードを決めてください。忘れるとファイルは開けなくなります。", filepath.Base(path)),
		labels: []string{"新しいパスワード", "確認のためもう一度"},
		masked: true,
		check: func(v []string) string {
			switch {
			case v[0] == "":
				return "パスワードを入力してください"
			case v[0] != v[1]:
				return "2 つのパスワードが一致しません"
			}
			return ""
		},
	})
	if !ok {
		return false
	}
	return a.write(d, path, v[0])
}

func (a *App) write(d *document, path, password string) bool {
	a.flash("暗号化して保存しています...")
	data, err := vault.Encrypt(password, []byte(d.text()))
	if err == nil {
		err = vault.WriteFile(path, data)
	}
	if err != nil {
		a.status = ""
		a.errorBox("保存できませんでした: "+filepath.Base(path), err)
		return false
	}
	d.path, d.password, d.plain = path, password, false
	d.markSaved()
	a.status = "保存しました: " + d.title()
	return true
}

// confirmClose lets the user save a modified document before it goes away.
// It returns false when the document must stay open.
func (a *App) confirmClose(d *document) bool {
	if !d.modified() {
		return true
	}
	msg := fmt.Sprintf("「%s」への変更を保存しますか?", d.title())
	switch a.messageBox("変更の保存", msg, "保存(S)", "保存しない(N)", "キャンセル") {
	case 0:
		return a.save(d)
	case 1:
		return true
	}
	return false
}

func (a *App) cmdClose() {
	if !a.confirmClose(a.doc()) {
		return
	}
	a.docs = slices.Delete(a.docs, a.cur, a.cur+1)
	a.cur = max(min(a.cur, len(a.docs)-1), 0)
	if len(a.docs) == 0 {
		a.newDoc()
	}
}

func (a *App) cmdQuit() {
	for i, d := range a.docs {
		a.cur = i
		if !a.confirmClose(d) {
			return
		}
	}
	a.done = true
}

func (a *App) cmdUndo() {
	if !a.doc().undo() {
		a.status = "元に戻す操作はありません"
	}
}

func (a *App) cmdRedo() {
	if !a.doc().redo() {
		a.status = "やり直す操作はありません"
	}
}

func (a *App) cmdCopy() {
	if d := a.doc(); d.hasSel() {
		a.clip.set(d.selText())
	}
}

func (a *App) cmdCut() {
	if d := a.doc(); d.hasSel() {
		a.clip.set(d.selText())
		d.deleteSel()
	}
}

func (a *App) cmdPaste() { a.doc().paste(a.clip.get()) }

func (a *App) cmdGoto() {
	d := a.doc()
	line := 0
	_, ok := a.prompt(promptSpec{
		title:  "行へ移動",
		labels: []string{fmt.Sprintf("行番号 (1 - %d)", d.buf.LineCount())},
		check: func(v []string) string {
			n, err := strconv.Atoi(strings.TrimSpace(v[0]))
			if err != nil || n < 1 {
				return "行番号を数字で入力してください"
			}
			line = min(n, d.buf.LineCount()) - 1
			return ""
		},
	})
	if ok {
		d.moveTo(pos{Line: line}, false)
	}
}

func (a *App) cmdToggleWrap() {
	a.env.wrap = !a.env.wrap
	for _, d := range a.docs {
		d.topRow, d.left, d.follow = 0, 0, true
	}
}

func (a *App) cmdToggleIndent() { a.env.spaces = !a.env.spaces }

func (a *App) cmdCycleTabWidth() {
	switch a.env.tabW {
	case 2:
		a.env.tabW = 4
	case 4:
		a.env.tabW = 8
	default:
		a.env.tabW = 2
	}
}

func (a *App) cmdToggleEOL() {
	d := a.doc()
	d.crlf = !d.crlf
}

func (a *App) cmdDocList() {
	titles := make([]string, len(a.docs))
	for i, d := range a.docs {
		titles[i] = d.title()
		if d.modified() {
			titles[i] = "*" + titles[i]
		}
	}
	if i, ok := a.pickList("ドキュメント一覧", titles, a.cur); ok {
		a.cur = i
	}
}

func (a *App) cmdKeys() {
	a.messageBox("キー操作",
		"Ctrl+N 新規    Ctrl+O 開く    Ctrl+S 上書き保存\n"+
			"Ctrl+W 閉じる  Ctrl+Q 終了    Ctrl+P ドキュメント一覧\n"+
			"Ctrl+Z 元に戻す               Ctrl+Y やり直し\n"+
			"Ctrl+X 切り取り  Ctrl+C コピー  Ctrl+V 貼り付け\n"+
			"Ctrl+A すべて選択             Ctrl+G 行へ移動\n"+
			"Ctrl+F 検索    Ctrl+R 置換    F3 / Shift+F3 次・前を検索\n"+
			"Shift+矢印キー 選択           Ctrl+Left / Right 単語単位で移動\n"+
			"Ctrl+Home / End 文頭・文末    Ctrl+Up / Down スクロール\n"+
			"Tab / Shift+Tab インデントの増減\n"+
			"Alt+Z 右端で折り返す          F10 メニュー",
		"OK")
}

func (a *App) cmdAbout() {
	a.messageBox("バージョン情報",
		"secnote "+a.version+"\n"+
			"パスワードで暗号化して保存するテキストエディター\n\n"+
			"ファイル形式: age (scrypt パスフレーズ)\n"+
			"文字コード: UTF-8",
		"OK")
}
