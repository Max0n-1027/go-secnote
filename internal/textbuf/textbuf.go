// Package textbuf holds the text of a document as a slice of lines and keeps
// the undo history of every change made to it.
package textbuf

import (
	"slices"
	"strings"
)

// Pos addresses a point in the text. Col is a byte offset into the line.
type Pos struct{ Line, Col int }

// Before reports whether p comes earlier in the text than q.
func (p Pos) Before(q Pos) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col
}

// Sel is a cursor together with the other end of its selection. The two are
// equal when nothing is selected.
type Sel struct{ Cursor, Anchor Pos }

// Kind says what sort of change a group of edits is. Consecutive groups of the
// same kind, other than KindOther, are merged into one undo step.
type Kind int

const (
	KindOther Kind = iota
	KindType
	KindBackspace
	KindDelete
)

type edit struct {
	at       Pos
	removed  string
	inserted string
}

// group is one undo step.
type group struct {
	id            int
	kind          Kind
	edits         []edit
	before, after Sel
	sealed        bool // never merge later edits into this group
}

// Buffer is the text of one document. Lines are separated by "\n" on the way
// in and out and never contain it themselves.
type Buffer struct {
	lines      []string
	undo, redo []*group
	open       *group
	reopened   bool // open is already on the undo stack
	nextID     int
	savedID    int
}

// New returns a buffer holding text.
func New(text string) *Buffer {
	return &Buffer{lines: strings.Split(text, "\n")}
}

func (b *Buffer) LineCount() int    { return len(b.lines) }
func (b *Buffer) Line(i int) string { return b.lines[i] }

// End returns the position after the last byte of the text.
func (b *Buffer) End() Pos {
	last := len(b.lines) - 1
	return Pos{last, len(b.lines[last])}
}

// Clamp moves p to the nearest position that exists in the text.
func (b *Buffer) Clamp(p Pos) Pos {
	p.Line = min(max(p.Line, 0), len(b.lines)-1)
	p.Col = min(max(p.Col, 0), len(b.lines[p.Line]))
	return p
}

// Text returns the text between from and to.
func (b *Buffer) Text(from, to Pos) string {
	if from.Line == to.Line {
		return b.lines[from.Line][from.Col:to.Col]
	}
	var sb strings.Builder
	sb.WriteString(b.lines[from.Line][from.Col:])
	for i := from.Line + 1; i < to.Line; i++ {
		sb.WriteByte('\n')
		sb.WriteString(b.lines[i])
	}
	sb.WriteByte('\n')
	sb.WriteString(b.lines[to.Line][:to.Col])
	return sb.String()
}

// String returns the whole text.
func (b *Buffer) String() string {
	return strings.Join(b.lines, "\n")
}

// endOf returns where text ends when it starts at at.
func endOf(at Pos, text string) Pos {
	n := strings.Count(text, "\n")
	if n == 0 {
		return Pos{at.Line, at.Col + len(text)}
	}
	return Pos{at.Line + n, len(text) - strings.LastIndexByte(text, '\n') - 1}
}

func (b *Buffer) replace(from, to Pos, text string) (removed string) {
	removed = b.Text(from, to)
	ins := strings.Split(text, "\n")
	head := b.lines[from.Line][:from.Col]
	tail := b.lines[to.Line][to.Col:]
	last := len(ins) - 1
	if last == 0 {
		ins[0] = head + ins[0] + tail
	} else {
		ins[0] = head + ins[0]
		ins[last] += tail
	}
	b.lines = slices.Replace(b.lines, from.Line, to.Line+1, ins...)
	return removed
}

// Begin starts an undo step. before is the selection to restore when the step
// is undone. Every Begin must be followed by a Commit.
func (b *Buffer) Begin(kind Kind, before Sel) {
	if n := len(b.undo); n > 0 && kind != KindOther {
		if top := b.undo[n-1]; !top.sealed && top.kind == kind && top.after == before {
			b.open, b.reopened = top, true
			return
		}
	}
	b.nextID++
	b.open, b.reopened = &group{id: b.nextID, kind: kind, before: before}, false
}

// Replace swaps the text between from and to for text and returns the end of
// the inserted text. It may only be called between Begin and Commit.
func (b *Buffer) Replace(from, to Pos, text string) Pos {
	if b.open == nil {
		panic("textbuf: Replace outside Begin/Commit")
	}
	end := endOf(from, text)
	if from == to && text == "" {
		return end
	}
	removed := b.replace(from, to, text)
	b.record(edit{at: from, removed: removed, inserted: text})
	return end
}

// record appends e to the open group, folding it into the previous edit when
// the two are adjacent runs of typing or of deleting.
func (b *Buffer) record(e edit) {
	g := b.open
	if n := len(g.edits); n > 0 {
		last := &g.edits[n-1]
		switch {
		case e.removed == "" && last.removed == "" && e.at == endOf(last.at, last.inserted):
			last.inserted += e.inserted
			return
		case e.inserted == "" && last.inserted == "" && e.at == last.at:
			last.removed += e.removed
			return
		case e.inserted == "" && last.inserted == "" && endOf(e.at, e.removed) == last.at:
			last.at = e.at
			last.removed = e.removed + last.removed
			return
		}
	}
	g.edits = append(g.edits, e)
}

// Commit finishes the undo step started by Begin. after is the selection to
// restore when the step is redone.
func (b *Buffer) Commit(after Sel) {
	g, reopened := b.open, b.reopened
	b.open, b.reopened = nil, false
	if g == nil || len(g.edits) == 0 {
		return
	}
	g.after = after
	if !reopened {
		b.undo = append(b.undo, g)
	}
	b.redo = nil
}

// Seal stops later edits from being merged into the latest undo step.
func (b *Buffer) Seal() {
	if n := len(b.undo); n > 0 {
		b.undo[n-1].sealed = true
	}
}

// Undo reverts the latest step and returns the selection from before it.
func (b *Buffer) Undo() (Sel, bool) {
	n := len(b.undo)
	if n == 0 {
		return Sel{}, false
	}
	g := b.undo[n-1]
	b.undo = b.undo[:n-1]
	for i := len(g.edits) - 1; i >= 0; i-- {
		e := g.edits[i]
		b.replace(e.at, endOf(e.at, e.inserted), e.removed)
	}
	g.sealed = true
	b.redo = append(b.redo, g)
	b.Seal()
	return g.before, true
}

// Redo reapplies the step most recently undone and returns the selection from
// after it.
func (b *Buffer) Redo() (Sel, bool) {
	n := len(b.redo)
	if n == 0 {
		return Sel{}, false
	}
	g := b.redo[n-1]
	b.redo = b.redo[:n-1]
	for _, e := range g.edits {
		b.replace(e.at, endOf(e.at, e.removed), e.inserted)
	}
	b.undo = append(b.undo, g)
	return g.after, true
}

func (b *Buffer) CanUndo() bool { return len(b.undo) > 0 }
func (b *Buffer) CanRedo() bool { return len(b.redo) > 0 }

func (b *Buffer) topID() int {
	if n := len(b.undo); n > 0 {
		return b.undo[n-1].id
	}
	return 0
}

// Modified reports whether the text differs from what MarkSaved last saw.
func (b *Buffer) Modified() bool { return b.topID() != b.savedID }

// MarkSaved records the current text as the saved one.
func (b *Buffer) MarkSaved() {
	b.Seal()
	b.savedID = b.topID()
}
