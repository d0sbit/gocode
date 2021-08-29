# gocode Code Generator (wip)

Easily generate Go code following common patterns:

* SQL CRUD operations (`sqlcrud`)
* MongoDB CRUD operations (`mongocrud`)
* REST HTTP handlers (`resthttp`)

Generate code quickly and easily.  Customize the output to suite your project.

## Installation

TODO

put something here about making sure ~/go/bin is in your PATH, and instructions of how to fix

## Usage

### Primary Keys

While GoCode tries to infer as much information as possible without requiring explicit configuration,
the case of composite primary keys (more than one field acting as the unique identifier for a database record)
needs explicit configuration.  GoCode will identify primary keys (single fields or composite) using the following rules,
checked in sequence:

- If one or more fields have struct tags with `gocode:"pk"`, then those fields together form the composite primary key (or it is also okay if just one field tagged like this)
- If a field is named after the struct and followed by ID (e.g. type Xyz struct { XyzID string } ) it is chosen as the PK.
- If a field is named "ID" it is chosen as the PK.

Note that GoCode tries to avoid emitting field names where it can be avoided, for easier maintenance (instead reads them at runtime via reflect). But this may not be possible with primary keys, meaning if you change the primary key for a type you may need to regenerate or update methods emitted by GoCode by hand.

## How it Works

`gocode` operates by invoking a separate tool which performs analysis on existing Go code (usually a single package), and then uses one or more templates to generate the desired output.  The result is either written to a file, or merged into an existing file, according to the particular logic of the tool in question.

Each tool includes a set of built-in templates that it needs, and also supports reading template files from your project in order to accommodate project-specific tweaks.

<!--
## Notes

TODO:
* Get main test output building - DONE
  - debug gofmt DONE
  - add output for AStore if it doesn't exist DONE
  - add output for Store if it doesn't exists DONE
* Write import deduplicator (useful as it's own thing separate from the add import transform) DONE
* Make test case have a method that lights up a mongodb docker container and runs the whole thing
* Implement -dry-run
* Add remaining methods to get full CRUD
* Make a punchlist of what is left to round off mongdob
* Then move onto sql version (decide which library to use - sqlx is a decent choice)
  - with mysql docker test case

---

* we should add Vugu UI generation!

* multiple templates - so either when you install or just in general you can select from multiple sets of templates, e.g. the sqlcrud generator can be sqlx, dbr, etc.

* maybe there's a dryrun mode where the input can be the os filesystem but the output can be something in memory, and so
  allow us to create a full preview of the various changes

* gocode is the command
* gocode mongo-crud would invoke gocode-mongo-crud or similar
* the specific tool analyzes the code (usually a package) and performs some actions based on templates
* templates can be built-in or customized per project by putting template files in .gocode (should be a command to install them)
( gocode ui - should launch a browser and give command examples for each of the various things - could it produce a preview? that'd be really cool, also examples, also auto completion
* need to standardize on a help system and ui system that gocode can use to glean info from, or use json or something
* provide plugins and templates for: sqlstore crud, mongodb crud, http handler crud
* tests for templates would be really useful as well - it's very easy to mess up a template and then not know it until you have to generate your next thing.  Verifying that the result at least compiles would be useful
* interactive prompts might be nice, but decide if this is more useful than having a UI or even just decent documentation with lots of examples

Example command lines:

gocode mongodbcrud -struct Workspace -file workspace.go -package ./mstore -create -read -list -update -delete -all

gocode mongodbcrud -install-templates

-->
