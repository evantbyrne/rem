package rem

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Config struct {
	Table string
}

type Model[T any] struct {
	Fields        map[string]reflect.StructField
	PrimaryColumn string
	PrimaryField  string
	Table         string
	Type          reflect.Type
}

func (model *Model[T]) All(db *sql.DB) ([]*T, error) {
	query := &Query[T]{Model: model}
	return query.All(db)
}

func (model *Model[T]) AllToMap(db *sql.DB) ([]map[string]interface{}, error) {
	query := &Query[T]{Model: model}
	return query.AllToMap(db)
}

func (model *Model[T]) Context(context context.Context) *Query[T] {
	return &Query[T]{
		Config: QueryConfig{Context: context},
		Model:  model,
	}
}

func (model *Model[T]) Count(db *sql.DB) (uint, error) {
	query := &Query[T]{Model: model}
	return query.Count(db)
}

func (model *Model[T]) Dialect(dialect Dialect) *Query[T] {
	return &Query[T]{
		dialect: dialect,
		Model:   model,
	}
}

func (model *Model[T]) FetchRelated(columns ...string) *Query[T] {
	return &Query[T]{
		Config: QueryConfig{FetchRelated: columns},
		Model:  model,
	}
}

func (model *Model[T]) Filter(column interface{}, operator string, value interface{}) *Query[T] {
	query := &Query[T]{Model: model}
	return query.Filter(column, operator, value)
}

func (model *Model[T]) FilterAnd(clauses ...interface{}) *Query[T] {
	query := &Query[T]{Model: model}
	return query.FilterAnd(clauses...)
}

func (model *Model[T]) FilterOr(clauses ...interface{}) *Query[T] {
	query := &Query[T]{Model: model}
	return query.FilterOr(clauses...)
}

func (model *Model[T]) Insert(db *sql.DB, row *T) (sql.Result, error) {
	query := &Query[T]{Model: model}
	return query.Insert(db, row)
}

func (model *Model[T]) InsertMap(db *sql.DB, data map[string]interface{}) (sql.Result, error) {
	query := &Query[T]{Model: model}
	return query.InsertMap(db, data)
}

func (model *Model[T]) Query() *Query[T] {
	return &Query[T]{Model: model}
}

func (model *Model[T]) Scan(rows *sql.Rows) (*T, error) {
	data, err := model.ScanToMap(rows)
	if err != nil {
		return nil, err
	}
	return model.ScanMap(data)
}

func (model *Model[T]) ScanMap(data map[string]interface{}) (*T, error) {
	var row T
	value := reflect.ValueOf(&row).Elem()

	for column, v := range data {
		if field, ok := model.Fields[column]; ok {
			if field := value.FieldByName(field.Name); field.IsValid() {
				columnValue := reflect.ValueOf(v)

				if v == nil {
					// database/sql null types (NullString, etc) default to `Valid: false`.
					// rem.ForeignKey and rem.NullForeignKey also follow this convention.

				} else if columnValue.CanConvert(field.Type()) {
					field.Set(columnValue)

				} else if field.Kind() == reflect.Struct {
					if scanner, ok := reflect.New(field.Type()).Interface().(sql.Scanner); ok {
						scanner.Scan(v)
						field.Set(reflect.ValueOf(scanner).Elem())

					} else if strings.HasPrefix(field.Type().String(), "rem.ForeignKey[") || strings.HasPrefix(field.Type().String(), "rem.NullForeignKey[") {
						subModelQ := field.Addr().MethodByName("Model").Call(nil)
						subPrimaryField := reflect.Indirect(subModelQ[0]).FieldByName("PrimaryField").Interface().(string)
						subField := field.FieldByName("Row").Elem().FieldByName(subPrimaryField)
						if subField.IsValid() {
							// TODO: Handle primary keys that are nullable types
							subField.Set(columnValue)
							field.FieldByName("Valid").SetBool(true)
						}
					} else {
						return nil, fmt.Errorf("rem: unhandled struct conversion in scan from '%s' to '%s'", columnValue.Type(), field.Type())
					}

				} else {
					return nil, fmt.Errorf("rem: unhandled type conversion in scan from '%s' to '%s'", columnValue.Type(), field.Type())
				}
			}
		}
	}

	// OneToMany relationships.
	for _, field := range model.Fields {
		if strings.HasPrefix(field.Type.String(), "rem.OneToMany[") {
			oneToMany := value.FieldByName(field.Name)
			oneToMany.FieldByName("RelatedColumn").SetString(field.Tag.Get("db"))
			oneToMany.FieldByName("RowPk").Set(value.FieldByName(model.PrimaryField))
		}
	}

	return &row, nil
}

