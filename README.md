# Go REM

The next generation Golang ORM. The name Go REM stands for Relational Entity Mapper.

```go
type Accounts struct {
	Group rem.NullForeignKey[Groups] `db:"group_id"`
	Id    int64                      `db:"id" primary_key:"true"`
	Name  string                     `db:"name"`
}

type Groups struct {
	Accounts rem.OneToMany[Accounts] `related_column:"group_id"`
	Id       int64                   `db:"id" primary_key:"true"`
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

**Note:** Go REM is not yet stable and pre-1.0 releases may result in breaking changes.
