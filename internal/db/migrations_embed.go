package db

import _ "embed"

//go:embed migrations/0001_init.sql
var initSchemaSQL string

func InitSchemaSQL() string {
	return initSchemaSQL
}
