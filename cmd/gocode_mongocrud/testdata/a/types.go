package a

import "database/sql"

type T1 struct {
	T1ID              string `bson:"_id"`
	Name              string
	StrPtr            *string
	LocalType         SomeLocalType
	PrefixType        sql.NullString
	PrefixTypePtr     *sql.NullString
	PrefixTypePtrWTag *sql.NullString `sometag:"here"`
}

type SomeLocalType struct {
}
