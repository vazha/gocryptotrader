package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sync"

	"github.com/vazha/gocryptotrader/database/drivers"
)

// Db holds all information for a database instance
type Db struct {
	SQL      *sql.DB
	DataPath string
	Config   *Config

	Connected bool
	Mu        sync.RWMutex
}

// Config holds all database configurable options including enable/disabled & DSN settings
type Config struct {
	Enabled                   bool   `json:"enabled"`
	Verbose                   bool   `json:"verbose"`
	Driver                    string `json:"driver"`
	drivers.ConnectionDetails `json:"connectionDetails"`
}

var (
	// DB Global Database Connection
	DB = &Db{}

	// MigrationDir which folder to look in for current migrations
	MigrationDir = filepath.Join("..", "..", "database", "migrations")

	// ErrNoDatabaseProvided error to display when no database is provided
	ErrNoDatabaseProvided = errors.New("no database provided")

	// SupportedDrivers slice of supported database driver types
	SupportedDrivers = []string{DBSQLite, DBSQLite3, DBPostgreSQL}

	// DefaultSQLiteDatabase is the default sqlite3 database name to use
	DefaultSQLiteDatabase = "gocryptotrader.db"
)

const (
	// DBSQLite const string for sqlite across code base
	DBSQLite = "sqlite"
	// DBSQLite3 const string for sqlite3 across code base
	DBSQLite3 = "sqlite3"
	// DBPostgreSQL const string for PostgreSQL across code base
	DBPostgreSQL = "postgres"
)
