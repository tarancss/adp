// +build integration

package amqp

import (
	"sync"
	"testing"

	"github.com/streadway/amqp"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/msg"
)

// TestAMQP tests the broker functionality for AMQP ensuring integration between microservices wallet and explorer.
// This test requires an available RabbitMQ server at localhost:5672.
func TestAMQP(t *testing.T) {
	// create new broker
	r, err := New("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Errorf("Error creating broker:%e", err)
	}

	defer r.Close()

	// TestSetup - make sure the exchanges are created
	if err = r.Setup(nil); err != nil {
		t.Errorf("Error setting up broker:%e", err)
	}
	if r.ch, err = r.conn.Channel(); err != nil {
		t.Errorf("Error setting up channel:%e", err)
	}
	// test an exchange is not found
	err = r.ch.ExchangeDeclarePassive("xx", amqp.ExchangeTopic, true, false, false, false, nil)
	if err != nil && err.(*amqp.Error).Reason != "NOT_FOUND - no exchange 'xx' in vhost '/'" {
		t.Errorf("Exchange \"xx\" was found and it should not exist!! err:%v", err.(*amqp.Error))
	}

	// Test "wr" and "ee" exist
	if r.ch, err = r.conn.Channel(); err != nil {
		t.Errorf("Error setting up channel:%e", err)
	}
	err = r.ch.ExchangeDeclarePassive("wr", "topic", true, false, false, false, nil)
	if err != nil {
		t.Errorf("Exchange \"wr\" wasnt found!! err:%e", err)
	}
	err = r.ch.ExchangeDeclarePassive("ee", "topic", true, false, false, false, nil)
	if err != nil {
		t.Errorf("Exchange \"ee\" wasnt found!! err:%e", err)
	}

	// Test sending and getting requests
	var mut = new(sync.Mutex)
	req, _, errRe := r.GetReqs("net", mut)
	if errRe != nil {
		t.Errorf("Error getting requests:%e", errRe)
	}

	err = r.SendRequest("net", msg.WalletReq{Net: "net", Type: msg.ADDRESS, Obj: "0x1234567890", Act: msg.LISTEN})
	wr := <-req
	if err != nil || wr.Net != "net" || wr.Type != msg.ADDRESS || wr.Obj != "0x1234567890" || wr.Act != msg.LISTEN {
		t.Errorf("Error got request that does not match the sent one! err:%e wr:%+v", err, wr)
	}
	mut.Unlock()
	r.ch.Close()

	// Test sending and getting transaction events
	if r.ch, err = r.conn.Channel(); err != nil {
		t.Errorf("Error setting up channel:%e", err)
	}
	eve, _, errGe := r.GetEvents("net", mut)
	if errGe != nil {
		t.Errorf("Error getting events:%e", errGe)
	}

	err = r.SendTrans("net", []types.Trans{{Block: "0x2304ef", Hash: "0x5678901234567890"}})
	tx := <-eve
	if err != nil || tx.Block != "0x2304ef" || tx.Hash != "0x5678901234567890" {
		t.Errorf("Error got request that does not match the sent one! err:%e wr:%+v", err, wr)
	}
}
