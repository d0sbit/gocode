// Package srcedit provides high level Go source code editing functionality.
// Although packages like go/parser and friends provide detailed code editing
// facility, we need a nice high level façade that lets us do things like
// "if this function doesn't exist, create it with this source", etc.  And we
// want to centralize the reusable bits here, so individual gocode plugins
// don't have to be loaded with detailed and duplicative source editing
// functionality.
package srcedit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var identRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)

// OSWorkingFSDir returns a filesystem at the OS root (of the drive on Windows) and the path of the current working directory.
// The implementation is is DirFS and implements FileWriter.
func OSWorkingFSDir() (fs.FS, string, error) {
	origwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	origwd = filepath.Clean(origwd)
	wd := origwd
	for {
		newwd := filepath.Clean(filepath.Join(wd, ".."))
		if newwd == wd {
			// NOTE: on mac/linux this should be "/", and on Windows it should be the root of the drive, e.g. "C:\"
			return DirFS(wd), strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(origwd, wd)), "/"), nil
		}
		wd = newwd
	}
}

// FindOSWdModuleDir calls OSWorkingFSDir to split up the working dir into a root and path,
// and then calls FindModuleDir.
func FindOSWdModuleDir() (fs.FS, string, error) {

	fsys, dir, err := OSWorkingFSDir()
	if err != nil {
		return nil, "", err
	}

	modDir, err := FindModuleDir(fsys, dir)
	if err != nil {
		return nil, "", err
	}

	return fsys, modDir, nil
}

// FindModuleFS will start at startDir and look for a go.mod file
// in each parent directory until it is found or error if the root
// of the filesystem is reached and has no go.mod.
func FindModuleDir(fsys fs.FS, startDir string) (string, error) {
	dir := startDir

	for {

		// make sure the directory exists
		dirf, err := fsys.Open(dir)
		if err != nil {
			return "", fmt.Errorf("failed to open dir %q: %w", dir, err)
		}
		dirf.Close()

		// check for a go.mod file
		f, err := fsys.Open(path.Join(dir, "go.mod"))
		if err == nil {
			// if so, we're done
			return dir, f.Close()
		}

		// if not, move up one dir
		newdir := path.Clean(path.Join(dir, ".."))

		// check to see if we're at the top
		if newdir == dir {
			return "", errors.New("unable to file go.mod")
		}

		dir = newdir
	}

}

// Package provides methods to perform code edits on a package.
type Package struct {
	infs      fs.FS  // read files from
	outfs     fs.FS  // write updated files to
	fullName  string // full package name, import path
	localName string // local name from package statements or default

	fset      *token.FileSet       // Go parser needs this
	astf      map[string]*ast.File // each file that was parsed in the package with the filename (no path info) as the key
	fileBytes map[string][]byte    // filename to most recently read contents
}

// NewPackage returns a new Package with the specified input and output filesystems and the specified module name/path.
func NewPackage(infs, outfs fs.FS, fullName string) *Package {
	// FIXME: do we need to remove the "v2" from the name?
	return &Package{
		infs:     infs,
		outfs:    outfs,
		fullName: fullName,
	}
}

// FullName returns the full package path, e.g. "a/b/c"
func (p *Package) FullName() string {
	return p.fullName
}

// LocalName returns the name from the package statements inside the source files.
// If no source files exist then a default is derived from the import path.
func (p *Package) LocalName() string {
	return p.localName
}

// // Load will read in the package files and parse everything.
// func (p *Package) Load() error {

// 	fnl, err := p.fileNames()
// 	if err != nil {
// 		return err
// 	}

// 	p.fset = &token.FileSet{}
// 	p.astf = make(map[string]*ast.File, len(fnl))
// 	p.localName = ""

// 	pkgNames := make([]string, 0, 1)
// 	for _, fn := range fnl {
// 		b, err := p.readFile(fn)
// 		if err != nil {
// 			return fmt.Errorf("load for %q failed: %w", fn, err)
// 		}
// 		af, err := parser.ParseFile(p.fset, fn, b, parser.ParseComments)
// 		if err != nil {
// 			return err
// 		}
// 		pkgNames = append(pkgNames, af.Name.Name)
// 		p.astf = append(p.astf, af)
// 		// NOTE: ParseDir returns an ast.Package but it doesn't have any additional info,
// 		// a simple slice of *ast.File is just as well
// 		// NOTE: if we need SSA we'll just call sslutil.BuildPackage somewhere around here
// 	}

// 	switch len(pkgNames) {
// 	case 0:
// 		// derive from full package name
// 		_, n := path.Split(p.fullName)
// 		n = strings.NewReplacer("-", "").Replace(n)
// 		p.localName = n
// 	case 1:
// 		p.localName = pkgNames[0]
// 	default:
// 		for _, pn := range pkgNames {
// 			if strings.HasSuffix(pn, "_test") { // _test package is okay, disregard
// 				continue
// 			}
// 			if p.localName != "" {
// 				return fmt.Errorf("multiple package names found: %v", pkgNames)
// 			}
// 			p.localName = pn
// 		}
// 	}

