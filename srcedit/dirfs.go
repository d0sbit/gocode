package srcedit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DirFS does the same thing os.DirFS() does but profiles a WriteFile method.
// TODO: Open a ticket and complain about this - the fs package design is great
// but a lot is lost by not supporting writes even if by interfaces that are not
// stanardized in the fs package (i.e. why do I have to write this, I really
// should be able to just check for WriteFile method on the fs DirFS returns.)
type DirFS string

func containsAny(s, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

// Open is copied from os.dirFS.Open
func (dir DirFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || runtime.GOOS == "windows" && containsAny(name, `\:`) {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrInvalid}
	}
	f, err := os.Open(string(dir) + "/" + name)
	if err != nil {
		return nil, err // nil fs.File
	}
	return f, nil
}

// WriteFile is what one would expect os.dirFS to provide and implements our local fileWriter interface.
// This way we can work with memfs and os.DirFS interchangably.
func (dir DirFS) WriteFile(name string, data []byte, perm fs.FileMode) error {

	// split by slashes
	nameParts := strings.Split(name, "/")
	// remove any empty parts
	for i := 0; i < len(nameParts); {
		if nameParts[i] == "" {
			nameParts = append(nameParts[:i], nameParts[i+1:]...)
			continue
		}
		i++
	}
	// make an OS-specific path and call WriteFile
	fullPath := filepath.Join(string(dir), filepath.Join(nameParts...))
	return os.WriteFile(fullPath, data, perm)
}

// Sub implements fs.SubFS, so we can return an instance that still implements WriteFile.
func (dir DirFS) Sub(subDir string) (fs.FS, error) {
	if !fs.ValidPath(subDir) {
		return nil, &fs.PathError{Op: "sub", Path: subDir, Err: errors.New("invalid name")}
	}
	ret := DirFS(filepath.Join(string(dir), subDir))
	return ret, nil
}
