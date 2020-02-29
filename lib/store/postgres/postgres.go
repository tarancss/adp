// Implements the interface for PostgreSQL
package postgres

import (
	"github.com/tarancss/adp/lib/store"

	"database/sql"
	_ "github.com/lib/pq" // load the postgres driver that is used by the system
)

type Postgres struct {
	db *sql.DB
}

func New(connection string) (store.DB, error) {
	var err error
	var p Postgres
	p.db, err = sql.Open("postgres", connection)
	return &p, err
}

// DbClose will close any database connection. Must be called at termination time.
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
