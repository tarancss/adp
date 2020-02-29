// Package msg defines the interface for different message brokers.
//
package msg

import (
	"sync"

	"github.com/tarancss/adp/lib/block/types"
	mtype "github.com/tarancss/adp/lib/msg/types"
)

type MsgBroker interface {
	Setup(interface{}) error
	Close() error

	// methods for wallet service
	SendRequest(net string, r mtype.WalletReq) error
	GetEvents(net string, mut *sync.Mutex) (<-chan types.Trans, <-chan error, error)

	// methods for explorer service
	GetReqs(net string, mut *sync.Mutex) (<-chan mtype.WalletReq, <-chan error, error)
	SendTrans(net string, t []types.Trans) error
}
