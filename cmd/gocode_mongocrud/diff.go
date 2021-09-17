package main

// NOTE: moved to srcedit/diff

// import (
// 	"bytes"
// 	"errors"
// 	"fmt"
// 	"io/fs"
// 	"io/ioutil"

// 	"github.com/sergi/go-diff/diffmatchpatch"
// )

// func runDiff(in, out fs.FS, rootDir string, outType string) (map[string]string, error) {

// 	ret := make(map[string]string)

// 	trimPath := func(p string) string {
// 		// return strings.TrimPrefix(strings.TrimPrefix(p, rootDir), "/")
// 		return p
// 	}

// 	// walk the output directory and compare each file to the input
// 	err := fs.WalkDir(out, rootDir, fs.WalkDirFunc(func(p string, d fs.DirEntry, err error) error {
// 		if err != nil {
// 			return err
// 		}

// 		if d.IsDir() {
// 			return nil
// 		}

// 		outf, err := out.Open(p)
// 		if err != nil {
// 			return fmt.Errorf("error opening output file %q: %w", p, err)
// 		}
// 		defer outf.Close()
// 		outb, err := ioutil.ReadAll(outf)
// 		if err != nil {
// 			return err
// 		}

// 		inf, err := in.Open(p)
// 		if err != nil {
// 			if !errors.Is(err, fs.ErrNotExist) {
// 				return fmt.Errorf("error opening input file %q: %w", p, err)
// 			}
// 			// if no input file, then diff against empty string
// 			df := diffContents("", string(outb), outType)
// 			ret[trimPath(p)] = df
// 			return nil
// 		}
// 		defer inf.Close()
// 		inb, err := ioutil.ReadAll(inf)
// 		if err != nil {
// 			return err
// 		}

// 		// check if they are exactly the same
// 		if bytes.Equal(inb, outb) {
// 			return nil // no difference to report
// 		}

// 		// diff out against in file
// 		df := diffContents(string(inb), string(outb), outType)
// 		ret[trimPath(p)] = df
// 		return nil
// 	}))
// 	if err != nil {
// 		return nil, err
// 	}

// 	return ret, nil
// }

// // diffContents performs a diff on the from and to contents of a file.
// // The format of the returned byte slice depends on outType.
// func diffContents(from, to string, outType string) string {

// 	// 	// fileAdmp, fileBdmp, dmpStrings := dmp.DiffLinesToChars(fileAtext, fileBtext)
// 	// 	// _ = dmpStrings
// 	// 	diffs := dmp.DiffMain(fileAtext, fileBtext, false)
// 	// 	// diffs := dmp.DiffMain(fileAdmp, fileBdmp, false)
// 	// 	// diffs = dmp.DiffCharsToLines(diffs, dmpStrings)
// 	// 	// diffs = dmp.DiffCleanupSemantic(diffs)
// 	// 	diffs = dmp.DiffCleanupSemanticLossless(diffs)
// 	// 	// diffs = dmp.DiffCharsToLines(diffs, dmpStrings)

// 	dmp := diffmatchpatch.New()

// 	diffs := dmp.DiffMain(from, to, true)

// 	diffs = dmp.DiffCleanupSemanticLossless(diffs)

// 	if outType == "html" {
// 		return dmp.DiffPrettyHtml(diffs)
// 	}

// 	return dmp.DiffPrettyText(diffs)
// }
