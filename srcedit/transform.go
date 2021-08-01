package srcedit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

// Transform is implemented by the various transform types.  Each type has it's own data,
// so we just use a marker interface here.
type Transform interface {
	xform() // marker
}

// ImportTransform ensures a particular package is imported, optionally with a specific local name.
type ImportTransform struct {
	Filename string // write code to this file
	Name     string // local import name - "" means none, "_" is valid, or otherwise local name
	Path     string // package import path
}

func (t *ImportTransform) xform() {}

// AddFuncDeclTransform is used to add a function or method.
type AddFuncDeclTransform struct {
	Filename     string // write code to this file
	Name         string // the name of the function
	ReceiverType string // the receiver type, e.g. "*X" meaning pointer to type X
	Text         string // full function text including comments
	Replace      bool   // if true then any existing function or method with this name/name+receiver will be replaced
}

func (t *AddFuncDeclTransform) xform() {}

// // AddGenDeclTransform is used to add a package-level var, const or type declaration.
// type AddGenDeclTransform struct {
// 	Filename string // write code to this file
// 	Name     string // the const, var or type name
// 	Text     string // the full declaration text including comments
// 	Replace  bool   // if true then any existing declaration with the same name is replaced
// }
// func (t *AddGenDeclTransform) xform() {}

type AddConstDeclTransform struct {
	Filename string   // write code to this file
	NameList []string // the names
	Text     string   // the full declaration text including comments
	Replace  bool     // if true then any existing declaration with the same name is replaced
}

func (t *AddConstDeclTransform) xform() {}

type AddVarDeclTransform struct {
	Filename string   // write code to this file
	NameList []string // the names
	Text     string   // the full declaration text including comments
	Replace  bool     // if true then any existing declaration with the same name is replaced
}

func (t *AddVarDeclTransform) xform() {}

type AddTypeDeclTransform struct {
	Filename string // write code to this file
	Name     string // the type name
	Text     string // the full declaration text including comments
	Replace  bool   // if true then any existing declaration with the same name is replaced
}

func (t *AddTypeDeclTransform) xform() {}

// TODO: AddFuncLineTransform { FuncName, Line } adds a line to a function before the return - we'll need it for adding routes

// Transformers houses a collection of transforms.  More than meets the eye, robots in disguise.
type Transformers struct {
	transformList []Transform
}

// Add will add all of the given transforms to the list.
func (ts *Transformers) Add(tl []Transform) {
	ts.transformList = append(ts.transformList, tl...)
}

// func (ts *Transformers) AddImport(name, path string) {
// 	ts.transformList = append(ts.transformList, ImportTransform{
// 		Name: name, Path: path,
// 	})
// }

