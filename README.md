# REM

The retro Golang ORM. **R**etro **E**ntity **M**apper.

```go
type Accounts struct {
	Group rem.NullForeignKey[Groups] `db:"group_id"`
	Id    int64                      `db:"id" db_primary:"true"`
	Name  string                     `db:"name"`
}

type Groups struct {
	Accounts rem.OneToMany[Accounts] `db:"group_id"`
	Id       int64                   `db:"id" db_primary:"true"`
	Name     string                  `db:"name" db_max_length:"100"`
}
```

```go
// Only one additional query is executed to fetch all related accounts.
groups, err := rem.Use[Groups]().
	FetchRelated("Accounts").
	Filter("id", "IN", []interface{}{10, 20, 30}).
	Sort("name", "-id").
	All(db)

if err != nil {
	panic(err)
}
for _, group := range groups {
	// group *Groups
	// group.Accounts.Rows []*Accounts
}
```

## Features

- PostgreSQL and MySQL dialects. SQLite coming soon.
- Data and schema migrations that use the same model syntax.
- Optimized foreign key and one-to-many prefetching.
- Interface extensible query builder. Can be used for database-specific features.
- Negligible performance difference from using database/sql directly.
- Decoupled from database/sql connections and drivers.
- Partially or fully fallback to a safely parameterized SQL format as desired.
- Zero code gen. Models are just structs that may have your own fields and methods.
- Standardized safety with explicitly null and not-null types.
- Transaction and golang context support.
- Subqueries, joins, selective fetching, map scanning, and more.

## Installation

The `main` branch contains the latest release. From the shell:

```
go get github.com/evantbyrne/rem
```

**Note:** REM is not yet stable and pre-1.0 releases may result in breaking changes.


## Contributing

