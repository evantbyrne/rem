package rem

import (
	"fmt"
	"reflect"
)

//lint:file-ignore U1000 Ignore report
type testDialect struct{}

func (dialect testDialect) BuildDelete(QueryConfig) (string, []interface{}, error) {
	panic("Not implemented")
}

func (dialect testDialect) BuildInsert(QueryConfig, map[string]interface{}, ...string) (string, []interface{}, error) {
	panic("Not implemented")
}

func (dialect testDialect) BuildSelect(config QueryConfig) (string, []interface{}, error) {
	return fmt.Sprintf("SELECT|FILTER%+v|", config.Filters), nil, nil
}

func (dialect testDialect) BuildTableColumnAdd(QueryConfig, string) (string, error) {
	panic("Not implemented")
}

func (dialect testDialect) BuildTableColumnDrop(QueryConfig, string) (string, error) {
	panic("Not implemented")
}

func (dialect testDialect) BuildTableCreate(QueryConfig, TableCreateConfig) (string, error) {
	panic("Not implemented")
}

func (dialect testDialect) BuildTableDrop(QueryConfig, TableDropConfig) (string, error) {
	panic("Not implemented")
}

func (dialect testDialect) BuildUpdate(QueryConfig, map[string]interface{}, ...string) (string, []interface{}, error) {
	panic("Not implemented")
}

func (dialect testDialect) ColumnType(reflect.StructField) (string, error) {
	panic("Not implemented")
}

func (dialect testDialect) Param(identifier int) string {
	return fmt.Sprintf("$%d", identifier)
}

func (dialect testDialect) QuoteIdentifier(identifier string) string {
	return fmt.Sprintf(`"%s"`, identifier)
}
