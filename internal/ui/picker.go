package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type pickMode int

const (
	pickOpen pickMode = iota
	pickSave
)

// resolvePath turns what the user typed into the file picker into a full
// path. Relative names are taken from dir and a leading ~ is the home
// directory.
func resolvePath(text, dir string) string {
	if text == "~" || strings.HasPrefix(text, "~/") || strings.HasPrefix(text, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			text = filepath.Join(home, text[1:])
		}
	}
	// A bare drive such as "D:" means its root.
	if vol := filepath.VolumeName(text); vol != "" && text == vol {
		text += string(filepath.Separator)
	}
	if !filepath.IsAbs(text) {
		text = filepath.Join(dir, text)
	}
	return filepath.Clean(text)
}

// listDir returns the names in dir for the file picker: the parent first,
// then folders, then files. Folder names end in a slash.
func listDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs, files []string
	for _, e := range entries {
		if protected(e) {
			continue
		}
		isDir := e.IsDir()
		if !isDir && !e.Type().IsRegular() {
			// A link is listed as what it leads to, and left out when it
			// leads nowhere.
			fi, err := os.Stat(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			isDir = fi.IsDir()
		}
		if isDir {
			dirs = append(dirs, e.Name()+"/")
		} else {
			files = append(files, e.Name())
		}
	}
	byName := func(s []string) {
		sort.Slice(s, func(i, j int) bool { return strings.ToLower(s[i]) < strings.ToLower(s[j]) })
	}
	byName(dirs)
	byName(files)

	var names []string
	if filepath.Dir(dir) != dir {
		names = append(names, "../")
	}
	return append(append(names, dirs...), files...), nil
}

// pickFile asks the user for a file to open or to save to, starting in dir
// with name filled in. It returns false when the user cancelled.
func (a *App) pickFile(mode pickMode, dir, name string) (string, bool) {
	title, action := "開く", "開く"
	if mode == pickSave {
		title, action = "名前を付けて保存", "保存"
	}
	var (
		list    listBox
		input   lineInput
		errText string
		result  string
		ok      bool
	)

	load := func(to string) bool {
		names, err := listDir(to)
		if err != nil {
			errText = "フォルダーを開けません: " + reason(err)
			return false
		}
		dir = to
		list = listBox{items: names, sel: -1}
		return true
	}
	if !load(dir) {
		if wd, err := os.Getwd(); err != nil || !load(wd) {
			a.errorBox("フォルダーを開けません", nil)
			return "", false
		}
		errText = ""
	}
	input.set(name)
	input.selectAll()

	accept := func() bool {
		errText = ""
		text := strings.TrimSpace(input.text)
		if text == "" && list.sel >= 0 {
			text = list.items[list.sel]
		}
		if text == "" {
			return false
		}
		path := resolvePath(text, dir)
		fi, err := os.Stat(path)
		switch {
		case err == nil && fi.IsDir():
			if load(path) {
				input.set("")
			}
			return false
		case mode == pickOpen && err != nil:
			errText = "ファイルが見つかりません"
			return false
		case mode == pickSave && err == nil:
			msg := fmt.Sprintf("「%s」は既に存在します。上書きしますか?", filepath.Base(path))
			if a.messageBox(title, msg, "上書き(O)", "キャンセル") != 0 {
				return false
			}
		case mode == pickSave:
			if _, err := os.Stat(filepath.Dir(path)); err != nil {
				errText = "保存先のフォルダーがありません"
				return false
			}
		}
		result, ok = path, true
		return true
	}
	// highlight shows the highlighted name in the field.
	highlight := func() {
		if list.sel >= 0 {
			input.set(list.items[list.sel])
		}
	}

	panel := func() {
		sw, sh := a.scr.Size()
		in := a.frame(title, min(sw-8, 72), min(sh-5, 22))
		drawStr(a.scr, in.x, in.y, in.w, fitRight(dir, in.w), stPanel)
		list.draw(a.scr, rect{in.x, in.y + 2, in.w, max(in.h-6, 1)})

		y := in.y + in.h - 3
		const label = "ファイル名: "
		lw := drawStr(a.scr, in.x, y, in.w, label, stPanel)
		a.scr.ShowCursor(input.draw(a.scr, in.x+lw, y, in.w-lw, stField), y)
		if errText != "" {
			drawStr(a.scr, in.x, y+2, in.w, errText, stPanelErr)
		} else {
			drawStr(a.scr, in.x, y+2, in.w, "Enter: "+action+"  Esc: キャンセル", stPanelDim)
		}
	}

	a.modal(panel, func(ev tcell.Event) bool {
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch {
			case ev.Key() == tcell.KeyEsc:
				return true
			case ev.Key() == tcell.KeyEnter:
				return accept()
			case list.handleKey(ev):
				highlight()
			case a.inputKey(&input, ev):
				list.sel = -1
			}
		case *pasteEvent:
			input.insert(ev.text)
			list.sel = -1
		case *tcell.EventMouse:
			if list.handleWheel(ev) {
				highlight()
				break
			}
			if !a.press {
				break
			}
			x, y := ev.Position()
			if input.area.contains(x, y) {
				input.click(x)
			}
			if i := list.at(x, y); i >= 0 {
				// The first click highlights, the second one opens.
				again := a.clicks > 1 && i == list.sel
				list.sel = i
				highlight()
				if again {
					return accept()
				}
			}
		}
		return false
	})
	return result, ok
}

// reason returns the short cause of a file system error, without the
// operation and path that os wraps around it.
func reason(err error) string {
	if pe, ok := err.(*fs.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
