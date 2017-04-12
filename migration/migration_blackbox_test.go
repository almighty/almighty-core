package migration_test

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"testing"

	config "github.com/almighty/almighty-core/configuration"
	"github.com/almighty/almighty-core/log"
	"github.com/almighty/almighty-core/migration"
	"github.com/almighty/almighty-core/resource"

	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	errs "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// fn defines the type of function that can be part of a migration steps
type fn func(tx *sql.Tx) error

const (
	// Migration version where to start testing
	initialMigratedVersion = 45
	databaseName           = "test"
)

var (
	conf       *config.ConfigurationData
	migrations migration.Migrations
	dialect    gorm.Dialect
	gormDB     *gorm.DB
	sqlDB      *sql.DB
)

func setupTest() {
	var err error
	conf, err = config.GetConfigurationData()
	if err != nil {
		panic(fmt.Errorf("Failed to setup the configuration: %s", err.Error()))
	}
	configurationString := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s connect_timeout=%d",
		conf.GetPostgresHost(),
		conf.GetPostgresPort(),
		conf.GetPostgresUser(),
		conf.GetPostgresPassword(),
		conf.GetPostgresSSLMode(),
		conf.GetPostgresConnectionTimeout(),
	)

	db, err := sql.Open("postgres", configurationString)
	defer db.Close()
	if err != nil {
		panic(fmt.Errorf("Cannot connect to database: %s\n", err))
	}

	db.Exec("DROP DATABASE " + databaseName)

	_, err = db.Exec("CREATE DATABASE " + databaseName)
	if err != nil {
		panic(err)
	}

	migrations = migration.GetMigrations()
}

func TestMigrations(t *testing.T) {
	resource.Require(t, resource.Database)

	setupTest()

	configurationString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		conf.GetPostgresHost(),
		conf.GetPostgresPort(),
		conf.GetPostgresUser(),
		conf.GetPostgresPassword(),
		databaseName,
		conf.GetPostgresSSLMode(),
		conf.GetPostgresConnectionTimeout(),
	)
	var err error
	sqlDB, err = sql.Open("postgres", configurationString)
	defer sqlDB.Close()
	if err != nil {
		panic(fmt.Errorf("Cannot connect to DB: %s\n", err))
	}
	gormDB, err = gorm.Open("postgres", configurationString)
	defer gormDB.Close()
	if err != nil {
		panic(fmt.Errorf("Cannot connect to DB: %s\n", err))
	}
	dialect = gormDB.Dialect()
	dialect.SetDB(sqlDB)

	// We migrate the new database until initialMigratedVersion
	t.Run("TestMigration44", testMigration44)

	// Insert dummy test data to our database
	assert.Nil(t, runSQLscript(sqlDB, "044-insert-test-data.sql"))

	t.Run("TestMigration45", testMigration45)
	t.Run("TestMigration46", testMigration46)
	t.Run("TestMigration47", testMigration47)
	t.Run("TestMigration48", testMigration48)
	t.Run("TestMigration49", testMigration49)
	t.Run("TestMigration50", testMigration50)

	// Perform the migration
	if err := migration.Migrate(sqlDB, databaseName); err != nil {
		t.Fatalf("Failed to execute the migration: %s\n", err)
	}
}

func testMigration44(t *testing.T) {
	var err error
	m := migrations[:initialMigratedVersion]
	for nextVersion := int64(0); nextVersion < int64(len(m)) && err == nil; nextVersion++ {
		var tx *sql.Tx
		tx, err = sqlDB.Begin()
		if err != nil {
			t.Fatalf("Failed to start transaction: %s\n", err)
		}

		if err = migration.MigrateToNextVersion(tx, &nextVersion, m, databaseName); err != nil {
			t.Errorf("Failed to migrate to version %d: %s\n", nextVersion, err)

			if err = tx.Rollback(); err != nil {
				t.Fatalf("error while rolling back transaction: ", err)
			}
			t.Fatal("Failed to migrate to version after rolling back")
		}

		if err = tx.Commit(); err != nil {
			t.Fatalf("Error during transaction commit: %s\n", err)
		}
	}
}

func testMigration45(t *testing.T) {
	migrationToVersion(sqlDB, migrations[:(initialMigratedVersion+1)], (initialMigratedVersion + 1))

	assert.True(t, gormDB.HasTable("work_items"))
	assert.True(t, dialect.HasColumn("work_items", "execution_order"))
	assert.True(t, dialect.HasIndex("work_items", "order_index"))

	assert.Nil(t, runSQLscript(sqlDB, "045-update-work-items.sql"))
}

