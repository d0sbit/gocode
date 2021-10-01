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
* add to a list somewhere:
  - punchlist
    - try a few command line commands and make sure the basic stuff works
  - implement helpers in mongocrud (maybe move to backlog)
  	- idAssign
	  - createTimeTouch
	  - updateTimeTouch
	  - storeValidate
  - mongo Count() needs sort also just like SQL so it can determine the index (move to backlog)
  - backlog: break up the tests so they track with which methods are included, and add the options too (-create, -read, etc.)
* Handlers
  - get it building
  - Fill out the other CRUD methods
  - on testing decide if we are using an interface with stub stuff, or if we are doing the whole docker test stuff or what: maybe we need the equivalent of the "make me a test store" function that can be used by other test packages
  - NOTES:
  - see if we can express permissions with a super simple interface abstraction, e.g. CanRead(interface{}) bool, etc.
    it should be optional, but could let us have perms from the get-go without
  - both PUT and PATCH support
  - querying should default to "normal" way but have a few lines of commented code to switch to cursor
  - we can probably incorporate the key aspects of werr as helper methods - probably too simple to introduce a dependency
    - probably we should support the wrapped return value approach but also a simple helper method or two for outputting
      errors with a public message (since the controller usually handles that anyway), this way the only interface thing
      we need is the HTTP status code
    - or maybe not even bother with the wrapped error approach, as long as the helper methods are clear and simple
    - decide what to do with the other options: ID, location info
    - longer version, still good: if err != nil { w.WriteStatus(statusCode(err)); w.Write(logErr(err)); return }
    - maybe a bit more compact: if err != nil { writeErrf(w, 0, err, "something went wrong: %d", n) }
    - should there also be a writeErr(w, 0, err), what about writeErr(w, 0, err, "public message")
    - 0 means extract status from err or 500
    - writeErr can itself have the file:line and ID stuff in there, maybe file:line commented out by default
    - maybe we don't need wrap function at all
    - writeErrf(w http.ResponseWriter, status int, err error, responseFormat string, args ...interface{})
      - if status is 0 detect from err or 500
      - if err is nil then don't log
      - if responseFormat is "" then don't write to output
* Implement "gocode"
  - See if we can obviate the need for the UI entirely by making the commands just target the file name, and use the folders (as well as settings) to infer which tools and the various settings.
  - If we need to have some interactive stuff in there too that could be okay also (Did you mean X as the folder for ABC? [Y/n])
* Decide what we want to do about main program, need at least something for that
* Implement custom template support - ideally an option would write the default template (files) to a well-known location and it could be edited frmo there.
* Add to backlog:
  - HTTPStatusCode should probably go away and instead just check for things like sql.ErrNoRows, etc. in writeErr
  - fix -dry-run on all programs so -dry-run doesn't accept args, use -dry-run-html if neeeded
* UI (if still needed)
  - common flags approach so we can communicate to the UI what each program needs
  - diff'ed (dry-run) output
* Clean up main README and make some decent exampels of how to use
* Anything we can do about API doc?  Maybe something to generate what Swagger needs?

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
