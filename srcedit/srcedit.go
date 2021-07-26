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
	"sort"
	"strings"
)

// OSWorkingFSDir returns a filesystem at the OS root (of the drive on Windows) and the path of the current working directory.
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
			return os.DirFS(wd), strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(origwd, wd)), "/"), nil
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
	infs     fs.FS  // read files from
	outfs    fs.FS  // write updated files to
	fullName string // full package name

	fset *token.FileSet // Go parser needs this
	astf []*ast.File    // each file that was parsed in the package
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

// Name returns just the package name, e.g. given "a/b/c/", this will return "c".
func (p *Package) Name() string {
	_, n := path.Split(p.fullName)
	return n
}

// Load will read in the package files and parse everything.
func (p *Package) Load() error {

	fnl, err := p.fileNames()
	if err != nil {
		return err
	}

	p.fset = &token.FileSet{}
	p.astf = make([]*ast.File, 0, len(fnl))

	for _, fn := range fnl {
		b, err := p.readFile(fn)
		if err != nil {
			return fmt.Errorf("load for %q failed: %w", fn, err)
		}
		af, err := parser.ParseFile(p.fset, fn, b, parser.ParseComments)
		if err != nil {
			return err
		}
		p.astf = append(p.astf, af)
		// NOTE: ParseDir returns an ast.Package but it doesn't have any additional info,
		// a simple slice of *ast.File is just as well
		// NOTE: if we need SSA we'll just call sslutil.BuildPackage somewhere around here
	}

	return nil
}

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
