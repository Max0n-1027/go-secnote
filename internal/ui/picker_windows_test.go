package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func setAttributes(t *testing.T, path string, attrs uint32) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, attrs); err != nil {
		t.Fatal(err)
	}
}

func TestListDirLeavesOutProtectedSystemFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"note.age", "hidden.age", "pagefile.sys"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	setAttributes(t, filepath.Join(dir, "hidden.age"), syscall.FILE_ATTRIBUTE_HIDDEN)
	setAttributes(t, filepath.Join(dir, "pagefile.sys"), syscall.FILE_ATTRIBUTE_HIDDEN|syscall.FILE_ATTRIBUTE_SYSTEM)

	got, err := listDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A file that is merely hidden stays: the user may have hidden a note.
	if want := []string{"../", "hidden.age", "note.age"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %q, want %q", got, want)
	}
}
