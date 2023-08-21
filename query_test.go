package rem

import (
	"sort"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func TestQueryConfigure(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	query := Use[testModel]().Query()
	query.configure()
	columns := maps.Keys(query.Config.Fields)
	sort.Strings(columns)
	expectedColumns := []string{"test_id", "test_value_1", "test_value_2"}
	if !slices.Equal(columns, expectedColumns) {
		t.Errorf(`Expected '%+v', got '%+v'`, expectedColumns, columns)
	}
	if query.Config.Table != "testmodel" {
		t.Errorf(`Expected 'testmodel', got '%s'`, query.Config.Table)
	}
}

func TestQueryDetectDialect(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	query := &Query[testModel]{}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic")
		}
	}()
	query.detectDialect()
}

func TestQueryFilters(t *testing.T) {
	type testModel struct {
		Id     int64  `db:"test_id" db_primary:"true"`
		Value1 string `db:"test_value_1" db_max_length:"100"`
		Value2 string `db:"test_value_2" db_max_length:"100"`
	}

	query := Use[testModel]().Query().Filter("a", "=", "foo")
	expected := []FilterClause{
		{Left: "a", Operator: "=", Right: "foo", Rule: "WHERE"},
	}
	if !slices.Equal(query.Config.Filters, expected) {
		t.Errorf(`Expected '%+v', got '%+v'`, expected, query.Config.Filters)
	}

	query = query.Filter("b", "!=", "bar")
	expected = []FilterClause{
		{Left: "a", Operator: "=", Right: "foo", Rule: "WHERE"},
		{Rule: "AND"},
		{Left: "b", Operator: "!=", Right: "bar", Rule: "WHERE"},
	}
	if !slices.Equal(query.Config.Filters, expected) {
		t.Errorf(`Expected '%+v', got '%+v'`, expected, query.Config.Filters)
	}

	query = query.FilterOr(
		Q("c.one", "=", 1),
		Q("c.two", "=", 2),
	)
	expected = []FilterClause{
		{Left: "a", Operator: "=", Right: "foo", Rule: "WHERE"},
		{Rule: "AND"},
		{Left: "b", Operator: "!=", Right: "bar", Rule: "WHERE"},
		{Rule: "AND"},
		{Rule: "("},
		{Left: "c.one", Operator: "=", Right: 1, Rule: "WHERE"},
		{Rule: "OR"},
		{Left: "c.two", Operator: "=", Right: 2, Rule: "WHERE"},
		{Rule: ")"},
	}
	if !slices.Equal(query.Config.Filters, expected) {
		t.Errorf(`Expected '%+v', got '%+v'`, expected, query.Config.Filters)
	}

	query = query.FilterAnd(
		Q("d.one", "IS", nil),
		Q("d.two", "IS NOT", nil),
		Q("d.three", "IN", []string{"foo", "bar", "baz"}),
	)
	expected = []FilterClause{
		{Left: "a", Operator: "=", Right: "foo", Rule: "WHERE"},
		{Rule: "AND"},
		{Left: "b", Operator: "!=", Right: "bar", Rule: "WHERE"},
		{Rule: "AND"},
		{Rule: "("},
		{Left: "c.one", Operator: "=", Right: 1, Rule: "WHERE"},
		{Rule: "OR"},
		{Left: "c.two", Operator: "=", Right: 2, Rule: "WHERE"},
		{Rule: ")"},
		{Rule: "AND"},
		{Rule: "("},
		{Left: "d.one", Operator: "IS", Right: nil, Rule: "WHERE"},
		{Rule: "AND"},
		{Left: "d.two", Operator: "IS NOT", Right: nil, Rule: "WHERE"},
		{Rule: "AND"},
		{Left: "d.three", Operator: "IN", Right: []string{"foo", "bar", "baz"}, Rule: "WHERE"},
		{Rule: ")"},
	}
	if len(query.Config.Filters) != len(expected) || !slices.Equal(query.Config.Filters[:15], expected[:15]) {
		t.Errorf(`Expected '%+v', got '%+v'`, expected, query.Config.Filters)
	}
	if left := query.Config.Filters[15].Left; left != "d.three" {
		t.Errorf(`Expected '%s', got '%s'`, "d.three", left)
	}
	if right := query.Config.Filters[15].Right.([]string); !slices.Equal(right, []string{"foo", "bar", "baz"}) {
		t.Errorf(`Expected '%+v', got '%+v'`, []string{"foo", "bar", "baz"}, right)
	}
	if last := query.Config.Filters[len(query.Config.Filters)-1]; last.Rule != ")" {
		t.Errorf(`Expected '%+v', got '%+v'`, FilterClause{Rule: ")"}, last)
	}
}

