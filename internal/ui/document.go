package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"secnote/internal/textbuf"
)

type pos = textbuf.Pos

// undoPause is how long typing must stop before the next keystroke starts a
// new undo step.
const undoPause = time.Second

// env holds what every open document shares: the editing settings and the
// part of the screen documents are drawn in.
type env struct {
	tabW   int
	spaces bool // indent with spaces instead of tabs
	wrap   bool
	area   rect
}

// document is one open file: its text, where it is stored and how it is
// currently shown.
type document struct {
	env *env
	buf *textbuf.Buffer

	path      string // absolute; "" until the document has been saved
	name      string // shown while path is ""
	password  string // "" until one has been set
	plain     bool   // read from a file that was not encrypted
	crlf      bool   // lines end in CR LF when written out
	savedCRLF bool

	cur, anchor pos
	upper       bool // at a wrap point the cursor sits at the end of the upper row
	wantCol     int  // column that vertical moves aim for; -1 when unset

	topLine, topRow int
	left            int
	follow          bool // bring the cursor into view at the next draw
	lastEdit        time.Time
}

func newDocument(e *env, text string) *document {
	return &document{env: e, buf: textbuf.New(text), wantCol: -1}
}

func (d *document) title() string {
	if d.path != "" {
		return filepath.Base(d.path)
	}
	return d.name
}

func (d *document) modified() bool {
	return d.buf.Modified() || d.crlf != d.savedCRLF
}

// pristine reports whether the document is an untouched new one.
func (d *document) pristine() bool {
	return d.path == "" && !d.buf.CanUndo() && !d.buf.CanRedo() && d.buf.String() == ""
}

