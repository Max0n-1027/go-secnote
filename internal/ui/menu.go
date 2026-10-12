package ui

import (
	"fmt"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

type menu struct {
	label string
	items []menuItem
	x, w  int // where the title was last drawn
}

type menuItem struct {
	label   string        // "" draws a separator
	text    func() string // replaces label when set
	hint    string        // shortcut shown on the right
	run     func()
	enabled func() bool // nil means always
}

func (it menuItem) title() string {
	if it.text != nil {
		return it.text()
	}
	return it.label
}

func (it menuItem) usable() bool {
	return it.run != nil && (it.enabled == nil || it.enabled())
}

func (a *App) buildMenus() []menu {
	onOff := func(on bool) string {
		if on {
			return "オン"
		}
		return "オフ"
	}
	sep := menuItem{}
	return []menu{
		{label: "ファイル(F)", items: []menuItem{
			{label: "新規(N)", hint: "Ctrl+N", run: a.cmdNew},
			{label: "開く(O)...", hint: "Ctrl+O", run: a.cmdOpen},
			{label: "上書き保存(S)", hint: "Ctrl+S", run: a.cmdSave},
			{label: "名前を付けて保存(A)...", run: a.cmdSaveAs},
			{label: "閉じる(C)", hint: "Ctrl+W", run: a.cmdClose},
			sep,
			{label: "終了(X)", hint: "Ctrl+Q", run: a.cmdQuit},
		}},
		{label: "編集(E)", items: []menuItem{
			{label: "元に戻す(U)", hint: "Ctrl+Z", run: a.cmdUndo, enabled: func() bool { return a.doc().buf.CanUndo() }},
			{label: "やり直し(R)", hint: "Ctrl+Y", run: a.cmdRedo, enabled: func() bool { return a.doc().buf.CanRedo() }},
			sep,
			{label: "切り取り(T)", hint: "Ctrl+X", run: a.cmdCut, enabled: func() bool { return a.doc().hasSel() }},
			{label: "コピー(C)", hint: "Ctrl+C", run: a.cmdCopy, enabled: func() bool { return a.doc().hasSel() }},
			{label: "貼り付け(P)", hint: "Ctrl+V", run: a.cmdPaste},
			sep,
			{label: "検索(F)...", hint: "Ctrl+F", run: func() { a.findBar(false) }},
			{label: "置換(E)...", hint: "Ctrl+R", run: func() { a.findBar(true) }},
			{label: "次を検索(N)", hint: "F3", run: func() { a.findAgain(true) }},
			{label: "行へ移動(G)...", hint: "Ctrl+G", run: a.cmdGoto},
			sep,
			{label: "すべて選択(A)", hint: "Ctrl+A", run: func() { a.doc().selectAll() }},
		}},
		{label: "表示(V)", items: []menuItem{
			{text: func() string { return "右端で折り返す(W): " + onOff(a.env.wrap) }, hint: "Alt+Z", run: a.cmdToggleWrap},
			{text: func() string { return "インデント(I): " + a.indentName() }, run: a.cmdToggleIndent},
			{text: func() string { return fmt.Sprintf("タブ幅(T): %d", a.env.tabW) }, run: a.cmdCycleTabWidth},
			{text: func() string { return "改行コード(E): " + a.eolLabel() }, run: a.cmdToggleEOL},
			sep,
			{label: "ドキュメント一覧(D)...", hint: "Ctrl+P", run: a.cmdDocList},
		}},
		{label: "ヘルプ(H)", items: []menuItem{
			{label: "キー操作(K)", run: a.cmdKeys},
			{label: "バージョン情報(A)", run: a.cmdAbout},
		}},
	}
}

// openMenu drops down menu idx and runs whatever the user picks from it.
func (a *App) openMenu(idx int) {
	var (
		sel    int
		box    rect
		chosen func()
	)
	// step moves the highlight to the next usable item in direction dir.
	step := func(from, dir int) {
		items := a.menus[a.menuOpen].items
		for i, n := from, 0; n < len(items); n++ {
			i = (i + dir + len(items)) % len(items)
			if items[i].usable() {
				sel = i
				return
			}
		}
		sel = -1
	}
	open := func(i int) {
		a.menuOpen = (i + len(a.menus)) % len(a.menus)
		step(-1, 1)
	}
	pick := func(i int) bool {
		items := a.menus[a.menuOpen].items
		if i < 0 || i >= len(items) || !items[i].usable() {
			return false
		}
		chosen = items[i].run
		return true
	}
	open(idx)

	panel := func() {
		m := a.menus[a.menuOpen]
		labelW, hintW := 0, 0
		for _, it := range m.items {
			labelW = max(labelW, strWidth(it.title()))
			hintW = max(hintW, strWidth(it.hint))
		}
		sw, _ := a.scr.Size()
		box = rect{m.x, 1, labelW + hintW + 4, len(m.items)}
		box.x = max(min(box.x, sw-box.w), 0)
		for i, it := range m.items {
			y := box.y + i
			st := stPanel
			switch {
			case i == sel:
				st = stFocus
			case !it.usable():
				st = stPanelDim
			}
			fillRect(a.scr, box.x, y, box.w, 1, st)
			if it.title() == "" {
				for x := box.x + 1; x < box.x+box.w-1; x++ {
					a.scr.Put(x, y, "-", st)
				}
				continue
			}
			drawLabel(a.scr, box.x+1, y, labelW, it.title(), st)
			drawStr(a.scr, box.x+box.w-1-strWidth(it.hint), y, hintW, it.hint, st)
		}
	}

	a.modal(panel, func(ev tcell.Event) bool {
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc, tcell.KeyF10:
				return true
			case tcell.KeyLeft:
				open(a.menuOpen - 1)
			case tcell.KeyRight:
				open(a.menuOpen + 1)
			case tcell.KeyUp:
				step(sel, -1)
			case tcell.KeyDown:
				step(sel, 1)
			case tcell.KeyHome:
				step(-1, 1)
			case tcell.KeyEnd:
				step(0, -1)
			case tcell.KeyEnter:
				return pick(sel)
			case tcell.KeyRune:
				r := unicode.ToLower(ev.Rune())
				if ev.Modifiers()&tcell.ModAlt != 0 {
					for i, m := range a.menus {
						if accelOf(m.label) == r {
							open(i)
						}
					}
					break
				}
				for i, it := range a.menus[a.menuOpen].items {
					if accelOf(it.title()) == r {
						return pick(i)
					}
				}
			}

		case *tcell.EventMouse:
			x, y := ev.Position()
			onTitle := -1
			if y == 0 {
				for i, m := range a.menus {
					if x >= m.x && x < m.x+m.w {
						onTitle = i
					}
				}
			}
			item := -1
			if box.contains(x, y) {
				item = y - box.y
			}
			switch {
			case a.press && onTitle == a.menuOpen:
				return true
			case a.held && onTitle >= 0:
				if onTitle != a.menuOpen {
					open(onTitle)
				}
			case a.press && item < 0:
				return true
			case a.held && item >= 0:
				if a.menus[a.menuOpen].items[item].usable() {
					sel = item
				}
			case item >= 0:
				// The button came up over an item.
				return a.release && pick(item)
			}
		}
		return false
	})
	a.menuOpen = -1
	if chosen != nil {
		chosen()
	}
}
