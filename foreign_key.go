package rem

import (
	"database/sql"
	"encoding/json"
	"reflect"
)

type ForeignKey[To any] struct {
	Row   *To
	Valid bool
}

func (fk *ForeignKey[To]) Fetch(db *sql.DB) (*To, error) {
	query := &Query[To]{
		Model: fk.Model(),
	}
	value := reflect.ValueOf(&fk.Row).Elem()
	id := value.FieldByName(query.Model.PrimaryField).Interface()
	return query.Filter("id", "=", id).First(db)
}

func (fk ForeignKey[To]) MarshalJSON() ([]byte, error) {
	if !fk.Valid {
		return json.Marshal(nil)
	}
	return json.Marshal(fk.Model().ToJsonMap(fk.Row))
}

func (fk *ForeignKey[To]) Model() *Model[To] {
	if fk.Row == nil {
		var zero To
		fk.Row = &zero
	}
	return Use[To]()
}

func (fk *ForeignKey[To]) Query() *Query[To] {
	return &Query[To]{
		Model: Use[To](),
	}
}

type NullForeignKey[To any] struct {
	Row   *To
	Valid bool
}

func (fk *NullForeignKey[To]) Fetch(db *sql.DB) (*To, error) {
	query := &Query[To]{
		Model: Use[To](),
	}
	value := reflect.ValueOf(&fk.Row).Elem()
	id := value.FieldByName(query.Model.PrimaryField).Interface()
	return query.Filter("id", "=", id).First(db)
}

func (fk NullForeignKey[To]) MarshalJSON() ([]byte, error) {
	if !fk.Valid {
		return json.Marshal(nil)
	}
	return json.Marshal(fk.Model().ToJsonMap(fk.Row))
}

func (fk *NullForeignKey[To]) Model() *Model[To] {
	if fk.Row == nil {
		var zero To
		fk.Row = &zero
	}
	return Use[To]()
}

func (fk *NullForeignKey[To]) Query() *Query[To] {
	return &Query[To]{
		Model: Use[To](),
	}
}
