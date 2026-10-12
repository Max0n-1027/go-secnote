package ui

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// frame draws an empty panel in the middle of the screen with room for w by
// h cells of content and returns where that content goes.
func (a *App) frame(title string, w, h int) rect {
	sw, sh := a.scr.Size()
	w, h = min(w+4, sw), min(h+3, sh)
	x, y := (sw-w)/2, (sh-h)/2
	fillRect(a.scr, x, y, w, h, stPanel)
	fillRect(a.scr, x, y, w, 1, stTitle)
	drawStr(a.scr, x+1, y, w-2, title, stTitle)
	return rect{x + 2, y + 2, w - 4, h - 3}
}

// buttonRow is a row of push buttons.
type buttonRow struct {
	labels []string
	areas  []rect
}

func (b *buttonRow) width() int {
	total := 2 * (len(b.labels) - 1)
	for _, l := range b.labels {
		total += strWidth(l) + 4
	}
	return total
}

// draw centers the buttons in r and highlights button focus.
func (b *buttonRow) draw(scr tcell.Screen, r rect, focus int) {
	b.areas = b.areas[:0]
	x := r.x + max((r.w-b.width())/2, 0)
	for i, l := range b.labels {
		st := stPanel
		if i == focus {
			st = stFocus
		}
		w := strWidth(l) + 4
		fillRect(scr, x, r.y, w, 1, st)
		scr.Put(x, r.y, "[", st)
		drawLabel(scr, x+2, r.y, w-4, l, st)
		scr.Put(x+w-1, r.y, "]", st)
		b.areas = append(b.areas, rect{x, r.y, w, 1})
		x += w + 2
	}
}

// at returns the index of the button at x, y, or -1.
func (b *buttonRow) at(x, y int) int {
	for i, r := range b.areas {
		if r.contains(x, y) {
			return i
		}
	}
	return -1
}

// messageBox asks the user to pick one of the buttons and returns its index,
// or -1 when the box was dismissed with Esc.
func (a *App) messageBox(title, text string, labels ...string) int {
	buttons := buttonRow{labels: labels}
	focus, result := 0, -1

	panel := func() {
		sw, _ := a.scr.Size()
		lines := wrapText(text, max(sw-8, 10))
		w := max(buttons.width(), strWidth(title), 24)
		for _, l := range lines {
			w = max(w, strWidth(l))
		}
		in := a.frame(title, w, len(lines)+2)
		// On a screen too short for the whole text, the end is cut off
		// rather than the buttons.
		for i, l := range lines[:max(min(len(lines), in.h-2), 0)] {
			drawStr(a.scr, in.x, in.y+i, in.w, l, stPanel)
		}
		buttons.draw(a.scr, rect{in.x, in.y + in.h - 1, in.w, 1}, focus)
	}

	a.modal(panel, func(ev tcell.Event) bool {
		switch ev := ev.(type) {
		case *tcell.EventKey:
			n := len(labels)
			switch ev.Key() {
			case tcell.KeyEsc:
				return true
			case tcell.KeyEnter:
				result = focus
				return true
			case tcell.KeyLeft, tcell.KeyUp, tcell.KeyBacktab:
				focus = (focus + n - 1) % n
			case tcell.KeyRight, tcell.KeyDown, tcell.KeyTab:
				focus = (focus + 1) % n
			case tcell.KeyRune:
				r := unicode.ToLower(ev.Rune())
				if r == ' ' {
					result = focus
					return true
				}
				for i, l := range labels {
					if accelOf(l) == r {
						result = i
						return true
					}
				}
			}
		case *tcell.EventMouse:
			if i := buttons.at(ev.Position()); a.press && i >= 0 {
				result = i
				return true
			}
		}
		return false
	})
	return result
}

func (a *App) errorBox(msg string, err error) {
	if err != nil {
		msg += "\n" + err.Error()
	}
	a.messageBox("エラー", msg, "OK")
}

// promptSpec describes a dialog with one or more text fields.
type promptSpec struct {
	title  string
	note   string
	labels []string // one per field
	masked bool
	// check looks at the entered values when the user confirms. A non-empty
	// result is shown as an error and keeps the dialog open.
	check func(values []string) string
}

const promptWidth = 46

