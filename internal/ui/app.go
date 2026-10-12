// Package ui is the terminal user interface of the editor.
package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Options are the settings the editor starts with.
type Options struct {
	Version  string
	TabWidth int
	Tabs     bool // indent with tabs rather than spaces
	NoWrap   bool
}

// App is the running editor.
type App struct {
	scr     tcell.Screen
	version string
	env     env
	docs    []*document
	cur     int
	clip    clipboard
	find    findState
	menus   []menu

	layers   []func() // modal panels, drawn bottom to top
	menuOpen int      // index of the open menu, -1 when none
	findRows int      // rows the find bar takes from the editing area
	status   string   // message shown in place of the file name
	hits     []hit    // clickable parts of the status bar
	untitled int

	done bool // the user asked to quit
	dead bool // the screen is gone

	// Mouse state, kept up to date by nextEvent.
	held      bool // button 1 is down
	press     bool // the current event is the one that pressed it
	release   bool // the current event is the one that let it go
	clicks    int  // 1, 2 or 3 for single, double and triple clicks
	lastPress time.Time
	lastX     int
	lastY     int
	drag      dragKind

	paste   *strings.Builder // non-nil while a bracketed paste is arriving
	pasteCR bool
}

type hit struct {
	area rect
	run  func()
}

type dragKind int

const (
	dragNone dragKind = iota
	dragText
	dragScrollBar
)

// pasteEvent is a completed bracketed paste.
type pasteEvent struct {
	when time.Time
	text string
}

func (e *pasteEvent) When() time.Time { return e.when }

const doubleClick = 400 * time.Millisecond

