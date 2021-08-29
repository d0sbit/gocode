package srcedit

import (
	"bytes"
	"go/ast"
	"go/token"
	"strconv"
)

// import block
type iblock struct {
	genDecl   *ast.GenDecl
	countIn   int // number of incoming imports
	countKeep int // number that aren't duplicate
	ilines    []iline
}

// import spec line
type iline struct {
	path  string
	ispec *ast.ImportSpec
	keep  bool
}

func (p *Package) applyDedupImports(t *DedupImportsTransform) error {

	allNames := t.FilenameList == nil
	var nameSet map[string]struct{}
	if len(t.FilenameList) > 0 {
		nameSet = make(map[string]struct{}, len(t.FilenameList))
	}

	for _, fn := range t.FilenameList {
		nameSet[fn] = struct{}{}
	}

	for filename, inb := range p.fileBytes {
		filename, inb := filename, inb

		_, ok := nameSet[filename]
		if !(ok || allNames) {
			continue
		}

		// algorithm:
		// - loop over import decls and individual specs
		// - use a map to determine "have we seen this import before"
		// - if not just add regular
		// - if so add but flag it to be omitted during output
		// - make a list of the ranges to be omitted
		// - loop over the ranges and copy everything around them to the output

		af := p.astf[filename]
		if af == nil {
			// nothing to do if file doesn't exist
			return nil
		}

		im := make(map[string]struct{})

		blockList := make([]*iblock, 0, 1)

		for _, decl := range af.Decls {
			// find GenDecls
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			// that are imports
			if genDecl.Tok != token.IMPORT {
				continue
			}

			// ast.Print(p.fset, genDecl)

			ib := &iblock{}
			blockList = append(blockList, ib)

			ib.genDecl = genDecl

			for _, spec := range genDecl.Specs {
				ispec, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}
				path, err := strconv.Unquote(ispec.Path.Value)
				if err != nil {
					return err
				}

				ib.ilines = append(ib.ilines, iline{
					path:  path,
					ispec: ispec,
				})
				ib.countIn++

				_, ok = im[path]
				if !ok {
					im[path] = struct{}{}
					ib.countKeep++
					ib.ilines[len(ib.ilines)-1].keep = true
				}
			}

		}

		outb := make([]byte, 0, len(inb))
		from := 0

		for _, ib := range blockList {
			// copy anything in between import blocks
			n := p.fset.Position(ib.genDecl.Pos()).Offset
			outb = append(outb, inb[from:n]...)
			from = n

			// if we're not keeping any lines in the import block, omit the whole block
			if ib.countKeep < 1 {
				from = p.fset.Position(ib.genDecl.End()).Offset
				continue
			}

			for _, il := range ib.ilines {
				// copy anything in between import lines
				n := p.fset.Position(il.ispec.Pos()).Offset
				outb = append(outb, inb[from:n]...)
				from = n

				// navigate over the line
				n = p.fset.Position(il.ispec.End()).Offset
				if il.keep { // append to output if we're keeping it
					outb = append(outb, inb[from:n]...)
				}
				from = n

			}
		}

		// the rest of the file
		outb = append(outb, inb[from:]...)

		// don't write if no changes made
		if !bytes.Equal(inb, outb) {
			err := p.writeFileNamed(filename, outb)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
