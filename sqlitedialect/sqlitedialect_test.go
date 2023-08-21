package sqlitedialect

import (
	"database/sql"
	"sort"
	"testing"
	"time"

	"github.com/evantbyrne/rem"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func TestAs(t *testing.T) {
	dialect := SqliteDialect{}
	expected := map[string]rem.SqlAs{
		"`x` AS `alias1`":        rem.As("x", "alias1"),
		"`x` AS `y` AS `alias2`": rem.As(rem.As("x", "y"), "alias2"),
		"count(*) AS `alias3`":   rem.As(rem.Unsafe("count(*)"), "alias3"),
	}
	for expected, alias := range expected {
		sql := alias.StringForDialect(dialect)
		if expected != sql {
			t.Errorf("Expected '%+v', got '%+v'", expected, sql)
		}
	}
}

func TestColumn(t *testing.T) {
	dialect := SqliteDialect{}
	expected := map[string]rem.SqlColumn{
		"`x`":         rem.Column("x"),
		"`x`.`y`":     rem.Column("x.y"),
		"`x`.`y`.`z`": rem.Column("x.y.z"),
		"`x```":       rem.Column("x`"),
	}
	for expected, column := range expected {
		sql := column.StringForDialect(dialect)
		if expected != sql {
			t.Errorf("Expected '%+v', got '%+v'", expected, sql)
		}
	}
}

func TestBuildDelete(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()

	query := model.Query()
	config := query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs := []interface{}{}
	expectedSql := "DELETE FROM `testmodel`"
	queryString, args, err := dialect.BuildDelete(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// WHERE
	query.Filter("test_id", "=", 1)
	config = query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{1}
	expectedSql = "DELETE FROM `testmodel` WHERE `test_id` = ?"
	queryString, args, err = dialect.BuildDelete(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// ORDER BY
	query.Sort("-test_id")
	config = query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedSql = "DELETE FROM `testmodel` WHERE `test_id` = ? ORDER BY `test_id` DESC"
	queryString, args, err = dialect.BuildDelete(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// LIMIT
	query.Limit(3)
	config = query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{1, 3}
	expectedSql = "DELETE FROM `testmodel` WHERE `test_id` = ? ORDER BY `test_id` DESC LIMIT ?"
	queryString, args, err = dialect.BuildDelete(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}
}

func TestBuildInsert(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()

	config := model.Query().Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs := []interface{}{"foo", "bar"}
	expectedSql := "INSERT INTO `testmodel` (`test_value_1`,`test_value_2`) VALUES (?,?)"
	queryString, args, err := dialect.BuildInsert(config, map[string]interface{}{
		"test_value_1": "foo",
		"test_value_2": "bar",
	}, "test_value_1", "test_value_2")
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}
}

func TestBuildSelect(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()

	config := model.Query().Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs := []interface{}{}
	expectedSql := "SELECT * FROM `testmodel`"
	queryString, args, err := dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// SELECT
	config = model.Select("id", "value1", rem.Unsafe("count(1) as `count`"), rem.As("value2", "value3")).Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{}
	expectedSql = "SELECT `id`,`value1`,count(1) as `count`,`value2` AS `value3` FROM `testmodel`"
	queryString, args, err = dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// WHERE
	config = model.Filter("id", "=", 1).Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{1}
	expectedSql = "SELECT * FROM `testmodel` WHERE `id` = ?"
	queryString, args, err = dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	config = model.Filter("id", "IN", rem.Sql(rem.Param(1), ",", rem.Param(2))).Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{1, 2}
	expectedSql = "SELECT * FROM `testmodel` WHERE `id` IN (?,?)"
	queryString, args, err = dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// JOIN
	config = model.Select(rem.Unsafe("*")).Join("groups", rem.Or(
		rem.Q("groups.id", "=", rem.Column("accounts.group_id")),
		rem.Q("groups.id", "IS", nil))).Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{}
	expectedSql = "SELECT * FROM `testmodel` INNER JOIN `groups` ON ( `groups`.`id` = `accounts`.`group_id` OR `groups`.`id` IS NULL )"
	queryString, args, err = dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// SORT
	config = model.Sort("test_id", "-test_value_1").Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{}
	expectedSql = "SELECT * FROM `testmodel` ORDER BY `test_id` ASC, `test_value_1` DESC"
	queryString, args, err = dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	// LIMIT and OFFSET
	config = model.Filter("id", "=", 1).Offset(20).Limit(10).Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{1, 10, 20}
	expectedSql = "SELECT * FROM `testmodel` WHERE `id` = ? LIMIT ? OFFSET ?"
	queryString, args, err = dialect.BuildSelect(config)
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}
}

func TestBuildTableColumnAdd(t *testing.T) {
	type testModel struct {
		Value string `db:"test_value" db_max_length:"100"`
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()
	config := rem.QueryConfig{
		Fields: model.Fields,
		Table:  "testmodel",
	}
	expectedSql := "ALTER TABLE `testmodel` ADD COLUMN `test_value` TEXT NOT NULL"
	queryString, err := dialect.BuildTableColumnAdd(config, "test_value")
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
}

func TestBuildTableColumnDrop(t *testing.T) {
	dialect := SqliteDialect{}
	config := rem.QueryConfig{Table: "testmodel"}
	expectedSql := "ALTER TABLE `testmodel` DROP COLUMN `test_value`"
	queryString, err := dialect.BuildTableColumnDrop(config, "test_value")
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
}

func TestBuildTableCreate(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()
	config := rem.QueryConfig{
		Fields: model.Fields,
		Table:  "testmodel",
	}
	expectedSql := "CREATE TABLE `testmodel` (\n" +
		"\t`test_id` INTEGER PRIMARY KEY NOT NULL,\n" +
		"\t`test_value_1` TEXT NOT NULL\n" +
		")"
	queryString, err := dialect.BuildTableCreate(config, rem.TableCreateConfig{})
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}

	expectedSql = "CREATE TABLE IF NOT EXISTS `testmodel` (\n" +
		"\t`test_id` INTEGER PRIMARY KEY NOT NULL,\n" +
		"\t`test_value_1` TEXT NOT NULL\n" +
		")"
	queryString, err = dialect.BuildTableCreate(config, rem.TableCreateConfig{IfNotExists: true})
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
}

func TestBuildTableDrop(t *testing.T) {
	dialect := SqliteDialect{}
	config := rem.QueryConfig{Table: "testmodel"}
	expectedSql := "DROP TABLE `testmodel`"
	queryString, err := dialect.BuildTableDrop(config, rem.TableDropConfig{})
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}

	expectedSql = "DROP TABLE IF EXISTS `testmodel`"
	queryString, err = dialect.BuildTableDrop(config, rem.TableDropConfig{IfExists: true})
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
}

func TestBuildUpdate(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()

	query := model.Filter("test_id", "=", 1)
	config := query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs := []interface{}{"foo", "bar", 1}
	expectedSql := "UPDATE `testmodel` SET `test_value_1` = ?,`test_value_2` = ? WHERE `test_id` = ?"
	queryString, args, err := dialect.BuildUpdate(config, map[string]interface{}{
		"id":           123,
		"test_value_1": "foo",
		"test_value_2": "bar",
	}, "test_value_1", "test_value_2")
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	query.Limit(3)
	config = query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedArgs = []interface{}{"foo", "bar", 1, 3}
	expectedSql = "UPDATE `testmodel` SET `test_value_1` = ?,`test_value_2` = ? WHERE `test_id` = ? LIMIT ?"
	queryString, args, err = dialect.BuildUpdate(config, map[string]interface{}{
		"id":           123,
		"test_value_1": "foo",
		"test_value_2": "bar",
	}, "test_value_1", "test_value_2")
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}

	query.Sort("-test_id")
	config = query.Config
	config.Fields = model.Fields
	config.Table = "testmodel"
	expectedSql = "UPDATE `testmodel` SET `test_value_1` = ?,`test_value_2` = ? WHERE `test_id` = ? ORDER BY `test_id` DESC LIMIT ?"
	queryString, args, err = dialect.BuildUpdate(config, map[string]interface{}{
		"id":           123,
		"test_value_1": "foo",
		"test_value_2": "bar",
	}, "test_value_1", "test_value_2")
	if err != nil {
		t.Errorf("Unexpected error %s", err.Error())
	}
	if queryString != expectedSql {
		t.Errorf("Expected '%s', got '%s'", expectedSql, queryString)
	}
	if !slices.Equal(args, expectedArgs) {
		t.Errorf("Expected '%s', got '%s'", expectedArgs, args)
	}
}