func TestQueryJoins(t *testing.T) {
	type testGroups struct {
		Id   int64  `db:"test_id" db_primary:"true"`
		Name string `db:"name" db_max_length:"100"`
	}
	type testAccounts struct {
		Id    int64                      `db:"id" db_primary:"true"`
		Group NullForeignKey[testGroups] `db:"group_id" db_on_delete:"SET NULL"`
		Name  string                     `db:"name" db_max_length:"100"`
	}

	assertJoins := func(t *testing.T, expected, actual []JoinClause) {
		if len(actual) != len(expected) {
			t.Errorf(`Expected '%+v', got '%+v'`, expected, actual)
		}
		for i, join := range actual {
			if join.Direction != expected[i].Direction {
				t.Errorf(`Expected '%s', got '%s'`, expected[i].Direction, join.Direction)
			}
			if join.Table != expected[i].Table {
				t.Errorf(`Expected '%s', got '%s'`, expected[i].Table, join.Table)
			}
			if !slices.Equal(join.On, expected[i].On) {
				t.Errorf(`Expected '%+v', got '%+v'`, expected[i].On, join.On)
			}
		}
	}

	model := Use[testAccounts]()
	query := model.Query().Join("testgroups", Q("testgroups.id", "=", "testaccounts.group_id"))
	expected := []JoinClause{
		{
			Direction: "INNER",
			Table:     "testgroups",
			On: []FilterClause{
				{Left: "testgroups.id", Operator: "=", Right: "testaccounts.group_id", Rule: "WHERE"},
			},
		},
	}
	assertJoins(t, expected, query.Config.Joins)

	query = model.Query().JoinLeft("testgroups", Q("testgroups.id", "=", "testaccounts.group_id"))
	expected[0].Direction = "LEFT"
	assertJoins(t, expected, query.Config.Joins)

	query = model.Query().JoinRight("testgroups", Q("testgroups.id", "=", "testaccounts.group_id"))
	expected[0].Direction = "RIGHT"
	assertJoins(t, expected, query.Config.Joins)

	query = model.Query().JoinFull("testgroups", Or(
		Q("testaccounts.group_id", "=", "testgroups.id"),
		Q("testaccounts.group_id", "IS", nil),
	))
	expected = []JoinClause{
		{
			Direction: "FULL",
			Table:     "testgroups",
			On: []FilterClause{
				{Rule: "("},
				{Left: "testaccounts.group_id", Operator: "=", Right: "testgroups.id", Rule: "WHERE"},
				{Rule: "OR"},
				{Left: "testaccounts.group_id", Operator: "IS", Right: nil, Rule: "WHERE"},
				{Rule: ")"},
			},
		},
	}
	assertJoins(t, expected, query.Config.Joins)
}

type testGroupsQuerySlice struct {
	Accounts OneToMany[testAccountsQuerySlice] `db:"group_id"`
	Id       int64                             `db:"id" db_primary:"true"`
	Name     string                            `db:"name" db_max_length:"100"`
}
type testAccountsQuerySlice struct {
	Group NullForeignKey[testGroupsQuerySlice] `db:"group_id" db_on_delete:"SET NULL"`
	Id    int64                                `db:"id" db_primary:"true"`
	Name  string                               `db:"name"`
}

