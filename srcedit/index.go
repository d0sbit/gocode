package srcedit

import (
	"go/ast"
	"log"
)

// Index extracts and makes available useful information from ast.Files.
type Index struct {
	declMap map[string]idxEntry
}

type idxEntry struct {
	decl ast.Decl
	file *ast.File
}

func NewIndex() *Index {
	return &Index{
		declMap: make(map[string]idxEntry),
	}
}

func (i *Index) AddFile(f *ast.File) {

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			i.declMap[d.Name.Name] = idxEntry{decl: d, file: f}
		default:
			log.Printf("skipping over unknown decl: %t / %v", decl, decl)
		}

	}

	for _, c := range f.Comments {
		log.Printf("comment: %v", c)
	}

}

func (i *Index) Lookup(name string) (ast.Decl, *ast.File) {
	e, ok := i.declMap[name]
	if !ok {
		return nil, nil
	}
	return e.decl, e.file
}
