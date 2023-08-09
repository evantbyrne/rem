package pqdialect

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/evantbyrne/rem"
	"golang.org/x/exp/maps"
)

type PqDialect struct{}

func (dialect PqDialect) BuildDelete(config rem.QueryConfig) (string, []interface{}, error) {
	args := append([]interface{}(nil), config.Params...)
	var queryString strings.Builder
	queryString.WriteString("DELETE FROM ")
	queryString.WriteString(dialect.QuoteIdentifier(config.Table))

	// WHERE
	where, args, err := dialect.buildWhere(config, args)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		queryString.WriteString(where)
	}

	// ORDER BY
	if len(config.Sort) > 0 {
		return "", nil, fmt.Errorf("rem: DELETE does not support ORDER BY")
	}

	// LIMIT
	if config.Limit != nil {
		return "", nil, fmt.Errorf("rem: DELETE does not support LIMIT")
	}

	// OFFSET
	if config.Offset != nil {
		return "", nil, fmt.Errorf("rem: DELETE does not support OFFSET")
	}

	return queryString.String(), args, nil
}

func (dialect PqDialect) BuildInsert(config rem.QueryConfig, rowMap map[string]interface{}, columns ...string) (string, []interface{}, error) {
	args := make([]interface{}, 0)
	var queryString strings.Builder

	queryString.WriteString("INSERT INTO ")
	queryString.WriteString(dialect.QuoteIdentifier(config.Table))
	queryString.WriteString(" (")
	first := true
	for _, column := range columns {
		if arg, ok := rowMap[column]; ok {
			if _, ok := config.Fields[column]; !ok {
				return "", nil, fmt.Errorf("rem: field for column '%s' not found on model for table '%s'", column, config.Table)
			}
			args = append(args, arg)
			if first {
				first = false
			} else {
				queryString.WriteString(",")
			}
			queryString.WriteString(dialect.QuoteIdentifier(column))
		} else {
			return "", nil, fmt.Errorf("rem: invalid column '%s' on INSERT", column)
		}
	}

	queryString.WriteString(") VALUES (")
	for i := 1; i <= len(rowMap); i++ {
		if i > 1 {
			queryString.WriteString(",")
		}
		queryString.WriteString(dialect.Param(i))
	}
	queryString.WriteString(")")

	return queryString.String(), args, nil
}

func (dialect PqDialect) buildJoins(config rem.QueryConfig, args []interface{}) (string, []interface{}, error) {
	var queryPart strings.Builder
	if len(config.Joins) > 0 {
		for _, join := range config.Joins {
			if len(join.On) > 0 {
				queryPart.WriteString(fmt.Sprintf(" %s JOIN %s ON", join.Direction, dialect.QuoteIdentifier(join.Table)))
				for _, where := range join.On {
					queryWhere, whereArgs, err := where.StringWithArgs(dialect, args)
					if err != nil {
						return "", nil, err
					}
					args = whereArgs
					queryPart.WriteString(queryWhere)
				}
			}
		}
	}
	return queryPart.String(), args, nil
}

func (dialect PqDialect) BuildSelect(config rem.QueryConfig) (string, []interface{}, error) {
	args := append([]interface{}(nil), config.Params...)
	var queryString strings.Builder
	if config.Count {
		queryString.WriteString("SELECT count(*) FROM ")
	} else if len(config.Selected) > 0 {
		queryString.WriteString("SELECT ")
		for i, column := range config.Selected {
			if i > 0 {
				queryString.WriteString(",")
			}
			switch cv := column.(type) {
			case string:
				queryString.WriteString(dialect.QuoteIdentifier(cv))

			case rem.DialectStringer:
				queryString.WriteString(cv.StringForDialect(dialect))

			case fmt.Stringer:
				queryString.WriteString(cv.String())

			default:
				return "", nil, fmt.Errorf("rem: invalid column type %#v", column)
			}
		}
		queryString.WriteString(" FROM ")
	} else {
		queryString.WriteString("SELECT * FROM ")
	}
	queryString.WriteString(dialect.QuoteIdentifier(config.Table))

	// JOIN
	joins, args, err := dialect.buildJoins(config, args)
	if err != nil {
		return "", nil, err
	}
	if joins != "" {
		queryString.WriteString(joins)
	}

	// WHERE
	where, args, err := dialect.buildWhere(config, args)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		queryString.WriteString(where)
	}

	// ORDER BY
	if len(config.Sort) > 0 {
		queryString.WriteString(" ORDER BY ")
		for i, column := range config.Sort {
			if i > 0 {
				queryString.WriteString(", ")
			}
			if strings.HasPrefix(column, "-") {
				queryString.WriteString(dialect.QuoteIdentifier(column[1:]))
				queryString.WriteString(" DESC")
			} else {
				queryString.WriteString(dialect.QuoteIdentifier(column))
				queryString.WriteString(" ASC")
			}
		}
	}

	// LIMIT
	if config.Limit != nil {
		args = append(args, config.Limit)
		queryString.WriteString(" LIMIT ")
		queryString.WriteString(dialect.Param(len(args)))
	}

	// OFFSET
	if config.Offset != nil {
		args = append(args, config.Offset)
		queryString.WriteString(" OFFSET ")
		queryString.WriteString(dialect.Param(len(args)))
	}

	return queryString.String(), args, nil
}

