package srcedit

import (
	"fmt"
	"path"
	"strings"
)

// DirHasSuffix takes a directory and a suffix and returns true
// if the directory ends with the given suffix.  E.g. calling
// with ("some/dir/here", "here") returns true as does
// ("here", "here").
func DirHasSuffix(dir, suffix string) bool {
	dir = path.Clean(dir)
	suffix = path.Clean(suffix)
	return strings.HasSuffix(dir, suffix)
}

// DirResolveTo checks to ensure dir ends with fromSuffix, trims
// that and returns toSuffix appeneded.  E.g. calling with
// ("some/dir/here", "here", "there") returns ("some/dir/there", nil).
// If dir does not end with fromSuffix then an error is returned.
func DirResolveTo(dir, fromSuffix, toSuffix string) (string, error) {
	ndir := path.Clean(dir)
	fromSuffix = path.Clean(fromSuffix)
	toSuffix = path.Clean(toSuffix)
	if !strings.HasSuffix(ndir, fromSuffix) {
		return "", fmt.Errorf("dir %q does not end with suffix %q", dir, fromSuffix)
	}
	d := strings.TrimSuffix(ndir, fromSuffix)
	d = path.Clean(path.Join(d, toSuffix))
	if !strings.HasPrefix(dir, "/") { // only keep leading slash if dir had it
		d = strings.TrimPrefix(d, "/")
	}
	return d, nil
}
