package rem

import (
	"database/sql"
	"sort"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func TestModelScanMap(t *testing.T) {
	type testGroups struct {
		Id   int64  `db:"id" db_primary:"true"`
		Name string `db:"name" db_max_length:"100"`
	}
	type testAccounts struct {
		EditedAt sql.NullTime               `db:"edited_at"`
		Group    NullForeignKey[testGroups] `db:"group_id" db_on_delete:"SET NULL"`
		Id       int64                      `db:"id" db_primary:"true"`
		Name     string                     `db:"name"`
	}
	model := Use[testAccounts]()

	data := []map[string]interface{}{
		{
			"id":        int64(1),
			"name":      "foo",
			"edited_at": time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
			"group_id":  int64(10),
		},
		{
			"id":        int64(2),
			"name":      "bar",
			"edited_at": nil,
			"group_id":  int64(20),
		},
		{
			"id":        int64(3),
			"name":      "baz",
			"edited_at": nil,
			"group_id":  nil,
		},
	}
	expected := []testAccounts{
		{
			Id:   1,
			Name: "foo",
			EditedAt: sql.NullTime{
				Time:  time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
				Valid: true,
			},
			Group: NullForeignKey[testGroups]{
				Row:   &testGroups{Id: 10},
				Valid: true,
			},
		},
		{
			Id:   2,
			Name: "bar",
			Group: NullForeignKey[testGroups]{
				Row:   &testGroups{Id: 20},
				Valid: true,
			},
		},
		{Id: 3, Name: "baz"},
	}
	for i, row := range data {
		actual, err := model.ScanMap(row)
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}
		if actual.Id != expected[i].Id ||
			actual.Name != expected[i].Name ||
			actual.EditedAt != expected[i].EditedAt ||
			actual.Group.Valid != expected[i].Group.Valid ||
			(actual.Group.Valid && *actual.Group.Row != *expected[i].Group.Row) {
			t.Errorf("Expected '%+v', got '%+v'", expected[i], actual)
		}
	}
}

type testGroupsModelToMap struct {
	Accounts OneToMany[testAccountsModelToMap] `db:"group_id"`
	Id       int64                             `db:"id" db_primary:"true"`
	Name     string                            `db:"name" db_max_length:"100"`
}
type testAccountsModelToMap struct {
	EditedAt sql.NullTime                         `db:"edited_at"`
	Group    NullForeignKey[testGroupsModelToMap] `db:"group_id" db_on_delete:"SET NULL"`
	Id       int64                                `db:"id" db_primary:"true"`
	Name     string                               `db:"name"`
}

func assertMapDeepEquals(t *testing.T, actual map[string]interface{}, expected map[string]interface{}) {
	actualKeys := maps.Keys(actual)
	expectKeys := maps.Keys(expected)
	sort.Strings(actualKeys)
	sort.Strings(expectKeys)
	if !slices.Equal(actualKeys, expectKeys) {
		t.Errorf("Expected '%#v', got '%#v'", expected, actual)
	}
	for _, key := range actualKeys {
		actualValue := actual[key]
		expectValue := expected[key]

		switch av := actualValue.(type) {
		case []map[string]interface{}:
			ev, ok := expectValue.([]map[string]interface{})
			if !ok {
				t.Errorf("Expected\n'%#v', got\n'%#v'", expected, actual)
			}
			for i := range av {
				assertMapDeepEquals(t, av[i], ev[i])
			}
		case map[string]interface{}:
			ev, ok := expectValue.(map[string]interface{})
			if !ok {
				t.Errorf("Expected\n'%#v', got\n'%#v'", expected, actual)
			}
			assertMapDeepEquals(t, av, ev)
		default:
			if av != expectValue {
				t.Errorf("Expected '%#v', got '%#v'", expected, actual)
			}
		}
	}
}

func TestModelToJsonMap(t *testing.T) {
	model := Use[testAccountsModelToMap]()

	rows := []testAccountsModelToMap{
		{
			Id:   1,
			Name: "foo",
			EditedAt: sql.NullTime{
				Time:  time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
				Valid: true,
			},
			Group: NullForeignKey[testGroupsModelToMap]{
				Row:   &testGroupsModelToMap{Id: 10},
				Valid: true,
			},
		},
		{
			Id:   2,
			Name: "bar",
			Group: NullForeignKey[testGroupsModelToMap]{
				Row:   &testGroupsModelToMap{Id: 20},
				Valid: true,
			},
		},
		{Id: 3, Name: "baz"},
	}
	expected := []map[string]interface{}{
		{
			"id":       int64(1),
			"name":     "foo",
			"editedat": time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
			"group": map[string]interface{}{
				"accounts": []map[string]interface{}{},
				"name":     "",
				"id":       int64(10),
			},
		},
		{
			"id":       int64(2),
			"name":     "bar",
			"editedat": nil,
			"group": map[string]interface{}{
				"accounts": []map[string]interface{}{},
				"name":     "",
				"id":       int64(20),
			},
		},
		{
			"id":       int64(3),
			"name":     "baz",
			"editedat": nil,
			"group":    nil,
		},
	}

	for i, row := range rows {
		actual := model.ToJsonMap(&row)
		assertMapDeepEquals(t, actual, expected[i])
	}

	groupsModel := Use[testGroupsModelToMap]()
	groups := []testGroupsModelToMap{
		{
			Id:   1,
			Name: "foo",
		},
		{
			Id:   2,
			Name: "bar",
		},
	}
	expected = []map[string]interface{}{
		{
			"accounts": []map[string]interface{}{},
			"id":       int64(1),
			"name":     "foo",
		},
		{
			"accounts": []map[string]interface{}{},
			"id":       int64(2),
			"name":     "bar",
		},
	}
	for i, row := range groups {
		actual := groupsModel.ToJsonMap(&row)
		assertMapDeepEquals(t, actual, expected[i])
	}
}

