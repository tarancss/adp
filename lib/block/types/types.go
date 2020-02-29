// Defines common blockchain types
package types

import (
	"errors"
)

// Token is a blockchain asset
type Token struct {
	Name     string      `json:"name"`
	Symbol   string      `json:"symbol"`
	Decimals uint8       `json:"decimals"`
	Data     interface{} `json:"data"` // contains specific chain details
}

// Trans contains a simplified number of transaction fields. For the time being, we keep just one transfer from `From` to `To` but there are blockchains that have multiple transfers in one transaction.
type Trans struct {
	Block  string `json:"block"`
	Status uint8  `json:"status"`
	Hash   string `json:"hash"`
	From   string `json:"from"`
	To     string `json:"to"`
	Token  string `json:"token,omitempty"`
	Value  string `json:"value"`
	Data   string `json:"data,omitempty"`
	Gas    string `json:"gas"`
	Price  uint64 `json:"price"`
	Fee    uint64 `json:"fee"`
	Ts     uint32 `json:"ts"`
}

// Block contains a simplified list of block fields.
type Block struct {
	// contains other fields, but this ones are the important to us right now...
	Hash   string  `json:"hash"`
	PHash  string  `json:"parentHash"`
	Number string  `json:"number"`
	Ts     string  `json:"timestamp"`
	Tx     []Trans `json:"transactions"`
}

// Error codes
var (
	ErrBlockDecode         = errors.New("Unable to decode block data into Block type")
	ErrNoBlockNumber error = errors.New("ErrNoBlockNumber: Block data does not contain a block number")
	ErrNoTs          error = errors.New("ErrNoTs: Block data does not contain a timestamp")
	ErrNoHash        error = errors.New("ErrNoHash: Block data does not contain a hash")
	ErrNoParentHash  error = errors.New("ErrNoParentHash: Block data does not contain a parenthash")
	ErrNoBlock       error = errors.New("ErrNoBlock: Block not available yet")
	ErrNoTrx               = errors.New("ErrNoTrx: Transaction not found")
	ErrNoTrxHash           = errors.New("ErrNoTrxHash: Malformed tx data in block, field \"hash\" missing")
	ErrNoTrxInput          = errors.New("ErrNoTrxInput: Malformed tx data in block, field \"input\" missing")
	ErrNoTrxValue          = errors.New("ErrNoTrxValue: Malformed tx data in block, field \"value\" missing")
	ErrNoTrxFrom           = errors.New("ErrNoTrxFrom: Malformed tx data in block, field \"from\" missing")
	ErrTrxWrongLen         = errors.New("Malformed tx data in block, field \"input\" has wrong length for ERC20.Transfer method")
	ErrNoTrxStatus         = errors.New("Malformed tx data in block, field \"status\" missing")
	ErrNoTrxGasUsed        = errors.New("Malformed tx data in block, field \"gasUsed\" missing")
	ErrNoTrxGasPrice       = errors.New("Malformed tx data in block, field \"gasPrice\" missing")
	ErrWrongAmt            = errors.New("ErrWrongAmt: Amount length exceeds maximum (32)")
	ErrSendTokenData       = errors.New("SendTrx: cannot send token and data at same time!")
)
