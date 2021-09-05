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
	"sort"
	"strings"
	"text/template"

	"github.com/psanford/memfs"

	"github.com/d0sbit/gocode/srcedit"
)

//go:embed default.tmpl
var defaultTmplFS embed.FS

func main() {
	os.Exit(maine(
		flag.NewFlagSet(os.Args[0], flag.PanicOnError),
		os.Args[1:]))
}

// maine is broken out so it can be tested separately
func maine(flagSet *flag.FlagSet, args []string) int {

	// gocode mongocrud -type Workspace -file workspace.go -package ./mstore -create -read -update -delete -search -all

	typeF := flagSet.String("type", "", "Type name of Go struct with fields corresponding to the MongoDB document")
	fileF := flagSet.String("file", "", "Filename for the main type into which to add store code")
	testFileF := flagSet.String("test-file", "", "Test filename into which to add code, defaults to file with _test.go suffix")
	storeFileF := flagSet.String("store-file", "store.go", "Filename for the Store type")
	storeTestFileF := flagSet.String("store-test-file", "store_test.go", "Filename for the Store type tests")
	packageF := flagSet.String("package", "", "Package or directory to analyze/write to")
	dryRunF := flagSet.String("dry-run", "off", "Do not apply changes, only output diff of what would change. Value specifies format, 'term' for terminal pretty text, 'html' for HTML, or 'off' to disable.")
	noGofmtF := flagSet.Bool("no-gofmt", false, "Do not gofmt the output")
	jsonF := flagSet.Bool("json", false, "Write output as JSON")
	allF := flagSet.Bool("all", false, "Generate all methods")

	// TODO:
	// - -dry-run
	// - codeflag package with Usage() that dumps the flags as JSON
	// - example command lines

	flagSet.Parse(args)

	_, _, _, _ = typeF, fileF, packageF, allF

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

	// NOTES: review these later and compare to what we have!

	// flags
	// document the vital set somewhere, and maybe move to separate shared package

	// read in toml config

	// report any missing flags (or other errors) via JSON
	// what about optional flags?

	// values read from toml config should be reported but not as errors

	// example command lines?

	// analyze package glean structure

	// for values like the package of where the crud stuff goes, this stuff should probably be
	// written to the toml file so it doesn't have to be specified each time

	// read and execute templates (built-in, or from project or other dir)

	// write output files, and report on what happened via json

	// --------------------------------------

	// find go.mod
	rootFS, modDir, modPath, err := srcedit.FindOSWdModuleDir()
	if err != nil {
		log.Fatalf("error finding module directory: %v", err)
	}
	log.Printf("rootFS=%v; modDir=%v, modPath=%v", rootFS, modDir, modPath)

	// convert package into a path relative to go.mod
	packagePath := *packageF
	if strings.HasPrefix(packagePath, ".") {
		// FIXME: we should make this work - need to resolve the path and then
		// make it relative to module directory
		log.Fatalf("relative package path %q not supported (yet)", packagePath)
	}
	if packagePath == "" {
		log.Fatal("-package is required")
	}

	log.Printf("packagePath (subdir): %s", packagePath)

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
	} else {
		dryRunFS = memfs.New()
		// FIXME: Package.load() needs to create the directory in the output if it doesn't exist
		dryRunFS.MkdirAll("a", 0755)

		outFS = dryRunFS
	}

	// load the package with srcedit
	pkg := srcedit.NewPackage(inFS, outFS, modPath, packagePath)

	// get the definition for the specified type
	typeInfo, err := pkg.FindType(typeName)
	if err != nil {
		log.Fatalf("failed to find type %q: %v", typeName, err)
	}
	// log.Printf("typeInfo=%v", typeInfo)

	// populate Struct
	s := Struct{
		pkgImportedName: "", // TOOD: figure out what to do with prefixed types (not in the same package)
		name:            typeName,
		typeInfo:        typeInfo,
	}
	s.fields, err = s.makeFields()
	if err != nil {
		log.Fatalf("failed to extract field info from type %q: %v", typeName, err)
	}

	// execute template
	data := struct {
		Struct *Struct
	}{
		Struct: &s,
	}
	tmpl, err := template.New("_main_").ParseFS(defaultTmplFS, "default.tmpl")
	if err != nil {
		log.Fatalf("template parse error: %v", err)
	}

	fmtt := &srcedit.GofmtTransform{}
	var trs []srcedit.Transform

	{
		fn := "mongoutil.go"
		fmtt.FilenameList = append(fmtt.FilenameList, fn)
		trList, err := tmplToTransforms(fn, data, tmpl, "MongoUtil")
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
			// FIJXME: filter which things go here base on flags
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

	if *dryRunF != "off" {
		diffMap, err := runDiff(inFS, outFS, ".", *dryRunF)
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