func (model *Model[T]) ScanToMap(rows *sql.Rows) (map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	pointers := make([]interface{}, len(columns))
	for i, column := range columns {
		field, ok := model.Fields[column]
		if !ok {
			return nil, fmt.Errorf("rem: column '%s' not found on model '%T'", column, model)
		}
		fieldType := field.Type
		if strings.HasPrefix(fieldType.String(), "rem.ForeignKey[") || strings.HasPrefix(fieldType.String(), "rem.NullForeignKey[") {
			fk := reflect.New(fieldType)
			q := fk.MethodByName("Model").Call(nil)
			fkPrimaryField := reflect.Indirect(q[0]).FieldByName("PrimaryField").Interface().(string)
			pointers[i] = reflect.New(reflect.Indirect(reflect.Indirect(fk).FieldByName("Row")).FieldByName(fkPrimaryField).Type()).Interface()
			if strings.HasPrefix(fieldType.String(), "rem.NullForeignKey[") {
				switch pointers[i].(type) {
				case *bool:
					pointers[i] = new(sql.NullBool)
				case *byte:
					pointers[i] = new(sql.NullByte)
				case *int, *int64:
					pointers[i] = new(sql.NullInt64)
				case *int32:
					pointers[i] = new(sql.NullInt32)
				case *int8, *int16:
					pointers[i] = new(sql.NullInt16)
				case *float32, *float64:
					pointers[i] = new(sql.NullFloat64)
				case *string:
					pointers[i] = new(sql.NullString)
				case *time.Time:
					pointers[i] = new(sql.NullTime)
				}
			}
		} else {
			pointers[i] = reflect.New(fieldType).Interface()
		}
	}

	if err := rows.Scan(pointers...); err != nil {
		return nil, err
	}

	row := make(map[string]interface{})
	for i, column := range columns {
		switch vt := reflect.ValueOf(pointers[i]).Elem().Interface().(type) {
		case driver.Valuer:
			row[column], _ = vt.Value()
		default:
			row[column] = vt
		}
	}

	return row, nil
}

func (model *Model[T]) Select(columns ...interface{}) *Query[T] {
	return &Query[T]{
		Config: QueryConfig{Selected: columns},
		Model:  model,
	}
}

func (model *Model[T]) Sort(columns ...string) *Query[T] {
	return &Query[T]{
		Config: QueryConfig{Sort: columns},
		Model:  model,
	}
}

func (model *Model[T]) SqlAll(db *sql.DB, sql string, args ...interface{}) ([]*T, error) {
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	query := &Query[T]{
		Model: model,
		Rows:  rows,
	}
	return query.slice(db)
}

func (model *Model[T]) SqlAllToMap(db *sql.DB, sql string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	query := &Query[T]{
		Model: model,
		Rows:  rows,
	}
	defer query.Rows.Close()

	mapped := make([]map[string]interface{}, 0)
	for query.Rows.Next() {
		data, err := model.ScanToMap(query.Rows)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, data)
	}

	return mapped, nil
}

func (model *Model[T]) TableColumnAdd(db *sql.DB, column string) (sql.Result, error) {
	query := &Query[T]{Model: model}
	return query.TableColumnAdd(db, column)
}

func (model *Model[T]) TableColumnDrop(db *sql.DB, column string) (sql.Result, error) {
	query := &Query[T]{Model: model}
	return query.TableColumnDrop(db, column)
}