// 	if !identRE.MatchString(p.localName) {
// 		return fmt.Errorf("derived package name %q is not valid", p.localName)
// 	}

// 	return nil
// }

// CheckFuncExists returns true if the specified function/method exists in this package.
// Use a period to specify a type and check for a method, e.g. "SomeType.SomeMethod".
func (p *Package) CheckFuncExists(name string) (bool, error) {
	if p.fset == nil {
		return false, fmt.Errorf("package not Load()ed")
	}

	for _, af := range p.astf {
		//af.Decls
		log.Printf("af: %v", af)
		log.Printf("af.Name.Name: %s", af.Name.Name)
		for _, decl := range af.Decls {
			log.Printf("decl: %#v", decl)
			f, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			log.Printf("f.Name.Name: %s", f.Name.Name)
		}
	}

	return false, nil
}

// WriteCodeBlock will emit into the specified file some Go source code.
// The block of code must not include a package statement, but may include
// import statements (which will be deduplicated and moved to the top).
// Definitions contained in the source contents will cause the removal (replacement)
// of existing definitions in the package unless replace is false.
func (p *Package) WriteCodeBlock(filename, contents string, replace bool) error {

	// b, err := p.ReadFile(filename)
	// if err != nil {
	// 	// FIXME: we should create the file if it doesn't exist
	// 	// TODO: figure out how local package name should be established
	// 	return err
	// }
	// _ = b

	// var fset token.FileSet
	// pkgs, err := parser.ParseDir(&fset, "test1", nil, parser.ParseComments)
	// if err != nil {
	// 	return err
	// }
	// log.Printf("pkgs: %#v", pkgs)

	// f, err := parser.ParseFile(&fset, filename, b, parser.ParseComments)
	// if err != nil {
	// 	return err
	// }
	// _ = f

	return nil
}

