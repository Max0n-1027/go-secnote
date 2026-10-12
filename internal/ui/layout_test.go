package ui

import (
	"reflect"
	"strings"
	"testing"
)

// rowTexts returns the text of each row of l.
func rowTexts(l *layout, line string) []string {
	var rows []string
	for r := range l.rows {
		rows = append(rows, line[l.rowStart(r):l.rowEnd(r)])
	}
	return rows
}

func TestLayoutWrap(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		wrapW int
		want  []string
	}{
		{"empty", "", 10, []string{""}},
		{"fits", "hello", 10, []string{"hello"}},
		{"exact fit", "0123456789", 10, []string{"0123456789"}},
		{"between words", "hello brave world", 12, []string{"hello brave ", "world"}},
		{"long word is cut", "abcdefghijklmno", 6, []string{"abcdef", "ghijkl", "mno"}},
		{"full-width breaks anywhere", "あいうえおかきく", 6, []string{"あいう", "えおか", "きく"}},
		{"full-width never straddles", "aあいう", 4, []string{"aあ", "いう"}},
		{"mixed scripts", "ab あいう cd", 6, []string{"ab あ", "いう ", "cd"}},
		{"no wrap", "hello brave world", 0, []string{"hello brave world"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := layoutLine(tt.line, 4, tt.wrapW)
			if got := rowTexts(l, tt.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rows = %q, want %q", got, tt.want)
			}
			for r := range l.rows {
				if w := l.rowWidth(r); tt.wrapW > 0 && w > tt.wrapW {
					t.Errorf("row %d is %d cells wide, limit %d", r, w, tt.wrapW)
				}
			}
		})
	}
}

func TestLayoutTabs(t *testing.T) {
	line := "a\tbc\t\td"
	l := layoutLine(line, 4, 0)
	var cols []int
	for _, c := range l.cells {
		cols = append(cols, c.col)
	}
	if want := []int{0, 1, 4, 5, 6, 8, 12}; !reflect.DeepEqual(cols, want) {
		t.Fatalf("columns = %v, want %v", cols, want)
	}
	if got := l.rowWidth(0); got != 13 {
		t.Fatalf("width = %d, want 13", got)
	}
}

func TestLayoutNeverShowsControlCharacters(t *testing.T) {
	line := "a\x1b[31m\x00\x7f" + string(rune(0x9b)) + string(rune(0x200b)) + "b"
	for _, c := range layoutLine(line, 4, 0).cells {
		for _, r := range c.disp {
			if r < 0x20 || r >= 0x7f && r < 0xa0 {
				t.Fatalf("cell for %q draws the control character %U", line[c.off:c.end], r)
			}
		}
		if c.w < 1 {
			t.Fatalf("cell for %q has no width", line[c.off:c.end])
		}
	}
}

// Every cluster boundary must survive the trip to a screen position and
// back, on whichever row it is shown.
func TestLayoutPositionsRoundTrip(t *testing.T) {
	lines := []string{
		"",
		"plain ascii text that wraps a couple of times over",
		"日本語とEnglishが混ざった行。全角と半角 spaces\tand tabs",
		"e\u0301 👨\u200d👩\u200d👧 🇯🇵 か\u3099",
		strings.Repeat("長", 40),
	}
	for _, line := range lines {
		for _, wrapW := range []int{0, 7, 20} {
			l := layoutLine(line, 4, wrapW)
			for off := 0; ; off = nextBoundary(line, off) {
				for _, upper := range []bool{false, true} {
					r := l.rowOf(off, upper)
					if off < l.rowStart(r) || off > l.rowEnd(r) {
						t.Fatalf("%q wrap %d: offset %d placed on row %d (%d-%d)", line, wrapW, off, r, l.rowStart(r), l.rowEnd(r))
					}
					got, gotUpper := l.offAt(r, l.colOf(off, r))
					if got != off {
						t.Fatalf("%q wrap %d: offset %d on row %d came back as %d", line, wrapW, off, r, got)
					}
					if l.rowOf(got, gotUpper) != r {
						t.Fatalf("%q wrap %d: offset %d left row %d", line, wrapW, off, r)
					}
				}
				if off >= len(line) {
					break
				}
			}
		}
	}
}

func TestWrapText(t *testing.T) {
	got := wrapText("パスワードを入力してください。\n2行目", 28)
	want := []string{"パスワードを入力してください。", "2行目"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q, want %q: the full stop must hang, not start a line", got, want)
	}
	got = wrapText("あいうえお", 4)
	if want := []string{"あい", "うえ", "お"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
}
