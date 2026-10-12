package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"secnote/internal/vault"
)

func TestMain(m *testing.M) {
	vault.WorkFactor = 10
	os.Exit(m.Run())
}

// harness runs the editor on a simulated 80x24 screen.
type harness struct {
	t    *testing.T
	scr  tcell.SimulationScreen
	app  *App
	done chan struct{} // closed when the editor's main loop returns
}

func start(t *testing.T, paths ...string) *harness {
	t.Helper()
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	scr.SetSize(80, 24)
	h := &harness{t: t, scr: scr, done: make(chan struct{})}
	h.app = newApp(scr, Options{Version: "test", TabWidth: 4})
	go func() {
		defer close(h.done)
		h.app.run(paths)
	}()
	t.Cleanup(func() {
		// Stop the main loop from its own goroutine before tearing the
		// screen down under it.
		select {
		case <-h.done:
		default:
			h.post(tcell.NewEventInterrupt(func() { h.app.dead = true }))
			<-h.done
		}
		scr.Fini()
	})
	return h
}

// post queues ev behind the events already injected. PostEvent refuses
// events while the queue is full, so keep trying until there is room.
func (h *harness) post(ev tcell.Event) {
	h.t.Helper()
	for deadline := time.Now().Add(10 * time.Second); h.scr.PostEvent(ev) != nil; {
		if time.Now().After(deadline) {
			h.t.Fatal("the editor stopped handling events")
		}
		time.Sleep(time.Millisecond)
	}
}

// do runs fn on the editor's goroutine once every event injected so far has
// been handled and the screen redrawn.
func (h *harness) do(fn func()) {
	h.t.Helper()
	done := make(chan struct{})
	h.post(tcell.NewEventInterrupt(func() {
		defer close(done)
		fn()
	}))
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		h.t.Fatal("the editor stopped handling events")
	}
}

func (h *harness) key(k tcell.Key, mod ...tcell.ModMask) {
	m := tcell.ModNone
	for _, x := range mod {
		m |= x
	}
	h.scr.InjectKey(k, 0, m)
}

func (h *harness) ctrl(k tcell.Key) { h.scr.InjectKey(k, 0, tcell.ModCtrl) }

func (h *harness) alt(r rune) { h.scr.InjectKey(tcell.KeyRune, r, tcell.ModAlt) }

func (h *harness) typ(s string) {
	for _, r := range s {
		switch r {
		case '\n':
			h.key(tcell.KeyEnter)
		case '\t':
			h.key(tcell.KeyTab)
		default:
			h.scr.InjectKey(tcell.KeyRune, r, tcell.ModNone)
		}
	}
}

func (h *harness) click(x, y int) {
	h.scr.InjectMouse(x, y, tcell.Button1, tcell.ModNone)
	h.scr.InjectMouse(x, y, tcell.ButtonNone, tcell.ModNone)
}

func (h *harness) text() string {
	h.t.Helper()
	var s string
	h.do(func() { s = h.app.doc().buf.String() })
	return s
}

func (h *harness) wantText(want string) {
	h.t.Helper()
	if got := h.text(); got != want {
		h.t.Fatalf("text = %q, want %q", got, want)
	}
}

// screen returns the rows of the screen as text.
func (h *harness) screen() []string {
	h.t.Helper()
	var rows []string
	h.do(func() {
		w, ht := h.scr.Size()
		for y := 0; y < ht; y++ {
			var sb strings.Builder
			for x := 0; x < w; {
				s, _, cw := h.scr.Get(x, y)
				sb.WriteString(s)
				x += max(cw, 1)
			}
			rows = append(rows, strings.TrimRight(sb.String(), " "))
		}
	})
	return rows
}

func (h *harness) wantOnScreen(want string) {
	h.t.Helper()
	rows := h.screen()
	for _, row := range rows {
		if strings.Contains(row, want) {
			return
		}
	}
	h.t.Fatalf("%q is not on screen:\n%s", want, strings.Join(rows, "\n"))
}

func (h *harness) cursor() pos {
	h.t.Helper()
	var p pos
	h.do(func() { p = h.app.doc().cur })
	return p
}

func decrypt(t *testing.T, path, password string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !vault.IsEncrypted(data) {
		t.Fatalf("%s is not encrypted", path)
	}
	plaintext, err := vault.Decrypt(password, data)
	if err != nil {
		t.Fatal(err)
	}
	return string(plaintext)
}

