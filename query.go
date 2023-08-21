package rem

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
)

type JoinClause struct {
	Direction string
	On        []FilterClause
	Table     string
}

type QueryConfig struct {
	Count        bool
	Context      context.Context
	FetchRelated []string
	Fields       map[string]reflect.StructField
	Filters      []FilterClause
	Joins        []JoinClause
	Limit        interface{}
	Offset       interface{}
	Params       []interface{}
	Selected     []interface{}
	Sort         []string
	Table        string
	Transaction  *sql.Tx
}

type Query[T any] struct {
	Config QueryConfig
	Error  error
	Model  *Model[T]
	Rows   *sql.Rows

	dialect Dialect
}

func (query *Query[T]) All(db *sql.DB) ([]*T, error) {
	query.detectDialect()
	query.configure()

	queryString, args, err := query.dialect.BuildSelect(query.Config)
	if err != nil {
		return make([]*T, 0), err
	}

	rows, err := query.dbQuery(db, queryString, args...)
	if err != nil {
		return make([]*T, 0), err
	}
	query.Rows = rows
	return query.slice(db)
}

func (query *Query[T]) AllToMap(db *sql.DB) ([]map[string]interface{}, error) {
	query.detectDialect()
	query.configure()

	queryString, args, err := query.dialect.BuildSelect(query.Config)
	if err != nil {
		return nil, err
	}

	rows, err := query.dbQuery(db, queryString, args...)
	if err != nil {
		return nil, err
	}
	query.Rows = rows
	defer query.Rows.Close()

	mapped := make([]map[string]interface{}, 0)
	for query.Rows.Next() {
		data, err := query.Model.ScanToMap(query.Rows)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, data)
	}

	if query.Config.Context != nil {
		select {
		default:
		case <-query.Config.Context.Done():
			return nil, query.Config.Context.Err()
		}
	}

	return mapped, nil
}

func (query *Query[T]) configure() {
	query.Config.Fields = query.Model.Fields
	query.Config.Table = query.Model.Table
}

func (query *Query[T]) Context(context context.Context) *Query[T] {
	query.Config.Context = context
	return query
}

func (query *Query[T]) Count(db *sql.DB) (uint, error) {
	query.detectDialect()
	query.configure()

	var count uint

	query.Config.Count = true

	queryString, args, err := query.dialect.BuildSelect(query.Config)
	if err != nil {
		return count, err
	}

	if query.Config.Transaction != nil {
		if query.Config.Context != nil {
			err = query.Config.Transaction.QueryRowContext(query.Config.Context, queryString, args...).Scan(&count)
		} else {
			err = query.Config.Transaction.QueryRow(queryString, args...).Scan(&count)
		}
	} else if query.Config.Context != nil {
		err = db.QueryRowContext(query.Config.Context, queryString, args...).Scan(&count)
	} else {
		err = db.QueryRow(queryString, args...).Scan(&count)
	}
	if err != nil {
		return count, err
	}

	return count, nil
}

func (query *Query[T]) dbExec(db *sql.DB, queryString string, args ...interface{}) (sql.Result, error) {
	if query.Config.Transaction != nil {
		if query.Config.Context != nil {
			return query.Config.Transaction.ExecContext(query.Config.Context, queryString, args...)
		}
		return query.Config.Transaction.Exec(queryString, args...)
	}

	if query.Config.Context != nil {
		return db.ExecContext(query.Config.Context, queryString, args...)
	}
	return db.Exec(queryString, args...)
}

func (query *Query[T]) dbQuery(db *sql.DB, queryString string, args ...interface{}) (*sql.Rows, error) {
	if query.Config.Transaction != nil {
		if query.Config.Context != nil {
			return query.Config.Transaction.QueryContext(query.Config.Context, queryString, args...)
		}
		return query.Config.Transaction.Query(queryString, args...)
	}

	if query.Config.Context != nil {
		return db.QueryContext(query.Config.Context, queryString, args...)
	}
	return db.Query(queryString, args...)
}

func (query *Query[T]) Delete(db *sql.DB) (sql.Result, error) {
	query.detectDialect()
	query.configure()

	queryString, args, err := query.dialect.BuildDelete(query.Config)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString, args...)
}

func (query *Query[T]) detectDialect() {
	if query.dialect == nil {
		if defaultDialect != nil {
			query.dialect = defaultDialect
		} else {
			panic("rem: no dialect registered. Use rem.SetDialect(dialect rem.Dialect) to register a default for SQL queries")
		}
	}
}

func (query *Query[T]) Dialect(dialect Dialect) *Query[T] {
	query.dialect = dialect
	return query
}

