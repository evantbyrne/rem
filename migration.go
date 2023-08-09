package rem

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"
)

type Migration interface {
	Down(db *sql.DB) error
	Up(db *sql.DB) error
}

type MigrationLogs struct {
	CreatedAt     time.Time `db:"created_at"`
	Direction     string    `db:"direction" db_max_length:"10"`
	Id            int64     `db:"id" primary_key:"true"`
	MigrationType string    `db:"migration_type" db_max_length:"255"`
}

func MigrateDown(db *sql.DB, migrations []Migration) ([]string, error) {
	logs, latestIndex, err := migrateSetup(db, migrations)
	if err != nil {
		return logs, err
	}
	migrationLogs := Use[MigrationLogs]()

	for i := latestIndex; i > -1; i-- {
		migrationType := reflect.TypeOf(migrations[i]).String()
		logs = append(logs, "Migrating down to "+migrationType+"...")
		if err := migrations[i].Down(db); err != nil {
			return logs, errors.Join(fmt.Errorf("migration %s: failed", migrationType), err)
		}
		_, err := migrationLogs.Insert(db, &MigrationLogs{
			CreatedAt:     time.Now(),
			Direction:     "down",
			MigrationType: migrationType,
		})
		if err != nil {
			return logs, errors.Join(fmt.Errorf("migration %s: failed to insert migration logs", migrationType), err)
		}
	}

	return logs, nil
}

func migrateSetup(db *sql.DB, migrations []Migration) ([]string, int, error) {
	logs := make([]string, 0)
	migrationLogs := Use[MigrationLogs]()

	_, err := migrationLogs.TableCreate(db, TableCreateConfig{IfNotExists: true})
	if err != nil {
		return nil, -1, errors.Join(errors.New("rem: migrations setup: failed to create table for migration logs"), err)
	}

	latest, err := migrationLogs.Sort("-id").First(db)
	latestIndex := -1
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, -1, errors.Join(errors.New("rem: migrations setup: failed to get migrations list"), err)
		}
	} else {
		for i, migration := range migrations {
			if latest.MigrationType == reflect.TypeOf(migration).String() {
				if latest.Direction == "down" {
					latestIndex = i - 1
				} else {
					latestIndex = i
				}
				break
			}
		}
	}

	return logs, latestIndex, nil
}

func MigrateUp(db *sql.DB, migrations []Migration) ([]string, error) {
	logs, latestIndex, err := migrateSetup(db, migrations)
	if err != nil {
		return logs, err
	}
	migrationLogs := Use[MigrationLogs]()

	for i := latestIndex + 1; i < len(migrations); i++ {
		migrationType := reflect.TypeOf(migrations[i]).String()
		logs = append(logs, "Migrating up to "+migrationType+"...")
		if err := migrations[i].Up(db); err != nil {
			return logs, errors.Join(fmt.Errorf("rem: migration %s: failed", migrationType), err)
		}
		_, err := migrationLogs.Insert(db, &MigrationLogs{
			CreatedAt:     time.Now(),
			Direction:     "up",
			MigrationType: migrationType,
		})
		if err != nil {
			return logs, errors.Join(fmt.Errorf("rem: migration %s: failed to insert migration logs", migrationType), err)
		}
	}

	return logs, nil
}
