package srcedit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestIndex(t *testing.T) {

	i := NewIndex()

	b := []byte(`package test1

// F is a test.
// Let's see if it works
func F() {
	println("hello!")
}
	
`)

	var fset token.FileSet
	f, err := parser.ParseFile(&fset, "test1.go", b, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	i.AddFile(f)

	d, f := i.Lookup("F")
	t.Logf("d: %#v; f: %v", d, f)

	start := fset.PositionFor(d.Pos(), true)
	end := fset.PositionFor(d.End(), true)

	t.Logf("F source: %s", b[start.Offset:end.Offset])

	fd := d.(*ast.FuncDecl)
	t.Logf("F comment: %s", b[fset.Position(fd.Doc.Pos()).Offset:fset.Position(fd.Doc.End()).Offset])

}
