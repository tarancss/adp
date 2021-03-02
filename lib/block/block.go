// Package block defines the interface required for all blockchain or network connections.
package block

import (
	"log"
	"math/big"

	"github.com/tarancss/adp/lib/block/ethereum"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/config"
)

// Chain is an interface that contains the required methods. It has been designed to be as much standard as possible,
// however, there may be specific blockchains or networks that would require different types or more methods.
type Chain interface {
	// member-type methods
	MaxBlocks() int // number of blocks that are controlled for orphans (uncles)
	AvgBlock() int  // average block mining rate in seconds
	// methods
	Close()
	Balance(account, token string, bal, tokBal *big.Int) error
	GetBlock(block uint64, full bool, response interface{}) error
	DecodeBlock(b interface{}) (types.Block, error)
	DecodeTxs(t interface{}) ([]types.Trans, error)
	GetToken(token string) (types.Token, error)
	Send(fromAddress, toAddress, token, amount string, data []byte, key string, priceIn uint64,
		dryRun bool) (fee *big.Int, hash []byte, err error)
	Get(hash string) (blk uint64, ts int32, fee uint64, status uint8, token, data []byte, to, from, amount string,
		err error)
}

// Init loads all the clients read from the config to blockchains into a map.
func Init(bc []config.BlockConfig) (m map[string]Chain, err error) {
	m = make(map[string]Chain)

	for _, block := range bc {
		// connect
		var tmp interface{}

		if block.Name == "ropsten" || block.Name == "rinkeby" || block.Name == "mainNet" {
			if tmp, err = ethereum.Init(block.Node, block.Secret, block.MaxBlocks); err != nil {
				return
			}

			m[block.Name] = tmp.(Chain)
		} else {
			log.Printf("Blockchain interface not defined for %s. Ignoring...\n", block.Name)
		}
	}

	return
}

// End closes gracefully all the blockchain clients opened.
func End(bc map[string]Chain) {
	for _, block := range bc {
		block.Close()
	}
}
