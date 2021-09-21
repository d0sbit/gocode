// Package srcedit provides high level Go source code editing functionality.
// Although packages like go/parser and friends provide detailed code editing
// facility, we need a nice high level façade that lets us do things like
// "if this function doesn't exist, create it with this source", etc.  And we
// want to centralize the reusable bits here, so individual gocode plugins
// don't have to be loaded with detailed and duplicative source editing
// functionality.
package srcedit

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// ErrNotFound is returned in some cases where an explicity "not found" result is needed.
var ErrNotFound = errors.New("not found")

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
// and then calls FindModuleDir and extracts the module path from go.mod.
// The resolve param is a path to resolve from the current working directory into a package subdir path.
// For example, if you are in /home/joe/project/examplepjt/subpkg and call FindOSWdModuleDir("."),
// on Linux the return would be ("/", "home/joe/projects/examplepjt", "subpkg", "github.com/d0sbit/example", nil),
// or on Windows ("C:\", "user/joe/projects/examplepjt", "subpkg", "github.com/d0sbit/example", nil).
// If resolve is outside of the module directory then an error will be returned.
func FindOSWdModuleDir(resolve string) (rootFS fs.FS, moduleDir, resolved, modulePath string, err error) {

	fsys, dir, err := OSWorkingFSDir()
	if err != nil {
		return nil, "", "", "", err
	}

	r1 := path.Join(dir, resolve)
	if !strings.HasPrefix(r1, dir) {
		return nil, "", "", "", fmt.Errorf("resolve path %q is not at or under dir %q", resolve, dir)
	}
	resolved = strings.TrimPrefix(strings.TrimPrefix(r1, dir), "/")

	modDir, err := FindModuleDir(fsys, dir)
	if err != nil {
		return nil, "", "", "", err
	}

	modf, err := fsys.Open(path.Join(modDir, "go.mod"))
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer modf.Close()
	b, err := ioutil.ReadAll(modf)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	modFile, err := modfile.ParseLax("go.mod", b, nil)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to parse go.mod: %w", err)
	}
	modulePath = modFile.Module.Mod.Path

	return fsys, modDir, resolved, modulePath, nil
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
	infs       fs.FS  // read files from
	outfs      fs.FS  // write updated files to
	modulePath string // the module name from the `module` statement in go.mod
	subDir     string // subdirectory inside infs and outfs of where the code for this package lives
	localName  string // local name from package statements or default

	fset      *token.FileSet       // Go parser needs this
	astf      map[string]*ast.File // each file that was parsed in the package with the filename (no path info) as the key
	fileBytes map[string][]byte    // filename to most recently read contents
}

// NewPackage returns a new Package with the specified input and output filesystems and the specified module name/path.
func NewPackage(infs, outfs fs.FS, modulePath, subDir string) *Package {

	// TODO: stuff below has been handled - move this explalnation somewhere else

	// FIXME: fullName is not really clear here, and I don't think we even use it right now.
	// Are we expecting the caller to read go.mod and prepend whatever is in the `module` directive here?
	// Probably not, in which case this fullName is not a full name at all but a subdirectory to where
	// the package code lives within the module.  That should be made very clear.
	// Also we need to figure out what to do with the case where the package path is "." and the FS
	// is rooted at the module dir.
	// Probably having FindOSWdModuleDir read the go.mod and extract the module prefix would be a decent way to go,
	// so we get back the root filesystem ("C:\" or "/""), the subdir to the go.mod ("/home/joe/git/somepjt"),
	// the logical module prefix ("github.com/joe/somepjt"), and the subdir under that ("." or "internal/mstore" or whatever).
	// FIXME: do we need to remove the "v2" from the name?  So far this hasn't come up because we don't deal with
	// emitting import paths to files, but are just using them to read and manipulate source, without understanding
	// versions or even how the imports relate to each other.
	return &Package{
		infs:       infs,
		outfs:      outfs,
		modulePath: modulePath,
		subDir:     subDir,
	}
}

