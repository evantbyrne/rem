package rem

import (
	"testing"

	"golang.org/x/exp/slices"
)

func TestSql(t *testing.T) {
	dialect := testDialect{}
	sql, args, err := Sql(`SELECT count(1) AS "x"`).StringWithArgs(dialect, []interface{}{})
	expectedArgs := []interface{}{}
	expectedSql := `SELECT count(1) AS "x"`
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%+v', got '%+v'", expectedArgs, args)
	}
	if sql != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, sql)
	}

	sql, args, err = Sql("SELECT * FROM x WHERE y = ", Param(100), " AND z IS ", Param(true)).StringWithArgs(dialect, []interface{}{})
	expectedArgs = []interface{}{100, true}
	expectedSql = "SELECT * FROM x WHERE y = $1 AND z IS $2"
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%+v', got '%+v'", expectedArgs, args)
	}
	if sql != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, sql)
	}
}

func TestUnsafe(t *testing.T) {
	unsafe := Unsafe(`SELECT count(1) AS "x"`)
	if unsafe.Sql != `SELECT count(1) AS "x"` {
		t.Errorf(`Expected 'SELECT count(1) AS "x"', got '%s'`, unsafe.Sql)
	}
}