func (model *Model[T]) TableCreate(db *sql.DB, tableCreateConfig ...TableCreateConfig) (sql.Result, error) {
	query := &Query[T]{Model: model}
	return query.TableCreate(db, tableCreateConfig...)
}

func (model *Model[T]) TableDrop(db *sql.DB, tableDropConfig ...TableDropConfig) (sql.Result, error) {
	query := &Query[T]{Model: model}
	if len(tableDropConfig) > 0 {
		return query.TableDrop(db, tableDropConfig[0])
	}
	return query.TableDrop(db, TableDropConfig{})
}

func (model *Model[T]) ToMap(row *T) (map[string]interface{}, error) {
	args := make(map[string]interface{})
	value := reflect.ValueOf(*row)

	for column := range model.Fields {
		fieldName := model.Fields[column].Name
		field := value.FieldByName(fieldName)

		// Skip zero valued primary keys.
		if field.IsZero() && model.Fields[column].Tag.Get("db_primary") == "true" {
			continue
		}

		switch field.Kind() {
		case reflect.Struct:
			switch vv := field.Interface().(type) {
			case driver.Valuer:
				v, _ := vv.Value()
				args[column] = v

			case time.Time:
				args[column] = vv

			default:
				if strings.HasPrefix(field.Type().String(), "rem.ForeignKey[") || strings.HasPrefix(field.Type().String(), "rem.NullForeignKey[") {
					if !field.FieldByName("Valid").Interface().(bool) {
						args[column] = nil
					} else {
						q := reflect.New(field.Type()).MethodByName("Model").Call(nil)
						fkPrimaryField := reflect.Indirect(q[0]).FieldByName("PrimaryField").Interface().(string)
						args[column] = reflect.Indirect(field.FieldByName("Row")).FieldByName(fkPrimaryField).Interface()
					}
				} else if strings.HasPrefix(field.Type().String(), "rem.OneToMany[") {
					continue
				} else {
					return nil, fmt.Errorf("rem: unsupported field type '%s' for column '%s' on table '%s'", field.Type().String(), column, model.Table)
				}
			}

		default:
			args[column] = field.Interface()
		}
	}

	return args, nil
}

func (model *Model[T]) Transaction(transaction *sql.Tx) *Query[T] {
	return &Query[T]{
		Config: QueryConfig{Transaction: transaction},
		Model:  model,
	}
}

type TableCreateConfig struct {
	IfNotExists bool
}

type TableDropConfig struct {
	IfExists bool
}

var registeredModels = make(map[string]interface{})

func Register[T any](configs ...Config) *Model[T] {
	var model T
	modelType := reflect.TypeOf(model)
	modelTypeStr := modelType.String()
	for _, config := range configs {
		modelTypeStr = fmt.Sprintf("%s%+v", modelTypeStr, config)
	}

	m := Use[T](configs...)
	registeredModels[modelTypeStr] = m
	return m
}

func Use[T any](configs ...Config) *Model[T] {
	var model T
	modelType := reflect.TypeOf(model)
	modelTypeStr := modelType.String()
	for _, config := range configs {
		modelTypeStr = fmt.Sprintf("%s%+v", modelTypeStr, config)
	}

	if existing, ok := registeredModels[modelTypeStr]; ok {
		return existing.(*Model[T])
	}

	var primaryColumn string
	var primaryField string
	fields := make(map[string]reflect.StructField, 0)

	for _, field := range reflect.VisibleFields(modelType) {
		if column, ok := field.Tag.Lookup("db"); ok {
			if strings.HasPrefix(field.Type.String(), "rem.OneToMany[") {
				fields[field.Name] = field
			} else {
				fields[column] = field
				if field.Tag.Get("db_primary") == "true" {
					primaryColumn = column
					primaryField = field.Name
				}
			}
		}
	}

	table := strings.ToLower(modelType.Name())
	for _, config := range configs {
		if config.Table != "" {
			table = config.Table
		}
	}

	return &Model[T]{
		Fields:        fields,
		PrimaryColumn: primaryColumn,
		PrimaryField:  primaryField,
		Table:         table,
		Type:          modelType,
	}
}