// SubDir returns the subdirectory in which the code lives - "" means it lives directly in the root of the filesystem(s), i.e. directly in the module dir.
func (p *Package) SubDir() string {
	return p.subDir
}

// ModuleName returns the name of the module (from the `module` line in go.mod).
func (p *Package) ModuleName() string {
	return p.modulePath
}

// LocalName returns the name from the package statements inside the source files.
// If no source files exist then a default is derived from the import path.
func (p *Package) LocalName() string {
	return p.localName
}

// readFile will read a file from outfs if it exists there and if not from infs.
// This way if the specified file has been modified you'll get the modified file,
// otherwise the original unmodified one.  The filename should not have any path
// separators in it.
func (p *Package) readFile(filename string) ([]byte, error) {
	openPath := path.Join(p.subDir, filename)
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
// Any subdirectories are ignored.
func (p *Package) fileNames() ([]string, error) {

	retMap := make(map[string]struct{}, 8)

	subDir := "."
	if p.subDir != "" {
		subDir = p.subDir
	}
	dirEntryList, err := fs.ReadDir(p.outfs, subDir)
	// log.Printf("fs.ReadDir(outfs=%#v, %q) err: %v", p.outfs, subDir, err)
	if err != nil {
		return nil, fmt.Errorf("cannot load from output dir %q (did you forget to create it?): %w", subDir, err)
	}
	for _, de := range dirEntryList {
		if de.IsDir() {
			continue
		}
		if path.Ext(de.Name()) != ".go" {
			continue
		}
		retMap[de.Name()] = struct{}{}
	}

	dirEntryList, err = fs.ReadDir(p.infs, subDir)
	// log.Printf("fs.ReadDir(infs=%#v, %q) err: %v", p.infs, subDir, err)
	if err != nil {
		if !os.IsNotExist(err) { // package dir doesn't need to exist in input
			return nil, err
		}
	} else {
		for _, de := range dirEntryList {
			if de.IsDir() {
				continue
			}
			if path.Ext(de.Name()) != ".go" {
				continue
			}
			retMap[de.Name()] = struct{}{}
		}
	}

	var ret []string
	for fn := range retMap {
		ret = append(ret, fn)
	}
	sort.Strings(ret)

	return ret, nil
}

// writeFile is a helper so we don't have to cast outfs to a FileWriter all over the place
func (p *Package) writeFile(fullPath string, data []byte, perm fs.FileMode) error {
	fwriter, ok := p.outfs.(FileWriter)
	if !ok {
		return fmt.Errorf("p.outfs does not implement FileWriter, cannot write changes")
	}
	return fwriter.WriteFile(fullPath, data, perm)
}

// writeFileNamed is like writeFile but detects the FileMode from the existing file or uses 0644 default,
// and only accepts the name of the file not the path (since we always write to the package folder anyway)
func (p *Package) writeFileNamed(name string, data []byte) error {
	dir, _ := path.Split(name)
	if dir != "" {
		return fmt.Errorf("name %q appears to have a directory, cannot be used with writeFileNamed", name)
	}
	return p.writeFile(path.Join(p.subDir, name), data, p.getFileModeOrDefault(name, 0644))
}

// ApplyTransforms calls ApplyTransform for each one provided.  Note that transforms
// are not atomic and this may result in only a subset of the requested changes being applied
// upon error.  Consider using a separate output filesystem if you're concerned about this.
func (p *Package) ApplyTransforms(tr ...Transform) error {

	// log.Printf("ApplyTransforms, tr: %#v", tr)

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

	err := p.load()
	if err != nil {
		return fmt.Errorf("package load in ApplyTransform error: %w", err)
	}

	switch t := tr.(type) {

	case *AddFuncDeclTransform:
		err := p.applyAddFuncDecl(t)
		if err != nil {
			return fmt.Errorf("applyAddFuncDecl: %w", err)
		}
		return nil

	case *DedupImportsTransform:
		err := p.applyDedupImports(t)
		if err != nil {
			return fmt.Errorf("applyDedupImports: %w", err)
		}
		return nil

	case *ImportTransform:
		err := p.applyImport(t)
		if err != nil {
			return fmt.Errorf("applyImport: %w", err)
		}
		return nil

	case *AddConstDeclTransform:
		err := p.applyAddConstDecl(t)
		if err != nil {
			return fmt.Errorf("applyAddConstDecl: %w", err)
		}
		return nil

	case *AddVarDeclTransform:
		err := p.applyAddVarDecl(t)
		if err != nil {
			return fmt.Errorf("applyAddVarDecl: %w", err)
		}
		return nil

	case *AddTypeDeclTransform:
		err := p.applyAddTypeDecl(t)
		if err != nil {
			return fmt.Errorf("applyAddTypeDecl: %w", err)
		}
		return nil

	case *GofmtTransform:
		err := p.applyGoFmt(t)
		if err != nil {
			return fmt.Errorf("applyAddFuncDecl: %w", err)
		}
		return nil

	}

	return fmt.Errorf("unknown transform type: %t", tr)
}

func (p *Package) applyGoFmt(t *GofmtTransform) error {

	// log.Printf("applyGoFmt: %#v", t)

	allNames := t.FilenameList == nil
	var nameSet map[string]struct{}
	if len(t.FilenameList) > 0 {
		nameSet = make(map[string]struct{}, len(t.FilenameList))
	}

	for _, fn := range t.FilenameList {
		nameSet[fn] = struct{}{}
	}

	for fn, b := range p.fileBytes {
		fn, b := fn, b

		// make sure we're supposed to format this file
		if !allNames {
			_, ok := nameSet[fn]
			if !ok {
				continue
			}
		}

		// run gofmt and pipe input from b
		cmd := exec.Command("gofmt")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("gofmt command StdinPipe error: %w", err)
		}
		go func(b []byte) {
			stdin.Write(b)
			stdin.Close()
		}(b)

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gofmt command error: %w; full output: %s", err, out)
		}

		// check for minimum possible length, just as an added check to be sure we don't clobber the file here
		if len(out) == len("package a") {
			return fmt.Errorf("gofmt returned contents that are too short to valid: %s", out)
		}

		// do not write file if no change
		if bytes.Equal(b, out) {
			continue
		}

		// update bytes (map write with existing key should be safe during iterate)
		p.fileBytes[fn] = out
		err = p.writeFileNamed(fn, out)
		if err != nil {
			return fmt.Errorf("failed to write result of gofmt: %w", err)
		}

	}

	return nil
}