func TestQuerySlice(t *testing.T) {
	defer func() {
		defaultDialect = nil
	}()
	SetDialect(testDialect{})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal("failed to open sqlmock database:", err)
	}
	defer db.Close()

	expect := func(t *testing.T, actual []*testAccountsQuerySlice, expected []testAccountsQuerySlice) {
		if len(actual) != len(expected) {
			t.Errorf(`Expected '%#v', got '%#v'`, expected, actual)
		}
		for i, account := range expected {
			if actual[i].Id != account.Id ||
				actual[i].Name != account.Name ||
				actual[i].Group.Valid != account.Group.Valid ||
				(actual[i].Group.Valid && (actual[i].Group.Row.Id != account.Group.Row.Id || actual[i].Group.Row.Name != account.Group.Row.Name)) {
				t.Errorf(`Expected '%#v', got '%#v'`, account, *actual[i])
			}
		}
	}

	query := Use[testAccountsQuerySlice]().Query()

	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "group_id"}).
			AddRow(1, "foo", 10).
			AddRow(2, "bar", nil).
			AddRow(3, "baz", nil))
	rs, _ := db.Query("SELECT")
	defer rs.Close()
	query.Rows = rs
	actual, err := query.slice(db)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	expect(t, actual, []testAccountsQuerySlice{
		{Id: 1, Name: "foo", Group: NullForeignKey[testGroupsQuerySlice]{Row: &testGroupsQuerySlice{Id: 10}, Valid: true}},
		{Id: 2, Name: "bar"},
		{Id: 3, Name: "baz"},
	})

	// Foreign keys.
	query.FetchRelated("Group")
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "group_id"}).
			AddRow(1, "foo", 10).
			AddRow(2, "bar", 20).
			AddRow(3, "baz", nil))
	rs, _ = db.Query("SELECT")
	defer rs.Close()
	query.Rows = rs

	mock.ExpectQuery(`SELECT|FILTER[{Left:id Operator:IN Right:[10,20] Rule:WHERE}]|`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(10, "Group 10").
			AddRow(20, "Group 20"))

	actual, err = query.slice(db)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	expect(t, actual, []testAccountsQuerySlice{
		{Id: 1, Name: "foo", Group: NullForeignKey[testGroupsQuerySlice]{Row: &testGroupsQuerySlice{Id: 10, Name: "Group 10"}, Valid: true}},
		{Id: 2, Name: "bar", Group: NullForeignKey[testGroupsQuerySlice]{Row: &testGroupsQuerySlice{Id: 20, Name: "Group 20"}, Valid: true}},
		{Id: 3, Name: "baz"},
	})

	// One to many.
	query2 := Use[testGroupsQuerySlice]().Query()
	query2.FetchRelated("Accounts")
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(10, "Group 10").
			AddRow(20, "Group 20"))

	rs2, _ := db.Query("SELECT")
	defer rs2.Close()
	query2.Rows = rs2

	mock.ExpectQuery(`SELECT|FILTER[{Left:id Operator:IN Right:[10,20] Rule:WHERE}]|`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "group_id"}).
			AddRow(1, "foo", 10).
			AddRow(2, "bar", 20).
			AddRow(3, "baz", 10))

	actual2, err := query2.slice(db)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	expected2 := make([][]map[string]interface{}, 0)
	expected2 = append(expected2, []map[string]interface{}{
		{
			"Id":      int64(1),
			"Name":    "foo",
			"GroupId": int64(10),
		},
		{
			"Id":      int64(3),
			"Name":    "baz",
			"GroupId": int64(10),
		},
	})
	expected2 = append(expected2, []map[string]interface{}{
		{
			"Id":      int64(2),
			"Name":    "bar",
			"GroupId": int64(20),
		},
	})
	if len(actual2) != len(expected2) {
		t.Errorf(`Expected 2 groups, got '%#v'`, actual2)
	}
	for i, expect2 := range expected2 {
		if len(actual2[i].Accounts.Rows) != len(expect2) {
			t.Errorf(`Expected '%#v' accounts, got '%#v'`, expect2, actual2[i].Accounts.Rows)
		} else {
			for j, actualAccount2 := range actual2[i].Accounts.Rows {
				if actualAccount2.Id != expect2[j]["Id"] ||
					actualAccount2.Name != expect2[j]["Name"] ||
					!actualAccount2.Group.Valid ||
					actualAccount2.Group.Row.Id != expect2[j]["GroupId"] {
					t.Errorf(`Expected '%#v' accounts, got '%#v'`, expect2, actual2[i].Accounts.Rows)
				}
			}
		}
	}
}
