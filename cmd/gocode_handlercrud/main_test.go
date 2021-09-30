package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestMaineExec(t *testing.T) {

	// tests with execution of `go test` - requires "docker run ..." etc to work

	var modDir string
	modDir = t.TempDir()
	modDir, _ = ioutil.TempDir("", "TestMaine")
	t.Logf("modDir: %s", modDir)
	// 	must(t, os.MkdirAll(filepath.Join(modDir, "migrations"), 0755))
	// 	must(t, os.WriteFile(filepath.Join(modDir, "migrations/20210101000000_temp.sql"), []byte(`
	// -- +goose Up
	// CREATE TABLE a (
	//     id varchar(128) NOT NULL,
	//     name varchar(255) NOT NULL,
	//     PRIMARY KEY(id)
	// );

	// -- +goose Down
	// DROP TABLE a;
	// `), 0644))
	must(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module test1\n"), 0644))
	must(t, os.Mkdir(filepath.Join(modDir, "store"), 0755))
	must(t, os.WriteFile(filepath.Join(modDir, "store/types.go"), []byte(`package a

type Example struct {
	ID string `+"`db:\"id\"`"+`
	Name string `+"`db:\"name\"`"+`
}

func (a *Example) IDAssign() { a.ID = IDString() }

`), 0644))

	// 	// run the code generator
	must(t, os.Chdir(modDir))

	flset := flag.NewFlagSet(os.Args[0], flag.PanicOnError)
	ret := maine(flset, []string{"-v", "handlers/example.go"})
	if ret != 0 {
		t.Errorf("ret = %d", ret)
	}

	// cmd := exec.Command("go", "get", "./...")
	// cmd.Dir = modDir
	// b, err := cmd.CombinedOutput()
	// t.Logf("go get cmd output: %s", b)
	// must(t, err)

	// cmd = exec.Command("go", "test", "-v", "./...")
	// cmd.Dir = modDir
	// b, err = cmd.CombinedOutput()
	// t.Logf("test cmd output: %s", b)
	// must(t, err)

}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
