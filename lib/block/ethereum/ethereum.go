// Implements interface for ethereum networks
package ethereum

import (
	"errors"
	"log"
	"math/big"
	"strconv"

	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/ethcli"
)

// Ethereum implements a connection to an ethereum-type chain.
type Ethereum struct {
	c  *ethcli.EthCli
	mb int
}

// Ethereum ERC20 token methodID (keccak-256 of the function name and arguments)
const (
	ERC20transfer256     = "a9059cbb" // transfer(address,uint256)
	ERC20transferFrom256 = "23b872dd" // transferFrom(address,address,uint256)
	ERC20transfer        = "6cb927d8" // transfer(address,uint)
	ERC20transferFrom    = "a978501e" // transferFrom(address,address,uint)
)

// Transaction status constants
const (
	TrxPending uint8 = 0
	TrxFailed  uint8 = 1
	TrxSuccess uint8 = 2
)

// Init returns a connection to an ethereum node, using secret if necessary for authentication. maxBlocks is required to indicate how many blocks will be taken into account for uncle management.
func Init(node, secret string, maxBlocks int) (*Ethereum, error) {
	var c *ethcli.EthCli
	var err error
	if c = ethcli.Init(node, secret); c == nil {
		err = errors.New("Cannot connect to ethereum blockchain in" + node)
	}
	return &Ethereum{c: c, mb: maxBlocks}, err
}

// MaxBlocks returns how many blocks will be taken into account for uncle management.
func (e *Ethereum) MaxBlocks() int {
	return e.mb
}

// AvgBlock returns the average time to mine a block in seconds.
func (e *Ethereum) AvgBlock() int {
	return 15 // we could put this in the config file...
}

// Close ends a connection
func (e *Ethereum) Close() {
	e.c.End()
}

// Balance loads the ether balance, and the token balance if specified, onto the provided big.Int pointers, or error otherwise.
func (e *Ethereum) Balance(address, token string, ethBal, tokBal *big.Int) error {
	return e.c.GetBalance(address, token, ethBal, tokBal)
}

// GetBlock returns in response the block number requested. If full, it provides all the details of the transactions.
func (e *Ethereum) GetBlock(block uint64, full bool, response interface{}) (err error) {
	if err = e.c.GetBlockByNumber(block, full, response.(*map[string]interface{})); err == ethcli.ErrNoBlock {
		err = types.ErrNoBlock
	}
	return
}

// DecodeBlock returns a struct with the values from the block data. It is used after a call to GetBlock.
func (e *Ethereum) DecodeBlock(t interface{}) (b types.Block, err error) {
	m, ok := t.(map[string]interface{})
	if !ok {
		err = types.ErrBlockDecode
		return
	}
	if b.Hash, ok = m["hash"].(string); !ok {
		err = types.ErrNoHash
		return
	}
	if b.PHash, ok = m["parentHash"].(string); !ok {
		err = types.ErrNoParentHash
		return
	}
	if b.Number, ok = m["number"].(string); !ok {
		err = types.ErrNoBlockNumber
		return
	}
	if b.Ts, ok = m["timestamp"].(string); !ok {
		err = types.ErrNoTs
		return
	}
	return
}

