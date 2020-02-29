// package explorer implements the blockchain explorer microservice. The explorer scans transactions in the networks mined blocks and send events when a monitored address or account is involved in a transaction.
package explorer

import (
	"fmt"
	"log"
	"sync"
	"time"

	ne "github.com/tarancss/adp/explorer/netexplorer"
	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/msg"
	mtype "github.com/tarancss/adp/lib/msg/types"
	"github.com/tarancss/adp/lib/store"
)

// Explorer servive contains the data necessary to deliver the service
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

// Explore starts a go routine for each network available. The exploration of each network is controled by a NetExplorer (see package explorer/netexplorer for details) and contains a map of the addresses being monitored and the current status of scanned blocks. The explorer consumes wallet requests to monitor new addresses. In case of graceful termination, the explorer will wait for all the blocks being scanned to finish and sending the events if any.
func (e *Explorer) Explore() (ret chan string) {
	ret = make(chan string, 1)
	// channels to wait for chain explorers
	w := make([]chan string, 0, len(e.bc))
	var err error
	for net, _ := range e.bc {
		// get listened addresses from DB
		var addrs []store.ListenedAddresses
		if addrs, err = e.db.GetAddresses([]string{net}); err != nil {
			log.Printf("[%s] Cannot load listened addresses from DB, err:%e", net, err)
			continue
		}
		if len(addrs) == 0 || len(addrs[0].Addr) == 0 {
			log.Printf("[%s] No listened addresses to explore in DB.", net)
		}
		// set listened address map
		// TODO monitor transactions!!!
		if e.nem[net], err = ne.New(net, e.bc[net].MaxBlocks(), addrs, e.db); err != nil {
			log.Printf("[%s] netexplorer.New failed:%e", net, err)
			continue
		}

		// listen for wallet requests, if there are pending requests in the broker queues, they will be processes to DB so getAddresses starts with all the data loaded
		if err = e.ManageWalletRequests(net); err != nil {
			log.Printf("[%s] Cannot consume wallet requests from broker, err:%e", net, err)
			continue
		}
		// Explore
		w = append(w, e.ExploreChain(net))
	}

	// routine to wait for all chains to complete exploring...
	go func() {
		for i, ch := range w {
			log.Printf("Explore, waiting for channel %d/%d", i+1, len(w))
			log.Printf("Explore, channel %d/%d: %s", i+1, len(w), <-ch)
		}
		ret <- "Done!"
	}()

	return
}

// StopExplorer will send termination signals to all network explorer go routines.
func (e *Explorer) StopExplorer() {
	for _, nexp := range e.nem {
		nexp.Stop()
	}
}

// ExploreChain starts a network explorer go routine. Returns a channel used to control its graceful termination. When a network does not have any monitored addresses, the explorer will keep waiting and will not scan any mined blocks.
func (e *Explorer) ExploreChain(net string) (ret chan string) {
	var err error
	var c block.Chain = e.bc[net]
	var nexp *ne.NetExplorer = e.nem[net]

	ret = make(chan string, 1)
	log.Printf("[%s] Exploring at block %d... ", net, nexp.Block)

	go func() {
		defer func() {
			// save NetExplorer to DB
			errSave := e.db.SaveExplorer(net, nexp.ToStore())
			// write into channel
			ret <- "[" + net + "] Done!" + fmt.Sprintf(" err:%e", err) + fmt.Sprintf(" err2:%e", errSave)
		}()

		for nexp.Status() == ne.WORK {
			if len(nexp.Map) == 0 {
				// wait until there is something to explore for
				// log.Printf("[%s] Waiting for something to explore", net)
				time.Sleep(time.Duration(c.AvgBlock()) * time.Second)
			} else {
				// get next block's data
				var b map[string]interface{}
				time.Sleep(1 * time.Second) // limit rate at max. 1 block per second!
				if err = c.GetBlock(nexp.Block+1, true, &b); err != nil {
					if err == types.ErrNoBlock {
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
				var blk types.Block
				if blk, err = c.DecodeBlock(b); err != nil {
					return
				}
				log.Printf("[%s] Parsing block %d hash:%s pHash:%s", net, nexp.Block+1, blk.Hash, blk.PHash)
				// check block is chained
				if nexp.Chained(blk.PHash) {
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
				} else {
					// FindLastSync
					log.Printf("[%s] Block %d is not chained!! \n%+v\n%d", net, nexp.Block+1, nexp.Bh, nexp.Bhi)
					nexp.Stop()
					return
					// TODO
				}
			}
		}
	}()
	return
}

// ManageWalletRequests starts a go routine for each blockchain available to receive wallet requests for addresses to be monitored.
func (e *Explorer) ManageWalletRequests(net string) error {
	var mut *sync.Mutex = new(sync.Mutex)

	mut.Lock()
	reqCh, errCh, err := e.mb.GetReqs(net, mut)
	if err != nil {
		return err
	}
	nexp, _ := e.nem[net]

	// launch request channel reader
	go func() {
		log.Printf("[%s] Start listening to wallet request channel", net)
		for req := range reqCh {
			log.Printf("Received request %+v", req)
			// validate request
			if req.Net != net || (req.Type != mtype.ADDRESS && req.Type != mtype.TX) || len(req.Obj) == 0 || (req.Act != mtype.LISTEN && req.Act != mtype.UNLISTEN) {
				log.Printf("[%s] Request has wrong net %s, wrong type %d, missing objext %s or wrong action %d", net, req.Net, req.Type, req.Obj, req.Act)
			}
			// process object
			if req.Type == mtype.ADDRESS {
				a := store.Address{Addr: req.Obj}
				if req.Act == mtype.LISTEN {
					// save it to DB
					if _, err = e.db.AddAddress(a, net); err != nil {
						log.Printf("[%s] Error adding WalletReq address to DB %e", net, err)
					}
					// include it in NetExplorer
					nexp.Add(req.Obj, "listen")
					log.Printf("[%s] Added object %s to NetExplorer %+v", net, req.Obj, *nexp)
				} else {
					// delete from NetExplorer
					if _, ok := nexp.Del(req.Obj); !ok {
						log.Printf("[%s] Error deleting WalletReq address %s from NetExplorer. Not found. Ignoring...", net, req.Obj)
					}
					// delete from DB
					if err = e.db.RemoveAddress(a, net); err != nil {
						log.Printf("[%s] Error deleting WalletReq address from DB %e", net, err)
					}
					log.Printf("[%s] Removed object %s from NetExplorer %+v", net, req.Obj, *nexp)

				}
			} else if req.Type == mtype.TX {
				log.Printf("Un/listen to transaction TODO!!!!")
			}
			mut.Unlock()
		}
		log.Printf("[%s] Stop listening to wallet request channel", net)
	}()

	// launch error channel reader
	go func() {
		log.Printf("[%s] Start listening to err channel", net)
		for e := range errCh {
			log.Printf("[%s] Received error %+v", net, e)
		}
		log.Printf("[%s] Stop listening to err channel", net)
	}()

	return err
}