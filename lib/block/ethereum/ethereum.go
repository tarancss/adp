// Package ethereum implements interface for ethereum networks.
package ethereum

import (
	"errors"
	"fmt"
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

// Init returns a connection to an ethereum node, using secret if necessary for authentication. maxBlocks is required
// to indicate how many blocks will be taken into account for uncle management.
func Init(node, secret string, maxBlocks int) (*Ethereum, error) {
	c := ethcli.Init(node, secret)
	if c == nil {
		return nil, errors.New("cannot connect to ethereum blockchain in" + node)
	}

	return &Ethereum{c: c, mb: maxBlocks}, nil
}

// MaxBlocks returns how many blocks will be taken into account for uncle management.
func (e *Ethereum) MaxBlocks() int {
	return e.mb
}

// AvgBlock returns the average time to mine a block in seconds.
func (e *Ethereum) AvgBlock() int {
	return 15 //nolint:gomnd // we could put this in the config file...
}

// Close ends a connection.
func (e *Ethereum) Close() {
	err := e.c.End()
	if err != nil {
		log.Printf("ethereum: error closing client: %v", err)
	}
}

// Balance loads the ether balance, and the token balance if specified, onto the provided big.Int pointers, or error
// otherwise.
func (e *Ethereum) Balance(address, token string) (ethBal, tokBal *big.Int, err error) {
	return e.c.GetBalance(address, token)
}

// GetBlock returns in response the block number requested. If full, it provides all the details of the transactions.
func (e *Ethereum) GetBlock(block uint64, full bool, response interface{}) (err error) {
	if err = e.c.GetBlockByNumber(block, full, response.(*map[string]interface{})); errors.Is(err, ethcli.ErrNoBlock) {
		err = types.ErrNoBlock
	}

	return
}

// DecodeBlock returns a struct with the values from the block data. It is used after a call to GetBlock.
func (e *Ethereum) DecodeBlock(t interface{}) (types.Block, error) {
	m, ok := t.(map[string]interface{})
	if !ok {
		return types.Block{}, types.ErrBlockDecode
	}

	hash, ok := m["hash"].(string)
	if !ok {
		return types.Block{}, types.ErrNoHash
	}

	pHash, ok := m["parentHash"].(string)
	if !ok {
		return types.Block{}, types.ErrNoParentHash
	}

	number, ok := m["number"].(string)
	if !ok {
		return types.Block{}, types.ErrNoBlockNumber
	}

	ts, ok := m["timestamp"].(string)
	if !ok {
		return types.Block{}, types.ErrNoTS
	}

	return types.Block{Hash: hash, PHash: pHash, Number: number, TS: ts}, nil
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
					err = types.ErrNoTrxInput

					return
				}

				if tmp == "0x" || (len(tmp) > 2 && len(tmp) <= 10) ||
					(len(tmp) > 10 &&
						tmp[2:10] != ethcli.ERC20transfer &&
						tmp[2:10] != ethcli.ERC20transfer256 &&
						tmp[2:10] != ethcli.ERC20transferFrom &&
						tmp[2:10] != ethcli.ERC20transferFrom256) {
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
						if tmp[2:10] == ethcli.ERC20transfer || tmp[2:10] == ethcli.ERC20transfer256 {
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
						} else if tmp[2:10] == ethcli.ERC20transferFrom || tmp[2:10] == ethcli.ERC20transferFrom256 {
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
				// Token has to be parsed from Data
				// timestamp should be got from block's ts
				// fee is gas*price but gas here is the one sent, not consumed!!
				txs[i].Status = ethcli.TrxPending // status should be got from TransactionReceipt
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

// Send executes a transaction in the blockchain with the given parameters returning the expected fee, the transaction
// hash or an error otherwise. If 'dryRun' is true, the transaction will not be sent to the blockchain but still a
// valid hash will be returned.
func (e *Ethereum) Send(fromAddress, toAddress, token, amount string, data []byte, key string, priceIn uint64,
	dryRun bool) (fee *big.Int, hash []byte, err error) {
	var price, gas uint64
	price, gas, hash, err = e.c.SendTrx(fromAddress, toAddress, token, amount, data, key, priceIn, dryRun)
	fee = new(big.Int).SetUint64(price)

	var gB *big.Int = new(big.Int).SetUint64(gas)
	fee = fee.Mul(fee, gB)

	return
}

// Get returns the details of the transaction for the given hash.
func (e *Ethereum) Get(hash string) (*types.Trans, error) {
	trx, err := e.c.GetTrx(hash)
	if err != nil {
		return nil, fmt.Errorf("cannot get transaction for hash %s: %w", hash, err)
	}

	return &types.Trans{
		Block:  strconv.FormatUint(trx.Blk, 10),
		Hash:   trx.Hash,
		From:   trx.From,
		To:     trx.To,
		Token:  "0x" + string(trx.Token),
		Value:  trx.Amount,
		Data:   string(trx.Data),
		Gas:    strconv.FormatUint(trx.Gas, 10),
		Price:  trx.Price,
		Fee:    trx.Fee,
		Status: trx.Status,
		TS:     uint32(trx.TS),
	}, nil
}