// DecodeTxs returns a slice of transactions from the block data. It is used after a call to GetBlock.
func (e *Ethereum) DecodeTxs(t interface{}) (txs []types.Trans, err error) {
	var txList []interface{}
	var txObj map[string]interface{}

	m, ok := t.(map[string]interface{})
	if !ok {
		err = types.ErrNoTrx
		return
	}
	if txList, ok = m["transactions"].([]interface{}); !ok {
		err = types.ErrNoTrx
		return
	}

	if len(txList) > 0 {
		txs = make([]types.Trans, len(txList))
		switch txList[0].(type) {
		case string:
			for i := 0; i < len(txList); i++ {
				txs[i].Hash = txList[i].(string) // only transaction hashes
			}
		case map[string]interface{}:
			// full data of the transactions
			for i := 0; i < len(txList); i++ {
				txObj = txList[i].(map[string]interface{})
				if txs[i].Block, ok = txObj["blockNumber"].(string); !ok {
					err = types.ErrNoBlockNumber
					return
				}
				if txs[i].Hash, ok = txObj["hash"].(string); !ok {
					err = types.ErrNoTrxHash
					return
				}
				if txs[i].To, ok = txObj["to"].(string); !ok {
					continue // contract creation, so we dont care about this transaction's details
				}
				// input
				var tmp string
				if tmp, ok = txObj["input"].(string); !ok {
					println("No input found!!!")
					err = types.ErrNoTrxInput
					return
				}
				if tmp == "0x" || (len(tmp) > 2 && len(tmp) <= 10) || (len(tmp) > 10 && tmp[2:10] != ERC20transfer && tmp[2:10] != ERC20transfer256 && tmp[2:10] != ERC20transferFrom && tmp[2:10] != ERC20transferFrom256) {
					// this is an ether transfer
					if txs[i].Value, ok = txObj["value"].(string); !ok {
						err = types.ErrNoTrxValue
						return
					}
					if txs[i].From, ok = txObj["from"].(string); !ok {
						err = types.ErrNoTrxFrom
						return
					}
					// To is already done
					txs[i].Data = tmp

				} else {
					// this is a token transaction
					if len(tmp) > 10 {
						if tmp[2:10] == ERC20transfer || tmp[2:10] == ERC20transfer256 {
							if len(tmp) >= 138 {
								if txs[i].From, ok = txObj["from"].(string); !ok {
									err = types.ErrNoTrxFrom
									return
								}
								// To comes in "input" after 24 padded 0s
								txs[i].To = "0x" + tmp[10+24:74]

								// Value, trimming left zeroes
								var j int
								for j = 74; j < 138 && tmp[j] == '0'; j++ {
								}
								if j%2 == 1 {
									j-- // keep Value with even hex-digits
								}
								txs[i].Value = "0x" + tmp[j:138]

							} else {
								err = types.ErrTrxWrongLen
								return
							}
						} else if tmp[2:10] == ERC20transferFrom || tmp[2:10] == ERC20transferFrom256 {
							if len(tmp) >= 202 {
								// From comes in "input" after 24 padded 0s, then To after 24 padded 0s
								txs[i].From = "0x" + tmp[10+24:74]
								txs[i].To = "0x" + tmp[74+24:138]
								// Value, trimming left zeroes
								var j int
								for j = 138; j < 202 && tmp[j] == '0'; j++ {
								}
								if j%2 == 1 {
									j--
								}
								txs[i].Value = "0x" + tmp[j:202]
							} else {
								err = types.ErrTrxWrongLen
								return
							}
						}
						// Token it's the smart contract address that comes in "to"
						txs[i].Token, _ = txObj["to"].(string)
						// Data is empty
					} else {
						log.Printf("WARN: very short \"input\" field in block's transaction...%s\n", tmp)
					}
				}

				if txs[i].Gas, ok = txObj["gas"].(string); !ok {
					err = types.ErrNoTrxGasUsed
					return
				}
				if tmp, ok = txObj["gasPrice"].(string); !ok {
					err = types.ErrNoTrxGasPrice
					return
				}
				if txs[i].Price, err = strconv.ParseUint(tmp, 0, 64); err != nil {
					return
				}
				// timestamp should be got from block's ts
				// fee is gas*price but gas here is the one sent, not consumed!!
				txs[i].Status = TrxPending // status should be got from TransactionReceipt
				// Token has to be parsed from Data
			}
		default:
			log.Printf("NODE ERROR: unknown txList type %T\n", t)
		}
	}
	return
}

// GetToken returns the name, symbol and decimals of a valid ERC20 token.
func (e *Ethereum) GetToken(token string) (t types.Token, err error) {
	if t.Name, err = e.c.GetTokenName(token); err != nil {
		return
	}
	if t.Symbol, err = e.c.GetTokenSymbol(token); err != nil {
		return
	}
	var dec uint64
	if dec, err = e.c.GetTokenDecimals(token); err != nil {
		return
	}
	t.Decimals = uint8(dec)
	// ... TODO IcoOffer...
	return
}

// Send executes a transaction in the blockchain with the given parameters returning the expected fee, the transaction hash or an error otherwise.
func (e *Ethereum) Send(fromAddress, toAddress, token, amount string, data []byte, key string, priceIn uint64, dryRun bool) (fee *big.Int, hash []byte, err error) {

	var price, gas uint64
	price, gas, hash, err = e.c.SendTrx(fromAddress, toAddress, token, amount, data, key, priceIn, dryRun)
	fee = new(big.Int).SetUint64(price)
	var gB *big.Int = new(big.Int).SetUint64(gas)
	fee = fee.Mul(fee, gB)
	return
}

// Get returns the details of the transaction for the given hash.
func (e *Ethereum) Get(hash string) (blk uint64, ts int32, fee uint64, status uint8, token, data []byte, to, from, amount string, err error) {
	// var price, gas uint64
	blk, ts, _, _, status, fee, token, data, to, from, amount, err = e.c.GetTrx(hash)
	return
}
