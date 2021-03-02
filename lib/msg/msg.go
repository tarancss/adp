// Package msg defines the interface for different message brokers.
//
package msg

import (
	"sync"

	"github.com/tarancss/adp/lib/block/types"
)

// Types of object for wallet requests.
const (
	EXIT    = -1
	ADDRESS = 0
	TX      = 1
)

// Actions to be applied to objects for wallet requests.
const (
	LISTEN   = 0
	UNLISTEN = 1
)

// WalletReq defines the message that wallet service publishes to explorer to ask to explore an object.
type WalletReq struct {
	Net  string `json:"net"`
	Type int    `json:"type"` // type of object
	Obj  string `json:"obj"`
	Act  int    `json:"act"` // action to be applied
}

type MsgBroker interface {
	Setup(interface{}) error
	Close() error

	// methods for wallet service
	SendRequest(net string, r WalletReq) error
	GetEvents(net string, mut *sync.Mutex) (<-chan types.Trans, <-chan error, error)

	// methods for explorer service
	GetReqs(net string, mut *sync.Mutex) (<-chan WalletReq, <-chan error, error)
	SendTrans(net string, t []types.Trans) error
}
