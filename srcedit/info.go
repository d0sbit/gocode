package srcedit

import (
	"go/ast"
	"go/token"
)

// TypeInfo describes a type found via Package.FindType().
type TypeInfo struct {
	GenDecl   *ast.GenDecl   // GenDecl that corresponds to the type
	FileSet   *token.FileSet // FileSet for decoding position info
	Filename  string         // name of the file in which the declaration was found
	FileBytes []byte         // the contents of the file as a byte slice
}

// NodeSrc will return a byte slice of the source code corresponding to a given node,
// by looking at the Pos() and End() positions.
func (ti *TypeInfo) NodeSrc(n ast.Node) []byte {
	start := n.Pos()
	end := n.End()
	startPosition := ti.FileSet.Position(start)
	endPosition := ti.FileSet.Position(end)
	return ti.FileBytes[startPosition.Offset:endPosition.Offset]
}

// Name returns the local Go type name or empty string if it doesn't exist for some reason.
func (ti *TypeInfo) Name() string {
	if ti == nil {
		return ""
	}
	if ti.GenDecl == nil {
		return ""
	}
	if len(ti.GenDecl.Specs) < 1 {
		return ""
	}
	ts, ok := ti.GenDecl.Specs[0].(*ast.TypeSpec)
	if !ok || ts == nil {
		return ""
	}
	if ts.Name == nil {
		return ""
	}
	return ts.Name.Name
}
