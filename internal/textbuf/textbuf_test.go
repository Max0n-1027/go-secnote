package textbuf

import (
	"math/rand"
	"strings"
	"testing"
)

func at(p Pos) Sel { return Sel{p, p} }

// apply runs one Replace as its own undo step.
func apply(b *Buffer, kind Kind, from, to Pos, text string) Pos {
	b.Begin(kind, at(to))
	end := b.Replace(from, to, text)
	b.Commit(at(end))
	return end
}

func TestReplace(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		from, to Pos
		ins      string
		want     string
		wantEnd  Pos
	}{
		{"insert in line", "hello", Pos{0, 2}, Pos{0, 2}, "XY", "heXYllo", Pos{0, 4}},
		{"insert newline", "hello", Pos{0, 2}, Pos{0, 2}, "\n", "he\nllo", Pos{1, 0}},
		{"insert lines", "ab", Pos{0, 1}, Pos{0, 1}, "1\n2\n3", "a1\n2\n3b", Pos{2, 1}},
		{"delete in line", "hello", Pos{0, 1}, Pos{0, 4}, "", "ho", Pos{0, 1}},
		{"join lines", "a\nb\nc", Pos{0, 1}, Pos{1, 0}, "", "ab\nc", Pos{0, 1}},
		{"delete across lines", "abc\ndef\nghi", Pos{0, 1}, Pos{2, 2}, "", "ai", Pos{0, 1}},
		{"replace across lines", "abc\ndef\nghi", Pos{0, 1}, Pos{2, 2}, "X\nY", "aX\nYi", Pos{1, 1}},
		{"everything", "a\nb", Pos{0, 0}, Pos{1, 1}, "", "", Pos{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.text)
			end := apply(b, KindOther, tt.from, tt.to, tt.ins)
			if got := b.String(); got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
			if end != tt.wantEnd {
				t.Errorf("end = %v, want %v", end, tt.wantEnd)
			}
			if _, ok := b.Undo(); !ok || b.String() != tt.text {
				t.Errorf("after undo text = %q, want %q", b.String(), tt.text)
			}
			if _, ok := b.Redo(); !ok || b.String() != tt.want {
				t.Errorf("after redo text = %q, want %q", b.String(), tt.want)
			}
		})
	}
}

func TestTypingIsOneUndoStep(t *testing.T) {
	b := New("")
	p := Pos{}
	for _, r := range "hello" {
		p = apply(b, KindType, p, p, string(r))
	}
	if b.String() != "hello" {
		t.Fatalf("text = %q", b.String())
	}
	sel, _ := b.Undo()
	if b.String() != "" || sel != at(Pos{}) {
		t.Fatalf("after undo text = %q sel = %v", b.String(), sel)
	}
	if b.CanUndo() {
		t.Fatal("typing left more than one undo step")
	}
	sel, _ = b.Redo()
	if b.String() != "hello" || sel != at(Pos{0, 5}) {
		t.Fatalf("after redo text = %q sel = %v", b.String(), sel)
	}
}

func TestSealSplitsTyping(t *testing.T) {
	b := New("")
	p := apply(b, KindType, Pos{}, Pos{}, "a")
	b.Seal()
	apply(b, KindType, p, p, "b")
	b.Undo()
	if b.String() != "a" {
		t.Fatalf("text = %q, want %q", b.String(), "a")
	}
}

func TestMovingCursorSplitsTyping(t *testing.T) {
	b := New("")
	apply(b, KindType, Pos{}, Pos{}, "a")
	apply(b, KindType, Pos{}, Pos{}, "b") // typed at the start, not after "a"
	b.Undo()
	if b.String() != "a" {
		t.Fatalf("text = %q, want %q", b.String(), "a")
	}
}