func (p *Package) applyAddFuncDecl(t *AddFuncDeclTransform) error {

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

			// existingFilePath := path.Join(p.subDir, existingFilename)
			err := p.writeFileNamed(existingFilename, b)
			if err != nil {
				return err
			}

		}
	}

	// then write out the file indicated in the transform with the new func added to the bottom
	b := p.fileBytesOrNew(t.Filename)
	b = append(b, t.Text...)
	b = append(b, "\n"...)
	return p.writeFileNamed(t.Filename, b)

}

func (p *Package) applyImport(t *ImportTransform) error {

	// spin through and find the last import
	// block and add a line there if it's a block,
	// otherwise single import line, and if not that then
	// find the package line and add after that.

	b := p.fileBytes[t.Filename]

	af := p.astf[t.Filename]
	var lastImport *ast.GenDecl
	if af != nil {
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

			lastImport = genDecl
		}
	}

	switch {

	case lastImport != nil && lastImport.Rparen != token.NoPos:
		// add line at end of import block

		pos := p.fset.Position(lastImport.Rparen)

		buf := make([]byte, 0, len(b)+len(t.Name)+len(t.Path)+16)
		buf = append(buf, b[:pos.Offset]...)
		// add newline before paren if missing
		if buf[len(buf)-1] != '\n' {
			buf = append(buf, "\n"...)
		}
		buf = append(buf, "\t"...)
		if t.Name != "" {
			buf = append(buf, t.Name...)
			buf = append(buf, " "...)
		}
		buf = append(buf, "\""...)
		buf = append(buf, t.Path...)
		buf = append(buf, "\"\n"...)
		buf = append(buf, b[pos.Offset:]...)

		b = buf

	case lastImport != nil:
		// add separate import statement after last

		pos := p.fset.Position(lastImport.End())

		buf := make([]byte, 0, len(b)+len(t.Name)+len(t.Path)+16)
		buf = append(buf, b[:pos.Offset]...)
		// add newline before paren if missing
		if buf[len(buf)-1] != '\n' {
			buf = append(buf, "\n"...)
		}
		buf = append(buf, "import "...)
		if t.Name != "" {
			buf = append(buf, t.Name...)
			buf = append(buf, " "...)
		}
		buf = append(buf, "\""...)
		buf = append(buf, t.Path...)
		buf = append(buf, "\"\n"...)
		buf = append(buf, bytes.TrimPrefix(b[pos.Offset:], []byte("\n"))...)

		b = buf

	default:
		// add first import statement after package line

		// could be a file without imports or a brand new file we're creating now, no difference here
		b = p.fileBytesOrNew(t.Filename)

		// find package statement
		// pkgre := regexp.MustCompile(`\n\s*package\s*.*`)
		pkgre := regexp.MustCompile(`(?m)^\s*package\s*.*$`)
		pkgidx := pkgre.FindIndex(b)
		if pkgidx == nil {
			return fmt.Errorf("unable to find package line in %q", t.Filename)
		}

		buf := make([]byte, 0, len(b)+len(t.Name)+len(t.Path)+16)
		buf = append(buf, b[:pkgidx[1]]...)
		buf = append(buf, "\n\n"...)
		buf = append(buf, "import "...)
		if t.Name != "" {
			buf = append(buf, t.Name...)
			buf = append(buf, " "...)
		}
		buf = append(buf, "\""...)
		buf = append(buf, t.Path...)
		buf = append(buf, "\"\n"...)
		buf = append(buf, b[pkgidx[1]:]...)

		b = buf

	}

	return p.writeFileNamed(t.Filename, b)
	// fullPath := path.Join(p.subDir, t.Filename)
	// return p.writeFile(fullPath, b, p.getFileModeOrDefault(fullPath, 0644))
}

