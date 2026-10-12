// Command secnote is a terminal text editor that keeps its files encrypted
// with a password.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Max0n-1027/go-secnote/internal/ui"
)

var version = "0.1.0"

func main() {
	opts := ui.Options{Version: version}
	showVersion := flag.Bool("version", false, "バージョンを表示して終了する")
	flag.IntVar(&opts.TabWidth, "tabwidth", 4, "タブ幅 (1-16)")
	flag.BoolVar(&opts.Tabs, "tabs", false, "インデントにスペースではなくタブ文字を使う")
	flag.BoolVar(&opts.NoWrap, "nowrap", false, "右端で折り返さない")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "使い方: %s [オプション] [ファイル ...]\n\nオプション:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("secnote", version)
		return
	}
	if err := ui.Run(opts, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "secnote:", err)
		os.Exit(1)
	}
}
