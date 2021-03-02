// Package types common blockchain types.
package types

import (
	"errors"
)

// Token is a blockchain asset.
type Token struct {
	Name     string      `json:"name"`
	Symbol   string      `json:"symbol"`
	Decimals uint8       `json:"decimals"`
	Data     interface{} `json:"data"` // contains specific chain details
}

// Trans contains a simplified number of transaction fields. For the time being, we keep just one transfer from `From`
// to `To` but there are blockchains that have multiple transfers in one transaction.
type Trans struct {
	Block  string `json:"block"`
	Hash   string `json:"hash"`
	From   string `json:"from"`
	To     string `json:"to"`
	Token  string `json:"token,omitempty"`
	Value  string `json:"value"`
	Data   string `json:"data,omitempty"`
	Gas    string `json:"gas"`
	Price  uint64 `json:"price"`
	Fee    uint64 `json:"fee"`
	Status uint8  `json:"status"`
	TS     uint32 `json:"ts"`
}

// Block contains a simplified list of block fields.
type Block struct {
	// contains other fields, but this ones are the important to us right now...
	Hash   string  `json:"hash"`
	PHash  string  `json:"parentHash"`
	Number string  `json:"number"`
	TS     string  `json:"timestamp"`
	Tx     []Trans `json:"transactions"`
}

// Error codes.
var (
	ErrBlockDecode   = errors.New("unable to decode block data into Block type")
	ErrNoBlockNumber = errors.New("block data does not contain a block number")
	ErrNoTS          = errors.New("block data does not contain a timestamp")
	ErrNoHash        = errors.New("block data does not contain a hash")
	ErrNoParentHash  = errors.New("block data does not contain a parenthash")
	ErrNoBlock       = errors.New("block not available yet")
	ErrNoTrx         = errors.New("transaction not found")
	ErrNoTrxHash     = errors.New("malformed tx data in block, field 'hash' missing")
	ErrNoTrxInput    = errors.New("malformed tx data in block, field 'input' missing")
	ErrNoTrxValue    = errors.New("malformed tx data in block, field 'value' missing")
	ErrNoTrxFrom     = errors.New("malformed tx data in block, field 'from' missing")
	ErrTrxWrongLen   = errors.New("malformed tx data in block, field 'input' has wrong length for ERC20.Transfer")
	ErrNoTrxStatus   = errors.New("malformed tx data in block, field 'status' missing")
	ErrNoTrxGasUsed  = errors.New("malformed tx data in block, field 'gasUsed' missing")
	ErrNoTrxGasPrice = errors.New("malformed tx data in block, field 'gasPrice' missing")
	ErrWrongAmt      = errors.New("amount length exceeds maximum (32)")
	ErrSendTokenData = errors.New("cannot send token and data at same time")
)