func encryptTo(t *testing.T, path, password, text string) {
	t.Helper()
	data, err := vault.Encrypt(password, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTypingAndUndo(t *testing.T) {
	h := start(t)
	h.typ("hello\n    world\nx")
	// The second Enter carries the indentation over.
	h.wantText("hello\n    world\n    x")

	h.ctrl(tcell.KeyCtrlZ)
	h.wantText("hello\n    world\n    ")
	h.ctrl(tcell.KeyCtrlZ)
	h.wantText("hello\n    world")
	h.ctrl(tcell.KeyCtrlY)
	h.ctrl(tcell.KeyCtrlY)
	h.wantText("hello\n    world\n    x")
}

func TestJapaneseAndClusters(t *testing.T) {
	h := start(t)
	h.typ("日本語がき👨‍👩‍👧x")
	h.key(tcell.KeyLeft)
	h.key(tcell.KeyBackspace) // the whole family emoji goes at once
	h.wantText("日本語がきx")
	h.key(tcell.KeyHome)
	h.key(tcell.KeyRight)
	h.key(tcell.KeyDelete)
	h.wantText("日語がきx")
	if got := h.cursor(); got != (pos{Line: 0, Col: len("日")}) {
		t.Fatalf("cursor = %v", got)
	}
	h.wantOnScreen("日語がきx")
}

func TestWordMovementStopsAtScriptChange(t *testing.T) {
	h := start(t)
	h.typ("今日はコーヒーを飲む foo_bar")
	h.key(tcell.KeyHome)
	var stops []string
	for range 7 {
		h.key(tcell.KeyRight, tcell.ModCtrl)
		c := h.cursor()
		stops = append(stops, h.text()[:c.Col])
	}
	want := []string{"今日", "今日は", "今日はコーヒー", "今日はコーヒーを", "今日はコーヒーを飲", "今日はコーヒーを飲む ", "今日はコーヒーを飲む foo_bar"}
	for i := range want {
		if stops[i] != want[i] {
			t.Fatalf("stop %d = %q, want %q", i, stops[i], want[i])
		}
	}
}

func TestSelectionAndClipboard(t *testing.T) {
	h := start(t)
	h.typ("one two three")
	h.key(tcell.KeyHome)
	for range 3 {
		h.key(tcell.KeyRight, tcell.ModShift)
	}
	h.ctrl(tcell.KeyCtrlC)
	h.key(tcell.KeyEnd)
	h.ctrl(tcell.KeyCtrlV)
	h.wantText("one two threeone")

	h.key(tcell.KeyLeft, tcell.ModShift, tcell.ModCtrl)
	h.ctrl(tcell.KeyCtrlX)
	h.wantText("one two ")
	h.ctrl(tcell.KeyCtrlA)
	h.typ("z")
	h.wantText("z")
}

func TestIndentSelectedLines(t *testing.T) {
	h := start(t)
	h.typ("a\nb\nc")
	h.key(tcell.KeyUp, tcell.ModShift)
	h.key(tcell.KeyUp, tcell.ModShift)
	h.key(tcell.KeyTab)
	h.wantText("    a\n    b\n    c")
	h.key(tcell.KeyBacktab)
	h.wantText("a\nb\nc")
	h.ctrl(tcell.KeyCtrlZ)
	h.ctrl(tcell.KeyCtrlZ)
	h.wantText("a\nb\nc")
}

func TestBracketedPaste(t *testing.T) {
	h := start(t)
	h.typ("  x")
	h.post(tcell.NewEventPaste(true))
	h.typ("a")
	h.key(tcell.KeyEnter)
	h.ctrl(tcell.KeyCtrlJ) // CR LF from the terminal is one line break
	h.typ("b")
	h.ctrl(tcell.KeyCtrlJ)
	h.typ("c")
	h.post(tcell.NewEventPaste(false))
	// Pasted line breaks do not auto-indent.
	h.wantText("  xa\nb\nc")
	h.ctrl(tcell.KeyCtrlZ)
	h.wantText("  x")
}

func TestSaveAsThenOverwriteKeepsPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.age")
	h := start(t)
	h.typ("秘密のメモ")

	h.ctrl(tcell.KeyCtrlS)
	h.wantOnScreen("名前を付けて保存")
	h.typ(path)
	h.key(tcell.KeyEnter)
	h.wantOnScreen("パスワードの設定")

	// A mismatch keeps the dialog open.
	h.typ("hunter2")
	h.key(tcell.KeyEnter)
	h.typ("hunter3")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("一致しません")
	h.typ("hunter2")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("保存しました")

	if got := decrypt(t, path, "hunter2"); got != "秘密のメモ" {
		t.Fatalf("saved text = %q", got)
	}

	// Saving again asks for nothing and reuses the password.
	h.typ("\n追記")
	h.ctrl(tcell.KeyCtrlS)
	h.wantOnScreen("保存しました")
	if got := decrypt(t, path, "hunter2"); got != "秘密のメモ\n追記" {
		t.Fatalf("saved text = %q", got)
	}
	h.do(func() {
		if h.app.doc().modified() {
			t.Error("document still modified after saving")
		}
	})
}

func TestOpenAsksForPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memo.age")
	encryptTo(t, path, "correct horse", "line1\r\nline2\r\n")

	h := start(t, path)
	h.wantOnScreen("memo.age")
	h.typ("wrong")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("パスワードが違います")
	h.typ("correct horse") // replaces the rejected entry
	h.key(tcell.KeyEnter)
	h.wantText("line1\nline2\n")
	h.wantOnScreen("CRLF")
	h.wantOnScreen("暗号化")

	// Overwriting uses the password the file was opened with and keeps the
	// line endings.
	h.typ("new")
	h.ctrl(tcell.KeyCtrlS)
	h.wantOnScreen("保存しました")
	if got := decrypt(t, path, "correct horse"); got != "newline1\r\nline2\r\n" {
		t.Fatalf("saved text = %q", got)
	}
}

func TestOpenCancelledPasswordOpensNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memo.age")
	encryptTo(t, path, "pw", "secret")
	h := start(t, path)
	h.wantOnScreen("パスワード")
	h.key(tcell.KeyEsc)
	h.wantText("")
	h.do(func() {
		if len(h.app.docs) != 1 || h.app.doc().path != "" {
			t.Errorf("a document was opened without its password")
		}
	})
}

func TestOpenFromFilePicker(t *testing.T) {
	dir := t.TempDir()
	encryptTo(t, filepath.Join(dir, "b.age"), "pw", "from b")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	encryptTo(t, filepath.Join(dir, "sub", "c.age"), "pw", "from c")

	h := start(t)
	h.ctrl(tcell.KeyCtrlO)
	h.typ(dir)
	h.key(tcell.KeyEnter) // a folder: go into it
	h.wantOnScreen("sub/")
	h.wantOnScreen("b.age")

	// The list is "../", "sub/", "b.age".
	h.key(tcell.KeyDown)
	h.key(tcell.KeyDown)
	h.key(tcell.KeyEnter)
	h.wantOnScreen("c.age")
	h.key(tcell.KeyDown)
	h.key(tcell.KeyDown)
	h.key(tcell.KeyEnter)
	h.typ("pw")
	h.key(tcell.KeyEnter)
	h.wantText("from c")
}

func TestPlainFileIsSavedAsEncryptedCopy(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "todo.txt")
	if err := os.WriteFile(plain, []byte("buy milk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := start(t, plain)
	h.wantText("buy milk\n")
	h.wantOnScreen("暗号化されていないファイルです")
	h.key(tcell.KeyHome) // any key clears the notice
	h.wantOnScreen("平文")

	h.typ("- ")
	h.ctrl(tcell.KeyCtrlS)
	h.wantOnScreen("todo.txt.age") // offered as the name to save under
	h.key(tcell.KeyEnter)
	h.typ("pw")
	h.key(tcell.KeyEnter)
	h.typ("pw")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("保存しました")

	if got := decrypt(t, plain+".age", "pw"); got != "- buy milk\n" {
		t.Fatalf("encrypted copy = %q", got)
	}
	if got, _ := os.ReadFile(plain); string(got) != "buy milk\n" {
		t.Fatalf("the plain original was changed to %q", got)
	}
}

func TestRejectsNonUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sjis.txt")
	if err := os.WriteFile(path, []byte{0x93, 0xfa, 0x96, 0x7b}, 0o644); err != nil {
		t.Fatal(err)
	}
	h := start(t, path)
	h.wantOnScreen("UTF-8 のテキストではありません")
	h.key(tcell.KeyEnter)
	h.wantText("")
}

func TestQuitAsksAboutUnsavedChanges(t *testing.T) {
	h := start(t)
	h.typ("draft")
	h.ctrl(tcell.KeyCtrlQ)
	h.wantOnScreen("変更を保存しますか")
	h.key(tcell.KeyEsc)
	h.do(func() {
		if h.app.done {
			t.Error("quit despite cancelling")
		}
	})
	h.ctrl(tcell.KeyCtrlQ)
	h.typ("n")
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not quit")
	}
}

