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

	"github.com/Max0n-1027/go-secnote/internal/vault"
)

// maxFileSize is the largest file the editor will load. Everything is held
// in memory, so something far bigger than any note is refused outright.
const maxFileSize = 64 << 20

const utf8BOM = "\xEF\xBB\xBF"

func (a *App) newDoc() *document {
	a.untitled++
	d := newDocument(&a.env, "")
	d.name = fmt.Sprintf(a.msg.Untitled, a.untitled)
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
	m := a.msg
	path, err := filepath.Abs(path)
	if err != nil {
		a.errorBox(fmt.Sprintf(m.CannotOpen, path), err)
		return
	}
	for i, d := range a.docs {
		if d.path == path {
			a.cur = i
			return
		}
	}

	data, err := a.readFile(path)
	if create && errors.Is(err, fs.ErrNotExist) {
		d := newDocument(&a.env, "")
		d.path = path
		a.addDoc(d)
		return
	}
	if err != nil {
		a.errorBox(fmt.Sprintf(m.CannotOpen, filepath.Base(path)), err)
		return
	}

	password := ""
	encrypted := vault.IsEncrypted(data)
	if encrypted {
		_, ok := a.prompt(promptSpec{
			title:  m.PasswordTitle,
			note:   fmt.Sprintf(m.PasswordNote, filepath.Base(path)),
			labels: []string{m.PasswordLabel},
			masked: true,
			check: func(v []string) string {
				if v[0] == "" {
					return m.EnterPassword
				}
				a.flash(m.Decrypting)
				defer func() { a.status = "" }()
				plaintext, err := vault.Decrypt(v[0], data)
				switch {
				case errors.Is(err, vault.ErrWrongPassword):
					return m.WrongPassword
				case err != nil:
					return fmt.Sprintf(m.CannotDecrypt, err)
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
		a.errorBox(fmt.Sprintf(m.CannotOpen, filepath.Base(path))+"\n"+m.NotUTF8, nil)
		return
	}
	text := strings.TrimPrefix(string(data), utf8BOM)
	d := newDocument(&a.env, normalizeEOL(text))
	d.path, d.password, d.plain = path, password, !encrypted
	d.crlf = strings.Contains(text, "\r\n")
	d.savedCRLF = d.crlf
	a.addDoc(d)
	if d.plain {
		a.status = m.PlainNotice
	}
}

func (a *App) readFile(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, errors.New(a.msg.IsFolder)
	}
	if fi.Size() > maxFileSize {
		return nil, fmt.Errorf(a.msg.TooLarge, maxFileSize>>20)
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
		title:  a.msg.NewPasswordTitle,
		note:   fmt.Sprintf(a.msg.NewPasswordNote, filepath.Base(path)),
		labels: []string{a.msg.NewPassword, a.msg.ConfirmPassword},
		masked: true,
		check: func(v []string) string {
			switch {
			case v[0] == "":
				return a.msg.EnterPassword
			case v[0] != v[1]:
				return a.msg.PasswordsDiffer
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
	a.flash(a.msg.Saving)
	data, err := vault.Encrypt(password, []byte(d.text()))
	if err == nil {
		err = vault.WriteFile(path, data)
	}
	if err != nil {
		a.status = ""
		a.errorBox(fmt.Sprintf(a.msg.CannotSave, filepath.Base(path)), err)
		return false
	}
	d.path, d.password, d.plain = path, password, false
	d.markSaved()
	a.status = fmt.Sprintf(a.msg.Saved, d.title())
	return true
}

// confirmClose lets the user save a modified document before it goes away.
// It returns false when the document must stay open.
func (a *App) confirmClose(d *document) bool {
	if !d.modified() {
		return true
	}
	m := a.msg
	switch a.messageBox(m.SaveChangesTitle, fmt.Sprintf(m.SaveChanges, d.title()), m.SaveButton, m.DontSave, m.Cancel) {
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
		a.status = a.msg.NothingToUndo
	}
}

func (a *App) cmdRedo() {
	if !a.doc().redo() {
		a.status = a.msg.NothingToRedo
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
		title:  a.msg.GoToLineTitle,
		labels: []string{fmt.Sprintf(a.msg.LineNumber, d.buf.LineCount())},
		check: func(v []string) string {
			n, err := strconv.Atoi(strings.TrimSpace(v[0]))
			if err != nil || n < 1 {
				return a.msg.EnterLineNumber
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
	if i, ok := a.pickList(a.msg.DocumentsTitle, titles, a.cur); ok {
		a.cur = i
	}
}

func (a *App) cmdKeys() {
	a.messageBox(a.msg.KeysTitle, a.msg.KeysText, a.msg.OK)
}

func (a *App) cmdAbout() {
	a.messageBox(a.msg.AboutTitle, fmt.Sprintf(a.msg.AboutText, a.version), a.msg.OK)
}