func TestModelToMap(t *testing.T) {
	model := Use[testAccountsModelToMap]()

	rows := []testAccountsModelToMap{
		{
			Id:   1,
			Name: "foo",
			EditedAt: sql.NullTime{
				Time:  time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
				Valid: true,
			},
			Group: NullForeignKey[testGroupsModelToMap]{
				Row:   &testGroupsModelToMap{Id: 10},
				Valid: true,
			},
		},
		{
			Id:   2,
			Name: "bar",
			Group: NullForeignKey[testGroupsModelToMap]{
				Row:   &testGroupsModelToMap{Id: 20},
				Valid: true,
			},
		},
		{Id: 3, Name: "baz"},
	}
	expected := []map[string]interface{}{
		{
			"id":        int64(1),
			"name":      "foo",
			"edited_at": time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
			"group_id":  int64(10),
		},
		{
			"id":        int64(2),
			"name":      "bar",
			"edited_at": nil,
			"group_id":  int64(20),
		},
		{
			"id":        int64(3),
			"name":      "baz",
			"edited_at": nil,
			"group_id":  nil,
		},
	}
	for i, row := range rows {
		actual, err := model.ToMap(&row)
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}
		if !maps.Equal(actual, expected[i]) {
			t.Errorf("Expected '%#v', got '%#v'", expected[i], actual)
		}
	}

	groupsModel := Use[testGroupsModelToMap]()
	groups := []testGroupsModelToMap{
		{
			Id:   1,
			Name: "foo",
		},
		{
			Id:   2,
			Name: "bar",
		},
	}
	expected = []map[string]interface{}{
		{
			"id":   int64(1),
			"name": "foo",
		},
		{
			"id":   int64(2),
			"name": "bar",
		},
	}
	for i, row := range groups {
		actual, err := groupsModel.ToMap(&row)
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}
		if !maps.Equal(actual, expected[i]) {
			t.Errorf("Expected '%#v', got '%#v'", expected[i], actual)
		}
	}
}

func TestRegister(t *testing.T) {
	defer func() {
		registeredModels = make(map[string]interface{})
	}()
	type testModel struct {
		Id   int64  `db:"id" db_primary:"true"`
		Name string `db:"name"`
	}
	m1 := Use[testModel]()
	m2 := Register[testModel]()
	m3 := Use[testModel]()
	m4 := Use[testModel](Config{Table: "testmodelwithconfig"})
	if m1 == m2 || m1 == m3 {
		t.Errorf("Expected '%#v' to be different from '%#v' and '%#v'", m1, m2, m3)
	}
	if m2 != m3 {
		t.Errorf("Expected '%#v', got '%#v'", m2, m3)
	}
	if m2 == m4 {
		t.Errorf("Expected '%#v' to be different from '%#v'", m2, m4)
	}
}

func TestScanToMap(t *testing.T) {
	type testAccounts struct {
		EditedAt sql.NullTime `db:"edited_at"`
		Id       int64        `db:"id" db_primary:"true"`
		Name     string       `db:"name"`
	}
	accounts := Use[testAccounts]()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal("failed to open sqlmock database:", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "edited_at"}).
		AddRow(1, "foo", time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC)).
		AddRow(2, "bar", nil).
		AddRow(3, "baz", nil)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	rs, _ := db.Query("SELECT")
	defer rs.Close()

	expected := []map[string]interface{}{
		{
			"id":        int64(1),
			"name":      "foo",
			"edited_at": time.Date(2009, time.January, 2, 3, 0, 0, 0, time.UTC),
		},
		{
			"id":        int64(2),
			"name":      "bar",
			"edited_at": nil,
		},
		{
			"id":        int64(3),
			"name":      "baz",
			"edited_at": nil,
		},
	}
	i := 0
	for rs.Next() {
		row, err := accounts.ScanToMap(rs)
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}
		if !maps.Equal(row, expected[i]) {
			t.Errorf("Expected '%#v', got '%#v'", expected[i], row)
		}
		i++
	}
}

func TestUse(t *testing.T) {
	type testAccounts struct {
		EditedAt sql.NullTime `db:"edited_at"`
		Id       int64        `db:"id" db_primary:"true"`
		Name     string       `db:"name"`
	}
	type testGroups struct {
		Id       int64                   `db:"id" db_primary:"true"`
		Name     string                  `db:"name" db_max_length:"100"`
		Accounts OneToMany[testAccounts] `db:"group_id"`
	}
	groups := Use[testGroups]()
	columns := maps.Keys(groups.Fields)
	sort.Strings(columns)
	expectedColumns := []string{"Accounts", "id", "name"}
	expectedPrimaryColumn := "id"
	expectedPrimaryField := "Id"
	expectedTable := "testgroups"
	if !slices.Equal(columns, expectedColumns) {
		t.Errorf("Expected '%+v', got '%+v'", expectedColumns, columns)
	}
	if groups.PrimaryColumn != expectedPrimaryColumn {
		t.Errorf("Expected '%s', got '%s'", expectedPrimaryColumn, groups.PrimaryColumn)
	}
	if groups.PrimaryField != expectedPrimaryField {
		t.Errorf("Expected '%s', got '%s'", expectedPrimaryField, groups.PrimaryField)
	}
	if groups.Table != expectedTable {
		t.Errorf("Expected '%s', got '%s'", expectedTable, groups.Table)
	}
}