func TestBackspaceAndDeleteMerge(t *testing.T) {
	b := New("abcdef")
	for col := 6; col > 3; col-- {
		b.Begin(KindBackspace, at(Pos{0, col}))
		b.Replace(Pos{0, col - 1}, Pos{0, col}, "")
		b.Commit(at(Pos{0, col - 1}))
	}
	for range 2 {
		b.Begin(KindDelete, at(Pos{}))
		b.Replace(Pos{}, Pos{0, 1}, "")
		b.Commit(at(Pos{}))
	}
	if b.String() != "c" {
		t.Fatalf("text = %q, want %q", b.String(), "c")
	}
	b.Undo()
	if b.String() != "abc" {
		t.Fatalf("after first undo text = %q, want %q", b.String(), "abc")
	}
	sel, _ := b.Undo()
	if b.String() != "abcdef" || sel != at(Pos{0, 6}) {
		t.Fatalf("after second undo text = %q sel = %v", b.String(), sel)
	}
}

func TestModified(t *testing.T) {
	b := New("x")
	if b.Modified() {
		t.Fatal("new buffer is modified")
	}
	p := apply(b, KindType, Pos{0, 1}, Pos{0, 1}, "a")
	if !b.Modified() {
		t.Fatal("edit did not mark the buffer modified")
	}
	b.MarkSaved()
	if b.Modified() {
		t.Fatal("still modified after MarkSaved")
	}

	// Typing after a save must not merge into the saved step.
	apply(b, KindType, p, p, "b")
	if !b.Modified() {
		t.Fatal("typing after save did not mark the buffer modified")
	}
	b.Undo()
	if b.Modified() || b.String() != "xa" {
		t.Fatalf("undo back to the saved text: modified = %v, text = %q", b.Modified(), b.String())
	}
	b.Undo()
	if !b.Modified() {
		t.Fatal("undo past the saved text is not modified")
	}
	b.Redo()
	if b.Modified() {
		t.Fatal("redo back to the saved text is modified")
	}
}

func TestEditAfterUndoDropsRedo(t *testing.T) {
	b := New("")
	apply(b, KindOther, Pos{}, Pos{}, "a")
	b.Undo()
	apply(b, KindOther, Pos{}, Pos{}, "b")
	if b.CanRedo() {
		t.Fatal("redo survived a new edit")
	}
}

func TestEmptyGroupLeavesNoUndoStep(t *testing.T) {
	b := New("abc")
	b.Begin(KindOther, Sel{})
	b.Replace(Pos{0, 1}, Pos{0, 1}, "")
	b.Commit(Sel{})
	if b.CanUndo() || b.Modified() {
		t.Fatal("a no-op edit left an undo step")
	}
}

// TestRandomEdits checks the buffer against plain string surgery, then undoes
// and redoes the whole history.
func TestRandomEdits(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pieces := []string{"", "a", "bc", "\n", "x\ny", "\n\n", "日本語", "q\n"}

	b := New("")
	want := ""
	history := []string{want}

	offset := func(p Pos) int {
		lines := strings.Split(want, "\n")
		off := p.Col
		for _, l := range lines[:p.Line] {
			off += len(l) + 1
		}
		return off
	}
	randPos := func() Pos {
		line := rng.Intn(b.LineCount())
		return Pos{line, rng.Intn(len(b.Line(line)) + 1)}
	}

	for range 500 {
		from, to := randPos(), randPos()
		if to.Before(from) {
			from, to = to, from
		}
		ins := pieces[rng.Intn(len(pieces))]
		lo, hi := offset(from), offset(to)
		if got := b.Text(from, to); got != want[lo:hi] {
			t.Fatalf("Text(%v, %v) = %q, want %q", from, to, got, want[lo:hi])
		}
		want = want[:lo] + ins + want[hi:]
		apply(b, KindOther, from, to, ins)
		if b.String() != want {
			t.Fatalf("text = %q, want %q", b.String(), want)
		}
		// Replacing text with itself is still an undo step; only an empty
		// replacement of nothing leaves none.
		if from != to || ins != "" {
			history = append(history, want)
		}
	}

	for i := len(history) - 2; i >= 0; i-- {
		if _, ok := b.Undo(); !ok || b.String() != history[i] {
			t.Fatalf("undo to step %d: text = %q, want %q", i, b.String(), history[i])
		}
	}
	if b.CanUndo() {
		t.Fatal("undo steps left over")
	}
	for i := 1; i < len(history); i++ {
		if _, ok := b.Redo(); !ok || b.String() != history[i] {
			t.Fatalf("redo to step %d: text = %q, want %q", i, b.String(), history[i])
		}
	}
}