func (query *Query[T]) Exists(db *sql.DB) (bool, error) {
	query.detectDialect()
	query.configure()

	query.Config.Limit = 1

	queryString, args, err := query.dialect.BuildSelect(query.Config)
	if err != nil {
		return false, err
	}

	rows, err := query.dbQuery(db, queryString, args...)
	if err != nil {
		return false, err
	}
	return rows.Next(), nil
}

func (query *Query[T]) FetchRelated(columns ...string) *Query[T] {
	query.Config.FetchRelated = columns
	return query
}

func (query *Query[T]) Filter(column interface{}, operator string, value interface{}) *Query[T] {
	if len(query.Config.Filters) > 0 {
		query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "AND"})
	}
	query.Config.Filters = append(query.Config.Filters, Q(column, operator, value))
	return query
}

func (query *Query[T]) FilterAnd(clauses ...interface{}) *Query[T] {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	if len(query.Config.Filters) > 0 {
		query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "AND"})
	}
	query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "("})
	indent := 0
	for i, clause := range flat {
		if i > 0 && indent == 0 {
			query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "AND"})
		}
		if clause.Rule == "(" {
			indent++
		} else if clause.Rule == ")" {
			indent--
		}
		query.Config.Filters = append(query.Config.Filters, clause)
	}

	query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: ")"})
	return query
}

func (query *Query[T]) FilterOr(clauses ...interface{}) *Query[T] {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	if len(query.Config.Filters) > 0 {
		query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "AND"})
	}
	query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "("})
	indent := 0
	for i, clause := range flat {
		if i > 0 && indent == 0 {
			query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: "OR"})
		}
		if clause.Rule == "(" {
			indent++
		} else if clause.Rule == ")" {
			indent--
		}
		query.Config.Filters = append(query.Config.Filters, clause)
	}

	query.Config.Filters = append(query.Config.Filters, FilterClause{Rule: ")"})
	return query
}

func (query *Query[T]) First(db *sql.DB) (*T, error) {
	query.detectDialect()
	query.configure()

	query.Limit(1)

	queryString, args, err := query.dialect.BuildSelect(query.Config)
	if err != nil {
		return nil, err
	}

	rows, err := query.dbQuery(db, queryString, args...)
	if err != nil {
		return nil, err
	}
	query.Rows = rows

	defer query.Rows.Close()
	if query.Rows.Next() {
		return query.Model.Scan(query.Rows)
	}

	if query.Config.Context != nil {
		select {
		default:
		case <-query.Config.Context.Done():
			return nil, query.Config.Context.Err()
		}
	}

	return nil, sql.ErrNoRows
}

func (query *Query[T]) FirstToMap(db *sql.DB) (map[string]interface{}, error) {
	query.detectDialect()
	query.configure()

	query.Limit(1)

	queryString, args, err := query.dialect.BuildSelect(query.Config)
	if err != nil {
		return nil, err
	}

	rows, err := query.dbQuery(db, queryString, args...)
	if err != nil {
		return nil, err
	}
	query.Rows = rows

	defer query.Rows.Close()
	if query.Rows.Next() {
		return query.Model.ScanToMap(query.Rows)
	}

	if query.Config.Context != nil {
		select {
		default:
		case <-query.Config.Context.Done():
			return nil, query.Config.Context.Err()
		}
	}

	return nil, sql.ErrNoRows
}

func (query *Query[T]) Insert(db *sql.DB, row *T) (sql.Result, error) {
	query.detectDialect()
	query.configure()
	rowMap, err := query.Model.ToMap(row)
	if err != nil {
		return nil, err
	}
	queryString, args, err := query.dialect.BuildInsert(query.Config, rowMap, maps.Keys(rowMap)...)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString, args...)
}

func (query *Query[T]) InsertMap(db *sql.DB, data map[string]interface{}) (sql.Result, error) {
	query.detectDialect()
	query.configure()
	queryString, args, err := query.dialect.BuildInsert(query.Config, data, maps.Keys(data)...)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString, args...)
}

func (query *Query[T]) Join(table string, clauses ...interface{}) *Query[T] {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	query.Config.Joins = append(query.Config.Joins, JoinClause{
		Direction: "INNER",
		On:        flat,
		Table:     table,
	})
	return query
}

func (query *Query[T]) JoinFull(table string, clauses ...interface{}) *Query[T] {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	query.Config.Joins = append(query.Config.Joins, JoinClause{
		Direction: "FULL",
		On:        flat,
		Table:     table,
	})
	return query
}

func (query *Query[T]) JoinLeft(table string, clauses ...interface{}) *Query[T] {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	query.Config.Joins = append(query.Config.Joins, JoinClause{
		Direction: "LEFT",
		On:        flat,
		Table:     table,
	})
	return query
}

