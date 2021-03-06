// Package store defines the interface for database implementations to the wallet and explorer microservices.
package store

import (
	"errors"
)

// DB defines required methods for wallets and explorers.
type DB interface {
	// methods for wallet service
	AddAddress(Address, string) ([]byte, error)
	RemoveAddress(Address, string) error
	GetAddresses([]string) ([]ListenedAddresses, error)
	// methods for explorer service
	LoadExplorer(string) (NetExplorer, error)
	SaveExplorer(string, NetExplorer) error
	DeleteExplorer(string) error
}

var (
	ErrAddrNotFound = errors.New("address was not found in store")
	ErrDataNotFound = errors.New("data was not found in store")
)