// Run starts the editor on the terminal, opens paths and returns when the
// user quits.
func Run(opts Options, paths []string) error {
	scr, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := scr.Init(); err != nil {
		return err
	}
	// Give the terminal back before a panic prints its trace.
	defer func() {
		scr.Fini()
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	a := newApp(scr, opts)
	a.clip.system = true
	a.run(paths)
	return nil
}

func newApp(scr tcell.Screen, opts Options) *App {
	scr.EnableMouse(tcell.MouseButtonEvents | tcell.MouseDragEvents)
	scr.EnablePaste()
	a := &App{scr: scr, version: opts.Version, menuOpen: -1}
	a.env = env{tabW: opts.TabWidth, spaces: !opts.Tabs, wrap: !opts.NoWrap}
	if a.env.tabW < 1 || a.env.tabW > 16 {
		a.env.tabW = 4
	}
	a.clip.scr = scr
	a.menus = a.buildMenus()
	a.newDoc()
	a.resize()
	return a
}

func (a *App) run(paths []string) {
	for _, p := range paths {
		a.openPath(p, true)
	}
	for !a.done {
		ev := a.nextEvent()
		if ev == nil {
			return
		}
		a.handle(ev)
	}
}

func (a *App) doc() *document { return a.docs[a.cur] }

// nextEvent redraws the screen and returns the next key, mouse or paste
// event. It returns nil once the screen has been shut down.
func (a *App) nextEvent() tcell.Event {
	for !a.dead {
		if !a.scr.HasPendingEvent() {
			a.draw()
		}
		switch ev := a.scr.PollEvent().(type) {
		case nil:
			a.dead = true
		case *tcell.EventResize:
			a.scr.Sync()
		case *tcell.EventPaste:
			if ev.Start() {
				a.paste, a.pasteCR = &strings.Builder{}, false
			} else if a.paste != nil {
				text := a.paste.String()
				a.paste = nil
				return &pasteEvent{when: ev.When(), text: text}
			}
		case *tcell.EventKey:
			if a.paste != nil {
				a.collectPaste(ev)
				continue
			}
			return ev
		case *tcell.EventMouse:
			a.trackMouse(ev)
			return ev
		case *tcell.EventInterrupt:
			// A function posted to the event queue runs here, on the
			// goroutine that owns the editor's state.
			if fn, ok := ev.Data().(func()); ok {
				a.draw()
				fn()
			}
		}
	}
	return nil
}

// collectPaste adds one key of a bracketed paste to the text being gathered.
func (a *App) collectPaste(ev *tcell.EventKey) {
	cr := false
	switch ev.Key() {
	case tcell.KeyRune:
		a.paste.WriteRune(ev.Rune())
	case tcell.KeyTab:
		a.paste.WriteByte('\t')
	case tcell.KeyEnter:
		a.paste.WriteByte('\n')
		cr = true
	case tcell.KeyCtrlJ:
		// The LF of a CR LF pair has already been written.
		if !a.pasteCR {
			a.paste.WriteByte('\n')
		}
	}
	a.pasteCR = cr
}

func (a *App) trackMouse(ev *tcell.EventMouse) {
	held := ev.Buttons()&tcell.Button1 != 0
	a.press, a.release = held && !a.held, a.held && !held
	a.held = held
	if !held {
		a.drag = dragNone
	}
	if !a.press {
		return
	}
	x, y := ev.Position()
	if x == a.lastX && y == a.lastY && ev.When().Sub(a.lastPress) < doubleClick && a.clicks < 3 {
		a.clicks++
	} else {
		a.clicks = 1
	}
	a.lastX, a.lastY, a.lastPress = x, y, ev.When()
}

// modal shows panel on top of everything else and feeds events to handle
// until it returns true.
func (a *App) modal(panel func(), handle func(tcell.Event) bool) {
	a.layers = append(a.layers, panel)
	defer func() { a.layers = a.layers[:len(a.layers)-1] }()
	for {
		ev := a.nextEvent()
		if ev == nil || handle(ev) {
			return
		}
	}
}

// flash shows msg in the status bar right away, ahead of slow work.
func (a *App) flash(msg string) {
	a.status = msg
	a.draw()
}

func (a *App) resize() {
	w, h := a.scr.Size()
	a.env.area = rect{0, 1, w, max(h-2-a.findRows, 1)}
}

func (a *App) draw() {
	a.resize()
	a.scr.Clear()
	a.scr.HideCursor()
	a.doc().draw(a.scr, len(a.layers) == 0)
	a.drawMenuBar()
	a.drawStatus()
	for _, panel := range a.layers {
		panel()
	}
	a.scr.Show()
}

func (a *App) drawMenuBar() {
	w, _ := a.scr.Size()
	fillRect(a.scr, 0, 0, w, 1, stBar)
	x := 1
	for i := range a.menus {
		m := &a.menus[i]
		st := stBar
		if i == a.menuOpen {
			st = stFocus
		}
		m.x, m.w = x, strWidth(m.label)+2
		fillRect(a.scr, m.x, 0, m.w, 1, st)
		drawLabel(a.scr, m.x+1, 0, m.w, m.label, st)
		x += m.w
	}
}

func (a *App) drawStatus() {
	w, h := a.scr.Size()
	y := h - 1
	d := a.doc()
	fillRect(a.scr, 0, y, w, 1, stBar)

	state := "未保存"
	switch {
	case d.password != "":
		state = "暗号化"
	case d.plain:
		state = "平文"
	}
	segments := []struct {
		text string
		run  func()
	}{
		{state, nil},
		{fmt.Sprintf("行 %d, 列 %d", d.cur.Line+1, d.column()), a.cmdGoto},
		{a.indentLabel(), a.cmdToggleIndent},
		{a.eolLabel(), a.cmdToggleEOL},
		{"UTF-8", nil},
	}
	left := a.status
	if left == "" {
		left = d.title()
		if d.modified() {
			left = "*" + left
		}
		if len(a.docs) > 1 {
			left += fmt.Sprintf("  [%d/%d]", a.cur+1, len(a.docs))
		}
	}
	leftEnd := 1 + drawStr(a.scr, 1, y, w-2, left, stBar)

	// The segments fill in from the right as far as the message leaves room.
	a.hits = a.hits[:0]
	x := w - 1
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		sw := strWidth(seg.text)
		if x-sw < leftEnd+2 {
			break
		}
		x -= sw
		drawStr(a.scr, x, y, sw, seg.text, stBar)
		if seg.run != nil {
			a.hits = append(a.hits, hit{rect{x, y, sw, 1}, seg.run})
		}
		x -= 3
	}
}

func (a *App) indentName() string {
	if a.env.spaces {
		return "スペース"
	}
	return "タブ"
}

func (a *App) indentLabel() string {
	return fmt.Sprintf("%s: %d", a.indentName(), a.env.tabW)
}

func (a *App) eolLabel() string {
	if a.doc().crlf {
		return "CRLF"
	}
	return "LF"
}

func (a *App) handle(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		a.status = ""
		a.handleKey(ev)
	case *tcell.EventMouse:
		a.handleMouse(ev)
	case *pasteEvent:
		a.status = ""
		a.doc().paste(ev.text)
	}
}

