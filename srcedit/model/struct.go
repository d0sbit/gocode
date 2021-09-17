package model

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/d0sbit/gocode/srcedit"
)

// NewStruct returns a Struct given srcedit.TypeInfo.  The TypeInfo must describe
// a struct or problems will ensue.  pkgImportedName is the name of the imported
// package to prefix the qualified type with (see QName).  It may be empty to indicate
// that references are in the local package.
func NewStruct(ti *srcedit.TypeInfo, pkgImportedName string) (*Struct, error) {
	typeSpec, ok := ti.GenDecl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return nil, fmt.Errorf("no TypeSpec found")
	}
	s := Struct{
		pkgImportedName: pkgImportedName,
		typeInfo:        ti,
		name:            typeSpec.Name.Name,
	}
	var err error
	s.fields, err = s.makeFields()
	return &s, err
}

// Struct is used in templating to provide info about the type that corresponds
// to the database table/collection.
type Struct struct {
	pkgImportedName string
	name            string

	fields StructFieldList

	typeInfo *srcedit.TypeInfo
}

// StructField represents a field on a Struct.
type StructField struct {
	s        *Struct             // parent Struct
	name     string              // name of field, e.g. "ID"
	typeExpr string              // Go expression for the type e.g. "string", or "*int"
	tagParts map[string][]string // key is struct tag key name, parts is value split by commas
	// gocodeTagParts []string // contents of the gocode:"" tag
	// bsonTagParts   []string // contents of the bson:"" tag (for mongodb)
	isPK bool // is this field a primary key, based on pk detection logic

	astField *ast.Field
}

func (sf *StructField) GoName() string {
	return sf.name
}

func (sf *StructField) GoTypeExpr() string {
	return sf.typeExpr
}

// TagFirst returns the first thing before a comment in the specified struct tag section.
// E.g. for struct tag `json:"a,omitempty"`, TagFirst("json") returns "a".  An empty string
// is returned if not found.
func (sf *StructField) TagFirst(tagName string) (n string) {
	v := sf.tagParts[tagName]
	if len(v) > 0 {
		n = v[0]
	}
	// if n == "" {
	// 	n = sf.name
	// }
	return n
}

// func (sf *StructField) BSONName() (n string) {
// 	if len(sf.bsonTagParts) > 0 {
// 		n = sf.bsonTagParts[0]
// 	}
// 	if n == "" {
// 		n = sf.name
// 	}
// 	return n
// }

type StructFieldList []StructField

// WithGoName returns the field with the specified Go type name or nil of not found.
func (l StructFieldList) WithGoName(n string) *StructField {
	for i := 0; i < len(l); i++ {
		if l[i].name == n {
			return &l[i]
		}
	}
	return nil
}

// PK returns a filtered field list of just the primary key field(s).
func (l StructFieldList) PK() (ret StructFieldList) {
	for _, f := range l {
		if f.isPK {
			ret = append(ret, f)
		}
	}
	return ret
}

// FIXME: decide on naming convention - do we put Go in front of a bunch of these to distinguish from "bson" or somethign else?

// QName with any qualifying package prefix.  E.g. either "X" for types in the same
// package or "pkg.X" for type "X" in "pkg" package.
func (s *Struct) QName() string {
	if s.pkgImportedName != "" {
		return s.pkgImportedName + "." + s.name
	}
	return s.name
}

func (s *Struct) FieldList() StructFieldList {
	return s.fields
}

// IsPKAutoIncr returns true if the primary key has auto-increment properties.
// For now, this returns true if there is a single PK field of type int64.
// A way to more explicitly specify this may be added with a struct tag later.
func (s *Struct) IsPKAutoIncr() bool {

	pks := s.FieldList().PK()
	if len(pks) != 1 {
		return false
	}

	if pks[0].GoTypeExpr() != "int64" {
		return false
	}

	return true
}

// LocalName is the type name without any package prefix.
func (s *Struct) LocalName() string {
	return s.name
}

func (s *Struct) makeFields() (ret StructFieldList, err error) {

	specs := s.typeInfo.GenDecl.Specs
	if l := len(specs); l != 1 {
		return nil, fmt.Errorf("len(Specs) is %d instead of 1", l)
	}

	spec := specs[0]
	typeSpec, ok := spec.(*ast.TypeSpec)
	if !ok {
		return nil, fmt.Errorf("failed to cast spec (%t) to *ast.TypeSpec", spec)
	}

	t := typeSpec.Type
	structType, ok := t.(*ast.StructType)
	if !ok {
		return nil, fmt.Errorf("typeSpec.Type is %t, not a struct", t)
	}

	fieldSlice := structType.Fields.List

	foundPk := false

fieldLoop:
	for i := 0; i < len(fieldSlice); i++ {

		field := fieldSlice[i]

		sf := StructField{
			s:        s,
			astField: field,
		}

		lenfn := len(field.Names)
		switch lenfn {
		case 0:
			continue fieldLoop
		case 1:
			sf.name = field.Names[0].Name
		default:
			return nil, fmt.Errorf("field at index %d had len(field.Names) == %d instead of 1", i, lenfn)
		}

		sf.typeExpr = string(s.typeInfo.NodeSrc(field.Type))

		if field.Tag != nil {

			stag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

			var tNames []string

			sections := strings.Split(string(stag), " ")
			for _, section := range sections {
				p := strings.SplitN(section, ":", 2)
				tNames = append(tNames, p[0])
			}

			if len(tNames) > 0 {
				sf.tagParts = make(map[string][]string, len(tNames))
				for _, tn := range tNames {
					parts := strings.Split(stag.Get(tn), ",")
					sf.tagParts[tn] = parts

					// check for pk while we're here
					if tn == "gocode" && parts[0] == "pk" {
						sf.isPK = true
						foundPk = true
					}

				}
			}

			// // examine the struct tag for an explicit primary key
			// tagBody := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("gocode")
			// if tagBody != "" {
			// 	sf.gocodeTagParts = strings.Split(tagBody, ",")
			// 	if len(sf.gocodeTagParts) > 0 {
			// 		if sf.gocodeTagParts[0] == "pk" {
			// 			sf.isPK = true
			// 			foundPk = true
			// 		}
			// 	}
			// }
			// // extract out the bson tag parts while we're here
			// tagBody = reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("bson")
			// if tagBody != "" {
			// 	sf.bsonTagParts = strings.Split(tagBody, ",")
			// }
		}

		ret = append(ret, sf)
	}

	// if no pk found above, then apply the ID naming rules
	if !foundPk {

		tidField := ret.WithGoName(s.name + "ID")
		idField := ret.WithGoName("ID")
		if tidField != nil {
			tidField.isPK = true
		} else if idField != nil {
			idField.isPK = true
		}

	}

	// verify that primary keys look good or error if not
	pkFields := ret.PK()
	if len(pkFields) == 0 {
		return nil, fmt.Errorf("no primary key fields found for type %q", s.name)
	}
	// for _, pkf := range pkFields {
	// 	if pkf.TagFirst("bson") == "" {
	// 		// if len(pkf.bsonTagParts) < 1 || pkf.bsonTagParts[0] == "" {
	// 		return nil, fmt.Errorf("primary key field %q on type %q does not have a `bson` struct tag (or the name is empty)", pkf.name, s.name)
	// 	}
	// }

	return ret, nil
}