// readFile will read a file from outfs if it exists there and if not from infs.
// This way if the specified file has been modified you'll get the modified file,
// otherwise the original unmodified one.  The filename should not have any path
// separators in it.
func (p *Package) readFile(filename string) ([]byte, error) {
	openPath := path.Join(p.fullName, filename)
	f, err := p.outfs.Open(openPath)
	if err != nil {
		f, err := p.infs.Open(openPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return ioutil.ReadAll(f)
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

// fileNames returns the merged list of files from the package dir.
// The names returned are just the file names, not the full path.
func (p *Package) fileNames() ([]string, error) {

	retMap := make(map[string]struct{}, 8)

	dirEntryList, err := fs.ReadDir(p.outfs, p.fullName)
	if err != nil {
		return nil, err
	}
	for _, de := range dirEntryList {
		retMap[de.Name()] = struct{}{}
	}

	dirEntryList, err = fs.ReadDir(p.infs, p.fullName)
	if err != nil {
		return nil, err
	}
	for _, de := range dirEntryList {
		retMap[de.Name()] = struct{}{}
	}

	var ret []string
	for fn := range retMap {
		ret = append(ret, fn)
	}
	sort.Strings(ret)

	return ret, nil
}

// ApplyTransforms calls ApplyTransform for each one provided.  Note that transforms
// are not atomic and this may result in only a subset of the requested changes being applied
// upon error.  Consider using a separate output filesystem if you're concerned about this.
func (p *Package) ApplyTransforms(tr ...Transform) error {

	for _, t := range tr {
		err := p.ApplyTransform(t)
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyTransform will take a transform and apply it to the package,
// writing whatever output is needed to the output FS.
func (p *Package) ApplyTransform(tr Transform) error {

	fwriter, ok := p.outfs.(FileWriter)
	if !ok {
		return fmt.Errorf("p.outfs does not implement FileWriter, cannot write changes")
	}

	err := p.load()
	if err != nil {
		return err
	}

	switch t := tr.(type) {
	case *AddFuncDeclTransform:

		// check if the func already exists in the package (could be in another file)
		existingFilename, existingDecl := p.findFunc(t.ReceiverType, t.Name)

		if existingDecl != nil {
			// if so and not replacing, no change needed
			if !t.Replace {
				return nil
			} else {
				// if so and replacing, write the file out with the specific portion omitted

				endPos := existingDecl.End()
				startPos := existingDecl.Pos()
				if existingDecl.Doc != nil {
					startPos = existingDecl.Doc.Pos() // begin at the comment if it exists
				}

				startOffset := p.fset.Position(startPos).Offset
				endOffset := p.fset.Position(endPos).Offset

				// local byte slice updated in case the code below writes to the same file
				b := p.fileBytes[existingFilename]
				b = append(b[:startOffset], b[endOffset:]...)
				p.fileBytes[existingFilename] = b

				existingFilePath := path.Join(p.fullName, existingFilename)
				// FIXME: should detect file mode from either outfs or infs, whichever is present
				err := fwriter.WriteFile(existingFilePath, b, getFileModeOrDefault(p.outfs, existingFilePath, 0755))
				if err != nil {
					return err
				}

			}
		}

		// then write out the file indicated in the transform with the new func added to the bottom
		b := p.fileBytes[t.Filename]
		if b == nil { // might be a new file, which we start with just a package statement
			b = []byte("package " + p.localName + "\n\n")
		}
		b = append(b, t.Text...)
		b = append(b, "\n"...)
		fullPath := path.Join(p.fullName, t.Filename)
		// FIXME: should detect file mode from either outfs or infs, whichever is present
		err = fwriter.WriteFile(fullPath, b, getFileModeOrDefault(p.outfs, fullPath, 0755))
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown transform type: %t", tr)
	}

	return nil

}

// findFunc looks through what has been load()ed and searches for a function
// declaration that matches the specified receiver type and name, e.g. "*X", "Y"
// will find: func (x *X) Y(){} and return the filename it was found in along with it.
// if nothing found then ("",nil) will be returned
func (p *Package) findFunc(recv, name string) (fileName string, funcDecl *ast.FuncDecl) {

eachFile:
	for fn, f := range p.astf {
		for _, decl := range f.Decls {
			fdecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			//ast.Print(p.fset, fdecl)
			recvTypeExpr, funcName := splitFuncDecl(fdecl)
			if recvTypeExpr == recv && funcName == name {
				fileName = fn
				funcDecl = fdecl
				break eachFile
			}

		}
	}

	return
}

// load will read in the package files and parse everything.
func (p *Package) load() error {

	fnl, err := p.fileNames()
	if err != nil {
		return err
	}

	p.fset = &token.FileSet{}
	p.astf = make(map[string]*ast.File, len(fnl))
	p.fileBytes = make(map[string][]byte, len(fnl))
	p.localName = ""

	pkgNames := make([]string, 0, 1)
	for _, fn := range fnl {
		b, err := p.readFile(fn)
		if err != nil {
			return fmt.Errorf("load for %q failed: %w", fn, err)
		}
		p.fileBytes[fn] = b
		af, err := parser.ParseFile(p.fset, fn, b, parser.ParseComments)
		if err != nil {
			return err
		}
		pkgNames = append(pkgNames, af.Name.Name)
		p.astf[fn] = af
		// NOTE: ParseDir returns an ast.Package but it doesn't have any additional info,
		// a simple slice of *ast.File is just as well (plus we need the separate filesystem support)
		// NOTE: if we need SSA we'll just call sslutil.BuildPackage somewhere around here
	}

	switch len(pkgNames) {
	case 0:
		// derive from full package name
		_, n := path.Split(p.fullName)
		n = strings.NewReplacer("-", "").Replace(n)
		p.localName = n
	case 1:
		p.localName = pkgNames[0]
	default:
		for _, pn := range pkgNames {
			if strings.HasSuffix(pn, "_test") { // _test package is okay, disregard
				continue
			}
			if p.localName != "" {
				return fmt.Errorf("multiple package names found: %v", pkgNames)
			}
			p.localName = pn
		}
	}

	if !identRE.MatchString(p.localName) {
		return fmt.Errorf("derived package name %q is not valid", p.localName)
	}

	return nil
}

// splitFuncDecl examines a FuncDecl and returns the type expression and name.
func splitFuncDecl(f *ast.FuncDecl) (recvTypeExpr, funcName string) {

	funcName = f.Name.Name

	// extract receiver type
	if f.Recv.NumFields() == 1 {
		switch typ := f.Recv.List[0].Type.(type) {
		case *ast.Ident:
			recvTypeExpr = typ.Name
		case *ast.StarExpr:
			if i, ok := typ.X.(*ast.Ident); ok {
				recvTypeExpr = "*" + i.Name
			}
			// } else {
			// 	return ret, fmt.Errorf("StarExpr with unknown X: %t / %v", typ.X, typ.X)
			// }
		default:
			// return ret, fmt.Errorf("unexpected receiver type: %t / %v", typ, typ)
		}
	}

	return
}

// FileWriter is an FS that has a WriteFile method on it.
type FileWriter interface {
	// memfs and other implementations should provide such a method to support writes
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// // writeFile looks at the fs implementations and calls the appropriate WriteFile method
// func writeFile(fsys fs.FS, name string, data []byte, perm fs.FileMode) error {

// 	// check for fileWriter implementation (memfs and this package's DirFS implement this)
// 	tfs, ok := fsys.(FileWriter)
// 	if ok {
// 		return tfs.WriteFile(name, data, perm)
// 	}

// 	// // then special case for OS filesystem
// 	// typ := reflect.TypeOf(fsys)
// 	// for typ.Kind() == reflect.Ptr {
// 	// 	typ = typ.Elem()
// 	// }
// 	// // os.dirFS is implemented as a string, we can read it with reflect,
// 	// // this is a hack, but really dirFS should have write methods on it that can be
// 	// // checked for with an interface - I think it's pretty silly this isn't even supported
// 	// // and discards a good portion of the benefits of the
// 	// if typ.PkgPath() == "os" && typ.Kind() == reflect.String {

// 	// }

// 	return fmt.Errorf("writeFile unsupported for filesystem type %t", fsys)
// }

func getFileModeOrDefault(fsys fs.FS, fpath string, defaultMode fs.FileMode) fs.FileMode {
	f, err := fsys.Open(fpath)
	if err != nil {
		return defaultMode
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return defaultMode
	}
	return st.Mode()
}
