package srcedit

import (
	"io/fs"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/psanford/memfs"
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

func TestApplyTransformFunc(t *testing.T) {

	tfs := memfs.New()
	// TODO: move this to go:embed
	must(t, tfs.MkdirAll("test1", 0777))
	must(t, tfs.WriteFile("test1/testb.go", []byte(`package test1

func A() error { return nil }

func (b *B) C() {}

func (d D) E() {}

`), 0777))
	p := NewPackage(tfs, tfs, "test1")

	trList, err := ParseTransforms("testa.go", `
// A is a test func
func A() error {
	return nil
}
	`)
	must(t, err)
	trList[0].(*AddFuncDeclTransform).Replace = true

	must(t, p.ApplyTransforms(trList...))

	// f, tfs.Open("test1/testa.go")
	//log.Printf("OUT: %#v", tfs)
	spew.Dump(tfs)

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