func (a *App) handleKey(ev *tcell.EventKey) {
	d := a.doc()
	mod := ev.Modifiers()
	shift := mod&tcell.ModShift != 0
	ctrl := mod&tcell.ModCtrl != 0
	alt := mod&tcell.ModAlt != 0

	switch ev.Key() {
	case tcell.KeyRune:
		switch r := unicode.ToLower(ev.Rune()); {
		case alt && !ctrl && r == 'z':
			a.cmdToggleWrap()
		case alt && !ctrl:
			for i, m := range a.menus {
				if accelOf(m.label) == r {
					a.openMenu(i)
					break
				}
			}
		case !ctrl:
			d.typeText(string(ev.Rune()))
		}

	case tcell.KeyLeft:
		d.moveLeft(shift, ctrl)
	case tcell.KeyRight:
		d.moveRight(shift, ctrl)
	case tcell.KeyUp:
		if ctrl {
			d.scroll(-1)
		} else {
			d.moveRows(-1, shift)
		}
	case tcell.KeyDown:
		if ctrl {
			d.scroll(1)
		} else {
			d.moveRows(1, shift)
		}
	case tcell.KeyPgUp:
		d.movePage(-1, shift)
	case tcell.KeyPgDn:
		d.movePage(1, shift)
	case tcell.KeyHome:
		if ctrl {
			d.moveTo(pos{}, shift)
		} else {
			d.moveHome(shift)
		}
	case tcell.KeyEnd:
		if ctrl {
			d.moveTo(d.buf.End(), shift)
		} else {
			d.moveEnd(shift)
		}

	case tcell.KeyEnter:
		d.newline()
	case tcell.KeyTab:
		d.tab()
	case tcell.KeyBacktab:
		d.backtab()
	case tcell.KeyBackspace:
		if ctrl || alt {
			d.deleteWordLeft()
		} else {
			d.backspace()
		}
	case tcell.KeyDelete:
		switch {
		case ctrl:
			d.deleteWordRight()
		case shift:
			a.cmdCut()
		default:
			d.deleteForward()
		}
	case tcell.KeyInsert:
		switch {
		case ctrl:
			a.cmdCopy()
		case shift:
			a.cmdPaste()
		}

	case tcell.KeyCtrlN:
		a.cmdNew()
	case tcell.KeyCtrlO:
		a.cmdOpen()
	case tcell.KeyCtrlS:
		a.cmdSave()
	case tcell.KeyCtrlW:
		a.cmdClose()
	case tcell.KeyCtrlQ:
		a.cmdQuit()
	case tcell.KeyCtrlZ:
		a.cmdUndo()
	case tcell.KeyCtrlY:
		a.cmdRedo()
	case tcell.KeyCtrlX:
		a.cmdCut()
	case tcell.KeyCtrlC:
		a.cmdCopy()
	case tcell.KeyCtrlV:
		a.cmdPaste()
	case tcell.KeyCtrlA:
		d.selectAll()
	case tcell.KeyCtrlF:
		a.findBar(false)
	case tcell.KeyCtrlR:
		a.findBar(true)
	case tcell.KeyCtrlG:
		a.cmdGoto()
	case tcell.KeyCtrlP:
		a.cmdDocList()
	case tcell.KeyF3:
		a.findAgain(!shift)
	case tcell.KeyF10:
		a.openMenu(0)
	}
}

func (a *App) handleMouse(ev *tcell.EventMouse) {
	d := a.doc()
	x, y := ev.Position()
	area := a.env.area
	textX := area.x + d.gutterW()

	switch btn := ev.Buttons(); {
	case btn&tcell.WheelUp != 0:
		d.scroll(-3)
		return
	case btn&tcell.WheelDown != 0:
		d.scroll(3)
		return
	}

	if a.press {
		a.status = ""
		switch {
		case y == 0:
			for i, m := range a.menus {
				if x >= m.x && x < m.x+m.w {
					a.openMenu(i)
					break
				}
			}
		case !area.contains(x, y):
			for _, h := range a.hits {
				if h.area.contains(x, y) {
					h.run()
					break
				}
			}
		case x == area.x+area.w-1:
			a.drag = dragScrollBar
			d.scrollToBar(y - area.y)
		default:
			p, upper := d.posAt(x-textX, y-area.y)
			switch a.clicks {
			case 1:
				a.drag = dragText
				d.moveTo(p, ev.Modifiers()&tcell.ModShift != 0)
				d.upper = upper
			case 2:
				d.selectWord(p)
			default:
				d.selectLines(p.Line, p.Line)
			}
		}
		return
	}

	switch a.drag {
	case dragText:
		// Dragging past the top or bottom edge scrolls the text along.
		if y < area.y {
			d.scroll(-1)
		} else if y >= area.y+area.h {
			d.scroll(1)
		}
		p, upper := d.posAt(x-textX, min(max(y-area.y, 0), area.h-1))
		d.moveTo(p, true)
		d.upper = upper
	case dragScrollBar:
		d.scrollToBar(y - area.y)
	}
}
