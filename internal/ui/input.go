package ui

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// nextBoundary returns the offset of the cluster boundary after off.
func nextBoundary(s string, off int) int {
	if off >= len(s) {
		return len(s)
	}
	c, _, _, _ := uniseg.FirstGraphemeClusterInString(s[off:], -1)
	return off + len(c)
}

// prevBoundary returns the offset of the cluster boundary before off.
func prevBoundary(s string, off int) int {
	prev, state := 0, -1
	for i, rest := 0, s; i < off && rest != ""; {
		var c string
		c, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		prev = i
		i += len(c)
	}
	return prev
}

// singleLine makes text safe for a one-line field.
func singleLine(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}

// lineInput is a one-line text field.
type lineInput struct {
	text   string
	cur    int  // byte offset of the cursor
	all    bool // the whole text is selected; typing replaces it
	masked bool // show stars instead of the text
	scroll int  // first visible column

	area rect // where it was last drawn
}

func (in *lineInput) set(text string) {
	in.text, in.cur, in.all, in.scroll = text, len(text), false, 0
}

func (in *lineInput) selectAll() {
	in.cur, in.all = len(in.text), in.text != ""
}

func (in *lineInput) insert(text string) {
	text = singleLine(text)
	if text == "" {
		return
	}
	if in.all {
		in.set("")
	}
	in.text = in.text[:in.cur] + text + in.text[in.cur:]
	in.cur += len(text)
}

// handleKey edits the field and reports whether it used the key.
func (in *lineInput) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
			return false
		}
		in.insert(string(ev.Rune()))
	case tcell.KeyBackspace:
		if in.all {
			in.set("")
		} else if in.cur > 0 {
			p := prevBoundary(in.text, in.cur)
			in.text = in.text[:p] + in.text[in.cur:]
			in.cur = p
		}
	case tcell.KeyDelete:
		if in.all {
			in.set("")
		} else if in.cur < len(in.text) {
			in.text = in.text[:in.cur] + in.text[nextBoundary(in.text, in.cur):]
		}
	case tcell.KeyLeft:
		if in.all {
			in.cur = 0
		} else {
			in.cur = prevBoundary(in.text, in.cur)
		}
	case tcell.KeyRight:
		in.cur = nextBoundary(in.text, in.cur)
	case tcell.KeyHome:
		in.cur = 0
	case tcell.KeyEnd:
		in.cur = len(in.text)
	case tcell.KeyCtrlA:
		in.selectAll()
		return true
	default:
		return false
	}
	in.all = false
	return true
}

// cells returns what the field shows: the text itself, or one star per
// character when masked.
func (in *lineInput) cells() *layout {
	l := layoutLine(in.text, 1, 0)
	if in.masked {
		for i := range l.cells {
			c := &l.cells[i]
			c.disp, c.col, c.w, c.special = "*", i, 1, false
		}
	}
	return l
}

// draw paints the field and returns the screen column of its cursor.
func (in *lineInput) draw(scr tcell.Screen, x, y, w int, st tcell.Style) int {
	in.area = rect{x, y, w, 1}
	l := in.cells()
	cur := l.colOf(in.cur, 0)
	if cur < in.scroll {
		in.scroll = cur
	}
	if cur >= in.scroll+w {
		in.scroll = cur - w + 1
	}

	fillRect(scr, x, y, w, 1, st)
	if in.all {
		st = stFieldSel
	}
	for _, c := range l.cells {
		col := c.col - in.scroll
		if col < 0 {
			continue
		}
		if col+c.w > w {
			break
		}
		disp := c.disp
		if disp == "" {
			disp = " "
		}
		scr.Put(x+col, y, disp, st)
	}
	return x + cur - in.scroll
}

// click moves the cursor to screen column x of the last drawn field.
func (in *lineInput) click(x int) {
	in.cur, _ = in.cells().offAt(0, x-in.area.x+in.scroll)
	in.all = false
}
