package ui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"secnote/internal/textbuf"
)

// findState is what the find bar remembers between uses.
type findState struct {
	needle, repl          lineInput
	caseSens, word, regex bool
}

// matcher finds occurrences of a search term within single lines.
type matcher struct {
	re    *regexp.Regexp
	word  bool // only whole words match
	regex bool // the replacement may refer to groups as $1
}

// matcher compiles the current search term. It returns nil when there is
// nothing to search for.
func (f *findState) matcher() (*matcher, error) {
	pattern := f.needle.text
	if pattern == "" {
		return nil, nil
	}
	if !f.regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !f.caseSens {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &matcher{re: re, word: f.word, regex: f.regex}, nil
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// wordBreak reports whether offset off of s is not in the middle of a word.
func wordBreak(s string, off int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:off])
	after, _ := utf8.DecodeRuneInString(s[off:])
	return !isWordRune(before) || !isWordRune(after)
}

// matches returns the matches in line, each as the index pairs of the match
// and its groups. Empty matches are left out.
func (m *matcher) matches(line string) [][]int {
	all := m.re.FindAllStringSubmatchIndex(line, -1)
	out := all[:0]
	for _, loc := range all {
		if loc[0] == loc[1] {
			continue
		}
		if m.word && !(wordBreak(line, loc[0]) && wordBreak(line, loc[1])) {
			continue
		}
		out = append(out, loc)
	}
	return out
}

// replacement returns what the match loc of line turns into.
func (m *matcher) replacement(line string, loc []int, repl string) string {
	if !m.regex {
		return repl
	}
	return string(m.re.ExpandString(nil, repl, line, loc))
}

// find returns the first match at or after from, or with forward false the
// last match that starts before from. The search wraps around the text.
func (d *document) find(m *matcher, from pos, forward bool) (start, end pos, ok bool) {
	n := d.buf.LineCount()
	hit := func(line int, loc []int) (pos, pos, bool) {
		return pos{Line: line, Col: loc[0]}, pos{Line: line, Col: loc[1]}, true
	}
	// Step 0 covers the part of the first line on the far side of from and
	// step n the rest of that line, after going all the way around.
	for i := 0; i <= n; i++ {
		if forward {
			line := (from.Line + i) % n
			for _, loc := range m.matches(d.buf.Line(line)) {
				if i == 0 && loc[0] < from.Col || i == n && loc[0] >= from.Col {
					continue
				}
				return hit(line, loc)
			}
		} else {
			line := ((from.Line-i)%n + n) % n
			locs := m.matches(d.buf.Line(line))
			for k := len(locs) - 1; k >= 0; k-- {
				if loc := locs[k]; !(i == 0 && loc[0] >= from.Col || i == n && loc[0] < from.Col) {
					return hit(line, loc)
				}
			}
		}
	}
	return pos{}, pos{}, false
}

// countMatches returns the number of matches in the text and the number,
// counting from 1, of the one that starts at at (0 if none does).
func (d *document) countMatches(m *matcher, at pos) (total, index int) {
	for line := 0; line < d.buf.LineCount(); line++ {
		for _, loc := range m.matches(d.buf.Line(line)) {
			total++
			if line == at.Line && loc[0] == at.Col {
				index = total
			}
		}
	}
	return total, index
}

// replaceMatch replaces the selection when it is exactly a match and
// reports whether it did.
func (d *document) replaceMatch(m *matcher, repl string) bool {
	from, to := d.selRange()
	if from.Line != to.Line {
		return false
	}
	line := d.buf.Line(from.Line)
	for _, loc := range m.matches(line) {
		if loc[0] == from.Col && loc[1] == to.Col {
			text := m.replacement(line, loc, repl)
			d.edit(textbuf.KindOther, func() { d.replaceSel(text) })
			return true
		}
	}
	return false
}