func (query *Query[T]) JoinRight(table string, clauses ...interface{}) *Query[T] {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	query.Config.Joins = append(query.Config.Joins, JoinClause{
		Direction: "RIGHT",
		On:        flat,
		Table:     table,
	})
	return query
}

func (query *Query[T]) Limit(limit interface{}) *Query[T] {
	query.Config.Limit = limit
	return query
}

func (query *Query[T]) Offset(offset interface{}) *Query[T] {
	query.Config.Offset = offset
	return query
}

func (query *Query[T]) Select(columns ...interface{}) *Query[T] {
	query.Config.Selected = columns
	return query
}

func (query *Query[T]) slice(db *sql.DB) ([]*T, error) {
	rows := make([]*T, 0)
	if query.Error != nil {
		return rows, query.Error
	}
	defer query.Rows.Close()

	relatedPks := make(map[string]relatedPk)
	for query.Rows.Next() {
		row, err := query.Model.Scan(query.Rows)
		if err != nil {
			return rows, err
		}
		if len(query.Config.FetchRelated) > 0 {
			value := reflect.ValueOf(*row)
			for _, column := range query.Config.FetchRelated {
				valueFk := value.FieldByName(column)
				if !valueFk.IsValid() {
					return rows, fmt.Errorf("rem: invalid field '%s' for fetching related. Field does not exist on model", column)
				}
				if strings.HasPrefix(valueFk.Type().String(), "rem.ForeignKey[") || strings.HasPrefix(valueFk.Type().String(), "rem.NullForeignKey[") {
					if valueFk.FieldByName("Valid").Interface().(bool) {
						r := reflect.New(valueFk.Type()).MethodByName("Model").Call(nil)
						rpk, ok := relatedPks[column]
						if !ok {
							rpk = relatedPk{
								RelatedColumn: reflect.Indirect(r[0]).FieldByName("PrimaryColumn").Interface().(string),
								RelatedField:  reflect.Indirect(r[0]).FieldByName("PrimaryField").Interface().(string),
								RelatedValues: make([]interface{}, 0),
							}
						}
						rpk.RelatedValues = append(rpk.RelatedValues, valueFk.FieldByName("Row").Elem().FieldByName(rpk.RelatedField).Interface())
						relatedPks[column] = rpk
					}
				} else if strings.HasPrefix(valueFk.Type().String(), "rem.OneToMany[") {
					rpk, ok := relatedPks[column]
					if !ok {
						relatedColumn := valueFk.FieldByName("RelatedColumn").Interface().(string)
						r := reflect.New(valueFk.Type()).MethodByName("Model").Call(nil)
						fkModelFields := reflect.Indirect(r[0]).FieldByName("Fields").MapRange()
						var relatedField string
						for fkModelFields.Next() {
							fkModelField := fkModelFields.Value().FieldByName("Tag").MethodByName("Get").Call([]reflect.Value{reflect.ValueOf("db")})[0].Interface().(string)
							if fkModelField == relatedColumn {
								relatedField = fkModelFields.Value().FieldByName("Name").Interface().(string)
								break
							}
						}

						if relatedField == "" {
							return rows, fmt.Errorf("rem: invalid db tag of '%s' for fetching related on field '%s'. No fields with a matching column exist on the related model", relatedColumn, column)
						}

						rpk = relatedPk{
							RelatedColumn: relatedColumn,
							RelatedField:  relatedField,
							RelatedValues: make([]interface{}, 0),
						}
					}
					rpk.RelatedValues = append(rpk.RelatedValues, value.FieldByName(query.Model.PrimaryField).Interface())
					relatedPks[column] = rpk
				} else {
					return rows, fmt.Errorf("rem: invalid field '%s' for fetching related. Field must be of type rem.ForeignKey[To], rem.NullForeignKey[To], or rem.OneToMany[To, From]", column)
				}
			}
		}
		rows = append(rows, row)
	}

	if len(relatedPks) > 0 {
		var temp T
		modelValue := reflect.ValueOf(&temp).Elem()

		for column, rpk := range relatedPks {
			if len(rpk.RelatedValues) > 0 {
				fk := reflect.New(modelValue.FieldByName(column).Type())

				q := fk.MethodByName("Query").Call(nil)
				q = q[0].MethodByName("Filter").Call([]reflect.Value{
					reflect.ValueOf(rpk.RelatedColumn),
					reflect.ValueOf("IN"),
					reflect.ValueOf(rpk.RelatedValues),
				})
				q = q[0].MethodByName("All").Call([]reflect.Value{
					reflect.ValueOf(db),
				})
				rowsValue := reflect.ValueOf(rows)
				for i := 0; i < rowsValue.Len(); i++ {
					value := rowsValue.Index(i).Elem()
					valueFk := value.FieldByName(column)

					if strings.HasPrefix(fk.Type().String(), "*rem.OneToMany[") {
						mq := fk.MethodByName("Model").Call(nil)
						fkPrimaryField := reflect.Indirect(mq[0]).FieldByName("PrimaryField").Interface().(string)
						for j := 0; j < q[0].Len(); j++ {
							fkRow := q[0].Index(j).Elem()
							relatedFieldId := fkRow.FieldByName(rpk.RelatedField).FieldByName("Row").Elem().FieldByName(fkPrimaryField).Interface()
							if value.FieldByName(query.Model.PrimaryField).Interface() == relatedFieldId {
								valueFk.FieldByName("Rows").Set(reflect.Append(valueFk.FieldByName("Rows"), fkRow.Addr()))
							}
						}
					} else if valueFk.FieldByName("Valid").Interface().(bool) {
						mq := fk.MethodByName("Model").Call(nil)
						fkPrimaryField := reflect.Indirect(mq[0]).FieldByName("PrimaryField").Interface().(string)
						for j := 0; j < q[0].Len(); j++ {
							fkRow := q[0].Index(j)
							if valueFk.FieldByName("Row").Elem().FieldByName(query.Model.PrimaryField).Interface() == fkRow.Elem().FieldByName(fkPrimaryField).Interface() {
								valueFk.FieldByName("Row").Set(fkRow)
								break
							}
						}
					}
				}
			}
		}
	}

	if query.Config.Context != nil {
		select {
		default:
		case <-query.Config.Context.Done():
			return nil, query.Config.Context.Err()
		}
	}

	return rows, nil
}

