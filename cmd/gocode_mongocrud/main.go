package main

import "flag"

func main() {

	// gocode mongodbcrud -struct Workspace -file workspace.go -package ./mstore -create -read -list -update -delete -all

	structF := flag.String("struct", "", "Name of Go struct with fields corresponding to the MongoDB")
	fileF := flag.String("file", "", "Filename into which to add code")
	packageF := flag.String("package", "", "Package or directory to analyze/write to")
	allF := flag.Bool("all", false, "Generate all methods")

	flag.Parse()

	_, _, _, _ = structF, fileF, packageF, allF

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

	b, err := fs.ReadFile("example.tmpl")
	if err != nil {
		panic(err)
	}
	println(string(b))

}
