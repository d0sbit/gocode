package main

import (
	"bytes"
	"embed"
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

	"github.com/d0sbit/gocode/config"
	"github.com/d0sbit/gocode/srcedit"
	"github.com/d0sbit/gocode/srcedit/diff"
	"github.com/d0sbit/gocode/srcedit/model"
	"github.com/psanford/memfs"
	"github.com/pterm/pterm"
)

//go:embed handlercrud.tmpl
var defaultTmplFS embed.FS

func main() {
	os.Exit(maine(
		flag.NewFlagSet(os.Args[0], flag.PanicOnError),
		os.Args[1:]))
}

// maine is broken out so it can be tested separately
func maine(flagSet *flag.FlagSet, args []string) int {

	// typeF := flagSet.String("type", "", "Type name of Go struct with fields corresponding to the MongoDB document")
	// fileF := flagSet.String("file", "", "Filename for the main type into which to add store code")
	// testFileF := flagSet.String("test-file", "", "Test filename into which to add code, defaults to file with _test.go suffix")
	// storeFileF := flagSet.String("store-file", "store.go", "Filename for the Store type")
	// storeTestFileF := flagSet.String("store-test-file", "store_test.go", "Filename for the Store type tests")
	// packageF := flagSet.String("package", "", "Package directory within module to analyze/edit")
	// migrationsPackageF := flagSet.String("migrations-package", "", "Package directory to use for migrations, will default to ../migrations resolved against the package directory")
	dryRunF := flagSet.Bool("dry-run", false, "Do not apply changes, only output diff of what would change.")
	noGofmtF := flagSet.Bool("no-gofmt", false, "Do not gofmt the output")
	// jsonF := flagSet.Bool("json", false, "Write output as JSON")
	vF := flagSet.Bool("v", false, "Verbose output")
	// allF := flagSet.Bool("all", false, "Generate all methods")

	flagSet.Parse(args)

	pterm.Info.Println("Hello!")

	_ = vF

	fileArgs := flagSet.Args()
	if len(fileArgs) != 1 {
		log.Fatalf("you must provide exactly one file name (found %d instead)", len(fileArgs))
	}
	fileArg := fileArgs[0]

	// absFileArg, err := filepath.Abs(fileArg)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fileNamePart := filepath.Base(absFileArg)
	fileNamePart := filepath.Base(fileArg)

	rootFS, modDir, wdPackagePath, modPath, err := srcedit.FindOSWdModuleDir(filepath.Dir(fileArg))
	if err != nil {
		log.Fatalf("error finding module directory: %v", err)
	}
	// _, _, _, _ = rootFS, modDir, wdPackagePath, modPath
	// log.Printf("FindOSWdModuleDir(%q) returned rootFS=%q, modDir=%q, wdPackagePath=%q, modPath=%q",
	// 	absFileArg, rootFS, modDir, wdPackagePath, modPath)
	if *vF {
		log.Printf("FindOSWdModuleDir(%q) returned rootFS=%q, modDir=%q, wdPackagePath=%q, modPath=%q",
			fileArg, rootFS, modDir, wdPackagePath, modPath)
	}

	// set up file systems
	inFS, err := fs.Sub(rootFS, modDir)
	if err != nil {
		log.Fatalf("fs.Sub error while construct input fs: %v", err)
	}

	// output is either same as input or memory for dry-run
	var outFS fs.FS
	var dryRunFS *memfs.FS
	if !*dryRunF {
		outFS = inFS
		// if migrationsPackagePath != "" {
		// 	mda, ok := outFS.(srcedit.MkdirAller)
		// 	if ok {
		// 		err := mda.MkdirAll(migrationsPackagePath, 0755)
		// 		if err != nil {
		// 			log.Fatalf("mda.MkdirAll for %q: %v", migrationsPackagePath, err)
		// 		}
		// 	}
		// 	//outFS.MkdirAll(migrationsPackagePath, 0755)
		// }
	} else {
		dryRunFS = memfs.New()
		// if packagePath != "" {
		// 	dryRunFS.MkdirAll(packagePath, 0755)
		// }
		// if migrationsPackagePath != "" {
		// 	// log.Printf("creating migrationsPackagePath %q", migrationsPackagePath)
		// 	err := dryRunFS.MkdirAll(migrationsPackagePath, 0755)
		// 	// log.Printf("migrations MkdirAll returned: %v", err)
		// 	_ = err
		// }
		outFS = dryRunFS
	}

	// FIXME: how does config work with dry run? (it probably should be part of the dry-run output)
	// which means the dry run FS stuff should move up here

	// load config
	// moduleFS, err := fs.Sub(rootFS, modDir)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	config, err := config.LoadFS(inFS, true)
	if err != nil {
		log.Fatalf("config.LoadFS failed: %v", err)
	}
	// TODO: we should distinguish between the directory with stores and the directory with types at some later point,
	// but for now we assume these are always the same
	storeDir := config.GetString("store_dir", "store")

	handlersDir := config.GetString("handlers_dir", "handlers")

	// TODO: make functions that:
	// 1. verify a given path matches the specified suffix (i.e. "is this in the handlers folder")
	// 2. given that we are in one folder, then translate to another (i.e. "verify we are in the handlers folder and give me the path for the store folder")

	if !srcedit.DirHasSuffix(wdPackagePath, handlersDir) {
		log.Fatalf("%q is not in the handlers_dir %q", wdPackagePath, handlersDir)
	}

	// make sure the handlers dir exists in the output
	outFSMDA, ok := outFS.(srcedit.MkdirAller)
	if ok {
		err := outFSMDA.MkdirAll(wdPackagePath, 0755)
		if err != nil {
			log.Fatalf("MkdirAll for %q: %v", wdPackagePath, err)
		}
	}

	storePkgPath, err := srcedit.DirResolveTo(wdPackagePath, handlersDir, storeDir)
	if err != nil {
		log.Fatal(err)
	}
	if *vF {
		log.Printf("storePkgPath: %s", storePkgPath)
	}

	// NVM: if there's no store then there's no types so don't bother
	// check if storeDir exists, if not then prompt before mkdir
	// Store directory %q does not exist, press Ctrl+C to abort, enter to create it, or enter a new directory name to use that instead:
	// save to config if they entered anything

	// that handles: type package... maybe check for "store" and/or prompt

	// parse the store/type package
	storePkg := srcedit.NewPackage(inFS, outFS, modPath, storePkgPath)

	// and handlers package while we're at it
	handlersPkg := srcedit.NewPackage(inFS, outFS, modPath, wdPackagePath)
	_ = handlersPkg

	// look at the file name and check for a matching type
	typeSearch := strings.TrimSuffix(fileNamePart, ".go")
	typeInfo, err := storePkg.FindTypeLoose(typeSearch)
	if err != nil {
		log.Fatalf("failed to find type for %q: %v", typeSearch, err)
	}
	_ = typeInfo

	// file is the file
	// test file is formed by just adding _test.go
	// handler package is folder
	// then store in config file (use https://github.com/BurntSushi/toml for now and figure out comment preservation later)

	// that should be what we need to
	// parse the template
	// emit everything

	s, err := model.NewStruct(typeInfo, "")
	if err != nil {
		log.Fatalf("NewStruct failed: %v", err)
	}

	// execute template
	data := struct {
		Struct          *model.Struct
		StoreImportPath string
	}{
		Struct:          s,
		StoreImportPath: path.Join(modPath, storePkgPath),
	}
	tmpl, err := template.New("_main_").Funcs(funcMap).ParseFS(defaultTmplFS, "handlercrud.tmpl")
	if err != nil {
		log.Fatalf("template parse error: %v", err)
	}

	fmtt := &srcedit.GofmtTransform{}
	var trs []srcedit.Transform

	{
		fn := "handlerutil.go"
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "HandlerUtil")
		if err != nil {
			log.Fatalf("tmplToTransforms for %q error: %v", fn, err)
		}
		trs = append(trs, trList...)
	}

	{
		// fn := filepath.Join(wdPackagePath, fileNamePart)
		fn := fileNamePart
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "Handler", "HandlerMethods")
		if err != nil {
			log.Fatalf("tmplToTransforms for %q error: %v", fn, err)
		}
		trs = append(trs, trList...)
	}

	{
		// fn := filepath.Join(wdPackagePath, strings.TrimSuffix(fileNamePart, ".go")+"_test.go")
		fn := strings.TrimSuffix(fileNamePart, ".go") + "_test.go"
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "TestHandler")
		if err != nil {
			log.Fatalf("tmplToTransforms for %q error: %v", fn, err)
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

	err = handlersPkg.ApplyTransforms(trs...)
	if err != nil {
		log.Fatalf("apply transform error: %v", err)
	}

	if *dryRunF {
		diffMap, err := diff.Run(inFS, outFS, ".", "term")
		if err != nil {
			log.Fatalf("error running diff: %v", err)
		}
		// if *jsonF {
		// 	enc := json.NewEncoder(os.Stdout)
		// 	enc.Encode(map[string]interface{}{
		// 		"diff": diffMap,
		// 	})
		// } else {
		klist := make([]string, 0, len(diffMap))
		for k := range diffMap {
			klist = append(klist, k)
		}
		sort.Strings(klist)
		for _, k := range klist {
			fmt.Printf("### %s\n", k)
			fmt.Println(diffMap[k])
		}
		// }
	}

	// ----

	// 	_ = storeTestFileF

	// 	typeName := *typeF
	// 	if typeName == "" {
	// 		log.Fatalf("-type is required")
	// 	}

	// 	typeFilename := *fileF
	// 	if typeFilename == "" {
	// 		typeFilename = srcedit.LowerForType(typeName+"Store", "-") + ".go"
	// 	}

	// 	if *testFileF == "" {
	// 		*testFileF = strings.TrimSuffix(typeFilename, ".go") + "_test.go"
	// 	}

	// 	rootFS, modDir, packagePath, modPath, err := srcedit.FindOSWdModuleDir(*packageF)
	// 	if err != nil {
	// 		log.Fatalf("error finding module directory: %v", err)
	// 	}
	// 	if *vF {
	// 		log.Printf("rootFS=%v; modDir=%q, packagePath=%q, modPath=%q", rootFS, modDir, packagePath, modPath)
	// 	}

	// 	migrationsPackagePath := *migrationsPackageF
	// 	if migrationsPackagePath == "" {
	// 		migrationsPackagePath = strings.TrimPrefix(path.Join(packagePath, "../migrations"), "/")
	// 	}

	// 	// set up file systems
	// 	inFS, err := fs.Sub(rootFS, modDir)
	// 	if err != nil {
	// 		log.Fatalf("fs.Sub error while construct input fs: %v", err)
	// 	}

	// 	// output is either same as input or memory for dry-run
	// 	var outFS fs.FS
	// 	var dryRunFS *memfs.FS
	// 	if *dryRunF == "off" {
	// 		outFS = inFS
	// 		if migrationsPackagePath != "" {
	// 			mda, ok := outFS.(srcedit.MkdirAller)
	// 			if ok {
	// 				err := mda.MkdirAll(migrationsPackagePath, 0755)
	// 				if err != nil {
	// 					log.Fatalf("mda.MkdirAll for %q: %v", migrationsPackagePath, err)
	// 				}
	// 			}
	// 			//outFS.MkdirAll(migrationsPackagePath, 0755)
	// 		}
	// 	} else {
	// 		dryRunFS = memfs.New()
	// 		if packagePath != "" {
	// 			dryRunFS.MkdirAll(packagePath, 0755)
	// 		}
	// 		if migrationsPackagePath != "" {
	// 			// log.Printf("creating migrationsPackagePath %q", migrationsPackagePath)
	// 			err := dryRunFS.MkdirAll(migrationsPackagePath, 0755)
	// 			// log.Printf("migrations MkdirAll returned: %v", err)
	// 			_ = err
	// 		}
	// 		outFS = dryRunFS
	// 	}

	// 	// load the package with srcedit
	// 	pkg := srcedit.NewPackage(inFS, outFS, modPath, packagePath)

	// 	// load the migrations package
	// 	log.Printf("NewPackage for migrations: inFS=%#v, outFS=%#v, modPath=%#v, migrationsPackagePath=%#v",
	// 		inFS, outFS, modPath, migrationsPackagePath)
	// 	migrationsPkg := srcedit.NewPackage(inFS, outFS, modPath, migrationsPackagePath)

	// 	// get the definition for the specified type
	// 	typeInfo, err := pkg.FindType(typeName)
	// 	if err != nil {
	// 		log.Fatalf("failed to find type %q: %v", typeName, err)
	// 	}

	// 	// populate Struct
	// 	// s := Struct{
	// 	// 	pkgImportedName: "", // TOOD: figure out what to do with prefixed types (not in the same package)
	// 	// 	name:            typeName,
	// 	// 	typeInfo:        typeInfo,
	// 	// }
	// 	// s.fields, err = s.makeFields()
	// 	// if err != nil {
	// 	// 	log.Fatalf("failed to extract field info from type %q: %v", typeName, err)
	// 	// }

	// 	s, err := model.NewStruct(typeInfo, "")
	// 	if err != nil {
	// 		log.Fatalf("failed to find type %q: %v", typeName, err)
	// 	}

	// 	// execute template
	// 	data := struct {
	// 		Struct               *model.Struct
	// 		MigrationsImportPath string
	// 	}{
	// 		Struct:               s,
	// 		MigrationsImportPath: modPath + "/" + migrationsPackagePath,
	// 	}
	// 	tmpl, err := template.New("_main_").Funcs(funcMap).ParseFS(defaultTmplFS, "sqlcrud.tmpl")
	// 	if err != nil {
	// 		log.Fatalf("template parse error: %v", err)
	// 	}

	// 	fmtt := &srcedit.GofmtTransform{}
	// 	var trs []srcedit.Transform

	// 	{
	// 		fn := "sqlutil.go"
	// 		fmtt.FilenameList = append(fmtt.FilenameList, fn)
	// 		trList, err := tmplToTransforms(fn, data, tmpl, "SQLUtil")
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		trs = append(trs, trList...)
	// 	}

	// 	{
	// 		fn := *storeFileF
	// 		fmtt.FilenameList = append(fmtt.FilenameList, fn)
	// 		trList, err := tmplToTransforms(fn, data, tmpl, "Store", "StoreMethods")
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		trs = append(trs, trList...)
	// 	}

	// 	{
	// 		fn := *storeTestFileF
	// 		fmtt.FilenameList = append(fmtt.FilenameList, fn)
	// 		trList, err := tmplToTransforms(fn, data, tmpl, "TestStore")
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		trs = append(trs, trList...)
	// 	}

	// 	{
	// 		fn := typeFilename
	// 		fmtt.FilenameList = append(fmtt.FilenameList, fn)
	// 		trList, err := tmplToTransforms(fn, data, tmpl,
	// 			"TYPEStore",
	// 			"TYPEStoreMethods",
	// 			// FIXME: filter which things go here base on flags
	// 			"TYPEInsert",
	// 			"TYPEDelete",
	// 			"TYPEUpdate",
	// 			"TYPESelectByID",
	// 			"TYPESelect",
	// 			"TYPESelectCursor",
	// 			"TYPECount",
	// 		)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		trs = append(trs, trList...)
	// 	}

	// 	{
	// 		// TODO: -no-test flag
	// 		fn := *testFileF
	// 		fmtt.FilenameList = append(fmtt.FilenameList, fn)
	// 		trList, err := tmplToTransforms(fn, data, tmpl,
	// 			"TestTYPE",
	// 		)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		trs = append(trs, trList...)
	// 	}

	// 	dd := &srcedit.DedupImportsTransform{
	// 		FilenameList: fmtt.FilenameList,
	// 	}
	// 	trs = append(trs, dd)

	// 	if !*noGofmtF {
	// 		trs = append(trs, fmtt)
	// 	}

	// 	err = pkg.ApplyTransforms(trs...)
	// 	if err != nil {
	// 		log.Fatalf("apply transform error: %v", err)
	// 	}

	// 	// TODO: option to skip migrations stuff?
	// 	// do migrations package separately
	// 	{
	// 		fmtt := &srcedit.GofmtTransform{}
	// 		var trs []srcedit.Transform

	// 		fn := "migrations.go"
	// 		fmtt.FilenameList = append(fmtt.FilenameList, fn)
	// 		trList, err := tmplToTransforms(fn, data, tmpl, "Migrations")
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		trs = append(trs, trList...)

	// 		dd := &srcedit.DedupImportsTransform{
	// 			FilenameList: fmtt.FilenameList,
	// 		}
	// 		trs = append(trs, dd)

	// 		if !*noGofmtF {
	// 			trs = append(trs, fmtt)
	// 		}

	// 		err = migrationsPkg.ApplyTransforms(trs...)
	// 		if err != nil {
	// 			log.Fatalf("apply transform for migrations error: %v", err)
	// 		}

	// 		// mpf, err := inFS.Open(migrationsPackagePath)

	// 		needSampleMigration := true
	// 		err = fs.WalkDir(inFS, migrationsPackagePath, fs.WalkDirFunc(func(path string, d fs.DirEntry, err error) error {
	// 			if err != nil {
	// 				return err
	// 			}
	// 			if path == migrationsPackagePath {
	// 				return nil
	// 			}
	// 			if d.IsDir() { // only scan the immediate directory
	// 				return fs.SkipDir
	// 			}
	// 			if strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".SQL") {
	// 				needSampleMigration = false
	// 			}
	// 			return nil
	// 		}))
	// 		if err != nil && !os.IsNotExist(err) {
	// 			log.Fatalf("error walking migrations dir %q: %v", migrationsPackagePath, err)
	// 		}

	// 		if needSampleMigration {
	// 			b := []byte(`
	// -- +goose Up

	// -- +goose Down

	// `)
	// 			fname := time.Now().UTC().Format("20060102150405") + "_sample.sql"
	// 			fpath := path.Join(migrationsPackagePath, fname)
	// 			err := outFS.(srcedit.FileWriter).WriteFile(fpath, b, 0644)
	// 			if err != nil {
	// 				log.Fatalf("error creating example migration file %q: %v", fpath, err)
	// 			}
	// 		}

	// 	}

	// 	if *dryRunF != "off" {
	// 		diffMap, err := diff.Run(inFS, outFS, ".", *dryRunF)
	// 		if err != nil {
	// 			log.Fatalf("error running diff: %v", err)
	// 		}
	// 		if *jsonF {
	// 			enc := json.NewEncoder(os.Stdout)
	// 			enc.Encode(map[string]interface{}{
	// 				"diff": diffMap,
	// 			})
	// 		} else {
	// 			klist := make([]string, 0, len(diffMap))
	// 			for k := range diffMap {
	// 				klist = append(klist, k)
	// 			}
	// 			sort.Strings(klist)
	// 			for _, k := range klist {
	// 				fmt.Printf("### %s\n", k)
	// 				fmt.Println(diffMap[k])
	// 			}
	// 		}
	// 	}

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
