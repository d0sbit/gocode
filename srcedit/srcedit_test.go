package srcedit

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path"
	"testing"

	"github.com/psanford/memfs"
)

const (
	lf  = "\n"
	tab = "\t"
)

func TestOSWorkingFSDir(t *testing.T) {

	fsys, dir, err := OSWorkingFSDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fsys=%#v, dir=%s", fsys, dir)

}

func TestFindOSWdModuleDir(t *testing.T) {

	fsys, modDir, err := FindOSWdModuleDir()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("fsys=%v, modDir=%s", fsys, modDir)

	modFS, err := fs.Sub(fsys, modDir)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("modFS=%v", modFS)

	f, err := modFS.Open("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	f, err = modFS.Open("srcedit")
	if err != nil {
		t.Fatal(err)
	}
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Fatalf("Stat() says srcedit is not a dir")
	}
	f.Close()

}

func TestWriteCodeBlock(t *testing.T) {
	tfs := memfs.New()
	// TODO: move this to go:embed
	must(t, tfs.MkdirAll("test1", 0777))
	must(t, tfs.WriteFile("test1/test1.go", []byte("package test1\n\nfunc ExampleFunc() {}\n"), 0777))
	// must(t, tfs.WriteFile("test1/test1.go", []byte("abcd"), 0777))

	p := NewPackage(tfs, tfs, "test1")

	must(t, p.WriteCodeBlock("test1/test1.go", "import \"log\"\nfunc ExampleFunc() { log.Printf(`ExampleFunc here`)}\n", true))
	// must(t, p.WriteCodeBlock("test1/test1.go", "abcd", true))

	// ok, err := p.CheckFuncExists("ExampleFunc")
	// must(t, err)
	// t.Logf("result: %v", ok)
}

func TestApplyTransforms(t *testing.T) {

	type files map[string]string

	type tcase struct {
		name       string      // test name
		pkgPath    string      // package path/dir
		in         files       // input files
		transforms []Transform // transforms to apply
		eout       files       // expected output
	}

	tcaseList := []tcase{

		{
			name:    "func01",
			pkgPath: "test1",
			in: files{
				"a.go": `package test1` + lf +
					`func A() error { return nil }` + lf,
			},
			transforms: []Transform{
				&AddFuncDeclTransform{
					Filename:     "b.go",
					Name:         "A",
					ReceiverType: "",
					Text:         `func A() (err error) { return }`,
					Replace:      true,
				},
			},
			eout: files{
				"a.go": `package test1` + lf + lf,
				"b.go": `package test1` + lf + lf + `func A() (err error) { return }` + lf,
			},
		},

		{
			name:    "import01",
			pkgPath: "test1",
			in: files{
				"a.go": `package test1` + lf + lf +
					`import (` + lf +
					tab + `"io"` + lf +
					tab + `"os"` + lf +
					`)` + lf,
			},
			transforms: []Transform{
				&ImportTransform{
					Filename: "a.go",
					Name:     "",
					Path:     "io/ioutil",
				},
			},
			eout: files{
				"a.go": `package test1` + lf + lf +
					`import (` + lf +
					tab + `"io"` + lf +
					tab + `"os"` + lf +
					tab + `"io/ioutil"` + lf +
					`)` + lf,
			},
		},

		{
			name:    "import02",
			pkgPath: "test1",
			in: files{
				"a.go": `package test1` + lf + lf +
					`import "io"` + lf + lf,
			},
			transforms: []Transform{
				&ImportTransform{
					Filename: "a.go",
					Name:     "",
					Path:     "io/ioutil",
				},
			},
			eout: files{
				"a.go": `package test1` + lf + lf +
					`import "io"` + lf +
					`import "io/ioutil"` + lf + lf,
			},
		},

		{
			name:    "import03",
			pkgPath: "test1",
			in:      files{
				// no input files
			},
			transforms: []Transform{
				&ImportTransform{
					Filename: "a.go",
					Name:     "",
					Path:     "io/ioutil",
				},
			},
			eout: files{
				"a.go": `package test1` + lf + lf +
					`import "io/ioutil"` + lf + lf + lf,
			},
		},
	}

	for _, tc := range tcaseList {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			infs := memfs.New()  // gets loaded with tc.in
			outfs := memfs.New() // output written here so it can be compared to tc.eout

			must(t, infs.MkdirAll(tc.pkgPath, 0755))
			for fname, contents := range tc.in {
				must(t, infs.WriteFile(path.Join(tc.pkgPath, fname), []byte(contents), 0644))
			}

			must(t, outfs.MkdirAll(tc.pkgPath, 0755))

			// create Package and apply Transforms
			p := NewPackage(infs, outfs, tc.pkgPath)
			must(t, p.ApplyTransforms(tc.transforms...))

			// walk the output fs, skip dirs, compare each file, then remove from eout map
			must(t, fs.WalkDir(outfs, tc.pkgPath, fs.WalkDirFunc(func(fpath string, dirEntry fs.DirEntry, err error) error {
				// t.Logf("checking fpath %q", fpath)
				if err != nil {
					return fmt.Errorf("error from WalkDirFunc (fpath=%q): %w", fpath, err)
				}
				if dirEntry.IsDir() {
					return nil
				}

				f, err := outfs.Open(fpath)
				if err != nil {
					return fmt.Errorf("error from Open in WalkDirFunc(fpath=%q): %w", fpath, err)
				}
				defer f.Close()

				outb, err := ioutil.ReadAll(f)
				if err != nil {
					return fmt.Errorf("error from ReadAll in WalkDirFunc(fpath=%q): %w", fpath, err)
				}

				_, baseName := path.Split(fpath)
				eoutb := []byte(tc.eout[baseName])
				if len(eoutb) == 0 {
					return fmt.Errorf("output generated for %q but not found in eout", baseName)
				}

				if !bytes.Equal(outb, eoutb) {
					t.Errorf("match failed for file %q", baseName)
					t.Logf("expected: %q", eoutb)
					t.Logf("actual:   %q", outb)
				}

				// remove from eout to indicate we've already analyzed it
				delete(tc.eout, baseName)

				return nil
			})))

			// see if eout has anything left over
			if len(tc.eout) > 0 {
				for fn := range tc.eout {
					t.Errorf("file %q found in eout but not produced in output", fn)
				}
			}

		})
	}

}

// func TestCheckFuncExists(t *testing.T) {
// 	tfs := memfs.New()
// 	// TODO: move this to go:embed
// 	must(t, tfs.MkdirAll("test1", 0777))
// 	must(t, tfs.WriteFile("test1/test1.go", []byte("package test1\n\nfunc ExampleFunc() {}\n"), 0777))
// 	// must(t, tfs.WriteFile("test1/test1.go", []byte("abcd"), 0777))

// 	p := NewPackage(tfs, tfs, "test1")
// 	err := p.Load()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Logf("LocalName: %s", p.LocalName())

// 	//must(t, p.WriteCodeBlock("test1/test1.go", "import \"log\"\nfunc ExampleFunc() { log.Printf(`ExampleFunc here`)}\n", true))
// 	// must(t, p.WriteCodeBlock("test1/test1.go", "abcd", true))

// 	ok, err := p.CheckFuncExists("ExampleFunc")
// 	must(t, err)
// 	t.Logf("result: %v", ok)
// }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
