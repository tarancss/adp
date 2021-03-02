package wallet

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

const timeout = 15

// Init sets up and starts the http/https server to service the RESTful API for a wallet service. If sslPort, ssCert
// and sslKey are informed, it will start an https (TLS) server on the specified endpoint.
func (w *Wallet) Init(endpoint, port, sslPort, sslCert, sslKey string) string {
	var err, errTLS error

	// API definition
	r := mux.NewRouter()
	r.HandleFunc("/", w.homeHandler)
	r.HandleFunc("/networks", w.networksHandler).Methods("GET")         // get all available blockchains
	r.HandleFunc("/address/{address}", w.addrBalHandler).Methods("GET") // get address balance
	r.HandleFunc("/address", w.hdAddrHandler).Methods("GET")            // get address from HD wallet
	r.HandleFunc("/listen/{address}", w.listenHandler)                  // listen events related to the address
	r.HandleFunc("/listen", w.getAddrHandler).Methods("GET")            // Get listened addresses
	r.HandleFunc("/send", w.sendHandler).Methods("POST")                // send a transaction
	r.HandleFunc("/tx/{hash}", w.txHandler).Methods("GET")              // get transaction details
	http.Handle("/", r)

	// setup shutdown channel
	w.sc = make(chan struct{})

	// start http server
	if port != "" {
		w.s = &http.Server{
			Handler: r,
			Addr:    endpoint + ":" + port,
			// Good practice: enforce timeouts for servers you create!
			WriteTimeout: timeout * time.Second,
			ReadTimeout:  timeout * time.Second,
		}

		go func() {
			err = w.s.ListenAndServe()
		}()

		log.Printf("Listening to API http requests on %s:%s", endpoint, port)
	}
	// start https server
	if sslPort != "" && sslCert != "" && sslKey != "" {
		w.ss = &http.Server{
			Handler: r,
			Addr:    endpoint + ":" + sslPort,
			// Good practice: enforce timeouts for servers you create!
			WriteTimeout: timeout * time.Second,
			ReadTimeout:  timeout * time.Second,
		}

		go func() {
			errTLS = w.ss.ListenAndServeTLS(sslCert, sslKey)
		}()

		log.Printf("Listening to API https requests on %s:%s", endpoint, sslPort)
	}
	// wait for servers to be shutdown
	<-w.sc

	return fmt.Sprintf("shutdown http server:%e, https server:%e", err, errTLS)
}