func TestFindAndReplace(t *testing.T) {
	h := start(t)
	h.typ("Foo bar\nfoo baz\nfood")
	h.key(tcell.KeyHome, tcell.ModCtrl)

	h.ctrl(tcell.KeyCtrlF)
	h.typ("foo")
	h.wantOnScreen("1/3")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("2/3")
	h.alt('c') // match case
	h.wantOnScreen("1/2")
	h.alt('w') // whole words
	h.wantOnScreen("1/1")
	if got := h.cursor(); got != (pos{Line: 1, Col: 3}) {
		t.Fatalf("cursor = %v, want the end of the match on line 2", got)
	}
	h.alt('c')
	h.alt('w')

	// Replace one, then the rest.
	h.ctrl(tcell.KeyCtrlR)
	h.typ("qux")
	h.key(tcell.KeyEnter)
	h.wantText("Foo bar\nqux baz\nfood")
	h.alt('a')
	h.wantText("qux bar\nqux baz\nquxd")
	h.wantOnScreen("2 件置換しました")
	h.key(tcell.KeyEsc)

	h.ctrl(tcell.KeyCtrlZ)
	h.wantText("Foo bar\nqux baz\nfood")
}

func TestRegexReplace(t *testing.T) {
	h := start(t)
	h.typ("2024-01-31\n2025-12-01")
	h.ctrl(tcell.KeyCtrlR)
	h.typ(`(\d+)-(\d+)-(\d+)`)
	h.alt('r')
	h.key(tcell.KeyTab)
	h.typ("$3/$2/$1")
	h.alt('a')
	h.key(tcell.KeyEsc)
	h.wantText("31/01/2024\n01/12/2025")
}

func TestGotoLine(t *testing.T) {
	h := start(t)
	h.typ(strings.Repeat("x\n", 99))
	h.ctrl(tcell.KeyCtrlG)
	h.typ("abc")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("数字で入力")
	h.typ("42")
	h.key(tcell.KeyEnter)
	if got := h.cursor(); got != (pos{Line: 41}) {
		t.Fatalf("cursor = %v", got)
	}
	h.wantOnScreen("行 42, 列 1")
}

func TestMouse(t *testing.T) {
	h := start(t)
	h.typ("alpha beta\ngamma")
	gutter := 5

	h.click(gutter+7, 1)
	if got := h.cursor(); got != (pos{Line: 0, Col: 7}) {
		t.Fatalf("cursor after click = %v", got)
	}
	h.click(gutter+7, 1) // second click on the same spot: select the word
	h.do(func() {
		if got := h.app.doc().selText(); got != "beta" {
			t.Errorf("double click selected %q", got)
		}
	})

	// Drag across the first word of line 2.
	h.scr.InjectMouse(gutter, 2, tcell.Button1, tcell.ModNone)
	h.scr.InjectMouse(gutter+3, 2, tcell.Button1, tcell.ModNone)
	h.scr.InjectMouse(gutter+3, 2, tcell.ButtonNone, tcell.ModNone)
	h.do(func() {
		if got := h.app.doc().selText(); got != "gam" {
			t.Errorf("drag selected %q", got)
		}
	})
}

func TestMenu(t *testing.T) {
	h := start(t)
	h.typ("abc")
	h.alt('e')
	h.wantOnScreen("元に戻す(U)")
	h.typ("a") // すべて選択(A)
	h.do(func() {
		if got := h.app.doc().selText(); got != "abc" {
			t.Errorf("Select All from the menu selected %q", got)
		}
	})

	// With the mouse: open 表示 and switch wrapping off.
	h.click(25, 0)
	h.wantOnScreen("右端で折り返す(W): オン")
	h.click(27, 1)
	h.do(func() {
		if h.app.env.wrap {
			t.Error("the menu item did not switch wrapping off")
		}
	})
}