func testMigration46(t *testing.T) {
	migrationToVersion(sqlDB, migrations[:(initialMigratedVersion+2)], (initialMigratedVersion + 2))

	assert.True(t, gormDB.HasTable("oauth_state_references"))
	assert.True(t, dialect.HasColumn("oauth_state_references", "referrer"))
	assert.True(t, dialect.HasColumn("oauth_state_references", "id"))

	assert.Nil(t, runSQLscript(sqlDB, "046-insert-oauth-states.sql"))
}

func testMigration47(t *testing.T) {
	migrationToVersion(sqlDB, migrations[:(initialMigratedVersion+3)], (initialMigratedVersion + 3))

	assert.True(t, gormDB.HasTable("codebases"))
	assert.True(t, dialect.HasColumn("codebases", "type"))
	assert.True(t, dialect.HasColumn("codebases", "url"))
	assert.True(t, dialect.HasColumn("codebases", "space_id"))
	assert.True(t, dialect.HasIndex("codebases", "ix_codebases_space_id"))

	assert.Nil(t, runSQLscript(sqlDB, "047-insert-codebases.sql"))
}

func testMigration48(t *testing.T) {
	migrationToVersion(sqlDB, migrations[:(initialMigratedVersion+4)], (initialMigratedVersion + 4))

	assert.True(t, dialect.HasIndex("iterations", "ix_name"))

	// This script execution has to fail
	assert.NotNil(t, runSQLscript(sqlDB, "048-unique-idx-failed-insert.sql"))
}

func testMigration49(t *testing.T) {
	migrationToVersion(sqlDB, migrations[:(initialMigratedVersion+5)], (initialMigratedVersion + 5))

	assert.True(t, dialect.HasIndex("areas", "ix_area_name"))

	// Tests that migration 49 set the system.are to the work_items and its value
	// is 71171e90-6d35-498f-a6a7-2083b5267c18
	rows, err := sqlDB.Query("SELECT count(*), fields->>'system.area' FROM work_items where fields != '{}' GROUP BY fields")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var fields string
		var count int
		err = rows.Scan(&count, &fields)
		assert.True(t, count == 2)
		assert.True(t, fields == "71171e90-6d35-498f-a6a7-2083b5267c18")
	}
}

func testMigration50(t *testing.T) {
	migrationToVersion(sqlDB, migrations[:(initialMigratedVersion+6)], (initialMigratedVersion + 6))

	assert.True(t, dialect.HasColumn("users", "company"))

	assert.Nil(t, runSQLscript(sqlDB, "050-users-add-column-company.sql"))
}

// runSQLscript loads the given filename from the packaged SQL test files and
// executes it on the given database. Golang text/template module is used
// to handle all the optional arguments passed to the sql test files
func runSQLscript(db *sql.DB, sqlFilename string) error {
	var tx *sql.Tx
	tx, err := db.Begin()
	if err != nil {
		return errs.New(fmt.Sprintf("Failed to start transaction: %s\n", err))
	}
	if err := executeSQLTestFile(sqlFilename)(tx); err != nil {
		log.Warn(nil, nil, "Failed to execute data insertion of version: %s\n", err)
		if err = tx.Rollback(); err != nil {
			return errs.New(fmt.Sprintf("error while rolling back transaction: ", err))
		}
	}
	if err = tx.Commit(); err != nil {
		return errs.New(fmt.Sprintf("Error during transaction commit: %s\n", err))
	}

	return nil
}

// executeSQLTestFile loads the given filename from the packaged SQL files and
// executes it on the given database. Golang text/template module is used
// to handle all the optional arguments passed to the sql test files
func executeSQLTestFile(filename string, args ...string) fn {
	return func(db *sql.Tx) error {
		data, err := Asset(filename)
		if err != nil {
			return errs.WithStack(err)
		}

		if len(args) > 0 {
			tmpl, err := template.New("sql").Parse(string(data))
			if err != nil {
				return errs.WithStack(err)
			}
			var sqlScript bytes.Buffer
			writer := bufio.NewWriter(&sqlScript)
			err = tmpl.Execute(writer, args)
			if err != nil {
				return errs.WithStack(err)
			}
			// We need to flush the content of the writer
			writer.Flush()
			_, err = db.Exec(sqlScript.String())
		} else {
			_, err = db.Exec(string(data))
		}

		return errs.WithStack(err)
	}
}

func migrationToVersion(db *sql.DB, m migration.Migrations, version int64) {
	var (
		tx  *sql.Tx
		err error
	)
	tx, err = db.Begin()
	if err != nil {
		panic(fmt.Errorf("Failed to start transaction: %s\n", err))
	}

	if err = migration.MigrateToNextVersion(tx, &version, m, databaseName); err != nil {
		panic(fmt.Errorf("Failed to migrate to version %d: %s\n", version, err))
	}

	if err = tx.Commit(); err != nil {
		panic(fmt.Errorf("Error during transaction commit: %s\n", err))
	}
}