// prompt shows the dialog and returns the entered values, or false when the
// user cancelled.
func (a *App) prompt(p promptSpec) ([]string, bool) {
	inputs := make([]lineInput, len(p.labels))
	for i := range inputs {
		inputs[i].masked = p.masked
	}
	buttons := buttonRow{labels: []string{"OK", "キャンセル"}}
	focus, errText, ok := 0, "", false

	values := func() []string {
		v := make([]string, len(inputs))
		for i := range inputs {
			v[i] = inputs[i].text
		}
		return v
	}
	submit := func() bool {
		if p.check != nil {
			if errText = p.check(values()); errText != "" {
				inputs[focus].selectAll()
				return false
			}
		}
		ok = true
		return true
	}

	panel := func() {
		var note []string
		if p.note != "" {
			note = append(wrapText(p.note, promptWidth), "")
		}
		in := a.frame(p.title, promptWidth, len(note)+3*len(inputs)+2)
		y := in.y
		for _, l := range note {
			drawStr(a.scr, in.x, y, in.w, l, stPanel)
			y++
		}
		for i := range inputs {
			drawStr(a.scr, in.x, y, in.w, p.labels[i], stPanel)
			if x := inputs[i].draw(a.scr, in.x, y+1, in.w, stField); i == focus {
				a.scr.ShowCursor(x, y+1)
			}
			y += 3
		}
		drawStr(a.scr, in.x, y, in.w, errText, stPanelErr)
		buttons.draw(a.scr, rect{in.x, y + 1, in.w, 1}, -1)
	}

	a.modal(panel, func(ev tcell.Event) bool {
		n := len(inputs)
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc:
				return true
			case tcell.KeyEnter:
				if focus == n-1 {
					return submit()
				}
				focus++
			case tcell.KeyTab, tcell.KeyDown:
				focus = (focus + 1) % n
			case tcell.KeyBacktab, tcell.KeyUp:
				focus = (focus + n - 1) % n
			default:
				a.inputKey(&inputs[focus], ev)
			}
		case *pasteEvent:
			inputs[focus].insert(ev.text)
		case *tcell.EventMouse:
			if !a.press {
				break
			}
			x, y := ev.Position()
			for i := range inputs {
				if inputs[i].area.contains(x, y) {
					focus = i
					inputs[i].click(x)
				}
			}
			switch buttons.at(x, y) {
			case 0:
				return submit()
			case 1:
				return true
			}
		}
		return false
	})
	return values(), ok
}

// inputKey passes a key to a text field, adding paste from the clipboard.
func (a *App) inputKey(in *lineInput, ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlV {
		in.insert(a.clip.get())
		return true
	}
	return in.handleKey(ev)
}

// listBox is a scrolling list with one highlighted row.
type listBox struct {
	items []string
	sel   int // -1 when nothing is highlighted
	top   int
	area  rect
}

func (l *listBox) move(n int) {
	if len(l.items) > 0 {
		l.sel = min(max(l.sel+n, 0), len(l.items)-1)
	}
}

func (l *listBox) draw(scr tcell.Screen, r rect) {
	l.area = r
	if l.sel >= 0 {
		l.top = max(min(l.top, l.sel), l.sel-r.h+1)
	}
	l.top = max(min(l.top, len(l.items)-r.h), 0)

	fillRect(scr, r.x, r.y, r.w, r.h, stField)
	for i := 0; i < r.h && l.top+i < len(l.items); i++ {
		st := stField
		if l.top+i == l.sel {
			st = stFocus
			fillRect(scr, r.x, r.y+i, r.w, 1, st)
		}
		drawStr(scr, r.x+1, r.y+i, r.w-2, l.items[l.top+i], st)
	}
}

// at returns the index of the item drawn at x, y, or -1.
func (l *listBox) at(x, y int) int {
	if i := l.top + y - l.area.y; l.area.contains(x, y) && i < len(l.items) {
		return i
	}
	return -1
}

// handleKey moves the highlight and reports whether it used the key.
func (l *listBox) handleKey(ev *tcell.EventKey) bool {
	page := max(l.area.h-1, 1)
	switch ev.Key() {
	case tcell.KeyUp:
		l.move(-1)
	case tcell.KeyDown:
		l.move(1)
	case tcell.KeyPgUp:
		l.move(-page)
	case tcell.KeyPgDn:
		l.move(page)
	default:
		return false
	}
	return true
}

// handleWheel moves the highlight on a wheel event and reports whether ev
// was one.
func (l *listBox) handleWheel(ev *tcell.EventMouse) bool {
	switch btn := ev.Buttons(); {
	case btn&tcell.WheelUp != 0:
		l.move(-3)
	case btn&tcell.WheelDown != 0:
		l.move(3)
	default:
		return false
	}
	return true
}

// pickList lets the user choose one of items and returns its index, or
// false when the user cancelled.
func (a *App) pickList(title string, items []string, sel int) (int, bool) {
	list := listBox{items: items, sel: sel}
	ok := false

	panel := func() {
		_, sh := a.scr.Size()
		w := max(strWidth(title), 30)
		for _, it := range items {
			w = max(w, strWidth(it)+2)
		}
		in := a.frame(title, min(w, 70), min(len(items), max(sh-6, 1)))
		list.draw(a.scr, in)
	}

	a.modal(panel, func(ev tcell.Event) bool {
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc:
				return true
			case tcell.KeyEnter:
				ok = true
				return true
			case tcell.KeyHome:
				list.move(-len(items))
			case tcell.KeyEnd:
				list.move(len(items))
			default:
				list.handleKey(ev)
			}
		case *tcell.EventMouse:
			if list.handleWheel(ev) || !a.press {
				break
			}
			if i := list.at(ev.Position()); i >= 0 {
				// The first click highlights, the second one chooses.
				ok = a.clicks > 1 && i == list.sel
				list.sel = i
				return ok
			}
		}
		return false
	})
	return list.sel, ok
}
