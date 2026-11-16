package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Max0n-1027/go-secnote/internal/i18n"
)

func TestEnglishInterface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.age")
	h := startIn(t, i18n.English)
	h.wantOnScreen("File  Edit  View  Help")
	h.wantOnScreen("Untitled 1")
	h.wantOnScreen("Not saved   Ln 1, Col 1   Spaces: 4   LF")

	h.typ("hello")
	h.alt('f')
	h.wantOnScreen("Save As...")
	h.typ("a") // Save &As...
	h.wantOnScreen("File name:")
	h.typ(path)
	h.key(tcell.KeyEnter)
	h.wantOnScreen("Set Password")
	h.wantOnScreen("Choose a password to encrypt note.age.")
	h.typ("pw")
	h.key(tcell.KeyEnter)
	h.typ("px")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("The passwords do not match")
	h.typ("pw")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("Saved note.age")
	if got := decrypt(t, path, "pw"); got != "hello" {
		t.Fatalf("saved text = %q", got)
	}

	h.ctrl(tcell.KeyCtrlF)
	h.typ("zzz")
	h.wantOnScreen("Find:")
	h.wantOnScreen("Match Case")
	h.wantOnScreen("No matches")
	h.key(tcell.KeyEsc)

	h.typ("!")
	h.ctrl(tcell.KeyCtrlQ)
	h.wantOnScreen("Save changes to note.age?")
	h.typ("n") // Do&n't Save
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not quit after Don't Save")
	}
}

func TestEnglishErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memo.age")
	encryptTo(t, path, "right", "secret")
	h := startIn(t, i18n.English, path)
	h.wantOnScreen("Enter the password for memo.age.")
	h.typ("wrong")
	h.key(tcell.KeyEnter)
	h.wantOnScreen("Wrong password")
	h.key(tcell.KeyEsc)

	sjis := filepath.Join(dir, "sjis.txt")
	if err := os.WriteFile(sjis, []byte{0x93, 0xfa, 0x96, 0x7b}, 0o644); err != nil {
		t.Fatal(err)
	}
	h.ctrl(tcell.KeyCtrlO)
	h.typ(sjis)
	h.key(tcell.KeyEnter)
	h.wantOnScreen("Could not open sjis.txt")
	h.wantOnScreen("It is not UTF-8 text.")
}

// Every menu entry and button needs an accelerator, unique among its
// neighbours, in every language. The find bar's options and the buttons of
// the save prompt are also bound to fixed Alt and letter keys, so those
// letters must not change between languages either.
func TestAcceleratorsInEveryLanguage(t *testing.T) {
	for _, lang := range i18n.Langs {
		scr := tcell.NewSimulationScreen("UTF-8")
		if err := scr.Init(); err != nil {
			t.Fatal(err)
		}
		a := newApp(scr, Options{Lang: lang, TabWidth: 4})

		titles := map[rune]string{}
		for _, m := range a.menus {
			k := accelOf(m.label)
			if k == 0 || titles[k] != "" {
				t.Errorf("%s: menu %q has no accelerator or shares %q with %q", lang, m.label, k, titles[k])
			}
			titles[k] = m.label
			items := map[rune]string{}
			for _, it := range m.items {
				if it.title() == "" {
					continue
				}
				k := accelOf(it.title())
				if k == 0 || items[k] != "" {
					t.Errorf("%s: %q has no accelerator or shares %q with %q", lang, it.title(), k, items[k])
				}
				items[k] = it.title()
			}
		}

		m := a.msg
		fixed := []struct {
			label string
			key   rune
		}{
			{m.MatchCase, 'c'}, {m.WholeWord, 'w'}, {m.Regex, 'r'},
			{m.SaveButton, 's'}, {m.DontSave, 'n'}, {m.Overwrite, 'o'},
		}
		for _, f := range fixed {
			if got := accelOf(f.label); got != f.key {
				t.Errorf("%s: %q has accelerator %q, want %q", lang, f.label, got, f.key)
			}
		}
		scr.Fini()
	}
}

func TestLabelText(t *testing.T) {
	tests := []struct {
		label, text string
		accel       int
		key         rune
	}{
		{"&File", "File", 0, 'f'},
		{"Save &As...", "Save As...", 5, 'a'},
		{"ファイル(&F)", "ファイル(F)", len("ファイル("), 'f'},
		{"OK", "OK", -1, 0},
		{"Trailing&", "Trailing&", -1, 0},
	}
	for _, tt := range tests {
		text, accel := labelText(tt.label)
		if text != tt.text || accel != tt.accel || accelOf(tt.label) != tt.key {
			t.Errorf("labelText(%q) = %q, %d, key %q; want %q, %d, key %q",
				tt.label, text, accel, accelOf(tt.label), tt.text, tt.accel, tt.key)
		}
		if labelWidth(tt.label) != strWidth(tt.text) {
			t.Errorf("labelWidth(%q) = %d, want %d", tt.label, labelWidth(tt.label), strWidth(tt.text))
		}
	}
}
