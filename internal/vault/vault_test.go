package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	WorkFactor = 10
	os.Exit(m.Run())
}

func TestRoundTrip(t *testing.T) {
	for _, plaintext := range []string{"", "hello", "日本語のメモ\r\n2行目\n"} {
		ct, err := Encrypt("pässwörd", []byte(plaintext))
		if err != nil {
			t.Fatal(err)
		}
		if !IsEncrypted(ct) {
			t.Fatalf("IsEncrypted = false for %q", plaintext)
		}
		if plaintext != "" && bytes.Contains(ct, []byte(plaintext)) {
			t.Fatalf("ciphertext contains the plaintext %q", plaintext)
		}
		got, err := Decrypt("pässwörd", ct)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != plaintext {
			t.Fatalf("got %q, want %q", got, plaintext)
		}
	}
}

func TestEncryptIsRandomized(t *testing.T) {
	a, _ := Encrypt("pw", []byte("same"))
	b, _ := Encrypt("pw", []byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same text are identical")
	}
}

func TestWrongPassword(t *testing.T) {
	ct, err := Encrypt("right", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt("wrong", ct); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
}

func TestTamperedCiphertext(t *testing.T) {
	ct, err := Encrypt("pw", []byte("secret secret secret"))
	if err != nil {
		t.Fatal(err)
	}
	ct[len(ct)-1] ^= 1
	_, err = Decrypt("pw", ct)
	if err == nil || errors.Is(err, ErrWrongPassword) {
		t.Fatalf("err = %v, want a corruption error", err)
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted([]byte("just a plain note\n")) {
		t.Fatal("plain text reported as encrypted")
	}
	if IsEncrypted(nil) {
		t.Fatal("empty file reported as encrypted")
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.age")

	if err := WriteFile(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("new file mode = %v, want 0600", fi.Mode().Perm())
		}
		if err := os.Chmod(path, 0o640); err != nil {
			t.Fatal(err)
		}
	}

	if err := WriteFile(path, []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Fatalf("content = %q, want %q", got, "two")
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(path)
		if fi.Mode().Perm() != 0o640 {
			t.Fatalf("mode after overwrite = %v, want 0640", fi.Mode().Perm())
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries, want only the note", len(entries))
	}
}

func TestWriteFileThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real.age")
	link := filepath.Join(dir, "link.age")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced by a regular file")
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Fatalf("target content = %q, want %q", got, "new")
	}
}
