// Command secnote is a terminal text editor that keeps its files encrypted
// with a password.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Max0n-1027/go-secnote/internal/i18n"
	"github.com/Max0n-1027/go-secnote/internal/ui"
)

var version = "0.1.0"

func main() {
	opts := ui.Options{Version: version}
	langName := flag.String("lang", "", "")
	showVersion := flag.Bool("version", false, "")
	flag.IntVar(&opts.TabWidth, "tabwidth", 4, "")
	flag.BoolVar(&opts.Tabs, "tabs", false, "")
	flag.BoolVar(&opts.NoWrap, "nowrap", false, "")

	// The help comes in the language -lang asks for when it came earlier on
	// the command line, and in the one from the environment otherwise.
	flag.Usage = func() {
		m := i18n.For(chooseLang(*langName))
		for name, text := range map[string]string{
			"lang":     m.FlagLang,
			"version":  m.FlagVersion,
			"tabwidth": m.FlagTabWidth,
			"tabs":     m.FlagTabs,
			"nowrap":   m.FlagNoWrap,
		} {
			flag.Lookup(name).Usage = text
		}
		fmt.Fprintf(flag.CommandLine.Output(), m.Usage, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *langName != "" {
		if _, ok := i18n.Parse(*langName); !ok {
			m := i18n.For(i18n.Detect())
			fmt.Fprintln(os.Stderr, "secnote: "+fmt.Sprintf(m.BadLang, *langName))
			os.Exit(2)
		}
	}
	opts.Lang = chooseLang(*langName)

	if *showVersion {
		fmt.Println("secnote", version)
		return
	}
	if err := ui.Run(opts, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "secnote:", err)
		os.Exit(1)
	}
}

// chooseLang returns the language named by -lang, or the one from the
// environment when -lang is empty or names no supported language.
func chooseLang(name string) i18n.Lang {
	if l, ok := i18n.Parse(name); ok {
		return l
	}
	return i18n.Detect()
}
