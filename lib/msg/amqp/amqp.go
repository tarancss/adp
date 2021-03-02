// Package amqp implements the message broker interface for AMQP compliant brokers (ie RabbitMQ).
package amqp

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/streadway/amqp"

	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/msg"
)

// AMQP implements a connection to a broker and a channel for reuse.
type AMQP struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// New instantiates a new amqp broker.
func New(uri string) (*AMQP, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("cannot dial amqp: %w", err)
	}

	log.Printf("Connected to %s", uri)

	return &AMQP{conn: conn, ch: nil}, nil
}

// Setup obtains an amqp channel and declares the message broker exchanges:
//
// - wr ("wallet requests"): the wallet service publishes requests to this exchange
//
// - ee ("explorer events"): the explorer service publishes events to this exchange.
func (r *AMQP) Setup(x interface{}) error {
	// obtain a one-use channel
	channel, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("cannot get amqp channel: %w", err)
	}
	defer channel.Close() //nolint:errcheck //it is safe to ignore this error

	// declare exchanges
	if err = channel.ExchangeDeclare("wr", "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("cannot declare exchange 'wr': %w", err)
	}

	if err = channel.ExchangeDeclare("ee", "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("cannot declare exchange 'ee': %w", err)
	}

	return nil
}

// Close terminages gracefully the connection to the AMQP message broker. It will close also all channel consumers
// ending the go routines of GetReqs and GetEvents.
func (r *AMQP) Close() error {
	if r.ch != nil {
		if err := r.ch.Close(); err != nil {
			log.Printf("Error closing amqp.Channel:%e", err)
		}

		r.ch = nil

		log.Printf("amqp.Channel closed!")
	}

	return r.conn.Close()
}

// SendTrans publishes transaction events to the "ee" exchange.
func (r *AMQP) SendTrans(net string, txs []types.Trans) error {
	for _, t := range txs {
		// marshal to JSON
		jsonDoc, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("error marshaling transaction: %w", err)
		}

		// obtain channel if not present
		if r.ch == nil {
			if r.ch, err = r.conn.Channel(); err != nil {
				return fmt.Errorf("cannot get amqp channel: %w", err)
			}
		}
		// build body
		msg := amqp.Publishing{ //nolint:exhaustivestruct // many other members
			Headers:     amqp.Table{"x-trans-name": net + "." + t.Hash},
			Body:        jsonDoc,
			ContentType: "application/json",
		}
		// publish
		if err = r.ch.Publish("ee", net+".trans."+t.Hash, false, false, msg); err != nil {
			log.Printf("[%s] Error sending transaction event to message broker %e", net, err)
		}
	}

	return nil
}

// SendRequest publishes a new wallet request to the "wr" exchange.
func (r *AMQP) SendRequest(net string, wr msg.WalletReq) error {
	// marshal to JSON
	jsonDoc, err := json.Marshal(wr)
	if err != nil {
		return fmt.Errorf("error marshaling transaction: %w", err)
	}
	// obtain channel if not present
	if r.ch == nil {
		if r.ch, err = r.conn.Channel(); err != nil {
			return fmt.Errorf("cannot get amqp channel: %w", err)
		}
	}
	// build body
	msg := amqp.Publishing{ //nolint:exhaustivestruct // many other members
		Headers:     amqp.Table{"x-wreq-name": net + "." + wr.Obj},
		Body:        jsonDoc,
		ContentType: "application/json",
	}
	// publish
	if err = r.ch.Publish("wr", net+"."+strconv.Itoa(wr.Type)+"."+wr.Obj, false, false, msg); err != nil {
		log.Printf("[%s] Error sending request to message broker %e", net, err)
	}

	return nil
}

// GetEvents consumes requests from the "ee" exchange pushing them to the returned channel.
// The Mutex pointer is provided to ensure the consumed message has been fully dealt with by the management
// function, so the message consumed is only acknowledged when the mutex is unlocked.
//nolint:dupl // subscribers are similar
func (r *AMQP) GetEvents(net string, mut *sync.Mutex) (<-chan types.Trans, <-chan error, error) {
	var err error
	if r.ch == nil {
		if r.ch, err = r.conn.Channel(); err != nil {
			return nil, nil, fmt.Errorf("cannot get amqp channel: %w", err)
		}
	}

	_, err = r.ch.QueueDeclare("ee"+net, true, false, false, false, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot declare queue ee%s: %w", net, err)
	}

	// bind queue to exchange
	if err = r.ch.QueueBind("ee"+net, net+".*.*", "ee", false, nil); err != nil {
		return nil, nil, fmt.Errorf("cannot bind queue ee%s: %w", net, err)
	}

	// create channel for receiving requests
	msgs, errCons := r.ch.Consume("ee"+net, "explorer-"+net, false, false, false, false, nil)
	if errCons != nil {
		return nil, nil, fmt.Errorf("cannot consume events from ee%s: %w", net, errCons)
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

			err = m.Ack(false)
			if err != nil {
				log.Printf("error ack'ing event: %s", err)
			}
		}
	}()

	return eves, errors, nil
}

// GetReqs consumes requests from the "wr" exchange for the specified network pushing them to the returned channel.
// The Mutex pointer is provided to ensure the consumed message has been fully dealt with by the management function,
// so the message consumed is only acknowledged when the mutex is unlocked.
//nolint:dupl // subscribers are similar
func (r *AMQP) GetReqs(net string, mut *sync.Mutex) (<-chan msg.WalletReq, <-chan error, error) {
	var err error
	if r.ch == nil {
		if r.ch, err = r.conn.Channel(); err != nil {
			return nil, nil, fmt.Errorf("cannot get amqp channel: %w", err)
		}
	}

	_, err = r.ch.QueueDeclare("wr"+net, true, false, false, false, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot declare queue wr%s: %w", net, err)
	}

	// bind queue to exchange
	if err = r.ch.QueueBind("wr"+net, net+".*.*", "wr", false, nil); err != nil {
		return nil, nil, fmt.Errorf("cannot bind queue wr%s: %w", net, err)
	}

	// create channel for receiving requests
	msgs, errCons := r.ch.Consume("wr"+net, "explorer-"+net, false, false, false, false, nil)
	if errCons != nil {
		return nil, nil, fmt.Errorf("cannot consume requests from wr%s: %w", net, errCons)
	}

	// define channels to return
	reqs := make(chan msg.WalletReq)
	errors := make(chan error)

	// start routine to consume messages from broker
	go func() {
		for m := range msgs {
			var req *msg.WalletReq = new(msg.WalletReq)

			err := json.Unmarshal(m.Body, req)
			if err != nil {
				errors <- err

				continue
			}

			reqs <- *req

			mut.Lock() // wait for explorer to finish processing the request

			err = m.Ack(false)
			if err != nil {
				log.Printf("error ack'ing request: %s", err)
			}
		}
	}()

	return reqs, errors, nil
}