// text returns the document as it is written to disk.
func (d *document) text() string {
	s := d.buf.String()
	if d.crlf {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	return s
}

func (d *document) markSaved() {
	d.buf.MarkSaved()
	d.savedCRLF = d.crlf
}

// Geometry

func (d *document) gutterW() int {
	return max(len(fmt.Sprint(d.buf.LineCount())), 3) + 2
}

// textW is the width of the text columns: the area minus the line numbers
// and the scroll bar.
func (d *document) textW() int {
	return max(d.env.area.w-d.gutterW()-1, 1)
}

func (d *document) layout(line int) *layout {
	wrapW := 0
	if d.env.wrap {
		// One column stays free so the cursor fits after a full row.
		wrapW = max(d.textW()-1, 2)
	}
	return layoutLine(d.buf.Line(line), d.env.tabW, wrapW)
}

// rows returns how many screen rows line takes.
func (d *document) rows(line int) int {
	if !d.env.wrap {
		return 1
	}
	return len(d.layout(line).rows)
}

// advance moves n screen rows down (or up, when n is negative) from row of
// line. It stops at either end of the text and returns where it got to and
// how far it moved.
func (d *document) advance(line, row, n int) (toLine, toRow, moved int) {
	if n > 0 {
		rows := d.rows(line)
		for ; moved < n; moved++ {
			switch {
			case row < rows-1:
				row++
			case line < d.buf.LineCount()-1:
				line++
				row, rows = 0, d.rows(line)
			default:
				return line, row, moved
			}
		}
	}
	for ; moved > n; moved-- {
		switch {
		case row > 0:
			row--
		case line > 0:
			line--
			row = d.rows(line) - 1
		default:
			return line, row, moved
		}
	}
	return line, row, moved
}

// rowsBetween counts the screen rows from row of line down to toRow of
// toLine, giving up once the count reaches limit.
func (d *document) rowsBetween(line, row, toLine, toRow, limit int) int {
	if toLine-line >= limit {
		return limit
	}
	n := toRow - row
	for ; line < toLine && n < limit; line++ {
		n += d.rows(line)
	}
	return n
}

func (d *document) scroll(n int) {
	d.topLine, d.topRow, _ = d.advance(d.topLine, d.topRow, n)
}

// scrollToBar moves the view so its position in the text matches row y of
// the scroll bar.
func (d *document) scrollToBar(y int) {
	h := d.env.area.h
	y = min(max(y, 0), h-1)
	d.topLine = y * (d.buf.LineCount() - 1) / max(h-1, 1)
	d.topRow = 0
}

func (d *document) scrollToCursor() {
	l := d.layout(d.cur.Line)
	row := l.rowOf(d.cur.Col, d.upper)
	h := d.env.area.h
	if d.cur.Line < d.topLine || d.cur.Line == d.topLine && row < d.topRow {
		d.topLine, d.topRow = d.cur.Line, row
	} else if d.rowsBetween(d.topLine, d.topRow, d.cur.Line, row, h) >= h {
		d.topLine, d.topRow, _ = d.advance(d.cur.Line, row, -(h - 1))
	}

	if d.env.wrap {
		d.left = 0
		return
	}
	col, w := l.colOf(d.cur.Col, row), d.textW()
	if col < d.left {
		d.left = col
	} else if col >= d.left+w {
		d.left = col - w + 1
	}
}

// posAt returns the text position shown at column x, row y of the text
// columns. Points outside the text map to the nearest position.
func (d *document) posAt(x, y int) (p pos, upper bool) {
	if y < 0 {
		y = 0
	}
	line, row, moved := d.advance(d.topLine, d.topRow, y)
	if moved < y {
		return d.buf.End(), false
	}
	off, upper := d.layout(line).offAt(row, max(x, 0)+d.left)
	return pos{Line: line, Col: off}, upper
}

func (d *document) draw(scr tcell.Screen, focused bool) {
	a := d.env.area
	d.topLine = min(d.topLine, d.buf.LineCount()-1)
	d.topRow = min(d.topRow, d.rows(d.topLine)-1)
	if d.follow {
		d.scrollToCursor()
		d.follow = false
	}

	gutter, textW := d.gutterW(), d.textW()
	textX := a.x + gutter
	from, to := d.selRange()
	curX, curY := -1, -1

	line, row := d.topLine, d.topRow
	var l *layout
	for y := a.y; y < a.y+a.h && line < d.buf.LineCount(); y++ {
		if l == nil {
			l = d.layout(line)
		}
		if row == 0 {
			drawStr(scr, a.x, y, gutter, fmt.Sprintf("%*d", gutter-1, line+1), stGutter)
		}

		// The selected byte range of this line, and whether the selection
		// runs on past its end.
		selFrom, selTo, selEOL := 0, 0, false
		if from != to && line >= from.Line && line <= to.Line {
			selTo, selEOL = l.size, true
			if line == from.Line {
				selFrom = from.Col
			}
			if line == to.Line {
				selTo, selEOL = to.Col, false
			}
		}

		span := l.rows[row]
		for _, c := range l.cells[span.first:span.last] {
			x := c.col - d.left
			if x >= textW {
				break
			}
			if x+c.w <= 0 {
				continue
			}
			st := stText
			switch {
			case c.off >= selFrom && c.off < selTo:
				st = stSelection
			case c.special:
				st = stSpecial
			}
			if c.disp == "" || x < 0 || x+c.w > textW {
				// A tab, or a wide character cut off by the edge.
				for i := max(x, 0); i < min(x+c.w, textW); i++ {
					scr.Put(textX+i, y, " ", st)
				}
			} else {
				scr.Put(textX+x, y, c.disp, st)
			}
		}
		last := row == len(l.rows)-1
		if x := l.rowWidth(row) - d.left; last && selEOL && x >= 0 && x < textW {
			scr.Put(textX+x, y, " ", stSelection)
		}
		if line == d.cur.Line && row == l.rowOf(d.cur.Col, d.upper) {
			if x := l.colOf(d.cur.Col, row) - d.left; x >= 0 && x < textW {
				curX, curY = textX+x, y
			}
		}

		if row++; last {
			line, row, l = line+1, 0, nil
		}
	}

	if total := d.buf.LineCount(); total > a.h {
		thumb := max(a.h*a.h/total, 1)
		top := d.topLine * (a.h - thumb) / (total - 1)
		fillRect(scr, a.x+a.w-1, a.y+top, 1, thumb, stThumb)
	}
	if focused {
		scr.ShowCursor(curX, curY)
	}
}

// Selection

func (d *document) sel() textbuf.Sel { return textbuf.Sel{Cursor: d.cur, Anchor: d.anchor} }

func (d *document) hasSel() bool { return d.cur != d.anchor }

func (d *document) selRange() (from, to pos) {
	from, to = d.anchor, d.cur
	if to.Before(from) {
		from, to = to, from
	}
	return from, to
}

func (d *document) selText() string {
	from, to := d.selRange()
	return d.buf.Text(from, to)
}

// selectRange selects from anchor to cursor.
func (d *document) selectRange(anchor, cursor pos) {
	d.moveTo(cursor, false)
	d.anchor = anchor
}

func (d *document) selectAll() {
	d.selectRange(pos{}, d.buf.End())
	d.follow = false
}

// selectLines selects lines first to last whole, keeping the direction of
// the current selection.
func (d *document) selectLines(first, last int) {
	from, to := pos{Line: first}, pos{Line: last, Col: len(d.buf.Line(last))}
	if last+1 < d.buf.LineCount() {
		to = pos{Line: last + 1}
	}
	if d.cur.Before(d.anchor) {
		from, to = to, from
	}
	d.selectRange(from, to)
}

// selLines returns the lines the selection touches. A selection that ends at
// the very start of a line does not count that line.
func (d *document) selLines() (first, last int) {
	from, to := d.selRange()
	if to.Line > from.Line && to.Col == 0 {
		to.Line--
	}
	return from.Line, to.Line
}

// Character classes for word-wise movement. Japanese text has no spaces, so
// a change of script is treated as a word boundary.
const (
	clsSpace = iota
	clsWord
	clsHiragana
	clsKatakana
	clsHan
	clsPunct
)

const prolongedSoundMark = 0x30FC

// classOf classifies the cluster at the start of s.
func classOf(s string) int {
	r, _ := utf8.DecodeRuneInString(s)
	switch {
	case unicode.IsSpace(r):
		return clsSpace
	case r == prolongedSoundMark || unicode.In(r, unicode.Katakana):
		return clsKatakana
	case unicode.In(r, unicode.Hiragana):
		return clsHiragana
	case unicode.In(r, unicode.Han):
		return clsHan
	case r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
		return clsWord
	}
	return clsPunct
}

// clusterStarts returns the offsets of the clusters of s that start before
// end.
func clusterStarts(s string, end int) []int {
	var starts []int
	for i := 0; i < end; i = nextBoundary(s, i) {
		starts = append(starts, i)
	}
	return starts
}

// wordAt returns the bounds of the word, or run of spaces or punctuation,
// around offset off of s.
func wordAt(s string, off int) (from, to int) {
	starts := clusterStarts(s, len(s))
	if len(starts) == 0 {
		return 0, 0
	}
	i := len(starts) - 1
	for i > 0 && starts[i] > off {
		i--
	}
	cls := classOf(s[starts[i]:])
	lo, hi := i, i
	for lo > 0 && classOf(s[starts[lo-1]:]) == cls {
		lo--
	}
	for hi+1 < len(starts) && classOf(s[starts[hi+1]:]) == cls {
		hi++
	}
	return starts[lo], nextBoundary(s, starts[hi])
}

// Cursor movement

func (d *document) moveTo(p pos, extend bool) {
	d.cur = p
	if !extend {
		d.anchor = p
	}
	d.upper = false
	d.wantCol = -1
	d.follow = true
}

func (d *document) leftOf(p pos) pos {
	if p.Col > 0 {
		return pos{Line: p.Line, Col: prevBoundary(d.buf.Line(p.Line), p.Col)}
	}
	if p.Line > 0 {
		return pos{Line: p.Line - 1, Col: len(d.buf.Line(p.Line - 1))}
	}
	return p
}

func (d *document) rightOf(p pos) pos {
	if s := d.buf.Line(p.Line); p.Col < len(s) {
		return pos{Line: p.Line, Col: nextBoundary(s, p.Col)}
	}
	if p.Line < d.buf.LineCount()-1 {
		return pos{Line: p.Line + 1}
	}
	return p
}

func (d *document) wordLeftOf(p pos) pos {
	if p.Col == 0 {
		return d.leftOf(p)
	}
	s := d.buf.Line(p.Line)
	starts := clusterStarts(s, p.Col)
	i := len(starts) - 1
	for i >= 0 && classOf(s[starts[i]:]) == clsSpace {
		i--
	}
	if i >= 0 {
		for cls := classOf(s[starts[i]:]); i >= 0 && classOf(s[starts[i]:]) == cls; {
			i--
		}
	}
	return pos{Line: p.Line, Col: starts[i+1]}
}

func (d *document) wordRightOf(p pos) pos {
	s := d.buf.Line(p.Line)
	if p.Col >= len(s) {
		return d.rightOf(p)
	}
	off := p.Col
	if cls := classOf(s[off:]); cls != clsSpace {
		for off < len(s) && classOf(s[off:]) == cls {
			off = nextBoundary(s, off)
		}
	}
	for off < len(s) && classOf(s[off:]) == clsSpace {
		off = nextBoundary(s, off)
	}
	return pos{Line: p.Line, Col: off}
}

func (d *document) moveLeft(extend, word bool) {
	switch from, _ := d.selRange(); {
	case d.hasSel() && !extend:
		d.moveTo(from, false)
	case word:
		d.moveTo(d.wordLeftOf(d.cur), extend)
	default:
		d.moveTo(d.leftOf(d.cur), extend)
	}
}

func (d *document) moveRight(extend, word bool) {
	switch _, to := d.selRange(); {
	case d.hasSel() && !extend:
		d.moveTo(to, false)
	case word:
		d.moveTo(d.wordRightOf(d.cur), extend)
	default:
		d.moveTo(d.rightOf(d.cur), extend)
	}
}

// moveRows moves the cursor n screen rows, keeping to the column it started
// from across a run of such moves.
func (d *document) moveRows(n int, extend bool) {
	l := d.layout(d.cur.Line)
	row := l.rowOf(d.cur.Col, d.upper)
	want := d.wantCol
	if want < 0 {
		want = l.colOf(d.cur.Col, row)
	}

	line, row, moved := d.advance(d.cur.Line, row, n)
	switch {
	case moved == n:
		off, upper := d.layout(line).offAt(row, want)
		d.moveTo(pos{Line: line, Col: off}, extend)
		d.upper = upper
	case n < 0:
		d.moveTo(pos{}, extend)
	default:
		d.moveTo(d.buf.End(), extend)
	}
	d.wantCol = want
}

func (d *document) movePage(dir int, extend bool) {
	n := dir * max(d.env.area.h-1, 1)
	d.scroll(n)
	d.moveRows(n, extend)
}

// moveHome goes to the first non-blank character of the line, or to the very
// start when already there. On a wrapped row it goes to the start of the row.
func (d *document) moveHome(extend bool) {
	s := d.buf.Line(d.cur.Line)
	l := d.layout(d.cur.Line)
	target := l.rowStart(l.rowOf(d.cur.Col, d.upper))
	if target == 0 {
		if indent := indentLen(s); d.cur.Col != indent {
			target = indent
		}
	}
	d.moveTo(d.snap(pos{Line: d.cur.Line, Col: target}), extend)
}

func (d *document) moveEnd(extend bool) {
	l := d.layout(d.cur.Line)
	row := l.rowOf(d.cur.Col, d.upper)
	d.moveTo(pos{Line: d.cur.Line, Col: l.rowEnd(row)}, extend)
	d.upper = row < len(l.rows)-1
}

// selectWord selects the word at p.
func (d *document) selectWord(p pos) {
	from, to := wordAt(d.buf.Line(p.Line), p.Col)
	d.selectRange(pos{Line: p.Line, Col: from}, pos{Line: p.Line, Col: to})
}

// column returns the cursor's column, counting from 1, as if the line were
// not wrapped.
func (d *document) column() int {
	return layoutLine(d.buf.Line(d.cur.Line), d.env.tabW, 0).colOf(d.cur.Col, 0) + 1
}

// Editing

// edit runs fn as one undo step.
func (d *document) edit(kind textbuf.Kind, fn func()) {
	now := time.Now()
	if now.Sub(d.lastEdit) > undoPause {
		d.buf.Seal()
	}
	d.lastEdit = now
	d.buf.Begin(kind, d.sel())
	fn()
	d.buf.Commit(d.sel())
}

// replaceSel puts text in place of the selection and leaves the cursor after
// it. It must run inside edit.
func (d *document) replaceSel(text string) {
	from, to := d.selRange()
	d.moveTo(d.snap(d.buf.Replace(from, to, text)), false)
}

// snap returns p moved forward to the end of the cluster it is in. An edit
// can leave a position inside a cluster: a letter typed in front of a
// combining mark joins up with it.
func (d *document) snap(p pos) pos {
	s := d.buf.Line(p.Line)
	col := 0
	for col < p.Col {
		col = nextBoundary(s, col)
	}
	return pos{Line: p.Line, Col: col}
}

// typeText inserts text the way typing it would.
func (d *document) typeText(text string) {
	kind := textbuf.KindType
	if d.hasSel() {
		kind = textbuf.KindOther
	}
	d.edit(kind, func() { d.replaceSel(text) })
}

// paste inserts text that came from outside the editor.
func (d *document) paste(text string) {
	text = strings.ToValidUTF8(text, string(rune(replacementChar)))
	text = normalizeEOL(text)
	if text == "" {
		return
	}
	d.edit(textbuf.KindOther, func() { d.replaceSel(text) })
}

func (d *document) deleteSel() {
	if d.hasSel() {
		d.edit(textbuf.KindOther, func() { d.replaceSel("") })
	}
}

// deleteTo removes the text between the cursor and p.
func (d *document) deleteTo(p pos, kind textbuf.Kind) {
	switch {
	case d.hasSel():
		d.deleteSel()
	case p != d.cur:
		from, to := p, d.cur
		if to.Before(from) {
			from, to = to, from
		}
		d.edit(kind, func() { d.moveTo(d.snap(d.buf.Replace(from, to, "")), false) })
	}
}

func (d *document) backspace() {
	p := d.leftOf(d.cur)
	// In space indentation, step back a whole indent level.
	if s := d.buf.Line(d.cur.Line); d.env.spaces && d.cur.Col > 0 && strings.Trim(s[:d.cur.Col], " ") == "" {
		p.Col = (d.cur.Col - 1) / d.env.tabW * d.env.tabW
	}
	d.deleteTo(p, textbuf.KindBackspace)
}

func (d *document) deleteForward()   { d.deleteTo(d.rightOf(d.cur), textbuf.KindDelete) }
func (d *document) deleteWordLeft()  { d.deleteTo(d.wordLeftOf(d.cur), textbuf.KindOther) }
func (d *document) deleteWordRight() { d.deleteTo(d.wordRightOf(d.cur), textbuf.KindOther) }

// newline breaks the line and carries its indentation over to the new one.
func (d *document) newline() {
	from, _ := d.selRange()
	s := d.buf.Line(from.Line)
	indent := s[:min(indentLen(s), from.Col)]
	d.edit(textbuf.KindOther, func() { d.replaceSel("\n" + indent) })
}

func indentLen(s string) int {
	return len(s) - len(strings.TrimLeft(s, " \t"))
}

func (d *document) tab() {
	if first, last := d.selLines(); first != last {
		d.indentLines(first, last)
		return
	}
	text := "\t"
	if d.env.spaces {
		from, _ := d.selRange()
		col := layoutLine(d.buf.Line(from.Line), d.env.tabW, 0).colOf(from.Col, 0)
		text = strings.Repeat(" ", d.env.tabW-col%d.env.tabW)
	}
	d.typeText(text)
}

func (d *document) indentLines(first, last int) {
	unit := "\t"
	if d.env.spaces {
		unit = strings.Repeat(" ", d.env.tabW)
	}
	d.edit(textbuf.KindOther, func() {
		for i := first; i <= last; i++ {
			if d.buf.Line(i) != "" {
				d.buf.Replace(pos{Line: i}, pos{Line: i}, unit)
			}
		}
		d.selectLines(first, last)
	})
}

// backtab removes one level of indentation from the selected lines, or from
// the cursor's line when nothing is selected.
func (d *document) backtab() {
	first, last := d.selLines()
	hadSel := d.hasSel()
	d.edit(textbuf.KindOther, func() {
		for i := first; i <= last; i++ {
			s := d.buf.Line(i)
			n := 0
			if strings.HasPrefix(s, "\t") {
				n = 1
			} else {
				for n < d.env.tabW && n < len(s) && s[n] == ' ' {
					n++
				}
			}
			d.buf.Replace(pos{Line: i}, pos{Line: i, Col: n}, "")
			if !hadSel {
				d.moveTo(d.snap(pos{Line: i, Col: max(d.cur.Col-n, 0)}), false)
			}
		}
		if hadSel {
			d.selectLines(first, last)
		}
	})
}

func (d *document) undo() bool {
	s, ok := d.buf.Undo()
	if ok {
		d.selectRange(s.Anchor, s.Cursor)
	}
	return ok
}

func (d *document) redo() bool {
	s, ok := d.buf.Redo()
	if ok {
		d.selectRange(s.Anchor, s.Cursor)
	}
	return ok
}

// normalizeEOL turns CR LF and lone CR line ends into LF.
func normalizeEOL(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	return strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
}