func (p *Package) applyAddConstDecl(t *AddConstDeclTransform) error {

	// names must match or be a superset

	// af := p.astf[t.Filename]
	filename, names, varOrConstDecl := p.findVarOrConstDecl(token.CONST, t.NameList)
	// log.Printf("filename=%q, names=%+v, varOrConstDecl=%#v", filename, names, varOrConstDecl)
	//p.findVarOrConstDecl(tok token.Token, withAnyNames []string) (filename string, names []string, varOrConstDecl *ast.GenDecl)

	if varOrConstDecl != nil {

		// if a block exists that is not a subset, it's not clear what to do, so error
		if !isStrSubset(names, t.NameList) {
			return fmt.Errorf("name list from transform const block %+v is not a subset of existing block %+v", t.NameList, names)
		}

		// if not replacing, then we're done
		if !t.Replace {
			return nil
		}

		// b := p.fileBytes[filename]
		b := p.fileBytesWithoutBlock(filename, varOrConstDecl)
		p.fileBytes[filename] = b
		err := p.writeFileNamed(t.Filename, b)
		if err != nil {
			return err
		}

	}

	b := p.fileBytesOrNew(t.Filename)
	b = append(b, t.Text...)
	b = append(b, "\n"...)
	return p.writeFileNamed(t.Filename, b)
}

