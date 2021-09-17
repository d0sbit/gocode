package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMaineExec(t *testing.T) {

	// tests with execution of `go test` - requires "docker run ..." etc to work

	var modDir string
	modDir = t.TempDir()
	modDir, _ = ioutil.TempDir("", "TestMaine")
	t.Logf("modDir: %s", modDir)
	must(t, os.MkdirAll(filepath.Join(modDir, "migrations"), 0755))
	must(t, os.WriteFile(filepath.Join(modDir, "migrations/20210101000000_temp.sql"), []byte(`
-- +goose Up
CREATE TABLE a (
    id varchar(128) NOT NULL,
    name varchar(255) NOT NULL,
    PRIMARY KEY(id)
);

-- +goose Down
DROP TABLE a;
`), 0644))
	must(t, os.Mkdir(filepath.Join(modDir, "a"), 0755))
	must(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module test1\n"), 0644))
	must(t, os.WriteFile(filepath.Join(modDir, "a/types.go"), []byte(`package a

type A struct {
	ID string `+"`db:\"id\"`"+`
	Name string `+"`db:\"name\"`"+`
}

func (a *A) IDAssign() { a.ID = IDString() }

`), 0644))

	// run the code generator
	must(t, os.Chdir(modDir))
	flset := flag.NewFlagSet(os.Args[0], flag.PanicOnError)
	ret := maine(flset, []string{"-package=a", "-type=A"})
	if ret != 0 {
		t.Errorf("ret = %d", ret)
	}

	cmd := exec.Command("go", "get", "./...")
	cmd.Dir = modDir
	b, err := cmd.CombinedOutput()
	t.Logf("go get cmd output: %s", b)
	must(t, err)

	cmd = exec.Command("go", "test", "-v", "./...")
	cmd.Dir = modDir
	b, err = cmd.CombinedOutput()
	t.Logf("test cmd output: %s", b)
	must(t, err)

}

// func TestMaineDryRun(t *testing.T) {

// 	var modDir string
// 	modDir = t.TempDir()
// 	// modDir, _ = ioutil.TempDir("", "TestMaine")
// 	t.Logf("modDir: %s", modDir)
// 	must(t, os.Mkdir(filepath.Join(modDir, "a"), 0755))
// 	must(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module test1\n"), 0644))
// 	must(t, os.WriteFile(filepath.Join(modDir, "a/types.go"), []byte(`package a

// import "go.mongodb.org/mongo-driver/bson/primitive"

// type A struct {
// 	ID primitive.ObjectID `+"`bson:\"_id\"`"+`
// 	Name string `+"`bson:\"name\"`"+`
// }
// `), 0644))

// 	// run the code generator
// 	must(t, os.Chdir(modDir))
// 	flset := flag.NewFlagSet(os.Args[0], flag.PanicOnError)
// 	ret := maine(flset, []string{"-package=a", "-type=A", "-dry-run=html", "-json"})
// 	if ret != 0 {
// 		t.Errorf("ret = %d", ret)
// 	}

// }

// func TestMaineSubDirs(t *testing.T) {

// 	// make sure subdirs in the package folder don't cause an issue

// 	var modDir string
// 	modDir = t.TempDir()
// 	// modDir, _ = ioutil.TempDir("", "TestMaine")
// 	t.Logf("modDir: %s", modDir)
// 	must(t, os.Mkdir(filepath.Join(modDir, "a"), 0755))
// 	must(t, os.Mkdir(filepath.Join(modDir, "a/tmp"), 0755))
// 	must(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module test1\n"), 0644))
// 	must(t, os.WriteFile(filepath.Join(modDir, "a/types.go"), []byte(`package a

// import "go.mongodb.org/mongo-driver/bson/primitive"

// type A struct {
// 	ID primitive.ObjectID `+"`bson:\"_id\"`"+`
// 	Name string `+"`bson:\"name\"`"+`
// }
// `), 0644))

// 	// run the code generator
// 	must(t, os.Chdir(modDir))
// 	flset := flag.NewFlagSet(os.Args[0], flag.PanicOnError)
// 	ret := maine(flset, []string{"-package=a", "-type=A", "-dry-run=html", "-json"})
// 	if ret != 0 {
// 		t.Errorf("ret = %d", ret)
// 	}

// }

// func TestMaineOneDir(t *testing.T) {

// 	// confirm that it works when the package dir is empty
// 	// and everything is directly in the module dir

// 	var modDir string
// 	modDir = t.TempDir()
// 	// modDir, _ = ioutil.TempDir("", "TestMaine")
// 	t.Logf("modDir: %s", modDir)
// 	must(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module test1\n"), 0644))
// 	must(t, os.WriteFile(filepath.Join(modDir, "types.go"), []byte(`package a

// import "go.mongodb.org/mongo-driver/bson/primitive"

// type A struct {
// 	ID primitive.ObjectID `+"`bson:\"_id\"`"+`
// 	Name string `+"`bson:\"name\"`"+`
// }
// `), 0644))

// 	// run the code generator
// 	must(t, os.Chdir(modDir))
// 	flset := flag.NewFlagSet(os.Args[0], flag.PanicOnError)
// 	ret := maine(flset, []string{"-package=.", "-type=A", "-dry-run=html", "-json"})
// 	if ret != 0 {
// 		t.Errorf("ret = %d", ret)
// 	}

// }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