// ParseTransforms will read Go source code and return a slice of the transforms indicated.
// The snippet should not start with a package statement, import statements are allowed and
// converted to ImportTransform, funcs are converted to AddFuncDeclTransform, and const, var and type
// declarations are converted to AddGenDeclTransform.  Comments are preserved and included
// in the transform where possible.  Other unsupported source elements may error or be ignored
// (will try to make them error but no promises yet).
//
// This is basically here to facilitate easy templating.  So you can have a template that just outputs
// `func F() {}` or whatever and call this function and get back a slice with an AddFuncDeclTransform.
// Note that some transforms may not be expressable as snippets and need to be constructed otherwise
// using the appropriate ...Transform struct.
func ParseTransforms(filename, snippet string) (ret []Transform, reterr error) {

	pfx := "package snippet__\n\n"
	snippet = pfx + snippet

	// fix the offset of error messages so they report the right line
	fixerr := func(err error) error {
		if err == nil {
			return nil
		}
		elist, ok := err.(scanner.ErrorList)
		if !ok {
			return err
		}
		if len(elist) == 0 {
			return err
		}
		elist[0].Pos.Offset -= len(pfx)
		elist[0].Pos.Line -= 2
		return elist
	}

	var fset token.FileSet
	f, err := parser.ParseFile(&fset, filename, snippet, parser.ParseComments)
	if err != nil {
		//log.Printf("ParseFile err: %#v", err)
		return nil, fixerr(err)
	}

	// convenience to extract text from snippet
	sliceSnippet := func(from, to token.Pos) string {
		return snippet[fset.Position(from).Offset:fset.Position(to).Offset]
	}

	// form the text field from a code node and optional comment/doc node
	mkText := func(code, doc ast.Node) string {
		tcode := sliceSnippet(code.Pos(), code.End())
		var tdoc string
		if doc != nil {
			tdoc = sliceSnippet(doc.Pos(), doc.End())
		}
		var sb strings.Builder
		sb.Grow(len(tdoc) + len(tcode) + 1)
		sb.WriteString(tdoc)
		if len(tdoc) > 0 && !strings.HasSuffix(tdoc, "\n") { // append \n to comment if not present
			sb.WriteByte('\n')
		}
		sb.WriteString(tcode)
		return sb.String()
	}

	// each import becomes an ImportTransform
	for _, imp := range f.Imports {

		// TODO: preserve comments?

		localName := ""
		if imp.Name != nil {
			localName = imp.Name.Name
		}
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return ret, err
		}

		ret = append(ret, &ImportTransform{
			Filename: filename,
			Name:     localName,
			Path:     importPath,
		})
	}

	// loop through the rest of the declarations
	for _, decl := range f.Decls {

		switch d := decl.(type) {
		case *ast.FuncDecl:

			tr := &AddFuncDeclTransform{
				Filename: filename,
				Name:     d.Name.Name,
			}

			// extract receiver type
			if d.Recv.NumFields() == 1 {
				switch typ := d.Recv.List[0].Type.(type) {
				case *ast.Ident:
					tr.ReceiverType = typ.Name
				case *ast.StarExpr:
					if i, ok := typ.X.(*ast.Ident); ok {
						tr.ReceiverType = "*" + i.Name
					} else {
						return ret, fmt.Errorf("StarExpr with unknown X: %t / %v", typ.X, typ.X)
					}
				default:
					return ret, fmt.Errorf("unexpected receiver type: %t / %v", typ, typ)
				}
			}

			tr.Text = mkText(d, d.Doc)

			// var sb strings.Builder
			// var t, c string

			// // extract text
			// // fstart := d.Pos()
			// // fend := d.End()
			// // t = snippet[fset.Position(fstart).Offset:fset.Position(fend).Offset]
			// t = sliceSnippet(d.Pos(), d.End())

			// // check for comment
			// if d.Doc != nil {
			// 	// cstart := d.Doc.Pos()
			// 	// cend := d.Doc.End()
			// 	// c = snippet[fset.Position(cstart).Offset:fset.Position(cend).Offset]
			// 	c = sliceSnippet(d.Doc.Pos(), d.Doc.End())
			// }

			// // put it all together
			// sb.Grow(len(c) + len(t) + 1)
			// sb.WriteString(c)
			// if len(c) > 0 && !strings.HasSuffix(c, "\n") { // append \n to comment if not present
			// 	sb.WriteByte('\n')
			// }
			// sb.WriteString(t)

			// tr.Text = sb.String()

			ret = append(ret, tr)

		case *ast.GenDecl:

			switch d.Tok {

			case token.IMPORT: // skip over imports
				continue

			case token.VAR:

				tr := &AddVarDeclTransform{
					Filename: filename,
				}

				// extract name list
				for _, spec := range d.Specs {
					vspec, ok := spec.(*ast.ValueSpec)
					if !ok {
						return ret, fmt.Errorf("unexpected non-ValueSpec in Spec list for var: %t / %v", spec, spec)
					}
					for _, nameIdent := range vspec.Names {
						tr.NameList = append(tr.NameList, nameIdent.Name)
					}
				}

				tr.Text = mkText(d, d.Doc)

				ret = append(ret, tr)

			case token.CONST:

				tr := &AddConstDeclTransform{
					Filename: filename,
				}

				// extract name list
				for _, spec := range d.Specs {
					vspec, ok := spec.(*ast.ValueSpec)
					if !ok {
						return ret, fmt.Errorf("unexpected non-ValueSpec in Spec list for const: %t / %v", spec, spec)
					}
					for _, nameIdent := range vspec.Names {
						tr.NameList = append(tr.NameList, nameIdent.Name)
					}
				}

				tr.Text = mkText(d, d.Doc)

				ret = append(ret, tr)

			case token.TYPE:

				// log.Printf("got var")
				// ast.Print(&fset, d)

				// whoops... types only have one name, but const and var can be a number - maybe
				// we need to split out const or var as one type that supports multiple,
				// and an AddTypeDeclTransform
				// tr := &AddGenDeclTransform{
				// 	Filename: filename,
				// 	Name     string // the const, var or type name
				// 	Text     string // the full declaration text including comments

				// }

				// // snippet[fset.Position(d.Pos()).Offset:fset.Position(d.End()).Offset]
				// ret = append(ret, tr)

				tr := &AddTypeDeclTransform{
					Filename: filename,
				}

				// a type only has one name, extract it
				if len(d.Specs) != 1 {
					return ret, fmt.Errorf("decl for type Specs list does not have exactly 1 element, instead found %d: %v", len(d.Specs), d.Specs)
				}
				typeSpec, ok := d.Specs[0].(*ast.TypeSpec)
				if !ok {
					return ret, fmt.Errorf("decl for type Specs[0] is not a TypeSpec: %t / %v", d.Specs[0], d.Specs[0])
				}
				tr.Name = typeSpec.Name.Name

				tr.Text = mkText(d, d.Doc)

				ret = append(ret, tr)

			default:
				return ret, fmt.Errorf("unexpected GenDecl token: %v", d.Tok)
			}

		default:
			return ret, fmt.Errorf("unrecognized declaration type %T / %v", d, d)

		}

	}

	return ret, nil
}
