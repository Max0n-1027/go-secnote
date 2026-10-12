package ui

import (
	"math/rand"
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestRandomInput hammers the editor with random keys, mouse events and
// window sizes and checks after each one that the cursor is somewhere the
// text allows and that the screen still draws.
func TestRandomInput(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(40, 12)
	a := newApp(scr, Options{TabWidth: 4})

	seed := int64(7)
	if s := os.Getenv("SECNOTE_SEED"); s != "" {
		seed, _ = strconv.ParseInt(s, 10, 64)
	}
	rng := rand.New(rand.NewSource(seed))
	text := []rune("ab c_d  \tあいカタ漢字、。!e\u0301👍🇯🇵\u200b")
	keys := []tcell.Key{
		tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyHome, tcell.KeyEnd,
		tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyEnter, tcell.KeyTab, tcell.KeyBacktab,
		tcell.KeyBackspace, tcell.KeyDelete,
	}
	mods := []tcell.ModMask{tcell.ModNone, tcell.ModNone, tcell.ModShift, tcell.ModCtrl, tcell.ModShift | tcell.ModCtrl}
	commands := []tcell.Key{
		tcell.KeyCtrlZ, tcell.KeyCtrlZ, tcell.KeyCtrlY, tcell.KeyCtrlA, tcell.KeyCtrlC, tcell.KeyCtrlX, tcell.KeyCtrlV,
	}

	check := func(step int, what string) {
		t.Helper()
		d := a.doc()
		for _, p := range []pos{d.cur, d.anchor} {
			if p != d.buf.Clamp(p) {
				t.Fatalf("step %d (%s): position %v is outside the text", step, what, p)
			}
			line := d.buf.Line(p.Line)
			if p.Col < len(line) && !slices.Contains(clusterStarts(line, len(line)), p.Col) {
				t.Fatalf("step %d (%s): position %v splits a character of %q", step, what, p, line)
			}
		}
		a.draw()
		if d.follow {
			t.Fatalf("step %d (%s): draw left the cursor unfollowed", step, what)
		}
	}

	maxLines := 0
	for step := range 6000 {
		var what string
		switch n := rng.Intn(100); {
		case n < 35:
			r := text[rng.Intn(len(text))]
			what = "type " + string(r)
			a.handle(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		case n < 75:
			ev := tcell.NewEventKey(keys[rng.Intn(len(keys))], 0, mods[rng.Intn(len(mods))])
			what = ev.Name()
			a.handle(ev)
		case n < 85:
			ev := tcell.NewEventKey(commands[rng.Intn(len(commands))], 0, tcell.ModCtrl)
			what = ev.Name()
			a.handle(ev)
		case n < 88:
			what = "toggle wrap"
			a.handle(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt))
		case n < 90:
			w, h := 4+rng.Intn(60), 3+rng.Intn(20)
			what = "resize"
			scr.SetSize(w, h)
			a.draw()
		default:
			// Mouse, kept off the menu and status bars, which open dialogs.
			w, h := scr.Size()
			x, y := rng.Intn(w), 1+rng.Intn(max(h-2, 1))
			btn := []tcell.ButtonMask{tcell.Button1, tcell.Button1, tcell.ButtonNone, tcell.WheelUp, tcell.WheelDown}[rng.Intn(5)]
			what = "mouse"
			ev := tcell.NewEventMouse(x, y, btn, tcell.ModNone)
			a.trackMouse(ev)
			a.handle(ev)
		}
		check(step, what)
		maxLines = max(maxLines, a.doc().buf.LineCount())
	}
	if maxLines < 5 {
		t.Fatalf("the run only ever reached %d lines of text", maxLines)
	}
}