func (dialect PqDialect) BuildTableColumnAdd(config rem.QueryConfig, column string) (string, error) {
	field, ok := config.Fields[column]
	if !ok {
		return "", fmt.Errorf("rem: invalid column '%s' on model for table '%s'", column, config.Table)
	}

	columnType, err := dialect.ColumnType(field)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", dialect.QuoteIdentifier(config.Table), dialect.QuoteIdentifier(column), columnType), nil
}

func (dialect PqDialect) BuildTableColumnDrop(config rem.QueryConfig, column string) (string, error) {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", dialect.QuoteIdentifier(config.Table), dialect.QuoteIdentifier(column)), nil
}

func (dialect PqDialect) BuildTableCreate(config rem.QueryConfig, tableCreateConfig rem.TableCreateConfig) (string, error) {
	var sql strings.Builder
	sql.WriteString("CREATE TABLE ")
	if tableCreateConfig.IfNotExists {
		sql.WriteString("IF NOT EXISTS ")
	}
	sql.WriteString(dialect.QuoteIdentifier(config.Table))
	sql.WriteString(" (")
	fieldNames := maps.Keys(config.Fields)
	sort.Strings(fieldNames)
	for i, fieldName := range fieldNames {
		field := config.Fields[fieldName]
		columnType, err := dialect.ColumnType(field)
		if err != nil {
			return "", err
		}
		if i > 0 {
			sql.WriteString(",")
		}
		sql.WriteString("\n\t")
		sql.WriteString(dialect.QuoteIdentifier(fieldName))
		sql.WriteString(" ")
		sql.WriteString(columnType)
	}
	sql.WriteString("\n)")
	return sql.String(), nil
}

func (dialect PqDialect) BuildTableDrop(config rem.QueryConfig, tableDropConfig rem.TableDropConfig) (string, error) {
	var queryString strings.Builder
	queryString.WriteString("DROP TABLE ")
	if tableDropConfig.IfExists {
		queryString.WriteString("IF EXISTS ")
	}
	queryString.WriteString(dialect.QuoteIdentifier(config.Table))
	return queryString.String(), nil
}

func (dialect PqDialect) BuildUpdate(config rem.QueryConfig, rowMap map[string]interface{}, columns ...string) (string, []interface{}, error) {
	args := append([]interface{}(nil), config.Params...)
	var queryString strings.Builder

	queryString.WriteString("UPDATE ")
	queryString.WriteString(dialect.QuoteIdentifier(config.Table))
	queryString.WriteString(" SET ")

	first := true
	for _, column := range columns {
		if arg, ok := rowMap[column]; ok {
			args = append(args, arg)
			if first {
				first = false
			} else {
				queryString.WriteString(",")
			}
			queryString.WriteString(dialect.QuoteIdentifier(column))
			queryString.WriteString(" = ")
			queryString.WriteString(dialect.Param(len(args)))
		} else {
			return "", nil, fmt.Errorf("rem: invalid column '%s' on UPDATE", column)
		}
	}

	// WHERE
	where, args, err := dialect.buildWhere(config, args)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		queryString.WriteString(where)
	}

	return queryString.String(), args, nil
}

func (dialect PqDialect) buildWhere(config rem.QueryConfig, args []interface{}) (string, []interface{}, error) {
	var queryPart strings.Builder
	if len(config.Filters) > 0 {
		queryPart.WriteString(" WHERE")
		for _, where := range config.Filters {
			queryWhere, whereArgs, err := where.StringWithArgs(dialect, args)
			if err != nil {
				return "", nil, err
			}
			args = whereArgs
			queryPart.WriteString(queryWhere)
		}
	}
	return queryPart.String(), args, nil
}

