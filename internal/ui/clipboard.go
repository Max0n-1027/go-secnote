package ui

import (
	sysclip "github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
)

// clipboard copies and pastes through the operating system's clipboard
// where there is one and keeps its own copy for when there is not.
type clipboard struct {
	scr    tcell.Screen
	system bool // use the operating system's clipboard
	local  string
}

func (c *clipboard) set(text string) {
	c.local = text
	if c.system && sysclip.WriteAll(text) == nil {
		return
	}
	// No clipboard tool to hand, as over SSH: ask the terminal itself to
	// take the text. Terminals that cannot simply ignore the request.
	c.scr.SetClipboard([]byte(text))
}

func (c *clipboard) get() string {
	if c.system {
		if text, err := sysclip.ReadAll(); err == nil {
			return text
		}
	}
	return c.local
}
