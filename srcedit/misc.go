package srcedit

import (
	"bytes"
	"strings"
	"unicode"
)

// LowerForType accepts a type name and converts to lower case with a separator.
// Useful for deriving file names from a type name.  The separator is used to
// separate "words" found in the type name, you usually want "-" or "_".
// Some people will say that underscore is the convention for separators.
// I disagree because it can easily be confused with a build constraint,
// and so I recommend "-". Pick your poison.
func LowerForType(tName string, sep string) string {

	// slice of runes uses more memory but makes the "undo the last character"
	// operation easier
	b := make([]rune, 0, len(tName)+(4*len(sep)))

	addSep := func() {
		for _, c := range sep {
			b = append(b, c)
		}
	}

	// four states based on last char and this char:
	// uc-uc, uc-lc, lc-lc, lc-uc

	var lastC rune = 0
	// log.Printf("test: %v", unicode.IsUpper(lastC))
	for _, c := range tName {
		thisUpper := unicode.IsUpper(c)
		lastUpper := unicode.IsUpper(lastC) || lastC == 0
		lc := unicode.ToLower(c)
		switch {

		case !thisUpper && lastUpper:

			if len(b) > 0 {
				lb := len(b)
				prior := b[lb-1]
				b = b[:lb-1]
				addSep()
				b = append(b, prior)
			}
			fallthrough

		// case thisUpper && lastUpper:
		// case thisUpper && !lastUpper:
		// case !thisUpper && !lastUpper:

		default:
			b = append(b, lc)
		}
		lastC = c
	}

	var buf bytes.Buffer
	buf.Grow(len(b) + 2)
	for _, c := range b {
		buf.WriteRune(c)
	}

	return strings.TrimPrefix(buf.String(), sep)
}
