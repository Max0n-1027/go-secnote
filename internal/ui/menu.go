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
	m := a.msg
	onOff := func(on bool) string {
		if on {
			return m.On
		}
		return m.Off
	}
	sep := menuItem{}
	return []menu{
		{label: m.MenuFile, items: []menuItem{
			{label: m.New, hint: "Ctrl+N", run: a.cmdNew},
			{label: m.Open, hint: "Ctrl+O", run: a.cmdOpen},
			{label: m.Save, hint: "Ctrl+S", run: a.cmdSave},
			{label: m.SaveAs, run: a.cmdSaveAs},
			{label: m.Close, hint: "Ctrl+W", run: a.cmdClose},
			sep,
			{label: m.Exit, hint: "Ctrl+Q", run: a.cmdQuit},
		}},
		{label: m.MenuEdit, items: []menuItem{
			{label: m.Undo, hint: "Ctrl+Z", run: a.cmdUndo, enabled: func() bool { return a.doc().buf.CanUndo() }},
			{label: m.Redo, hint: "Ctrl+Y", run: a.cmdRedo, enabled: func() bool { return a.doc().buf.CanRedo() }},
			sep,
			{label: m.Cut, hint: "Ctrl+X", run: a.cmdCut, enabled: func() bool { return a.doc().hasSel() }},
			{label: m.Copy, hint: "Ctrl+C", run: a.cmdCopy, enabled: func() bool { return a.doc().hasSel() }},
			{label: m.Paste, hint: "Ctrl+V", run: a.cmdPaste},
			sep,
			{label: m.Find, hint: "Ctrl+F", run: func() { a.findBar(false) }},
			{label: m.Replace, hint: "Ctrl+R", run: func() { a.findBar(true) }},
			{label: m.FindNext, hint: "F3", run: func() { a.findAgain(true) }},
			{label: m.GoToLine, hint: "Ctrl+G", run: a.cmdGoto},
			sep,
			{label: m.SelectAll, hint: "Ctrl+A", run: func() { a.doc().selectAll() }},
		}},
		{label: m.MenuView, items: []menuItem{
			{text: func() string { return fmt.Sprintf(m.WordWrap, onOff(a.env.wrap)) }, hint: "Alt+Z", run: a.cmdToggleWrap},
			{text: func() string { return fmt.Sprintf(m.Indent, a.indentName()) }, run: a.cmdToggleIndent},
			{text: func() string { return fmt.Sprintf(m.TabWidth, a.env.tabW) }, run: a.cmdCycleTabWidth},
			{text: func() string { return fmt.Sprintf(m.LineEndings, a.eolLabel()) }, run: a.cmdToggleEOL},
			sep,
			{label: m.DocumentList, hint: "Ctrl+P", run: a.cmdDocList},
		}},
		{label: m.MenuHelp, items: []menuItem{
			{label: m.KeyList, run: a.cmdKeys},
			{label: m.AboutApp, run: a.cmdAbout},
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
			labelW = max(labelW, labelWidth(it.title()))
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