func (query *Query[T]) Sort(columns ...string) *Query[T] {
	query.Config.Sort = columns
	return query
}

func (query Query[T]) StringWithArgs(dialect Dialect, args []interface{}) (string, []interface{}, error) {
	query.dialect = dialect
	query.configure()
	query.Config.Params = args
	return query.dialect.BuildSelect(query.Config)
}

func (query *Query[T]) TableColumnAdd(db *sql.DB, column string) (sql.Result, error) {
	query.detectDialect()
	query.configure()
	queryString, err := query.dialect.BuildTableColumnAdd(query.Config, column)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString)
}

func (query *Query[T]) TableColumnDrop(db *sql.DB, column string) (sql.Result, error) {
	query.detectDialect()
	query.configure()
	queryString, err := query.dialect.BuildTableColumnDrop(query.Config, column)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString)
}

func (query *Query[T]) TableCreate(db *sql.DB, tableCreateConfig ...TableCreateConfig) (sql.Result, error) {
	query.detectDialect()
	query.configure()
	var config TableCreateConfig
	if len(tableCreateConfig) > 0 {
		config = tableCreateConfig[0]
	}
	queryString, err := query.dialect.BuildTableCreate(query.Config, config)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString)
}

func (query *Query[T]) TableDrop(db *sql.DB, tableDropConfig ...TableDropConfig) (sql.Result, error) {
	query.detectDialect()
	query.configure()
	var config TableDropConfig
	if len(tableDropConfig) > 0 {
		config = tableDropConfig[0]
	}
	queryString, err := query.dialect.BuildTableDrop(query.Config, config)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString)
}

func (query *Query[T]) Transaction(transaction *sql.Tx) *Query[T] {
	query.Config.Transaction = transaction
	return query
}

func (query *Query[T]) Update(db *sql.DB, row *T, columns ...string) (sql.Result, error) {
	query.detectDialect()
	query.configure()

	if len(columns) == 0 {
		return nil, fmt.Errorf("rem: no columns specified for update")
	}

	rowMap, err := query.Model.ToMap(row)
	if err != nil {
		return nil, err
	}

	queryString, args, err := query.dialect.BuildUpdate(query.Config, rowMap, columns...)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString, args...)
}

func (query *Query[T]) UpdateMap(db *sql.DB, data map[string]interface{}) (sql.Result, error) {
	query.detectDialect()
	query.configure()

	if len(data) == 0 {
		return nil, fmt.Errorf("rem: no columns specified for update")
	}

	columns := make([]string, 0)
	for column := range data {
		columns = append(columns, column)
	}

	queryString, args, err := query.dialect.BuildUpdate(query.Config, data, columns...)
	if err != nil {
		return nil, err
	}
	return query.dbExec(db, queryString, args...)
}

type relatedPk struct {
	RelatedColumn string
	RelatedField  string
	RelatedValues []interface{}
}
