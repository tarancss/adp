// Package db implements the opening and graceful closing of database connections.
package db

import (
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/store/mongo"
	"github.com/tarancss/adp/lib/store/postgres"
)

const (
	MONGODB  string = "mongodb"
	POSTGRES string = "postgresql"
)

// New returns a new database connection according to the options (database type).
func New(options, connection string) (store.DB, error) {
	switch options {
	case MONGODB:
		return mongo.New(connection)
	case POSTGRES:
		println("postgresql connection TODO")

		return postgres.New(connection)
	}

	return nil, nil
}

// Close gracefully closes the database connection.
func Close(options string, dh store.DB) error {
	switch options {
	case MONGODB:
		return dh.(*mongo.Mongo).CloseMongo()
	case POSTGRES:
		// println("closing postgresql connection")
		return dh.(*postgres.Postgres).ClosePostgres()
	}

	return nil
}
