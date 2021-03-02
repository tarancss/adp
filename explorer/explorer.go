// Package explorer implements the blockchain explorer microservice. The explorer scans transactions in the networks
// mined blocks and sends events when a monitored address or account is involved in a transaction.
package explorer

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	ne "github.com/tarancss/adp/explorer/netexplorer"
	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/msg"
	"github.com/tarancss/adp/lib/store"
)

// Explorer implements an explorer service.
type Explorer struct {
	dbtype string
	db     store.DB
	bc     map[string]block.Chain     // map of blockchain clients
	nem    map[string]*ne.NetExplorer // map of net explorers
	mb     msg.MsgBroker
}

// New instantiates a new explorer service.
func New(dbtype string, db store.DB, mb msg.MsgBroker, bc map[string]block.Chain) *Explorer {
	return &Explorer{
		dbtype: dbtype,
		db:     db,
		bc:     bc,
		nem:    make(map[string]*ne.NetExplorer),
		mb:     mb,
	}
}

// Explore starts a go routine for each network available. The exploration of each network is controlled by a
// NetExplorer (see package explorer/netexplorer for details) and contains a map of the addresses being monitored and
// the current status of scanned blocks. The explorer consumes wallet requests to monitor new addresses. In case of
// graceful termination, the explorer will wait for all the blocks being scanned to finish and sending the events if
// any.
func (e *Explorer) Explore() chan string {
	ret := make(chan string, 1)
	// channel to wait for chain explorers
	w := make(chan string, len(e.bc))

	for net := range e.bc {
		// get listened addresses from DB
		addrs, err := e.db.GetAddresses([]string{net})
		if err != nil {
			log.Printf("[%s] Cannot load listened addresses from DB, err:%e", net, err)

			continue
		}

		if len(addrs) == 0 || len(addrs[0].Addr) == 0 {
			log.Printf("[%s] No listened addresses to explore in DB.", net)
		}
		// set listened address map
		// TODO: extra functionality - monitor transactions!!!
		if e.nem[net], err = ne.New(net, e.bc[net].MaxBlocks(), addrs, e.db); err != nil {
			log.Printf("[%s] netexplorer.New failed:%e", net, err)

			continue
		}
		// listen for wallet requests, if there are pending requests in the broker queues, they will be processed to DB
		// so getAddresses starts with all the data loaded
		if err = e.ManageWalletRequests(net); err != nil {
			log.Printf("[%s] Cannot consume wallet requests from broker, err:%e", net, err)

			continue
		}
		// Explore
		e.ExploreChain(net, w)
	}
	// routine to wait for all chains to complete exploring...
	go func() {
		for i := 1; i < len(e.bc)+1; i++ {
			log.Printf("Explore, channel %d/%d returned: %s", i, len(e.bc), <-w)
		}
		ret <- "Done!"
	}()

	return ret
}

// StopExplorer will send termination signals to all network explorer go routines.
func (e *Explorer) StopExplorer() {
	for _, nexp := range e.nem {
		nexp.Stop()
	}
}

