package main

// NOTE: Struct moved to srcedit/model.Struct

// // // DocType represents the Go type that corresponds to our MongoDB document.
// // type DocType struct {
// // }

// // Struct is used in templating to provide info about the type that corresponds
// // to the database table/collection.
// type Struct struct {
// 	pkgImportedName string
// 	name            string

// 	fields StructFieldList

// 	typeInfo *srcedit.TypeInfo
// }

// // StructField represents a field on a Struct.
// type StructField struct {
// 	s              *Struct  // parent Struct
// 	name           string   // name of field, e.g. "ID"
// 	typeExpr       string   // Go expression for the type e.g. "string", or "*int"
// 	gocodeTagParts []string // contents of the gocode:"" tag
// 	bsonTagParts   []string // contents of the bson:"" tag (for mongodb)
// 	isPK           bool     // is this field a primary key, based on pk detection logic

// 	astField *ast.Field
// }

// func (sf *StructField) GoName() string {
// 	return sf.name
// }

// func (sf *StructField) GoTypeExpr() string {
// 	return sf.typeExpr
// }

// func (sf *StructField) BSONName() (n string) {
// 	if len(sf.bsonTagParts) > 0 {
// 		n = sf.bsonTagParts[0]
// 	}
// 	if n == "" {
// 		n = sf.name
// 	}
// 	return n
// }

// type StructFieldList []StructField

// // WithGoName returns the field with the specified Go type name or nil of not found.
// func (l StructFieldList) WithGoName(n string) *StructField {
// 	for i := 0; i < len(l); i++ {
// 		if l[i].name == n {
// 			return &l[i]
// 		}
// 	}
// 	return nil
// }

// // PK returns a filtered field list of just the primary key field(s).
// func (l StructFieldList) PK() (ret StructFieldList) {
// 	for _, f := range l {
// 		if f.isPK {
// 			ret = append(ret, f)
// 		}
// 	}
// 	return ret
// }

// // func (s *Struct) GoName() string {
// // 	return s.name
// // }

// // FIXME: decide on naming convention - do we put Go in front of a bunch of these to distinguish from "bson" or somethign else?

// // QName with any qualifying package prefix.  E.g. either "X" for types in the same
// // package or "pkg.X" for type "X" in "pkg" package.
// func (s *Struct) QName() string {
// 	if s.pkgImportedName != "" {
// 		return s.pkgImportedName + "." + s.name
// 	}
// 	return s.name
// }

// func (s *Struct) FieldList() StructFieldList {
// 	return s.fields
// }

// // LocalName is the type name without any package prefix.
// func (s *Struct) LocalName() string {
// 	return s.name
// }

// func (s *Struct) makeFields() (ret StructFieldList, err error) {

// 	specs := s.typeInfo.GenDecl.Specs
// 	if l := len(specs); l != 1 {
// 		return nil, fmt.Errorf("len(Specs) is %d instead of 1", l)
// 	}

// 	spec := specs[0]
// 	typeSpec, ok := spec.(*ast.TypeSpec)
// 	if !ok {
// 		return nil, fmt.Errorf("failed to cast spec (%t) to *ast.TypeSpec", spec)
// 	}

// 	t := typeSpec.Type
// 	structType, ok := t.(*ast.StructType)
// 	if !ok {
// 		return nil, fmt.Errorf("typeSpec.Type is %t, not a struct", t)
// 	}

// 	fieldSlice := structType.Fields.List

// 	foundPk := false

// fieldLoop:
// 	for i := 0; i < len(fieldSlice); i++ {

// 		field := fieldSlice[i]

// 		sf := StructField{
// 			s:        s,
// 			astField: field,
// 		}

// 		lenfn := len(field.Names)
// 		switch lenfn {
// 		case 0:
// 			continue fieldLoop
// 		case 1:
// 			sf.name = field.Names[0].Name
// 		default:
// 			return nil, fmt.Errorf("field at index %d had len(field.Names) == %d instead of 1", i, lenfn)
// 		}

// 		sf.typeExpr = string(s.typeInfo.NodeSrc(field.Type))

// 		if field.Tag != nil {

// 			// examine the struct tag for an explicit primary key
// 			tagBody := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("gocode")
// 			if tagBody != "" {
// 				sf.gocodeTagParts = strings.Split(tagBody, ",")
// 				if len(sf.gocodeTagParts) > 0 {
// 					if sf.gocodeTagParts[0] == "pk" {
// 						sf.isPK = true
// 						foundPk = true
// 					}
// 				}
// 			}
// 			// extract out the bson tag parts while we're here
// 			tagBody = reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("bson")
// 			if tagBody != "" {
// 				sf.bsonTagParts = strings.Split(tagBody, ",")
// 			}
// 		}

// 		ret = append(ret, sf)
// 	}

// 	// if no pk found above, then apply the ID naming rules
// 	if !foundPk {

// 		tidField := ret.WithGoName(s.name + "ID")
// 		idField := ret.WithGoName("ID")
// 		if tidField != nil {
// 			tidField.isPK = true
// 		} else if idField != nil {
// 			idField.isPK = true
// 		}

// 	}

// 	// verify that primary keys look good or error if not
// 	pkFields := ret.PK()
// 	if len(pkFields) == 0 {
// 		return nil, fmt.Errorf("no primary key fields found for type %q", s.name)
// 	}
// 	for _, pkf := range pkFields {
// 		if len(pkf.bsonTagParts) < 1 || pkf.bsonTagParts[0] == "" {
// 			return nil, fmt.Errorf("primary key field %q on type %q does not have a `bson` struct tag (or the name is empty)", pkf.name, s.name)
// 		}
// 	}

// 	return ret, nil
// }

// // // FIXME: we're going to need more metadata about these fields than just some text output,
// // // consider how we'll do the loop in SelectByID... and how we would emit calls to this
// // // from a controller, etc.
// // // NOTE: helpers for snake and camelCase, etc. should probably go in srcedit
// // // PKAsLocalVarList returns a variable list that corresponds to the primary keys of a type.
// // func (s *Struct) PKAsLocalVarList() string {
// // 	s.pkFieldList()
// // 	return ""
// // 	// panic("TODO")
// // }

// // func (s *Struct) pkFieldList() ([]*ast.Field, error) {

// // 	ast.Print(s.typeInfo.FileSet, s.typeInfo.GenDecl)

// // 	spec := s.typeInfo.GenDecl.Specs[0]
// // 	typeSpec := spec.(*ast.TypeSpec)
// // 	t := typeSpec.Type
// // 	structType := t.(*ast.StructType)

// // 	fieldSlice := structType.Fields.List

// // 	for i := 0; i < len(fieldSlice); i++ {

// // 		field := fieldSlice[i]
// // 		_ = field.Names[0].Name
// // 		//_ = fieldSlice[2].Type.

// // 		fieldType := field.Type
// // 		_ = fieldType

// // 		fmt.Printf("ft: %q\n", s.typeInfo.NodeSrc(fieldType))
// // 	}

// // 	// "string" is *ast.Ident
// // 	// "*string" is *ast.StarExpr with X as the *ast.Ident
// // 	//fieldType.Pos()

// // 	return nil, nil
// // }
