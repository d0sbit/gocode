package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMaine(t *testing.T) {

	var modDir string
	modDir = t.TempDir()
	modDir, _ = ioutil.TempDir("", "TestMaine")
	t.Logf("modDir: %s", modDir)
	must(t, os.Mkdir(filepath.Join(modDir, "a"), 0755))
	must(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module test1\n"), 0644))
	must(t, os.WriteFile(filepath.Join(modDir, "a/types.go"), []byte(`package a
type A struct {
	ID string `+"`bson:\"_id\"`"+`
}
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

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
