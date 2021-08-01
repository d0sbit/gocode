package srcedit

import (
	"reflect"
	"testing"
)

func TestParseTransform(t *testing.T) {

	type tcase struct {
		tname   string
		snippet string
		result  []Transform
		errtxt  string
	}

	tcaseList := []tcase{

		{
			tname: "parse_err",
			snippet: `import "example123
`,
			errtxt: "parse_err.go:1:8: string literal not terminated", // ensure errors have the right line number
		},

		{
			tname: "imports",
			snippet: `
import "abc"
import "def"
`,
			result: []Transform{
				&ImportTransform{
					Filename: "imports.go",
					Name:     "",
					Path:     "abc",
				},
				&ImportTransform{
					Filename: "imports.go",
					Name:     "",
					Path:     "def",
				},
			},
		},

		{
			tname: "import_block",
			snippet: `
import (
	"abc"
	"def"
)
`,
			result: []Transform{
				&ImportTransform{
					Filename: "import_block.go",
					Name:     "",
					Path:     "abc",
				},
				&ImportTransform{
					Filename: "import_block.go",
					Name:     "",
					Path:     "def",
				},
			},
		},

		{
			tname: "import_single",
			snippet: `
import "abc"
`,
			result: []Transform{
				&ImportTransform{
					Filename: "import_single.go",
					Name:     "",
					Path:     "abc",
				},
			},
		},

		{
			tname: "import_local",
			snippet: `
import _ "abc"
`,
			result: []Transform{
				&ImportTransform{
					Filename: "import_local.go",
					Name:     "_",
					Path:     "abc",
				},
			},
		},

		{
			tname: "func_init",
			snippet: `
// init some stuff
func init() {
	println("initing")
}
`,
			result: []Transform{
				&AddFuncDeclTransform{
					Filename:     "func_init.go",
					Name:         "init",
					ReceiverType: "",
					Text: `// init some stuff
func init() {
	println("initing")
}`,
				},
			},
		},

		{
			tname: "func_recvval",
			snippet: `
// F does something
func (a A) F() {}
`,
			result: []Transform{
				&AddFuncDeclTransform{
					Filename:     "func_recvval.go",
					Name:         "F",
					ReceiverType: "A",
					Text: `// F does something
func (a A) F() {}`,
				},
			},
		},

		{
			tname: "func_recvptr",
			snippet: `
// F does something
func (a *A) F() {}
`,
			result: []Transform{
				&AddFuncDeclTransform{
					Filename:     "func_recvptr.go",
					Name:         "F",
					ReceiverType: "*A",
					Text: `// F does something
func (a *A) F() {}`,
				},
			},
		},

		{
			tname: "var_const_type",
			snippet: `
// some vars
var (
	x = 1
	y = 2
)

// w here
var w = 3

// some consts
const (
	z = 1
	a = 2
)

// a type
type t struct {
	a, b, c int
}
`,
			result: []Transform{
				&AddVarDeclTransform{
					Filename: "var_const_type.go",
					NameList: []string{"x", "y"},
					Text: `// some vars
var (
	x = 1
	y = 2
)`,
				},
				&AddVarDeclTransform{
					Filename: "var_const_type.go",
					NameList: []string{"w"},
					Text: `// w here
var w = 3`,
				},
				&AddConstDeclTransform{
					Filename: "var_const_type.go",
					NameList: []string{"z", "a"},
					Text: `// some consts
const (
	z = 1
	a = 2
)`,
				}, &AddTypeDeclTransform{
					Filename: "var_const_type.go",
					Name:     "t",
					Text: `// a type
type t struct {
	a, b, c int
}`,
				},
			},
		},
	}

	for _, tc := range tcaseList {
		tc := tc
		t.Run(tc.tname, func(t *testing.T) {
			tl, err := ParseTransforms(tc.tname+".go", tc.snippet)
			if err != nil && tc.errtxt == "" {
				t.Fatal(err)
			}
			if tc.result != nil {
				if !reflect.DeepEqual(tc.result, tl) {
					t.Errorf("unexpected result: %#v", tl)
					for i, tr := range tl {
						t.Errorf("ret[%d]: %#v", i, tr)
					}
				}
			} else if tc.errtxt != "" {
				if tc.errtxt != err.Error() {
					t.Errorf("unexpected error: expected=%q, got=%q", tc.errtxt, err.Error())
				}
			}
		})
	}

}