func (dialect PqDialect) ColumnType(field reflect.StructField) (string, error) {
	tagType := field.Tag.Get("db_type")
	if tagType != "" {
		return tagType, nil
	}

	fieldInstance := reflect.Indirect(reflect.New(field.Type)).Interface()
	var columnNull string
	var columnPrimary string
	var columnType string

	if field.Tag.Get("primary_key") == "true" {
		columnPrimary = " PRIMARY KEY"

		switch fieldInstance.(type) {
		case int, int64:
			columnNull = " NOT NULL"
			columnType = "BIGSERIAL"

		case int32:
			columnNull = " NOT NULL"
			columnType = "SERIAL"

		case int8, int16:
			columnNull = " NOT NULL"
			columnType = "SMALLSERIAL"
		}
	}

	if columnType == "" {
		switch fieldInstance.(type) {
		case bool:
			columnNull = " NOT NULL"
			columnType = "BOOLEAN"

		case sql.NullBool:
			columnNull = " NULL"
			columnType = "BOOLEAN"

		case float64:
			columnNull = " NOT NULL"
			columnType = "DOUBLE PRECISION"

		case sql.NullFloat64:
			columnNull = " NULL"
			columnType = "DOUBLE PRECISION"

		case int, int64:
			columnNull = " NOT NULL"
			columnType = "BIGINT"

		case sql.NullInt64:
			columnNull = " NULL"
			columnType = "BIGINT"

		case int32:
			columnNull = " NOT NULL"
			columnType = "INTEGER"

		case sql.NullInt32:
			columnNull = " NULL"
			columnType = "INTEGER"

		case int8, int16:
			columnNull = " NOT NULL"
			columnType = "SMALLINT"

		case sql.NullInt16:
			columnNull = " NULL"
			columnType = "SMALLINT"

		case string:
			columnNull = " NOT NULL"
			if tagMaxLength := field.Tag.Get("db_max_length"); tagMaxLength != "" {
				columnType = fmt.Sprintf("VARCHAR(%s)", tagMaxLength)
			} else {
				columnType = "TEXT"
			}

		case sql.NullString:
			columnNull = " NULL"
			if tagMaxLength := field.Tag.Get("db_max_length"); tagMaxLength != "" {
				columnType = fmt.Sprintf("VARCHAR(%s)", tagMaxLength)
			} else {
				columnType = "TEXT"
			}

		case time.Time:
			columnNull = " NOT NULL"
			if tagTimeZone := field.Tag.Get("db_time_zone"); tagTimeZone == "true" {
				columnType = "TIMESTAMP WITH TIME ZONE"
			} else {
				columnType = "TIMESTAMP WITHOUT TIME ZONE"
			}

		case sql.NullTime:
			columnNull = " NULL"
			if tagTimeZone := field.Tag.Get("db_time_zone"); tagTimeZone == "true" {
				columnType = "TIMESTAMP WITH TIME ZONE"
			} else {
				columnType = "TIMESTAMP WITHOUT TIME ZONE"
			}

		default:
			if strings.HasPrefix(field.Type.String(), "rem.ForeignKey[") || strings.HasPrefix(field.Type.String(), "rem.NullForeignKey[") {
				// Foreign keys.
				fv := reflect.New(field.Type).Elem()
				subModelQ := fv.Addr().MethodByName("Model").Call(nil)
				subFields := reflect.Indirect(subModelQ[0]).FieldByName("Fields").Interface().(map[string]reflect.StructField)
				subPrimaryColumn := reflect.Indirect(subModelQ[0]).FieldByName("PrimaryColumn").Interface().(string)
				subTable := reflect.Indirect(subModelQ[0]).FieldByName("Table").Interface().(string)
				columnTypeTemp, err := dialect.ColumnType(subFields[subPrimaryColumn])
				if err != nil {
					return "", err
				}
				columnType = strings.SplitN(columnTypeTemp, " ", 2)[0]
				columnType = strings.Replace(columnType, "BIGSERIAL", "BIGINT", 1)
				columnType = strings.Replace(columnType, "SMALLSERIAL", "SMALLINT", 1)
				columnType = strings.Replace(columnType, "SERIAL", "INTEGER", 1)

				columnNull = " NOT NULL"
				if strings.HasPrefix(field.Type.String(), "rem.NullForeignKey[") {
					columnNull = " NULL"
				}
				columnNull = fmt.Sprintf("%s REFERENCES %s (%s)", columnNull, dialect.QuoteIdentifier(subTable), dialect.QuoteIdentifier(subPrimaryColumn))

				if tagOnUpdate := field.Tag.Get("db_on_update"); tagOnUpdate != "" {
					// ON UPDATE.
					columnNull = fmt.Sprint(columnNull, " ON UPDATE ", tagOnUpdate)
				}

				if tagOnDelete := field.Tag.Get("db_on_delete"); tagOnDelete != "" {
					// ON DELETE.
					columnNull = fmt.Sprint(columnNull, " ON DELETE ", tagOnDelete)
				}
			}
		}
	}

	if columnType == "" {
		return "", fmt.Errorf("rem: Unsupported column type: %T. Use the 'db_type' field tag to define a SQL type", fieldInstance)
	}

	if tagDefault := field.Tag.Get("db_default"); tagDefault != "" {
		// DEFAULT.
		columnNull += " DEFAULT " + tagDefault
	}

	if tagUnique := field.Tag.Get("db_unique"); tagUnique == "true" {
		// UNIQUE.
		columnNull += " UNIQUE"
	}

	return fmt.Sprint(columnType, columnPrimary, columnNull), nil
}

func (dialect PqDialect) Param(identifier int) string {
	var query strings.Builder
	query.WriteString("$")
	query.WriteString(strconv.Itoa(identifier))
	return query.String()
}

func (dialect PqDialect) QuoteIdentifier(identifier string) string {
	// 100-500ns all the way up to ~45us on early op for some reason.
	var query strings.Builder
	for i, part := range strings.Split(identifier, ".") {
		if i > 0 {
			query.WriteString(".")
		}
		query.WriteString(QuoteIdentifier(part))
	}
	return query.String()
}
