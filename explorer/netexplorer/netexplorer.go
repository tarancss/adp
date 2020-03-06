// Package netexplorer implements the exploration of block transactions for monitored addresses and adding/removing monitored addresses.
package netexplorer

import (
	"log"
	"sync"

	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/store"
)

// Status possible values, control whether a NextExplorer is working or is/has to stop
const (
	WORK int = 0
	STOP int = 1
)

// NetExplorer contains the fields and data structures required to manage the exploring of a network or blockchain.
type NetExplorer struct {
	l      sync.Mutex             // l is a mutex to ensure concurrent updating of addresses in the map
	status int                    `json:"status" bson:"status"`
	Block  uint64                 `json:"block" bson:"block"` // last block parsed
	Bh     []string               `json:"bh" bson:"bh"`       // contains the last blocks hashes (from Block-1 to Block-maxBlocks)
	Bhi    int                    `json:"bhi" bson:"bhi"`     // index to last block's hash in Bh
	Map    map[string]interface{} `json:"map" bson:"map"`     // Map of addresses/transactions (string) to the required information
}

//New tries to load from DB a previously saved status of the net explorer or creates a new one with default values (start monitoring at block 1) if not present in DB. A slice of length=1 (only for one network) of addresses to monitor can be passed in 'l' and returns a NetExplorer object.
func New(net string, max int, l []store.ListenedAddresses, db store.DB) (*NetExplorer, error) {
	var ne NetExplorer
	var s store.NetExplorer
	var err error

	if s, err = db.LoadExplorer(net); err != nil {
		if err == store.ErrDataNotFound {
			// if "explorer" was not present in DB, then we just create from block 0
			ne.Block = 0
			ne.Bhi = 0
			ne.Bh = make([]string, max)
			ne.status = WORK
			err = nil
		} else {
			return nil, err
		}
	} else {
		ne.FromStore(s)
	}
	ne.Map = make(map[string]interface{})
	if len(l) == 1 {
		for _, a := range l[0].Addr {
			ne.Map[a.Addr] = "listen" // TODO, so far we only set a symbolic value "listen", but here we could  set more info, such as userId, etc...
		}
	}
	log.Printf("[%s] netexplorer.New %+v", net, ne)
	return &ne, nil
}

// ScanTxs detects if the To or From addresses are being monitored within the NetExplorer and if so, includes the transaction in the returned slice.
func (n *NetExplorer) ScanTxs(txs []types.Trans) (r []types.Trans, err error) {
	r = make([]types.Trans, 0, 4) // capacity = 4 is more than enough for a block!
	var ok bool
	n.l.Lock()
	defer n.l.Unlock()
	for _, tx := range txs {
		if _, ok = n.Map[tx.From].(string); ok {
			r = append(r, tx)
		} else if _, ok = n.Map[tx.To].(string); ok {
			r = append(r, tx)
		}
	}
	return
}

// Chained checks if the supplied hash is the last block's hash and so blocks are chained.
func (n *NetExplorer) Chained(hash string) bool {
	n.l.Lock()
	defer n.l.Unlock()
	return n.Bh[n.Bhi] == hash || n.Bh[n.Bhi] == ""
}

// UpdateChain updates NetExplorer fields with new block hash.
func (n *NetExplorer) UpdateChain(hash string, maxBlocks int) {
	n.l.Lock()
	defer n.l.Unlock()
	n.Block++
	n.Bhi++
	n.Bhi %= maxBlocks
	n.Bh[n.Bhi] = hash
}

// Add adds an object and its value to the monitoring map.
func (n *NetExplorer) Add(obj string, value interface{}) {
	n.l.Lock()
	defer n.l.Unlock()
	n.Map[obj] = value

}

// Del deletes a monitored object from the map returning its value. 'ok' is returned as false if the object was not being monitored.
func (n *NetExplorer) Del(obj string) (value interface{}, ok bool) {
	n.l.Lock()
	defer n.l.Unlock()
	value, ok = n.Map[obj]
	delete(n.Map, obj)
	return

}

// ToStore returns a store.NetExplorer struct to be saved to store.
func (n *NetExplorer) ToStore() store.NetExplorer {
	return store.NetExplorer{
		Block: n.Block,
		Bh:    n.Bh,
		Bhi:   n.Bhi,
		Map:   n.Map,
	}
}

// FromStore loads the NetExplorer with the values read from store.
func (n *NetExplorer) FromStore(s store.NetExplorer) {
	n.Block = s.Block
	n.Bh = s.Bh
	n.Bhi = s.Bhi
	n.Map = s.Map
}

// Stop sets status to STOP.
func (n *NetExplorer) Stop() {
	n.l.Lock()
	n.status = STOP
	n.l.Unlock()
}

// Start sets status to WORK.
func (n *NetExplorer) Start() {
	n.l.Lock()
	n.status = WORK
	n.l.Unlock()
}

// Status returns the current NetExplorer status.
func (n *NetExplorer) Status() int {
	n.l.Lock()
	defer n.l.Unlock()
	return n.status
}