func (p *Package) applyAddVarDecl(t *AddVarDeclTransform) error {

	// names must match or be a superset

	// af := p.astf[t.Filename]
	filename, names, varOrConstDecl := p.findVarOrConstDecl(token.VAR, t.NameList)
	// log.Printf("filename=%q, names=%+v, varOrConstDecl=%#v", filename, names, varOrConstDecl)
	//p.findVarOrConstDecl(tok token.Token, withAnyNames []string) (filename string, names []string, varOrConstDecl *ast.GenDecl)

	if varOrConstDecl != nil {

		// if a block exists that is not a subset, it's not clear what to do, so error
		if !isStrSubset(names, t.NameList) {
			return fmt.Errorf("name list from transform var block %+v is not a subset of existing block %+v", t.NameList, names)
		}

		// if not replacing, then we're done
		if !t.Replace {
			return nil
		}

		// b := p.fileBytes[filename]
		b := p.fileBytesWithoutBlock(filename, varOrConstDecl)
		p.fileBytes[filename] = b
		err := p.writeFileNamed(t.Filename, b)
		if err != nil {
			return err
		}

	}

	b := p.fileBytesOrNew(t.Filename)
	b = append(b, t.Text...)
	b = append(b, "\n"...)
	return p.writeFileNamed(t.Filename, b)
}

func (p *Package) applyAddTypeDecl(t *AddTypeDeclTransform) error {

	filename, typeDecl := p.findTypeDecl(t.Name)
	// log.Printf("applyAddTypeDecl - filename=%q, typeDecl=%v", filename, typeDecl)

	if typeDecl != nil {

		// if not replacing, then we're done
		if !t.Replace {
			return nil
		}

		b := p.fileBytesWithoutBlock(filename, typeDecl)
		p.fileBytes[filename] = b
		err := p.writeFileNamed(t.Filename, b)
		if err != nil {
			return err
		}

	}

	b := p.fileBytesOrNew(t.Filename)
	b = append(b, t.Text...)
	b = append(b, "\n"...)
	return p.writeFileNamed(t.Filename, b)

}

func (p *Package) fileBytesOrNew(fname string) []byte {

	b := p.fileBytes[fname]
	if b == nil { // might be a new file, which we start with just a package statement
		b = []byte("package " + p.localName + "\n\n")
	}
	return b

}

func (p *Package) fileBytesWithoutBlock(fname string, node ast.Node) []byte {

	b := p.fileBytes[fname]
	if b == nil {
		return nil
	}

	// all nodes have a start and end
	start := node.Pos()
	end := node.End()

	// but if there's a comment block, move to start of that
	var cg *ast.CommentGroup
	switch d := node.(type) {
	case *ast.TypeSpec:
		cg = d.Doc
	case *ast.ValueSpec:
		cg = d.Doc
	case *ast.Field:
		cg = d.Doc
	case *ast.FuncDecl:
		cg = d.Doc
	case *ast.GenDecl:
		cg = d.Doc
	case *ast.ImportSpec:
		cg = d.Doc
	}

	if cg != nil {
		start = cg.Pos()
	}

	// convert to byte offset
	startOffset := p.fset.Position(start).Offset
	endOffset := p.fset.Position(end).Offset

	out := make([]byte, 0, len(b))

	out = append(out, b[:startOffset]...)
	out = append(out, b[endOffset:]...)

	return out
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

func (p *Package) findVarOrConstDecl(tok token.Token, withAnyNames []string) (filename string, names []string, varOrConstDecl *ast.GenDecl) {

	nmap := make(map[string]struct{}, len(withAnyNames))
	for _, n := range withAnyNames {
		nmap[n] = struct{}{}
	}

	for fn, af := range p.astf {

		for _, decl := range af.Decls {
			// only genDecls
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			// with matching token
			if genDecl.Tok != tok {
				continue
			}

			names = names[:0]
			foundMatch := false

			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range valueSpec.Names {
					names = append(names, n.Name)
					if _, ok := nmap[n.Name]; ok {
						foundMatch = true
					}
				}
				// log.Printf("spec: %#v", spec)
				// ast.ValueSec
			}

			if foundMatch {
				filename = fn
				varOrConstDecl = genDecl
				return
			}
		}
	}

	return "", nil, nil
}

