// Package amqp implements the message broker interface for AMQP compliant brokers (ie RabbitMQ)
package amqp

import (
	"encoding/json"
	"log"
	"strconv"
	"sync"

	"github.com/streadway/amqp"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/msg"
	mtype "github.com/tarancss/adp/lib/msg/types"
)

// Amqp implements a connection to a broker and a channel for reuse.
type Amqp struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// New instantiates a new amqp broker.
func New(uri string) (msg.MsgBroker, error) {
	r := Amqp{}
	var err error

	if r.conn, err = amqp.Dial(uri); err != nil {
		return &r, err
	}
	r.ch = nil
	log.Printf("Connected to %s", uri)

	return &r, err
}

// Setup obtains an amqp channel and declares the message broker exchanges:
//
// - wr ("wallet requests"): the wallet service publishes requests to this exchange
//
// - ee ("explorer events"): the explorer service publishes events to this exchange
func (r *Amqp) Setup(x interface{}) error {
	// obtain a one-use channel
	channel, err := r.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	// declare exchanges
	if err = channel.ExchangeDeclare("wr", "topic", true, false, false, false, nil); err != nil {
		return err
	}
	err = channel.ExchangeDeclare("ee", "topic", true, false, false, false, nil)
	return err
}

// Close terminages gracefully the connection to the AMQP message broker
func (r *Amqp) Close() error {
	if r.ch != nil {
		if err := r.ch.Close(); err != nil {
			log.Printf("Error closing amqp.Channel:%e", err)
		}
		r.ch = nil
		log.Printf("amqp.Channel closed!")
	}
	return r.conn.Close()
}

// SendTrans publishes transaction events to the "ee" exchange
func (r *Amqp) SendTrans(net string, txs []types.Trans) (err error) {
	for _, t := range txs {
		// marshal to JSON
		var jsonDoc []byte
		if jsonDoc, err = json.Marshal(t); err != nil {
			return
		}
		// obtain channel if not present
		if r.ch == nil {
			if r.ch, err = r.conn.Channel(); err != nil {
				return
			}
		}
		// build body
		msg := amqp.Publishing{
			Headers:     amqp.Table{"x-trans-name": net + "." + t.Hash},
			Body:        jsonDoc,
			ContentType: "application/json",
		}
		// publish
		if err = r.ch.Publish("ee", net+".trans."+t.Hash, false, false, msg); err != nil {
			log.Printf("[%s] Error sending transaction event to message broker %e", net, err)
		}
	}
	return
}

// SendRequest publishes a new wallet request to the "wr" exchange
func (r *Amqp) SendRequest(net string, wr mtype.WalletReq) (err error) {
	// marshal to JSON
	var jsonDoc []byte
	if jsonDoc, err = json.Marshal(wr); err != nil {
		return
	}
	// obtain channel if not present
	if r.ch == nil {
		if r.ch, err = r.conn.Channel(); err != nil {
			return
		}
	}
	// build body
	msg := amqp.Publishing{
		Headers:     amqp.Table{"x-wreq-name": net + "." + wr.Obj},
		Body:        jsonDoc,
		ContentType: "application/json",
	}
	// publish
	if err = r.ch.Publish("wr", net+"."+strconv.Itoa(wr.Type)+"."+wr.Obj, false, false, msg); err != nil {
		log.Printf("[%s] Error sending request to message broker %e", net, err)
	}
	return
}

// GetEvents consumes requests from the "ee" exchange pushing them to the returned channel. The Mutex pointer is provided to ensure the consumed message has been fully dealt with by the management function, so the message consumed is only acknowledged when the mutex is unlocked.
func (r *Amqp) GetEvents(net string, mut *sync.Mutex) (<-chan types.Trans, <-chan error, error) {
	var err error
	if r.ch == nil {
		if r.ch, err = r.conn.Channel(); err != nil {
			return nil, nil, err
		}
	}
	// declare queue
	var q amqp.Queue
	if q, err = r.ch.QueueDeclare("ee"+net, true, false, false, false, nil); err != nil {
		return nil, nil, err
	}
	_ = q // otherwise compiler yields error, q not used!! XXX

	// bind queue to exchange
	if err = r.ch.QueueBind("ee"+net, net+".*.*", "ee", false, nil); err != nil {
		return nil, nil, err
	}
	// create channel for receiving requests
	msgs, errCons := r.ch.Consume("ee"+net, "explorer-"+net, false, false, false, false, nil)
	if errCons != nil {
		return nil, nil, errCons
	}
	// define channels to return
	eves := make(chan types.Trans)
	errors := make(chan error)
	// start routine to consume messages from broker
	go func() {
		for m := range msgs {
			var tx *types.Trans = new(types.Trans)
			err := json.Unmarshal(m.Body, tx)
			if err != nil {
				errors <- err
				continue
			}
			eves <- *tx
			mut.Lock() // wait for wallet to finish processing the event
			m.Ack(false)
		}
	}()
	return eves, errors, nil
}

// GetReqs consumes requests from the "wr" exchange for the specified network pushing them to the returned channel.  The Mutex pointer is provided to ensure the consumed message has been fully dealt with by the management function, so the message consumed is only acknowledged when the mutex is unlocked.
func (r *Amqp) GetReqs(net string, mut *sync.Mutex) (<-chan mtype.WalletReq, <-chan error, error) {
	var err error
	if r.ch == nil {
		if r.ch, err = r.conn.Channel(); err != nil {
			return nil, nil, err
		}
	}
	// declare queue
	var q amqp.Queue
	if q, err = r.ch.QueueDeclare("wr"+net, true, false, false, false, nil); err != nil {
		return nil, nil, err
	}
	_ = q // otherwise compiler yields error, q not used

	// bind queue to exchange
	if err = r.ch.QueueBind("wr"+net, net+".*.*", "wr", false, nil); err != nil {
		return nil, nil, err
	}
	// create channel for receiving requests
	msgs, errCons := r.ch.Consume("wr"+net, "explorer-"+net, false, false, false, false, nil)
	if errCons != nil {
		return nil, nil, errCons
	}
	// define channels to return
	reqs := make(chan mtype.WalletReq)
	errors := make(chan error)
	// start routine to consume messages from broker
	go func() {
		for m := range msgs {
			var req *mtype.WalletReq = new(mtype.WalletReq)
			err := json.Unmarshal(m.Body, req)
			if err != nil {
				errors <- err
				continue
			}
			reqs <- *req
			mut.Lock() // wait for explorer to finish processing the request
			m.Ack(false)
		}
	}()
	return reqs, errors, nil
}