// replaceAll replaces every match in the text as one undo step and returns
// how many there were.
func (d *document) replaceAll(m *matcher, repl string) int {
	count := 0
	d.edit(textbuf.KindOther, func() {
		for i := 0; i < d.buf.LineCount(); i++ {
			line := d.buf.Line(i)
			locs := m.matches(line)
			if len(locs) == 0 {
				continue
			}
			var sb strings.Builder
			last := 0
			for _, loc := range locs {
				sb.WriteString(line[last:loc[0]])
				sb.WriteString(m.replacement(line, loc, repl))
				last = loc[1]
			}
			sb.WriteString(line[last:])
			d.buf.Replace(pos{Line: i}, pos{Line: i, Col: len(line)}, sb.String())
			count += len(locs)
		}
		if count > 0 {
			d.moveTo(pos{Line: d.cur.Line}, false)
		}
	})
	return count
}

// findAgain jumps to the next or previous match of the last search.
func (a *App) findAgain(forward bool) {
	m, _ := a.find.matcher()
	if m == nil {
		a.findBar(false)
		return
	}
	if !a.findStep(m, forward) {
		a.status = "見つかりません: " + a.find.needle.text
	}
}

// findStep selects the match after the selection, or the one before it.
func (a *App) findStep(m *matcher, forward bool) bool {
	d := a.doc()
	from, to := d.selRange()
	if forward {
		from = to
	}
	start, end, ok := d.find(m, from, forward)
	if ok {
		d.selectRange(start, end)
	}
	return ok
}

const findHint = "Enter/F3: 次  Shift+F3: 前  Tab: 欄の移動  Alt+A: すべて置換  Esc: 閉じる"

