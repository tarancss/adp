// Defines some constant values and types for message brokers.
package types

// Types of object for wallet requests
const (
	EXIT    = -1
	ADDRESS = 0
	TX      = 1
)

// Actions to be applied to objects for wallet requests
const (
	LISTEN   = 0
	UNLISTEN = 1
)

// WalletReq defines the message that wallet service publishes to explorer to ask to explore an object
type WalletReq struct {
	Net  string `json:"net"`
	Type int    `json:"type"` // type of object
	Obj  string `json:"obj"`
	Act  int    `json:"act"` // action to be applied
}