// ExploreChain starts a network explorer go routine for blockchain named 'net'. When the routine ends, returns its
// error status via the 'ret' channel given so the calling routine can control graceful termination. When a network
// does not have any monitored addresses, the explorer will keep waiting and will not scan any mined blocks.
func (e *Explorer) ExploreChain(net string, ret chan string) {
	nexp := e.nem[net]

	log.Printf("[%s] Exploring at block %d... ", net, nexp.Block)

	go func() {
		var err error

		c := e.bc[net]

		defer func() {
			// save NetExplorer to DB
			errSave := e.db.SaveExplorer(net, nexp.ToStore())
			// write into channel
			ret <- "[" + net + "] Done!" + fmt.Sprintf(" err:%e", err) + fmt.Sprintf(" err2:%e", errSave)
		}()

		for nexp.Status() == ne.WORK {
			if len(nexp.Map) == 0 {
				// wait until there is something to explore for
				log.Printf("[%s] Waiting for something to explore", net)
				time.Sleep(time.Duration(c.AvgBlock()) * time.Second)

				continue
			}
			// get next block's data
			var b map[string]interface{}

			time.Sleep(1 * time.Second) // limit rate at max. 1 block per second!

			if err = c.GetBlock(nexp.Block+1, true, &b); err != nil {
				if errors.Is(err, types.ErrNoBlock) {
					// lets wait for a new block to be mined
					time.Sleep(time.Duration(c.AvgBlock()) * time.Second)

					continue
				} else {
					log.Printf("[%s] ExploreChain GetBlock b:%+v err:%e", net, b, err)
					nexp.Stop()

					return
				}
			}
			// decode Hash
			blk, err := c.DecodeBlock(b)
			if err != nil {
				return
			}

			log.Printf("[%s] Parsing block %d hash:%s pHash:%s", net, nexp.Block+1, blk.Hash, blk.PHash)
			// check block is chained
			if !nexp.Chained(blk.PHash) {
				// TODO: extra functionality FindLastSync
				log.Printf("[%s] Block %d is not chained!! \n%+v\n%d", net, nexp.Block+1, nexp.Bh, nexp.Bhi)
				nexp.Stop()

				return
			}

			// decode transactions
			if blk.Tx, err = c.DecodeTxs(b); err != nil {
				return
			}
			// sync'ed - store hash and update other data
			nexp.UpdateChain(blk.Hash, c.MaxBlocks())
			// Scan transactions
			r, _ := nexp.ScanTxs(blk.Tx)
			// send events
			if len(r) > 0 {
				err = e.mb.SendTrans(net, r)
				log.Printf("[%s] Sending %d events:%+v err:%e\n", net, len(r), r, err)
			}
			// save netExplorer status to DB
			if errSave := e.db.SaveExplorer(net, nexp.ToStore()); errSave != nil {
				log.Printf("[%s] Error saving NetExplorer to DB, err:%e", net, errSave)

				break
			}
		}
	}()
}

// ManageWalletRequests starts a go routine to receive and manage wallet requests for objects (addresses, ...) to be
// monitored for the blockchain named 'net'.
func (e *Explorer) ManageWalletRequests(net string) error {
	var mut *sync.Mutex = new(sync.Mutex)

	mut.Lock()

	reqCh, errCh, err := e.mb.GetReqs(net, mut)
	if err != nil {
		return fmt.Errorf("explorer: cannot get requests: %w", err)
	}

	nexp := e.nem[net]

	// launch request channel reader
	go func() {
		log.Printf("[%s] Start listening to wallet request channel", net)

		for {
			select {
			case req, ok := (<-reqCh):
				if !ok {
					log.Printf("[%s] Stop listening to wallet request channel", net)

					break
				}

				log.Printf("Received request %+v", req)
				// validate request
				if req.Net != net || (req.Type != msg.ADDRESS && req.Type != msg.TX) ||
					len(req.Obj) == 0 || (req.Act != msg.LISTEN && req.Act != msg.UNLISTEN) {
					log.Printf("[%s] Request has wrong net %s, wrong type %d, missing objext %s or wrong action %d",
						net, req.Net, req.Type, req.Obj, req.Act)
				}
				// process object
				if req.Type == msg.ADDRESS {
					a := store.Address{Addr: req.Obj}

					if req.Act == msg.LISTEN {
						// save it to DB
						if _, err := e.db.AddAddress(a, net); err != nil {
							log.Printf("[%s] Error adding WalletReq address to DB %e", net, err)
						}
						// include it in NetExplorer
						nexp.Add(req.Obj, "listen")
						log.Printf("[%s] Added object %s to NetExplorer %v %v %v %v", net, req.Obj,
							nexp.Block, nexp.Bh, nexp.Bhi, nexp.Map)
					} else {
						// delete from NetExplorer
						if _, ok := nexp.Del(req.Obj); !ok {
							log.Printf("[%s] Error deleting WalletReq address %s from NetExplorer. Not found. Ignoring...", net, req.Obj)
						}
						// delete from DB
						if err := e.db.RemoveAddress(a, net); err != nil {
							log.Printf("[%s] Error deleting WalletReq address from DB %e", net, err)
						}
						log.Printf("[%s] Removed object %s from NetExplorer %v %v %v %v", net, req.Obj,
							nexp.Block, nexp.Bh, nexp.Bhi, nexp.Map)
					}
				} else if req.Type == msg.TX {
					log.Printf("Un/listen to transaction TODO!!!!")
				}

				mut.Unlock()
			case e, ok := (<-errCh):
				if !ok {
					log.Printf("[%s] Stop listening to wallet request channel", net)

					break
				}

				log.Printf("[%s] Received error %+v", net, e)
			}
		}
	}()

	return nil
}
