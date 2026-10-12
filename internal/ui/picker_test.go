package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.age", "A.txt", "c.age"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Zeta", "alpha"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"../", "alpha/", "Zeta/", "A.txt", "b.age", "c.age"}

	if runtime.GOOS != "windows" { // symlinks need privileges there
		if err := os.Symlink(filepath.Join(dir, "alpha"), filepath.Join(dir, "link-to-dir")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(dir, "b.age"), filepath.Join(dir, "link-to-file")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "broken-link")); err != nil {
			t.Fatal(err)
		}
		want = []string{"../", "alpha/", "link-to-dir/", "Zeta/", "A.txt", "b.age", "c.age", "link-to-file"}
	}

	got, err := listDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %q, want %q", got, want)
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	if got, want := resolvePath("note.age", dir), filepath.Join(dir, "note.age"); got != want {
		t.Errorf("relative name: got %q, want %q", got, want)
	}
	if got, want := resolvePath("../", dir), filepath.Dir(dir); got != want {
		t.Errorf("parent: got %q, want %q", got, want)
	}
	if got := resolvePath(dir, "/somewhere/else"); got != dir {
		t.Errorf("absolute path: got %q, want %q", got, dir)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if got, want := resolvePath("~/x.age", dir), filepath.Join(home, "x.age"); got != want {
			t.Errorf("home: got %q, want %q", got, want)
		}
	}
	if runtime.GOOS == "windows" {
		if got := resolvePath("C:", dir); got != `C:\` {
			t.Errorf("bare drive: got %q, want the drive root", got)
		}
	}
}
