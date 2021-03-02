// Package postgres implements the interface for PostgreSQL (TODO).
package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" //nolint:gci // load the postgres driver that is used by the system

	"github.com/tarancss/adp/lib/store"
)

type Postgres struct {
	db *sql.DB
}

// New returns a postgres client connection to the specified database in 'connection'.
func New(connection string) (*Postgres, error) {
	db, err := sql.Open("postgres", connection)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to DB in %s: %w", connection, err)
	}

	return &Postgres{db: db}, nil
}

// ClosePostgres will close any database connection. Must be called at termination time.
func (p *Postgres) ClosePostgres() error {
	return p.db.Close()
}

func (p *Postgres) AddAddress(a store.Address, net string) ([]byte, error) {
	println("postgres: AddAddress TODO!!!")

	return []byte{0x00}, nil
}

func (p *Postgres) RemoveAddress(a store.Address, net string) error {
	println("postgres: RemoveAddress TODO!!!")

	return nil
}

func (p *Postgres) GetAddresses(net []string) (addrs []store.ListenedAddresses, err error) {
	println("postgres: GetAddresses TODO!!!")

	return
}

func (p *Postgres) LoadExplorer(net string) (ne store.NetExplorer, err error) {
	println("postgres: LoadExplorer TODO!!!")

	return
}

func (p *Postgres) SaveExplorer(net string, ne store.NetExplorer) (err error) {
	println("postgres: SaveExplorer TODO!!!")

	return
}

func (p *Postgres) DeleteExplorer(net string) (err error) {
	println("postgres: DeleteExplorer TODO!!!")

	return
}
