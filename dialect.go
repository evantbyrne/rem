package rem

import (
	"reflect"
)

type Dialect interface {
	BuildDelete(QueryConfig) (string, []interface{}, error)
	BuildInsert(QueryConfig, map[string]interface{}, ...string) (string, []interface{}, error)
	BuildSelect(QueryConfig) (string, []interface{}, error)
	BuildTableColumnAdd(QueryConfig, string) (string, error)
	BuildTableColumnDrop(QueryConfig, string) (string, error)
	BuildTableCreate(QueryConfig, TableCreateConfig) (string, error)
	BuildTableDrop(QueryConfig, TableDropConfig) (string, error)
	BuildUpdate(QueryConfig, map[string]interface{}, ...string) (string, []interface{}, error)
	ColumnType(reflect.StructField) (string, error)
	Param(i int) string
	QuoteIdentifier(string) string
}

type DialectStringer interface {
	StringForDialect(Dialect) string
}

type DialectStringerWithArgs interface {
	StringWithArgs(Dialect, []interface{}) (string, []interface{}, error)
}

var defaultDialect Dialect

func SetDialect(dialect Dialect) {
	defaultDialect = dialect
}
