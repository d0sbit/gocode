package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/psanford/memfs"

	"github.com/d0sbit/gocode/srcedit"
	"github.com/d0sbit/gocode/srcedit/diff"
	"github.com/d0sbit/gocode/srcedit/model"
)

//go:embed sqlcrud.tmpl
var defaultTmplFS embed.FS

func main() {
	os.Exit(maine(
		flag.NewFlagSet(os.Args[0], flag.PanicOnError),
		os.Args[1:]))
}

// maine is broken out so it can be tested separately
func maine(flagSet *flag.FlagSet, args []string) int {

	typeF := flagSet.String("type", "", "Type name of Go struct with fields corresponding to the MongoDB document")
	fileF := flagSet.String("file", "", "Filename for the main type into which to add store code")
	testFileF := flagSet.String("test-file", "", "Test filename into which to add code, defaults to file with _test.go suffix")
	storeFileF := flagSet.String("store-file", "store.go", "Filename for the Store type")
	storeTestFileF := flagSet.String("store-test-file", "store_test.go", "Filename for the Store type tests")
	packageF := flagSet.String("package", "", "Package directory within module to analyze/edit")
	migrationsPackageF := flagSet.String("migrations-package", "", "Package directory to use for migrations, will default to ../migrations resolved against the package directory")
	dryRunF := flagSet.String("dry-run", "off", "Do not apply changes, only output diff of what would change. Value specifies format, 'term' for terminal pretty text, 'html' for HTML, or 'off' to disable.")
	noGofmtF := flagSet.Bool("no-gofmt", false, "Do not gofmt the output")
	jsonF := flagSet.Bool("json", false, "Write output as JSON")
	vF := flagSet.Bool("v", false, "Verbose output")
	// allF := flagSet.Bool("all", false, "Generate all methods")

	flagSet.Parse(args)

	fileArgList := flagSet.Args()
	if len(fileArgList) > 1 {
		log.Fatalf("you cannot specify more than one file (%d found)", len(fileArgList))
	}

	// FIXME: for now if they provide an arg we just infer the various other params and overwrite them,
	// this should be cleaned up at some point so it is consistent with gocode_handlercrud and whatever else
	if len(fileArgList) == 1 {
		fileArg := fileArgList[0]

		rootFS, modDir, packagePath, modPath, err := srcedit.FindOSWdModuleDir(filepath.Dir(fileArg))
		if err != nil {
			log.Fatalf("error finding module directory: %v", err)
		}
		// if *vF {
		// log.Printf("file arg step: rootFS=%v; modDir=%q, packagePath=%q, modPath=%q", rootFS, modDir, packagePath, modPath)
		// }
		inFS, err := fs.Sub(rootFS, modDir)
		if err != nil {
			log.Fatalf("fs.Sub error while construct input fs: %v", err)
		}

		// we won't be writing here so don't bother with outFS
		storePkg := srcedit.NewPackage(inFS, inFS, modPath, packagePath)

		// now that we have all of this stuff hacked in here,
		// we can look for the type based on the file name
		typeSearch := strings.TrimSuffix(filepath.Base(fileArg), ".go")
		typeInfo, err := storePkg.FindTypeLoose(typeSearch)
		if err != nil {
			log.Fatalf("failed to find type for %q: %v", typeSearch, err)
		}

		// ast.Print(typeInfo.FileSet, typeInfo.GenDecl)
		*typeF = typeInfo.Name()

		*fileF = filepath.Base(fileArg)

		// testFileF can just be inferred below
		// storeFileF and storeTestFileF are fine as-is too

		// packageF needs to come from FindOSWdModuleDir
		*packageF = packagePath

	}

	typeName := *typeF
	if typeName == "" {
		log.Fatalf("-type is required")
	}

	typeFilename := *fileF
	if typeFilename == "" {
		typeFilename = srcedit.LowerForType(typeName+"Store", "-") + ".go"
	}

	if *testFileF == "" {
		*testFileF = strings.TrimSuffix(typeFilename, ".go") + "_test.go"
	}

	rootFS, modDir, packagePath, modPath, err := srcedit.FindOSWdModuleDir(*packageF)
	if err != nil {
		log.Fatalf("error finding module directory: %v", err)
	}
	if *vF {
		log.Printf("rootFS=%v; modDir=%q, packagePath=%q, modPath=%q", rootFS, modDir, packagePath, modPath)
	}

	migrationsPackagePath := *migrationsPackageF
	if migrationsPackagePath == "" {
		migrationsPackagePath = strings.TrimPrefix(path.Join(packagePath, "../migrations"), "/")
	}

	// set up file systems
	inFS, err := fs.Sub(rootFS, modDir)
	if err != nil {
		log.Fatalf("fs.Sub error while construct input fs: %v", err)
	}

	// output is either same as input or memory for dry-run
	var outFS fs.FS
	var dryRunFS *memfs.FS
	if *dryRunF == "off" {
		outFS = inFS
		if migrationsPackagePath != "" {
			mda, ok := outFS.(srcedit.MkdirAller)
			if ok {
				err := mda.MkdirAll(migrationsPackagePath, 0755)
				if err != nil {
					log.Fatalf("mda.MkdirAll for %q: %v", migrationsPackagePath, err)
				}
			}
			//outFS.MkdirAll(migrationsPackagePath, 0755)
		}
	} else {
		dryRunFS = memfs.New()
		if packagePath != "" {
			dryRunFS.MkdirAll(packagePath, 0755)
		}
		if migrationsPackagePath != "" {
			// log.Printf("creating migrationsPackagePath %q", migrationsPackagePath)
			err := dryRunFS.MkdirAll(migrationsPackagePath, 0755)
			// log.Printf("migrations MkdirAll returned: %v", err)
			_ = err
		}
		outFS = dryRunFS
	}

	// load the package with srcedit
	pkg := srcedit.NewPackage(inFS, outFS, modPath, packagePath)

	// load the migrations package
	if *vF {
		log.Printf("NewPackage for migrations: inFS=%#v, outFS=%#v, modPath=%#v, migrationsPackagePath=%#v",
			inFS, outFS, modPath, migrationsPackagePath)
	}
	migrationsPkg := srcedit.NewPackage(inFS, outFS, modPath, migrationsPackagePath)

	// get the definition for the specified type
	typeInfo, err := pkg.FindType(typeName)
	if err != nil {
		log.Fatalf("failed to find type %q: %v", typeName, err)
	}

	// populate Struct
	// s := Struct{
	// 	pkgImportedName: "", // TOOD: figure out what to do with prefixed types (not in the same package)
	// 	name:            typeName,
	// 	typeInfo:        typeInfo,
	// }
	// s.fields, err = s.makeFields()
	// if err != nil {
	// 	log.Fatalf("failed to extract field info from type %q: %v", typeName, err)
	// }

	s, err := model.NewStruct(typeInfo, "")
	if err != nil {
		log.Fatalf("failed to find type %q: %v", typeName, err)
	}

	// execute template
	data := struct {
		Struct               *model.Struct
		MigrationsImportPath string
	}{
		Struct:               s,
		MigrationsImportPath: modPath + "/" + migrationsPackagePath,
	}
	tmpl, err := template.New("_main_").Funcs(funcMap).ParseFS(defaultTmplFS, "sqlcrud.tmpl")
	if err != nil {
		log.Fatalf("template parse error: %v", err)
	}

	fmtt := &srcedit.GofmtTransform{}
	var trs []srcedit.Transform

	{
		fn := "sqlutil.go"
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "SQLUtil")
		if err != nil {
			log.Fatal(err)
		}
		trs = append(trs, trList...)
	}

	{
		fn := *storeFileF
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "Store", "StoreMethods")
		if err != nil {
			log.Fatal(err)
		}
		trs = append(trs, trList...)
	}

	{
		fn := *storeTestFileF
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "TestStore")
		if err != nil {
			log.Fatal(err)
		}
		trs = append(trs, trList...)
	}

	{
		fn := typeFilename
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl,
			"TYPEStore",
			"TYPEStoreMethods",
			// FIXME: filter which things go here base on flags
			"TYPEInsert",
			"TYPEDelete",
			"TYPEUpdate",
			"TYPESelectByID",
			"TYPESelect",
			"TYPESelectCursor",
			"TYPECount",
		)
		if err != nil {
			log.Fatal(err)
		}
		trs = append(trs, trList...)
	}

	{
		// TODO: -no-test flag
		fn := *testFileF
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl,
			"TestTYPE",
		)
		if err != nil {
			log.Fatal(err)
		}
		trs = append(trs, trList...)
	}

	dd := &srcedit.DedupImportsTransform{
		FilenameList: fmtt.FilenameList,
	}
	trs = append(trs, dd)

	if !*noGofmtF {
		trs = append(trs, fmtt)
	}

	err = pkg.ApplyTransforms(trs...)
	if err != nil {
		log.Fatalf("apply transform error: %v", err)
	}

	// TODO: option to skip migrations stuff?
	// do migrations package separately
	{
		fmtt := &srcedit.GofmtTransform{}
		var trs []srcedit.Transform

		fn := "migrations.go"
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "Migrations")
		if err != nil {
			log.Fatal(err)
		}
		trs = append(trs, trList...)

		dd := &srcedit.DedupImportsTransform{
			FilenameList: fmtt.FilenameList,
		}
		trs = append(trs, dd)

		if !*noGofmtF {
			trs = append(trs, fmtt)
		}

		err = migrationsPkg.ApplyTransforms(trs...)
		if err != nil {
			log.Fatalf("apply transform for migrations error: %v", err)
		}

		// mpf, err := inFS.Open(migrationsPackagePath)

		needSampleMigration := true
		err = fs.WalkDir(inFS, migrationsPackagePath, fs.WalkDirFunc(func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == migrationsPackagePath {
				return nil
			}
			if d.IsDir() { // only scan the immediate directory
				return fs.SkipDir
			}
			if strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".SQL") {
				needSampleMigration = false
			}
			return nil
		}))
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("error walking migrations dir %q: %v", migrationsPackagePath, err)
		}

		if needSampleMigration {
			b := []byte(`
-- +goose Up

-- +goose Down

`)
			fname := time.Now().UTC().Format("20060102150405") + "_sample.sql"
			fpath := path.Join(migrationsPackagePath, fname)
			err := outFS.(srcedit.FileWriter).WriteFile(fpath, b, 0644)
			if err != nil {
				log.Fatalf("error creating example migration file %q: %v", fpath, err)
			}
		}

	}

	if *dryRunF != "off" {
		diffMap, err := diff.Run(inFS, outFS, ".", *dryRunF)
		if err != nil {
			log.Fatalf("error running diff: %v", err)
		}
		if *jsonF {
			enc := json.NewEncoder(os.Stdout)
			enc.Encode(map[string]interface{}{
				"diff": diffMap,
			})
		} else {
			klist := make([]string, 0, len(diffMap))
			for k := range diffMap {
				klist = append(klist, k)
			}
			sort.Strings(klist)
			for _, k := range klist {
				fmt.Printf("### %s\n", k)
				fmt.Println(diffMap[k])
			}
		}
	}

	return 0
}

func tmplToTransforms(fileName string, data interface{}, tmpl *template.Template, tmplName ...string) ([]srcedit.Transform, error) {

	var ret []srcedit.Transform

	for _, tName := range tmplName {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, tName, data)
		if err != nil {
			return ret, fmt.Errorf("%q template exec error: %v", tName, err)
		}

		trList, err := srcedit.ParseTransforms(fileName, buf.String())
		if err != nil {
			return ret, fmt.Errorf("%q transform parse error: %v", tName, err)
		}
		ret = append(ret, trList...)

	}

	return ret, nil

}

var funcMap = template.FuncMap(map[string]interface{}{
	"LowerForType": srcedit.LowerForType,
})