func TestColumnType(t *testing.T) {
	type testFkInt struct {
		Id int64 `db:"id" db_primary:"true"`
	}

	type testFkString struct {
		Id string `db:"id" db_primary:"true" db_max_length:"100"`
	}

	type testModel struct {
		BigInt         int64                         `db:"test_big_int"`
		BigIntNull     sql.NullInt64                 `db:"test_big_int_null"`
		Bool           bool                          `db:"test_bool"`
		BoolNull       sql.NullBool                  `db:"test_bool_null"`
		Custom         []byte                        `db:"test_custom" db_type:"BLOB NOT NULL"`
		Default        string                        `db:"test_default" db_default:"'foo'" db_max_length:"100"`
		Float          float32                       `db:"test_float"`
		Double         float64                       `db:"test_double"`
		DoubleNull     sql.NullFloat64               `db:"test_double_null"`
		Id             int64                         `db:"test_id" db_primary:"true"`
		Int            int32                         `db:"test_int"`
		IntNull        sql.NullInt32                 `db:"test_int_null"`
		SmallInt       int16                         `db:"test_small_int"`
		SmallIntNull   sql.NullInt16                 `db:"test_small_int_null"`
		Text           string                        `db:"test_text"`
		TextNull       sql.NullString                `db:"test_text_null"`
		Time           time.Time                     `db:"test_time"`
		TimeNow        time.Time                     `db:"test_time_now" db_default:"CURRENT_TIMESTAMP"`
		TimeNull       sql.NullTime                  `db:"test_time_null"`
		TinyInt        int8                          `db:"test_tiny_int"`
		Varchar        string                        `db:"test_varchar" db_max_length:"100"`
		VarcharNull    sql.NullString                `db:"test_varchar_null" db_max_length:"50"`
		ForiegnKey     rem.ForeignKey[testFkString]  `db:"test_fk_id" db_on_delete:"CASCADE"`
		ForiegnKeyNull rem.NullForeignKey[testFkInt] `db:"test_fk_null_id" db_on_delete:"SET NULL" db_on_update:"SET NULL"`
		Unique         string                        `db:"test_unique" db_max_length:"255" db_unique:"true"`
	}

	expected := map[string]string{
		"test_big_int":        "INTEGER NOT NULL",
		"test_big_int_null":   "INTEGER NULL",
		"test_bool":           "BOOLEAN NOT NULL",
		"test_bool_null":      "BOOLEAN NULL",
		"test_custom":         "BLOB NOT NULL",
		"test_default":        "TEXT NOT NULL DEFAULT 'foo'",
		"test_float":          "REAL NOT NULL",
		"test_double":         "REAL NOT NULL",
		"test_double_null":    "REAL NULL",
		"test_id":             "INTEGER PRIMARY KEY NOT NULL",
		"test_int":            "INTEGER NOT NULL",
		"test_int_null":       "INTEGER NULL",
		"test_small_int":      "INTEGER NOT NULL",
		"test_small_int_null": "INTEGER NULL",
		"test_time":           "DATETIME NOT NULL",
		"test_time_now":       "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
		"test_time_null":      "DATETIME NULL",
		"test_text":           "TEXT NOT NULL",
		"test_text_null":      "TEXT NULL",
		"test_tiny_int":       "INTEGER NOT NULL",
		"test_varchar":        "TEXT NOT NULL",
		"test_varchar_null":   "TEXT NULL",
		"test_fk_id":          "TEXT NOT NULL REFERENCES `testfkstring` (`id`) ON DELETE CASCADE",
		"test_fk_null_id":     "INTEGER NULL REFERENCES `testfkint` (`id`) ON UPDATE SET NULL ON DELETE SET NULL",
		"test_unique":         "TEXT NOT NULL UNIQUE",
	}

	dialect := SqliteDialect{}
	model := rem.Use[testModel]()
	fieldKeys := maps.Keys(model.Fields)
	sort.Strings(fieldKeys)

	for _, fieldName := range fieldKeys {
		field := model.Fields[fieldName]
		columnType, err := dialect.ColumnType(field)
		if err != nil {
			t.Fatalf(`dialect.ColumnType() threw error for '%#v': %s`, field, err)
		}
		if columnType != expected[fieldName] {
			t.Fatalf(`dialect.ColumnType() returned '%s', but expected '%s' for '%#v'`, columnType, expected[fieldName], field)
		}
	}
}

func TestQuoteIdentifier(t *testing.T) {
	values := map[string]string{
		"abc":    "`abc`",
		"a`bc":   "`a``bc`",
		"a``b`c": "`a````b``c`",
		"`abc":   "```abc`",
		"abc`":   "`abc```",
		"ab\\`c": "`ab\\``c`",
		"abc\\":  "`abc\\`",
	}

	for identifier, expected := range values {
		actual := QuoteIdentifier(identifier)
		if actual != expected {
			t.Errorf("Expected %s, got %s", expected, actual)
		}
	}
}
