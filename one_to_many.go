package rem

import (
	"database/sql"
	"encoding/json"
)

type OneToMany[To any] struct {
	RelatedColumn string
	RowPk         interface{}
	Rows          []*To
}

func (field *OneToMany[To]) All(db *sql.DB) ([]*To, error) {
	return field.Query().Filter(field.RelatedColumn, "=", field.RowPk).All(db)
}

func (field OneToMany[To]) MarshalJSON() ([]byte, error) {
	model := field.Model()
	results := make([]map[string]interface{}, len(field.Rows))
	for i, row := range field.Rows {
		results[i] = model.ToJsonMap(row)
	}
	return json.Marshal(results)
}

func (field *OneToMany[To]) Model() *Model[To] {
	return Use[To]()
}

func (field *OneToMany[To]) Query() *Query[To] {
	return &Query[To]{
		Model: Use[To](),
	}
}
