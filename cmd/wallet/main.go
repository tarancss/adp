// Package main: wallet service.
//
// Warning: The DB used by the microservice is just in order to serve requests of monitored addresses so it should be
// the same database used by the explorer microservice. Currently, the wallet does not persist any data. Alternatively,
// these requests could be replied asking the explorer service, so avoiding this DB requirement but creating more
// message broker traffic. To be considered.
package main

import (
	"encoding/hex"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/config"
	"github.com/tarancss/adp/lib/msg"
	"github.com/tarancss/adp/lib/msg/amqp"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/store/db"
	"github.com/tarancss/adp/wallet"
	"github.com/tarancss/hd"
)

func main() {
	// get command line flags
	confPath := flag.String("c", "", "flag to get configuration from json file")
	monitor := flag.Bool("m", false, "flag to monitor the server with Prometheus at http://localhost:9090")
	flag.Parse()

	// extract configuration
	conf, err := config.ExtractConfiguration(*confPath)
	if err != nil {
		panic(err)
	}

	log.Printf("Configuration:%+v", conf)

	// connect to database
	var dbConn store.DB

	if conf.DBConn != "" {
		if dbConn, err = db.New(conf.DBType, conf.DBConn); err != nil {
			panic(err)
		}

		log.Printf("Connecting to database:%+v\n", conf.DBConn)
	}

	// load all blockchains
	blocks, err := block.Init(conf.Bc)
	if err != nil {
		panic(err)
	}

	log.Print("Blockchain clients loaded")

	// load Prometheus monitor
	if *monitor {
		go func() {
			log.Println("Serving metrics API")

			h := http.NewServeMux()

			h.Handle("/metrics", promhttp.Handler())
			http.ListenAndServe(":9100", h)
		}()
	}

	// load message broker
	var mb msg.MsgBroker

	switch conf.MbType {
	case "amqp":
		if mb, err = amqp.New(conf.MbConn); err != nil {
			time.Sleep(10 * time.Second) // wait 10s for AMQP to be ready and try to reconnect

			if mb, err = amqp.New(conf.MbConn); err != nil {
				panic(err)
			}
		}

		if err = mb.Setup(nil); err != nil {
			panic(err)
		}

		defer func() {
			errClose := mb.Close()
			log.Printf("Closing messageBroker: %e", errClose)
		}()
	default:
		log.Printf("Unknown message broker type: %s\n", conf.MbType)
	}

	// load HD wallet
	seed, err := hex.DecodeString(conf.Seed)
	if err != nil {
		panic(err)
	}

	hdw, err := hd.Init(seed)
	if err != nil {
		panic(err)
	}

	// create wallet service
	w := wallet.New(conf.DBType, dbConn, mb, blocks, hdw)

	// capture CTRL+C or docker's SIGTERM for gracious exit
	finish := make(chan int)

	go func() {
		sigchan := make(chan os.Signal, 10)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
		<-sigchan
		log.Println("Program killed !")
		// do last actions and wait for all write operations to end
		w.Stop()
		close(finish)
	}()

	// manage explorer events
	if err := w.ManageEvents(); err != nil {
		log.Printf("Error setting up broker readers for events:%e", err)
	}

	// init RESTful API, wait for its return and log response
	log.Printf("Wallet: %s\n", w.Init(conf.RestfulEndpoint, conf.Port, conf.SSLPort, conf.SSLCert, conf.SSLKey))

	<-finish
}