// FindType returns information about a type in the package.
// This causes the package to be (re)loaded/parsed into memory.
// If the type cannot be found then ErrNotFound is returned in err.
func (p *Package) FindType(withName string) (ret *TypeInfo, err error) {
	err = p.load()
	if err != nil {
		return nil, fmt.Errorf("load failed: %w", err)
	}
	filename, typeDecl := p.findTypeDecl(withName)
	if typeDecl == nil {
		err = ErrNotFound
	}
	ret = &TypeInfo{
		GenDecl:   typeDecl,
		FileSet:   p.fset,
		Filename:  filename,
		FileBytes: p.fileBytes[filename],
	}
	return
}

func (p *Package) findTypeDecl(withName string) (filename string, typeDecl *ast.GenDecl) {

	for fn, af := range p.astf {
		_ = fn

		for _, decl := range af.Decls {
			// only genDecls
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			// with `type` token
			if genDecl.Tok != token.TYPE {
				continue
			}

			// sanity check
			if len(genDecl.Specs) != 1 {
				continue
			}

			// should contain a TypeSpec
			typeSpec, ok := genDecl.Specs[0].(*ast.TypeSpec)
			if !ok {
				continue
			}

			// from which we can extract and check the name
			if typeSpec.Name.Name != withName {
				continue
			}

			return fn, genDecl

		}
	}

	return "", nil
}

// load will read in the package files and parse everything.
func (p *Package) load() error {

	fnl, err := p.fileNames()
	if err != nil {
		return fmt.Errorf("fileNames error: %w", err)
	}

	p.fset = &token.FileSet{}
	p.astf = make(map[string]*ast.File, len(fnl))
	p.fileBytes = make(map[string][]byte, len(fnl))
	p.localName = ""

	pkgNames := make([]string, 0, 1)
	pkgNameMap := make(map[string]struct{}, 2)
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
		p.astf[fn] = af
		pname := af.Name.Name
		_, ok := pkgNameMap[pname]
		if !ok {
			pkgNames = append(pkgNames, pname)
			pkgNameMap[pname] = struct{}{}
		}
		// NOTE: ParseDir returns an ast.Package but it doesn't have any additional info,
		// a simple slice of *ast.File is just as well (plus we need the separate filesystem support)
		// NOTE: if we need SSA we'll just call sslutil.BuildPackage somewhere around here
	}

	switch len(pkgNames) {
	case 0:
		// derive from subdir
		_, n := path.Split(p.subDir)
		n = strings.NewReplacer("-", "").Replace(n)
		p.localName = n

		// but if no subdir then look at last element of modulePath
		if p.localName == "" || p.localName == "." {
			_, n := path.Split(p.modulePath)
			p.localName = n
		}

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

func (p *Package) getFileModeOrDefault(fpath string, defaultMode fs.FileMode) fs.FileMode {
	{
		f, err := p.outfs.Open(fpath)
		if err != nil {
			goto checkInfs
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return defaultMode
		}
		return st.Mode()
	}
checkInfs:
	f, err := p.infs.Open(fpath)
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

// MkdirAller has a MkdirAll call matching the os.MkdirAll signature.
type MkdirAller interface {
	MkdirAll(path string, perm os.FileMode) error
}

// isStrSubset checks if s1 ⊆ s2
// (returns true if all elements of s1 are in s2)
func isStrSubset(s1, s2 []string) bool {

	if len(s2) < len(s1) {
		return false
	}

	for _, v1 := range s1 {
		found := false
		for _, v2 := range s2 {
			if v1 == v2 {
				found = true
			}
		}
		if !found {
			return false
		}
	}

	return true
}