func TestWrapKeepsCursorVisible(t *testing.T) {
	h := start(t)
	long := strings.Repeat("あいうえお ", 40)
	h.typ(long)
	rows := h.screen()
	if !strings.Contains(rows[1], "あいうえお") || !strings.Contains(rows[2], "あいうえお") {
		t.Fatalf("long line is not wrapped:\n%s", strings.Join(rows, "\n"))
	}
	h.do(func() {
		if x, y, visible := h.scr.GetCursor(); !visible || y < 2 || x >= 79 {
			t.Errorf("cursor at %d,%d visible=%v", x, y, visible)
		}
	})

	// Up and down move by screen rows within the one long line.
	h.key(tcell.KeyUp)
	if got := h.cursor(); got.Line != 0 || got.Col >= len(long) {
		t.Fatalf("cursor = %v", got)
	}
	h.alt('z')
	rows = h.screen()
	if strings.Contains(rows[2], "あいうえお") {
		t.Fatalf("line still wrapped after Alt+Z:\n%s", strings.Join(rows, "\n"))
	}
}

func TestMultipleDocuments(t *testing.T) {
	h := start(t)
	h.typ("first")
	h.ctrl(tcell.KeyCtrlN)
	h.typ("second")
	h.wantOnScreen("[2/2]")
	h.ctrl(tcell.KeyCtrlP)
	h.wantOnScreen("ドキュメント一覧")
	h.key(tcell.KeyUp)
	h.key(tcell.KeyEnter)
	h.wantText("first")

	h.ctrl(tcell.KeyCtrlW)
	h.typ("n") // do not save
	h.wantText("second")
}

func TestHomeEndOnWrappedRows(t *testing.T) {
	h := start(t)
	line := strings.Repeat("0123456789", 20) // 200 columns: three rows on an 80 column screen
	h.typ(line)
	h.key(tcell.KeyHome, tcell.ModCtrl)

	// End stops at the end of the first row and the cursor stays on that
	// row rather than jumping to the start of the next one.
	h.key(tcell.KeyEnd)
	var rowEnd int
	h.do(func() {
		d := h.app.doc()
		rowEnd = d.layout(0).rowEnd(0)
		if d.cur.Col != rowEnd || rowEnd >= len(line) {
			t.Errorf("cursor = %v, want the end of the first row (%d)", d.cur, rowEnd)
		}
		if _, y, _ := h.scr.GetCursor(); y != 1 {
			t.Errorf("cursor is on screen row %d, want 1", y)
		}
	})

	h.key(tcell.KeyDown)
	h.key(tcell.KeyHome)
	if got := h.cursor(); got.Col != rowEnd {
		t.Fatalf("Home on the second row went to %v, want column %d", got, rowEnd)
	}
	h.key(tcell.KeyEnd, tcell.ModCtrl)
	h.key(tcell.KeyHome)
	h.key(tcell.KeyHome)
	h.typ("x")
	h.do(func() {
		if d := h.app.doc(); d.cur.Line != 0 || d.cur.Col == 0 || d.cur.Col > len(line) {
			t.Errorf("cursor = %v", d.cur)
		}
	})
}

func TestScrollingFollowsCursor(t *testing.T) {
	h := start(t)
	var sb strings.Builder
	for i := 1; i <= 200; i++ {
		sb.WriteString("line ")
		sb.WriteString(strings.Repeat("#", i%7))
		sb.WriteString("\n")
	}
	h.post(tcell.NewEventPaste(true))
	h.typ(strings.ReplaceAll(sb.String(), "\n", "\r"))
	h.post(tcell.NewEventPaste(false))
	h.wantOnScreen(" 201")

	h.key(tcell.KeyHome, tcell.ModCtrl)
	h.wantOnScreen("   1 line #")
	h.key(tcell.KeyPgDn)
	h.key(tcell.KeyPgDn)
	h.do(func() {
		d := h.app.doc()
		if d.cur.Line != 42 || d.topLine != 42 {
			t.Errorf("after two pages: cursor line %d, top line %d, want 42", d.cur.Line, d.topLine)
		}
		if _, _, visible := h.scr.GetCursor(); !visible {
			t.Error("cursor scrolled out of view")
		}
	})

	// The wheel moves the view only; typing brings it back to the cursor.
	h.scr.InjectMouse(10, 10, tcell.WheelDown, tcell.ModNone)
	h.scr.InjectMouse(10, 10, tcell.WheelDown, tcell.ModNone)
	h.do(func() {
		if d := h.app.doc(); d.topLine != 48 || d.cur.Line != 42 {
			t.Errorf("after wheel: top line %d, cursor line %d", d.topLine, d.cur.Line)
		}
	})
	h.typ("!")
	h.do(func() {
		if d := h.app.doc(); d.topLine != 42 {
			t.Errorf("typing left the view at line %d", d.topLine)
		}
	})
}
