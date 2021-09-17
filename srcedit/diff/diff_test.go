package diff

import (
	"testing"

	"github.com/psanford/memfs"
)

func TestRunDiff(t *testing.T) {

	in := memfs.New()
	out := memfs.New()

	must(t, in.MkdirAll("a", 0755))
	must(t, in.WriteFile("a/test1.go", []byte("package a\n"), 0644))
	must(t, in.WriteFile("a/test2.go", []byte("package a\nfunc b(){\n}\n"), 0644))

	must(t, out.MkdirAll("a", 0755))
	must(t, out.WriteFile("a/test2.go", []byte("package a\nfunc b(){\n}\n\nfunc c(){\n}\n"), 0644))
	must(t, out.WriteFile("a/test3.go", []byte("package a\n"), 0644))

	m, err := Run(in, out, "a", "html")
	must(t, err)

	t.Logf("m: %#v", m)

	if len(m) != 2 {
		t.Errorf("unexpected len(m): %d", len(m))
	}

}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
