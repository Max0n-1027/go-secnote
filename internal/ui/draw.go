package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// The editing area uses the terminal's own colors. Bars and panels use the
// 16-color palette so they follow the terminal theme too.
var (
	stText      = tcell.StyleDefault
	stGutter    = tcell.StyleDefault.Dim(true)
	stSpecial   = tcell.StyleDefault.Dim(true)
	stSelection = tcell.StyleDefault.Reverse(true)
	stThumb     = tcell.StyleDefault.Reverse(true).Dim(true)
	stBar       = tcell.StyleDefault.Reverse(true)

	stPanel    = tcell.StyleDefault.Background(tcell.ColorSilver).Foreground(tcell.ColorBlack)
	stPanelDim = stPanel.Foreground(tcell.ColorGray)
	stPanelErr = stPanel.Foreground(tcell.ColorMaroon)
	stFocus    = tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
	stTitle    = stFocus.Bold(true)
	stField    = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	stFieldSel = stFocus
)

const (
	controlPictures = 0x2400 // U+2400 SYMBOL FOR NULL, start of the block
	deletePicture   = 0x2421 // U+2421 SYMBOL FOR DELETE
	replacementChar = 0xFFFD
)

// cluster splits the first grapheme cluster off s. disp is what to draw for
// it and w how many cells that takes. Control characters and clusters that
// would draw nothing get a visible stand-in, so no raw control byte ever
// reaches the terminal and every cluster has a cell the cursor can sit on.
func cluster(s string, state int) (raw, rest, disp string, w, next int) {
	raw, rest, w, next = uniseg.FirstGraphemeClusterInString(s, state)
	disp = raw
	switch r, _ := utf8.DecodeRuneInString(raw); {
	case r < 0x20:
		disp, w = string(rune(controlPictures)+r), 1
	case r == 0x7f:
		disp, w = string(rune(deletePicture)), 1
	case w == 0 || strings.ContainsFunc(raw, unicode.IsControl):
		disp, w = string(rune(replacementChar)), 1
	}
	return raw, rest, disp, w, next
}

// strWidth returns the number of cells drawStr needs for s.
func strWidth(s string) int {
	total, state := 0, -1
	for s != "" {
		var w int
		_, s, _, w, state = cluster(s, state)
		total += w
	}
	return total
}

// drawStr draws s at x, y using at most maxW cells and returns the number of
// cells used.
func drawStr(scr tcell.Screen, x, y, maxW int, s string, st tcell.Style) int {
	col, state := 0, -1
	for s != "" {
		var disp string
		var w int
		_, s, disp, w, state = cluster(s, state)
		if col+w > maxW {
			break
		}
		scr.Put(x+col, y, disp, st)
		col += w
	}
	return col
}

// drawLabel draws a label such as "開く(O)..." with its accelerator letter
// underlined.
func drawLabel(scr tcell.Screen, x, y, maxW int, label string, st tcell.Style) int {
	i := accelIndex(label)
	if i < 0 {
		return drawStr(scr, x, y, maxW, label, st)
	}
	n := drawStr(scr, x, y, maxW, label[:i], st)
	n += drawStr(scr, x+n, y, maxW-n, label[i:i+1], st.Underline(true))
	n += drawStr(scr, x+n, y, maxW-n, label[i+1:], st)
	return n
}

// accelIndex returns the byte index of the accelerator letter in a label that
// marks it as "(X)", or -1.
func accelIndex(label string) int {
	for i := 0; i+2 < len(label); i++ {
		if label[i] == '(' && label[i+2] == ')' && isAccel(label[i+1]) {
			return i + 1
		}
	}
	return -1
}

func isAccel(c byte) bool { return c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' }

// accelOf returns the lower-case accelerator letter of label, or 0.
func accelOf(label string) rune {
	if i := accelIndex(label); i >= 0 {
		return unicode.ToLower(rune(label[i]))
	}
	return 0
}

func fillRect(scr tcell.Screen, x, y, w, h int, st tcell.Style) {
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			scr.Put(col, row, " ", st)
		}
	}
}

// fitRight shortens s to at most maxW cells by cutting from the left, which
// keeps the end of a path visible.
func fitRight(s string, maxW int) string {
	if strWidth(s) <= maxW {
		return s
	}
	const mark = "..."
	for s != "" && strWidth(s)+len(mark) > maxW {
		_, s, _, _, _ = cluster(s, -1)
	}
	return mark + s
}

// noLineStart lists punctuation that must not begin a line. wrapText lets it
// hang past the right edge instead, where panels keep a margin.
const noLineStart = "、。，．,.!?！？)）]］}｝」』"

// wrapText breaks s into lines of at most w cells.
func wrapText(s string, w int) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		start, col, state := 0, 0, -1
		rest := para
		for off := 0; rest != ""; {
			var raw string
			var cw int
			raw, rest, _, cw, state = cluster(rest, state)
			if col+cw > w && col > 0 && !strings.Contains(noLineStart, raw) {
				lines = append(lines, para[start:off])
				start, col = off, 0
			}
			col += cw
			off += len(raw)
		}
		lines = append(lines, para[start:])
	}
	return lines
}

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}
