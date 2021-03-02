// Package wallet implements the wallet microservice.
//
// This microservice implements a RESTful API for clients to interact with multiple blockchains. The full documentation
// of the API is provided in https://github.com/tarancss/adp/blob/master/API.md.
package wallet

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/msg"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/store/db"
	"github.com/tarancss/hd"
)

// Wallet implements a wallet service.
type Wallet struct {
	// ideally, the GetAddreses should be requested to Explorer service, so we would not need any DB here!!
	dbtype string
	db     store.DB // db connection

	bc map[string]block.Chain // blockchain clients
	hd *hd.HdWallet           // HD wallet
	mb msg.MsgBroker
	s  *http.Server  // http server
	ss *http.Server  // https server
	sc chan struct{} // http server channel used for graceful shutdowns
}

// New returns a pointer to a new Wallet service with the given configuration.
func New(dbtype string, dbConn store.DB, mb msg.MsgBroker, bc map[string]block.Chain, hdw *hd.HdWallet) *Wallet {
	return &Wallet{
		dbtype: dbtype,
		db:     dbConn,
		mb:     mb,
		bc:     bc,
		hd:     hdw,
	}
}

// Stop shuts down the http servers implementing the RESTful API and closes gracefully the connections to message
// broker, monitoring service and database.
func (w *Wallet) Stop() {
	var err error
	// shutdown http server
	if w.s != nil {
		if err = w.s.Shutdown(context.Background()); err != nil {
			log.Printf("Error in http server shutdown:%e", err)
		}
	}

	if w.ss != nil {
		if err = w.ss.Shutdown(context.Background()); err != nil {
			log.Printf("Error in https server shutdown:%e", err)
		}
	}

	close(w.sc) // close server channels to indicate shutdowns have finished
	// close message broker
	if err = w.mb.Close(); err != nil {
		log.Printf("Error closing message broker:%e", err)
	}
	// close Prometheus ??? TODO

	// close database
	if w.db != nil {
		err = db.Close(w.dbtype, w.db)
		log.Printf("Disconnecting %v database, err:%e\n", w.dbtype, err)
	}
}

// ManageEvents starts go routines to consume the message broker queues for events sent by the explorer service. For
// each connected blockchain, two channels are opened, one for transaction events, and one for errors. A go routine is
// triggered reading for either channel to manage the events/errors.
func (w *Wallet) ManageEvents() error {
	// for each chain establish a process to read events from the broker queues
	for net := range w.bc {
		// open events channel
		var mut *sync.Mutex = new(sync.Mutex)

		mut.Lock()

		eveCh, errCh, err := w.mb.GetEvents(net, mut)
		if err != nil {
			return fmt.Errorf("cannot get events from broker for net %s: %w", net, err)
		}
		// launch request channel reader
		go func(netName string) {
			for {
				select {
				case eve, ok := (<-eveCh):
					if !ok {
						log.Printf("[%s] Stop listening to events channel", netName)

						break
					}

					log.Printf("[%s] Received event %+v", netName, eve) // we just log it to console!! XXX

					mut.Unlock()
				case e, ok := (<-errCh):
					if !ok {
						log.Printf("[%s] Stop listening to events channel", netName)

						break
					}

					log.Printf("[%s] Received error %+v", netName, e)
				}
			}
		}(net)
	}

	return nil
}
