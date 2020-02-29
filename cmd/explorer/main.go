// package main: explorer service
//
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tarancss/adp/explorer"
	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/config"
	"github.com/tarancss/adp/lib/msg"
	"github.com/tarancss/adp/lib/msg/amqp"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/store/db"
)

func main() {
	// get command line flags
	confPath := flag.String("c", "", "flag to get configuration from json file")
	monitor := flag.Bool("m", false, "flag to monitor the server with Prometheus at http://localhost:9090")
	flag.Parse()

	//extract configuration
	var err error
	var conf config.ServiceConfig
	if conf, err = config.ExtractConfiguration(*confPath); err != nil {
		panic(err)
	}
	log.Printf("Configuration:%+v", conf)

	// connect to database
	var dbConn store.DB
	if conf.DbConn != "" {
		log.Printf("Connecting to database:%+v\n", conf.DbConn)
		if dbConn, err = db.New(conf.DbType, conf.DbConn); err != nil {
			panic(err)
		}
	}

	// load all blockchains
	var blocks map[string]block.Chain
	if blocks, err = block.Init(conf.Bc); err != nil {
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
			err := mb.Close()
			log.Printf("Closing messageBroker: %e", err)
		}()
	default:
		log.Printf("Unknown message broker type: %s\n", conf.MbType)
	}

	// create explorer service
	e := explorer.New(conf.DbType, dbConn, mb, blocks)

	// capture CTRL+C or docker's SIGTERM for gracious exit
	go func() {
		sigchan := make(chan os.Signal, 10)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
		<-sigchan
		log.Println("Program killed !")
		// do last actions and wait for all write operations to end
		e.StopExplorer()
	}()

	// launch explorer (for each network) creating a waiting channel for each
	log.Printf("Explore: %s\n", <-e.Explore())
}