// findBar opens the search bar above the status bar, with a second row for
// replacing when replace is set, and returns when the user closes it.
func (a *App) findBar(replace bool) {
	d := a.doc()
	f := &a.find
	if from, to := d.selRange(); from != to && from.Line == to.Line {
		f.needle.set(d.selText())
	}
	f.needle.selectAll()

	var (
		m       *matcher
		errText string
		total   int
		index   int
		focus   = &f.needle
		toggles []hit
		passOn  *tcell.EventMouse
	)
	// origin is where a search for a changed term starts from, so that
	// typing more of a word keeps extending the same match.
	origin, _ := d.selRange()

	count := func() {
		total, index = 0, 0
		if m != nil {
			from, _ := d.selRange()
			total, index = d.countMatches(m, from)
		}
	}
	// refresh recompiles the term after it or an option changed and selects
	// the nearest match.
	refresh := func() {
		var err error
		m, err = f.matcher()
		errText = ""
		if err != nil {
			errText = "正規表現が正しくありません"
		}
		if m != nil {
			if start, end, ok := d.find(m, origin, true); ok {
				d.selectRange(start, end)
			} else {
				d.moveTo(origin, false)
			}
		}
		count()
	}
	step := func(forward bool) {
		if m != nil && a.findStep(m, forward) {
			origin, _ = d.selRange()
		}
		count()
	}
	replaceOne := func() {
		if m == nil {
			return
		}
		d.replaceMatch(m, f.repl.text)
		step(true)
	}
	replaceEvery := func() {
		if m == nil {
			return
		}
		n := d.replaceAll(m, f.repl.text)
		origin = d.cur
		count()
		a.status = fmt.Sprintf("%d 件置換しました", n)
	}
	setRows := func() {
		a.findRows = 1
		if replace {
			a.findRows = 2
		} else {
			focus = &f.needle
		}
		d.follow = true
	}
	setRows()
	refresh()
	a.status = findHint

	panel := func() {
		w, h := a.scr.Size()
		y := h - 1 - a.findRows
		fillRect(a.scr, 0, y, w, a.findRows, stPanel)

		options := []struct {
			label string
			on    *bool
		}{{"大小区別(C)", &f.caseSens}, {"単語単位(W)", &f.word}, {"正規表現(R)", &f.regex}}
		optW := 0
		for _, o := range options {
			optW += strWidth(o.label) + 5
		}
		const labelW, infoW = 7, 10
		inputW := max(w-labelW-optW-infoW-1, 12)

		drawStr(a.scr, 1, y, labelW, "検索:", stPanel)
		findX := f.needle.draw(a.scr, labelW, y, inputW, stField)
		x := labelW + inputW + 2
		toggles = toggles[:0]
		for _, o := range options {
			mark := "[ ]"
			if *o.on {
				mark = "[x]"
			}
			on := o.on
			ow := drawStr(a.scr, x, y, w-x, mark, stPanel)
			ow += drawLabel(a.scr, x+ow, y, w-x-ow, o.label, stPanel)
			toggles = append(toggles, hit{rect{x, y, ow, 1}, func() { *on = !*on }})
			x += ow + 2
		}
		switch {
		case errText != "":
			drawStr(a.scr, x, y, w-x, errText, stPanelErr)
		case m != nil && total == 0:
			drawStr(a.scr, x, y, w-x, "なし", stPanelErr)
		case m != nil && index == 0:
			drawStr(a.scr, x, y, w-x, fmt.Sprintf("%d 件", total), stPanel)
		case m != nil:
			drawStr(a.scr, x, y, w-x, fmt.Sprintf("%d/%d", index, total), stPanel)
		}

		replX := 0
		if replace {
			drawStr(a.scr, 1, y+1, labelW, "置換:", stPanel)
			replX = f.repl.draw(a.scr, labelW, y+1, inputW, stField)
		}
		if focus == &f.repl {
			a.scr.ShowCursor(replX, y+1)
		} else {
			a.scr.ShowCursor(findX, y)
		}
	}

	a.modal(panel, func(ev tcell.Event) bool {
		a.status = findHint
		switch ev := ev.(type) {
		case *tcell.EventKey:
			shift := ev.Modifiers()&tcell.ModShift != 0
			alt := ev.Modifiers()&tcell.ModAlt != 0
			switch key := ev.Key(); {
			case key == tcell.KeyEsc:
				return true
			case key == tcell.KeyEnter && focus == &f.repl:
				replaceOne()
			case key == tcell.KeyEnter, key == tcell.KeyDown:
				step(true)
			case key == tcell.KeyUp:
				step(false)
			case key == tcell.KeyF3:
				step(!shift)
			case key == tcell.KeyTab, key == tcell.KeyBacktab:
				if replace && focus == &f.needle {
					focus = &f.repl
				} else {
					focus = &f.needle
				}
			case key == tcell.KeyCtrlF:
				replace = false
				setRows()
			case key == tcell.KeyCtrlR:
				replace = true
				setRows()
				focus = &f.repl
			case key == tcell.KeyRune && alt:
				switch unicode.ToLower(ev.Rune()) {
				case 'c':
					f.caseSens = !f.caseSens
					refresh()
				case 'w':
					f.word = !f.word
					refresh()
				case 'r':
					f.regex = !f.regex
					refresh()
				case 'a':
					replaceEvery()
				}
			default:
				before := f.needle.text
				a.inputKey(focus, ev)
				if f.needle.text != before {
					refresh()
				}
			}

		case *pasteEvent:
			focus.insert(ev.text)
			if focus == &f.needle {
				refresh()
			}

		case *tcell.EventMouse:
			x, y := ev.Position()
			switch btn := ev.Buttons(); {
			case btn&tcell.WheelUp != 0:
				d.scroll(-3)
			case btn&tcell.WheelDown != 0:
				d.scroll(3)
			case !a.press:
			case f.needle.area.contains(x, y):
				focus = &f.needle
				focus.click(x)
			case replace && f.repl.area.contains(x, y):
				focus = &f.repl
				focus.click(x)
			case a.env.area.contains(x, y) || y == 0:
				// A click outside the bar closes it and then acts as usual.
				passOn = ev
				return true
			default:
				for _, t := range toggles {
					if t.area.contains(x, y) {
						t.run()
						refresh()
					}
				}
			}
		}
		return false
	})

	a.findRows = 0
	a.status = ""
	d.follow = true
	if passOn != nil {
		a.handleMouse(passOn)
	}
}
