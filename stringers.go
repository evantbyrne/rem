package rem

import (
	"fmt"
	"strings"
)

type SqlAs struct {
	Alias  string
	Column interface{}
}

func (as SqlAs) StringForDialect(dialect Dialect) string {
	switch cv := as.Column.(type) {
	case string:
		return fmt.Sprint(dialect.QuoteIdentifier(cv), " AS ", dialect.QuoteIdentifier(as.Alias))

	case DialectStringer:
		return fmt.Sprint(cv.StringForDialect(dialect), " AS ", dialect.QuoteIdentifier(as.Alias))

	case fmt.Stringer:
		return fmt.Sprint(cv.String(), " AS ", dialect.QuoteIdentifier(as.Alias))
	}

	panic(fmt.Sprintf("rem: unsupported type for rem.As '%#v'", as.Column))
}

func As(column interface{}, alias string) SqlAs {
	return SqlAs{Alias: alias, Column: column}
}

type SqlColumn string

func (column SqlColumn) StringForDialect(dialect Dialect) string {
	return dialect.QuoteIdentifier(string(column))
}

func Column(column string) SqlColumn {
	return SqlColumn(column)
}

type SqlParam struct {
	Value interface{}
}

func Param(value interface{}) SqlParam {
	return SqlParam{Value: value}
}

type SqlWithParams struct {
	Segments []interface{}
}

func (sqlWithParams SqlWithParams) StringWithArgs(dialect Dialect, args []interface{}) (string, []interface{}, error) {
	var queryString strings.Builder
	for _, part := range sqlWithParams.Segments {
		switch cv := part.(type) {
		case SqlParam:
			args = append(args, cv.Value)
			queryString.WriteString(dialect.Param(len(args)))
		case string:
			queryString.WriteString(cv)
		default:
			queryString.WriteString(fmt.Sprint(cv))
		}
	}
	return queryString.String(), args, nil
}

func Sql(segments ...interface{}) SqlWithParams {
	return SqlWithParams{Segments: segments}
}

type SqlUnsafe struct {
	Sql string
}

func (sqlUnsafe SqlUnsafe) String() string {
	return sqlUnsafe.Sql
}

func Unsafe(sql string) SqlUnsafe {
	return SqlUnsafe{Sql: sql}
}
