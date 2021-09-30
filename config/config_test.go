package config

import (
	"bytes"
	"testing"

	"github.com/psanford/memfs"
)

func TestConfig(t *testing.T) {

	mfs := memfs.New()
	must(t, mfs.WriteFile("example.config", []byte(`
val2 = "bleh"
val1 = "blah"
`), 0644))

	inf, err := mfs.Open("example.config")
	must(t, err)

	var c Config
	_, err = c.ReadFrom(inf)
	must(t, err)

	inf.Close()

	t.Logf("c.Settings: %#v", c.Settings)

	if c.Settings["val1"] != "blah" {
		t.Fail()
	}
	if c.Settings["val2"] != "bleh" {
		t.Fail()
	}

	c.Settings["val3"] = "blee"

	var buf bytes.Buffer
	_, err = c.WriteTo(&buf)
	must(t, err)
	t.Logf("out: %s", buf.Bytes())

	if !bytes.Contains(buf.Bytes(), []byte("blee")) {
		t.Fail()
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
