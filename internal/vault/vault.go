// Package vault encrypts and decrypts note files with a password.
//
// Files are written in the age v1 format (https://age-encryption.org/v1) with
// a single scrypt recipient, so they stay readable without this program:
// `age -d note.age` decrypts them.
package vault

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
	"filippo.io/age/armor"
)

const (
	binaryMagic = "age-encryption.org/v1\n"
	armorMagic  = "-----BEGIN AGE ENCRYPTED FILE-----"
)

// WorkFactor is the log2 of the scrypt cost used when encrypting. Decryption
// accepts whatever the file says, up to age's own limit.
var WorkFactor = 18

// ErrWrongPassword reports that the password does not open the file.
var ErrWrongPassword = errors.New("wrong password")

// IsEncrypted reports whether data looks like an age file.
func IsEncrypted(data []byte) bool {
	return bytes.HasPrefix(data, []byte(binaryMagic)) || isArmored(data)
}

func isArmored(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte(armorMagic))
}

// Encrypt seals plaintext under password.
func Encrypt(password string, plaintext []byte) ([]byte, error) {
	r, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, err
	}
	r.SetWorkFactor(WorkFactor)

	var out bytes.Buffer
	w, err := age.Encrypt(&out, r)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Decrypt opens data with password. It returns ErrWrongPassword when the
// password is not the one the file was sealed with.
func Decrypt(password string, data []byte) ([]byte, error) {
	id, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, err
	}

	var src io.Reader = bytes.NewReader(data)
	if isArmored(data) {
		src = armor.NewReader(src)
	}
	r, err := age.Decrypt(src, id)
	if err != nil {
		var noMatch *age.NoIdentityMatchError
		if errors.As(err, &noMatch) {
			return nil, ErrWrongPassword
		}
		return nil, err
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("file is corrupted: %w", err)
	}
	return plaintext, nil
}

// WriteFile replaces the file at path with data. The data goes to a temporary
// file in the same directory first and is renamed into place, so a crash or a
// full disk never leaves a half-written note behind.
func WriteFile(path string, data []byte) error {
	// Write through a symlink rather than replacing the link itself.
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}

	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
