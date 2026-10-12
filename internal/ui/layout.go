package ui

import "sort"

// cell is one grapheme cluster of a line placed on screen.
type cell struct {
	off, end int    // byte range in the line
	col, w   int    // column within its row and width in cells
	disp     string // what to draw; "" for a tab
	special  bool   // disp is a stand-in for something unprintable
	space    bool
}

// rowSpan is the run of cells that share one screen row.
type rowSpan struct{ first, last int }

// layout places the clusters of one line on one or more screen rows.
type layout struct {
	cells []cell
	rows  []rowSpan
	size  int // length of the line in bytes
}

// layoutLine lays out line with tab stops every tabW cells. When wrapW is
// positive the line is broken into rows at most wrapW cells wide, between
// words where possible.
func layoutLine(line string, tabW, wrapW int) *layout {
	l := &layout{size: len(line)}
	state := -1
	for off, rest := 0, line; rest != ""; {
		var raw, disp string
		var w int
		raw, rest, disp, w, state = cluster(rest, state)
		c := cell{off: off, end: off + len(raw), w: w, disp: disp, special: disp != raw}
		switch raw {
		case "\t":
			c.disp, c.special, c.space = "", false, true
		case " ":
			c.space = true
		}
		l.cells = append(l.cells, c)
		off = c.end
	}

	first, col := 0, 0
	for i := 0; i < len(l.cells); {
		c := &l.cells[i]
		w := c.w
		if c.disp == "" {
			w = tabW - col%tabW
			if wrapW > 0 && col < wrapW {
				w = min(w, wrapW-col)
			}
		}
		if wrapW > 0 && col > 0 && col+w > wrapW {
			brk := i
			for k := i; k > first; k-- {
				if l.breakBefore(k) {
					brk = k
					break
				}
			}
			l.rows = append(l.rows, rowSpan{first, brk})
			first, i, col = brk, brk, 0
			continue
		}
		c.col, c.w = col, w
		col += w
		i++
	}
	l.rows = append(l.rows, rowSpan{first, len(l.cells)})
	return l
}

// breakBefore reports whether a row may end just before cell i: after
// white space, or on either side of a full-width character.
func (l *layout) breakBefore(i int) bool {
	prev, cur := l.cells[i-1], l.cells[i]
	if prev.space {
		return !cur.space
	}
	return isWide(prev) || isWide(cur)
}

func isWide(c cell) bool { return c.disp != "" && c.w == 2 }

// rowStart returns the byte offset where row r begins.
func (l *layout) rowStart(r int) int {
	if i := l.rows[r].first; i < len(l.cells) {
		return l.cells[i].off
	}
	return l.size
}

// rowEnd returns the byte offset just past row r.
func (l *layout) rowEnd(r int) int {
	if i := l.rows[r].last; i < len(l.cells) {
		return l.cells[i].off
	}
	return l.size
}

func (l *layout) rowWidth(r int) int {
	span := l.rows[r]
	if span.first == span.last {
		return 0
	}
	c := l.cells[span.last-1]
	return c.col + c.w
}

// rowOf returns the row holding byte offset off. The offset where a line
// wraps belongs to both rows; upper picks the one above.
func (l *layout) rowOf(off int, upper bool) int {
	for r := len(l.rows) - 1; r > 0; r-- {
		if start := l.rowStart(r); off > start || off == start && !upper {
			return r
		}
	}
	return 0
}

// colOf returns the column of byte offset off within row r.
func (l *layout) colOf(off, r int) int {
	if off >= l.rowEnd(r) {
		return l.rowWidth(r)
	}
	i := sort.Search(len(l.cells), func(i int) bool { return l.cells[i].end > off })
	if i < l.rows[r].first {
		return 0
	}
	return l.cells[i].col
}

// offAt returns the byte offset nearest to column col of row r. upper is set
// when that offset is the wrap point at the end of the row.
func (l *layout) offAt(r, col int) (off int, upper bool) {
	span := l.rows[r]
	off = l.rowEnd(r)
	for _, c := range l.cells[span.first:span.last] {
		if col < c.col+c.w {
			off = c.off
			if (col-c.col)*2 >= c.w {
				off = c.end
			}
			break
		}
	}
	return off, off == l.rowEnd(r) && r < len(l.rows)-1
}