Please post feature requests, questions, and other feedback to the [discussions board](https://github.com/evantbyrne/rem/discussions). Submit bug reports to the [issue tracker](https://github.com/evantbyrne/rem/issues).


## Dialects

REM supports PostgreSQL and MySQL. SQLite support is planned. To use a dialect, import the appropriate package and set it as the default once on application bootup.

```go
import (
	// Choose one:
	"github.com/evantbyrne/rem/mysqldialect"
	"github.com/evantbyrne/rem/pqdialect"
	"github.com/evantbyrne/rem/sqlitedialect"

	// Don't forget to import your database driver.
)
```

```go
// Choose one:
rem.SetDialect(mysqldialect.MysqlDialect{})
rem.SetDialect(pqdialect.PqDialect{})
rem.SetDialect(sqlitedialect.SqliteDialect{})

// Then connect to your database as usual.
db, err := sql.Open("<driver>", "<connection string>")
if err != nil {
	panic(err)
}
defer db.Close()
```


## Models

Models are structs that define table schemas.

```go
type Accounts struct {
	Id   int64  `db:"id" db_primary:"true"`
	Name string `db:"name" db_max_length:"100"`
	Junk string
}
```

In the above struct definition, `Id` and `Name` are columns in the `accounts`. Their columns are defined by the `db` field tag. `Id` is also an auto-incrementing primary key. `Name` has a maximum character length of `100`. The `Junk` field is ignored by REM.

After defining a model, register it once on application bootup, then query the database.

```go
// rem.Register[To]() caches computed structure of the model.
rem.Register[Accounts]()

// rem.Use[To]() returns a query builder for the model.
rows, err := rem.Use[Accounts]().All(db)

// You can also reuse the Model[To] instance returned by rem.Register[To]() and rem.Use[To]().
accounts := rem.Use[Accounts]()
rows1, err1 := accounts.Filter("name", "=", "foo").All(db)
rows2, err2 := accounts.Filter("name", "=", "bar").All(db)

// Register and use a different table with the same model.
rem.Register[Accounts](rem.Config{Table: "groups"})
groups := rem.Use[Accounts](rem.Config{Table: "groups"})
```


## Migrations

REM provides the migrations interface as a way to simplify schema and data changes. The interface is just two methods to implement:

```go
// github.com/evantbyrne/rem/migrations.go:
type Migration interface {
	Down(db *sql.DB) error
	Up(db *sql.DB) error
}
```

Models are defined in the same way within migrations as they are in the rest of the application. Here's an example:

```go
type Migration0001Accounts struct{}

func (m Migration0001Accounts) Up(db *sql.DB) error {
	// We embed the Accounts model to avoid colliding with the package-level Accounts model used for queries. You could also use `rem.Config` as demonstrated in the Models documentation section.
	type Accounts struct {
		Id   int64  `db:"id" db_primary:"true"`
		Name string `db:"name" db_max_length:"100"`
	}

	// Note that we don't use rem.Register[To](), because we don't want to cache the model structure used within the migration.
	_, err := rem.Use[Accounts]().TableCreate(db)
	return err
}

func (m Migration0001Accounts) Down(db *sql.DB) error {
	// Fields aren't needed for dropping a table.
	type Accounts struct{}

	_, err := rem.Use[Accounts]().TableDrop(db)
	return err
}
```

Then run the migrations:

```go
logs, err := rem.MigrateUp(db, []rem.Migration{
	Migration0001Accounts{},
	// More migrations...
})
// logs []string
// For example: {"Migrating up to Migration0001Accounts..."}
```

REM will create a `migrationlogs` table to track which migrations have been run. Execution of subsequent migrations will stop if an error is returned. Use `rem.MigrateDown(*sql.DB, []rem.Migration)` to run migrations in reverse.


## Fields

### Field Types

REM determines column types based on Go field types. The following table shows the default column types for each Go primative.

**Note:** REM uses special Go types for nullable columns. Don't use pointers for model fields.

Go | MySQL | PostgreSQL | SQLite
--- | --- | --- | ---
`bool` | `BOOLEAN` | `BOOLEAN` | `BOOLEAN`\[1\]
`[]byte` | - | - | -
`int8` | `TINYINT` | `SMALLINT` | `INTEGER`
`int16` | `SMALLINT` | `SMALLINT` | `INTEGER`
`int32` | `INTEGER` | `INTEGER` | `INTEGER`
`int64` | `BIGINT` | `BIGINT` | `INTEGER`
`float32` | `FLOAT` | - | `REAL`
`float64` | `DOUBLE` | `DOUBLE PRECISION` | `REAL`
`string` | `VARCHAR`,`TEXT`\[2\] | `VARCHAR`,`TEXT`\[2\] | `TEXT`
`time.Time` | `DATETIME`\[3\] | `TIMESTAMP`\[4\] | `DATETIME`

\[1\] SQLite `BOOLEAN` behaves as an `INTEGER` internally. The SQLite driver should automatically convert `bool` field values to `0` or `1` when parameterized.

\[2\] The `VARCHAR` column type is used for `string` and `sql.NullString` fields when the `db_max_length` field tag is provided. Otherwise, `TEXT` is used.

\[3\] Go's most popular MySQL driver requires adding the `parseTime=true` GET parameter to the connection string to properly scan into `time.Time` and `sql.NullTime` fields.

\[4\] The PostgreSQL dialect defaults to `WITHOUT TIME ZONE` for time types. Add the `db_time_zone:"true"` field tag to use `WITH TIME ZONE` instead.

Columns are not nullable by default. REM uses the standard `database/sql` package types to represent nullable columns.

Not Null | Nullable
--- | ---
`bool` | `sql.NullBool`
`float64` | `sql.NullFloat64`
`int16` | `sql.NullInt16`
`int32` | `sql.NullInt32`
`int64` | `sql.NullInt64`
`rem.ForeignKey[To]` | `rem.NullForeignKey[To]`
`string` | `sql.NullString`
`time.Time` | `sql.NullTime`

Primary keys are specified with the `db_primary:"true"` field tag. All models must have a primary key. Integer fields that are primary keys will auto-increment.

```go
// An auto-incrementing primary key.
type A struct {
	Id int64  `db:"id" db_primary:"true"`
}

// VARCHAR primary key with no default value.
type B struct {
	Guid string `db:"guid" db_max_length:"36" db_primary:"true"`
}
```

### Default

The `db_default` field tag applies a default value to columns. It accepts any string.

**Note:** Values provided to `db_default` are not escaped or otherwise sanitized.

```go
// This timestamp uses the SQL function now() for its default value.
type Logs struct {
	CreatedAt time.Time `db:"created_at" db_default:"now()"`
	// ...
}
```

### Unique

The `db_unique:"true"` field tag applies a unique constraint to a column.

```go
type Accounts struct {
	Nickname string `db:"created_at" db_unique:"true"`
	// ...
}
```

### Custom Types

Custom column types can be set using the `db_type` field tag, which accpets any string value.

**Note:** Values provided to `db_type` are not escaped or otherwise sanitized.

```go
// An example of using PostgreSQL's JSONB type.
type A struct {
	Id   int64  `db:"id" db_primary:"true"`
	Data []byte `db:"data" db_type:"JSONB NOT NULL"`
}

// db_type takes priority over all other field tags, including primary key typing.
type B struct {
	Guid string `db:"guid" db_type:"CHAR(36) NOT NULL" db_primary:"true"`
}
```

Custom Go types may also be used for model fields, but they must implement the `driver.Valuer` and `sql.Scanner` interfaces in additon to being supported by your database driver.

### Foreign Keys

Foreign keys are specified with the `rem.ForeignKey[To]` and `rem.NullForeignKey[To]` field types. REM automatically matches the foreign key column type to the primary key of the target model.

On the other side of the relation, use `rem.OneToMany[To]`. On both sides of the relation, the `db` field tag signifies the column on the `rem.ForeignKey[To]` side.

```go
type Groups struct {
	Members rem.OneToMany[Members] `db:"group_id"`
	Id      int64                  `db:"id" db_primary:"true"`
}

type Members struct {
	Group rem.ForeignKey[Groups] `db:"group_id"`
	Id    int64                  `db:"id" db_primary:"true"`
}
```

See [Fetch Related](#fetch-related) for information on querying relationships effeciently.

Relations may also be queried lazily.

```go
// Lazily fetch from a one-to-many field.
group, err := rem.Use[Groups]().Filter("id", "=", 100).First(db)
if err != nil {
	panic(err)
}
accounts, err := group.Members.All(db)
// accounts []*Accounts

// Lazily fetch from a foreign key field.
account, err := rem.Use[Accounts]().Filter("id", "=", 200).First(db)
if err != nil {
	panic(err)
}
group, err := account.Group.Fetch(db)
// group *Groups
```

Foreign key `ON DELETE` and `ON UPDATE` constraints, such as `CASCADE` or `SET NULL`, may be set with the `db_on_delete` and `db_on_update` field tags.

```go
type Members struct {
	Group rem.NullForeignKey[Groups] `db:"group_id" db_on_delete:"SET NULL" db_on_update:"SET NULL"`
	// ...
}
```


## Reference

### All

Executes a query and returns a list of records.

```go
accounts, err := rem.Use[Accounts]().All(db)
// accounts []*Accounts

accounts, err := rem.Use[Accounts]().AllToMap(db)
// accounts []map[string]interface{}
```


### Context

Pass a Golang context to queries.

```go
var ctx context.Context
rem.Use[Accounts]().Context(ctx).All(db)
```


### Count

The `Count` convenience method returns the number of matching records.

```go
count, err := rem.Use[Accounts]().Filter("id", "<", 100).Count(db)
// count uint
```


### Delete

The `Delete` convenience method deletes matching records.

```go
results, err := rem.Use[Accounts]().Filter("id", "=", 100).Delete(db)
// results sql.Result
```


### Dialect

Set the dialect for a specific query. This takes priority over the default dialect.

```go
rem.Use[Accounts]().Dialect(mysqldialect.Dialect{}).All(db)
```


### Fetch Related

REM can optimize foreign key and one-to-many record lookups. This is done with the `FetchRelated` method, which takes any number of strings that represent the relation fields to prefetch.

Regardless of which side of the relationship you start from or how many records are being fetched initially, REM will only execute one additional query for prefetching.

```go
// Model definitions for Groups <->> Accounts relationship.
type Accounts struct {
	Group rem.ForeignKey[Groups] `db:"group_id"`
	Id    int64                  `db:"id" db_primary:"true"`
	Name  string                 `db:"name" db_max_length:"100"`
}

type Groups struct {
	Accounts rem.OneToMany[Accounts] `db:"group_id"`
	Id       int64                   `db:"id" db_primary:"true"`
	Name     string                  `db:"name" db_max_length:"100"`
}
```

```go
groups, err := rem.Use[Groups]().FetchRelated("Accounts").All(db)
for _, group := range groups {
	// group *Groups
	// group.Accounts.Rows []*Accounts
}

accounts, err := rem.Use[Accounts]().FetchRelated("Group").All(db)
for _, account := range accounts {
	// account *Accounts
	// account.Group.Row *Groups
	// account.Group.Valid bool
}
```


### Filter

REM provides a few mechanisms for filtering database results. The most basic is the `Filter` method, which takes a left side value, operator, and right side value.

Typically, the left side is a column name, which is represented by a `string`.

The operator is always a `string`. Use uppercase for alphabetical operators such as `"IN"`, `"NOT IN"`, `"IS"`, `"IS NOT"`, `"EXISTS"`, and so on.

The right side may be any value supported by the database driver for parameterization.

The left and right sides may also be `rem.DialectStringerWithArgs`, `rem.DialectStringer`, or `rem.SqlUnsafe`. These types are used for more advanced filtering, such as subqueries, joins, or SQL function calls.

```go
rem.Use[Accounts]().Filter("id", ">=", 100).All(db)

// Filters may be chained. This is equivalent to "SELECT * FROM accounts WHERE id >= 100 AND id < 200".
rem.Use[Accounts]().
	Filter("id", ">=", 100).
	Filter("id", "<", 200).
	All(db)

// Chain filters with an OR using `rem.Q`. This is equivalent to "SELECT * FROM accounts WHERE name = 'foo' OR (id >= 100 AND id < 200").
rem.Use[Accounts]().
	FilterOr(
		rem.Q("name", "=", "foo"),
		rem.And(
			rem.Q("id", ">=", 100),
			rem.Q("id", "<", 200),
		),
	).
	All(db)

// Complex chained and nested filters are fully supported.
rem.Use[Accounts]().
	FilterAnd(
		rem.Q("a", "=", "foo"),
		rem.Or(
			rem.Q("ab", "=", "bar"),
			rem.And(
				rem.Q("abc1", ">", 100),
				rem.Q("abc2", "<", 200),
			),
		),
	).
	FilterOr(
		rem.Q("b1", "IS", nil),
		rem.Q("b2", "IN", interface{}{10, 20, 30}),
	).
	All(db)
```

#### Custom SQL

Safely parameterized SQL may be embedded via the `rem.Sql()` and `rem.Param()` functions. String arguments to `rem.Sql()` are not escaped or otherwise sanitized. `rem.Param()` arguments are parameterized by the database driver.

```go
// SQL: SELECT * FROM logs WHERE data.tags ?| array[$1]
// Parameters: []interface{}{"foo"}
rem.Use[Logs]().
	Filter("data.tags", "?|", rem.Sql("array[", rem.Param("foo"), "]")).
	All(db)
```

Raw SQL may also be embedded into either the left or right side of a filters via the `rem.Unsafe()` function.

**Note:** Values provided to `rem.Unsafe()` are not escaped or otherwise sanitized. Only use this function with trusted values.

```go
// SQL: SELECT * FROM accounts WHERE upper(name) = $1
// Parameters: []interface{}{"FOO"}
rem.Use[Accounts]().
	Filter(rem.Unsafe("upper(name)"), "=", "FOO").
	All(db)
```

#### Subqueries

REM allows subqueries to be embedded via the standard query syntax.

```go
// SQL: SELECT * FROM accounts WHERE id IN (SELECT account_id FROM groups WHERE name = $1)
// Parameters: []interface{}{"Group 1"}
rem.Use[Accounts]().
	Filter("id", "IN", rem.Use[Groups]().Select("account_id").Filter("name", "=", "Group 1")).
	All(db)
```

The `rem.Exists()` and `rem.NotExists()` functions are provided as a convenience for subqueries that only need to check for the existence of a record.

`rem.Column()` is also used in the following example to properly handle the column name that is used on the right side of a filter.

```go
// SQL: SELECT * FROM groups WHERE EXISTS (SELECT * FROM accounts WHERE accounts.group_id = groups.id)
rem.Use[Groups]().
	FilterAnd(
		rem.Exists(rem.Use[Accounts]().Filter("accounts.group_id", "=", rem.Column("groups.id")))
	).
	All(db)
```


### First

The `First` convenience method returns a single record. A `sql.ErrNoRows` error is returned if no matching records are found.

```go
account, err := rem.Use[Accounts]().Filter("id", "=", 1).First(db)
// account *Accounts

account, err := rem.Use[Accounts]().Filter("id", "=", 1).FirstToMap(db)
// account map[string]interface{}
```


### Insert

The `Insert` method adds new records to the database.

The first argument is a `*sql.DB` instance.

The second argument is a pointer to the new record.

**Note:** Zero-valued primary keys aren't included in inserts via the `Insert` method.

```go
account := &Accounts{
	Name: "New Name",
}

results, err := rem.Use[Accounts]().Insert(db, account)
// results sql.Result
```

REM also provides a `UpdateMap` convenience method that updates matching records with all columns provided by a `map[string]interface{}`.

**Note:** Zero-valued primary keys **will** be included when provided to inserts via the `InsertMap` method.

```go
account := map[string]interface{}{
	"name": "New Name",
}

results, err := rem.Use[Accounts]().InsertMap(db, account)
```


### Join

The `Join`, `JoinFull`, `JoinLeft`, and `JoinRight` methods are for performing their respective types of SQL joins.

The first argument is the table to join.

The second argument takes any number of filters to join on.

```go
rows, err := rem.Use[Accounts]().
	Select("accounts.id", "accounts.name", rem.As("groups.name", "group_name")).
	Join("groups", rem.Q("groups.id", "=", rem.Column("accounts.group_id"))).
	AllToMap(db)

// Use a custom model.
type AccountsWithGroupName struct {
	GroupName string `db:"group_name"`
	Id        string `db:"id" db_primary:"true"`
	Name      string `db:"name"`
}

rows, err := rem.Use[AccountsWithGroupName](rem.Config{Table: "accounts"}).
	Select(rem.As("accounts.id", "id"), rem.As("accounts.name", "name"), rem.As("groups.name", "group_name")).
	Join("groups", rem.Q("groups.id", "=", rem.Column("accounts.group_id"))).
	All(db)

// Use Query() to join without selecting columns.
rows, err := rem.Use[Accounts]().
	Query().
	JoinFull("groups", rem.Or(
		rem.Q("groups.id", "IS", nil),
		rem.Q("groups.id", "=", rem.Column("accounts.group_id")),
	).
	AllToMap(db)
```


### Limit and Offset

The `Limit` and `Offset` methods both take a single `int64` argument.

```go
// LIMIT 10
rem.Use[Accounts]().Limit(10).All(db)

// LIMIT 10 OFFSET 20
rem.Use[Accounts]().Limit(10).Offset(20).All(db)
```


### Scan Map

The `ScanMap` convenience method converts a `map[string]interface{}` into a model pointer.

```go
data := map[string]interface{}{
	"id":   100,
	"name": "New Name",
}

account, err := rem.Use[Accounts].ScanMap(data)
// account *Accounts
```


### Select

By default, queries scans all columns on the model. The `Select` method takes any number of strings, which when present, represent the only columns to scan. It also accepts `rem.DialectStringer`, and `rem.SqlUnsafe` values for special cases.

```go
// SELECT id FROM accounts
rem.Use[Accounts]().Select("id").All(db)

// SELECT id, UPPER(name) as name FROM accounts
rem.Use[Accounts]().Select("id", rem.Unsafe("UPPER(name) as name")).All(db)
```


### Sort

The `Sort` method takes any number of strings, which represent columns. Using `-` as a prefix will sort in descending order.

```go
// ORDER BY name ASC
rem.Use[Accounts]().Sort("name").All(db)

// ORDER BY name DESC
rem.Use[Accounts]().Sort("-name").All(db)

// ORDER BY name ASC, id DESC
rem.Use[Accounts]().Sort("name", "-id").All(db)
```


### SQL All

Executes a raw SQL query with parameters and returns a list of records.

```go
accounts, err := rem.Use[Accounts]().SqlAll(db, "select * from accounts where id >= ?", 100)
// accounts []*Accounts

accounts, err := rem.Use[Accounts]().SqlAllToMap(db, "select * from accounts where id >= ?", 100)
// accounts []map[string]interface{}
```


### Table Column Add

The `TableColumnAdd` method adds a column to a table. A field must exist in the model struct for the column to be added.

```go
type Accounts struct {
	Id      int64  `db:"id" db_primary:"true"`
	Name    string `db:"name"`
	IsAdmin bool   `db:"is_admin"`
}

_, err := rem.Use[Accounts]().TableColumnAdd(db, "is_admin")
```


### Table Column Drop

The `TableColumnDrop` method drops a column to a table.

```go
_, err := rem.Use[Accounts]().TableColumnDrop(db, "is_admin")
```


### Table Create

The `TableCreate` method creates a table for the model.

```go
_, err := rem.Use[Accounts]().TableCreate(db)

// Override the table name.
_, err := rem.Use[Accounts](rem.Config{Table: "users"}).TableCreate(db)

// Only create the table if it doesn't exist.
_, err := rem.Use[Accounts]().TableCreate(db, rem.TableCreateConfig{IfNotExists: true})
```


### Table Drop

The `TableDrop` method drops a table for the model.

```go
_, err := rem.Use[Accounts]().TableDrop(db)

// Override the table name.
_, err := rem.Use[Accounts](rem.Config{Table: "users"}).TableDrop(db)

// Only drop the table if it exists.
_, err := rem.Use[Accounts]().TableDrop(db, rem.TableDropConfig{IfExists: true})
```


### To Map

The `ToMap` convenience method converts a model pointer into a `map[string]interface{}`. Keys on the returned map are column names.

**Note:** Zero-valued primary keys are excluded from the returned map.

**Note:** Fields that implement the `driver.Valuer` interface are converted to their `Value()` representation. For example, a `sql.NullString` will be converted to either `string` or `nil`.

```go
account := &Accounts{
    Id:   100,
    Name: "New Name",
}

data := rem.Use[Accounts]().ToMap(account)
// data map[string]interface{}
```


### Transaction

REM supports transactions via the `Transaction(*sql.Tx)` method.

```go
tx, _ := db.Begin()

_, err := rem.Use[Accounts]().
	Filter("id", "=", 100).
	Transaction(tx).
	Delete(db)

if err != nil {
	tx.Rollback()
	panic(err)
}

err = tx.Commit()
if err != nil {
	panic(err)
}
```


### Update

The `Update` method updates matching records.

The first argument is a `*sql.DB` instance.

The second argument is a pointer to the updated record.

The third argument is a spread of columns to update. If no columns are provided, the update will fail. This minor annoyance is by design and is intended to ensure that column updates are intentional.

```go
account := &Accounts{
	Id:   200,
	Name: "New Name",
}

// The `name` column will be updated, but `id` won't.
results, err := rem.Use[Accounts]().
	Filter("id", "=", 100).
	Update(db, account, "name")

// results sql.Result
```

REM also provides a `UpdateMap` convenience method that updates matching records with all columns provided by a `map[string]interface{}`.

```go
account := map[string]interface{}{
	"name": "New Name",
}

results, err := rem.Use[Accounts]().
	Filter("id", "=", 100).
	UpdateMap(db, account)
```
