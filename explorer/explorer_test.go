package explorer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tarancss/adp/explorer/netexplorer"
	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/config"
	"github.com/tarancss/adp/lib/msg/amqp"
	mtype "github.com/tarancss/adp/lib/msg/types"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/store/db"
)

// TestExploreChain is a component test!!
// Requires a mock server for the blockchain interface server to provide block data.
func TestExploreChain(t *testing.T) {
	// start a mock blockchain server
	mock := httptest.NewServer(http.HandlerFunc(handler))
	println("Warning: running tests against mock blockchain in:", mock.URL)
	defer mock.Close()

	// connect to DB
	dbType := db.MONGODB
	dbUri := "mongodb://localhost:27017"
	s, _ := db.New(dbType, dbUri)
	defer db.Close(dbType, s)

	// connect to msg broker
	mb, err := amqp.New("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Errorf("Error creating broker:%e", err)
		return
	}
	defer mb.Close() // closing the message broker will stop the go routine launched by the function
	mb.Setup(nil)    // setup the exchanges

	// define a chain at the mock server URL
	net := "ropsten"
	bc, err := block.Init([]config.BlockConfig{config.BlockConfig{Name: net, Node: mock.URL, Secret: "", MaxBlocks: 4}})
	defer block.End(bc)

	// delete netexplorer if found
	err = s.DeleteExplorer(net)
	if err != nil {
		t.Logf("DeleteExplorer err:%e", err)
	}

	// instantiate an explorer and setup a netExplorer with an address to monitor
	e := New(dbType, s, mb, bc)
	if e.nem[net], err = netexplorer.New(net, e.bc[net].MaxBlocks(), []store.ListenedAddresses{
		{Net: net, Addr: []store.Address{store.Address{ID: []byte{0x31}, Name: "test", Addr: "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4"}}},
	}, e.db); err != nil {
		t.Errorf("[%s] netexplorer.New failed:%e", net, err)
		return
	}

	// declare channels to receive the events
	var mut = new(sync.Mutex)
	mut.Lock()
	eveCh, eveErr, err := mb.GetEvents(net, mut)

	// launch ExploreChain
	var finish chan int = make(chan int)
	go func() {
		// run ExploreChain
		var ret chan string = make(chan string, 1)
		e.ExploreChain(net, ret)
		defer e.StopExplorer()
		t.Logf("ExploreChain finished: %s", <-ret)
		// stop test
		finish <- 1
	}()

	ts := []types.Trans{
		{Block: "0x29bf9b", Hash: "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", Token: "", Value: "0x16345785d8a0000"},
		{Block: "0x29bf9c", Hash: "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", Token: "0x7762440182222620a7435195208038708d27ee41", Value: "0x12309ce54000"},
		{Block: "0x29bf9d", Hash: "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", Token: "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", Value: "0x12309ce54000"},
		{Block: "0x29bf9e", Hash: "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf61", Token: "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", Value: "0x12309ce54000"},
	}

	// reveive events and run tests
	var i int
	for i >= 0 {
		select {
		case eve, ok := (<-eveCh):
			// t.Logf("[%s] Received event %d %+v", net, i+1, eve) // we just log it to console!! XXX
			if ts[i].Block != eve.Block || ts[i].Hash != eve.Hash || ts[i].Token != eve.Token || ts[i].Value != eve.Value {
				t.Errorf("Error in event %d received %v", i, eve)
			}
			if !ok {
				t.Log("eveCh not ok!")
			}
			// test whatever TODO
			mut.Unlock()
			i++
		case errEv, ok := (<-eveErr):
			t.Logf("[%s] Received error %+v", net, errEv) // we just log it to console!! XXX
			if !ok {
				t.Log("errCh not ok!")
			}
		case _ = <-finish:
			i = -1
			break
		}
	}
}

// mockRequest
type mockRequest struct {
	Version string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  *json.RawMessage `json:"params"`
	ID      *json.RawMessage `json:"id"`
}

// mockResponse
type mockResponse struct {
	Version string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result,omitempty"`
	Error   interface{}      `json:"error,omitempty"`
}

// define handler for mock HTTP server
var handler = func(w http.ResponseWriter, r *http.Request) {
	var req mockRequest
	var res mockResponse
	var err error
	// make sure we reply to request either with error or the response
	defer func() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		res.Version = "2.0"
		if err = json.NewEncoder(w).Encode(res); err != nil {
			fmt.Printf("[Mock server] Error encoding response:%e\n", err)
		}
	}()
	// read request body
	var body []byte = make([]byte, int(r.ContentLength))
	var n int
	n, err = r.Body.Read(body)
	if err == nil || (err == io.EOF && n == int(r.ContentLength)) {
		// fmt.Printf("[Mock server] Request Body:%s", body)
	} else {
		res.Error = errors.New(fmt.Sprintf("n:%d error:%e\n", n, err))
		return
	}
	// unmarshal JSON body
	if err = json.Unmarshal(body, &req); err != nil {
		res.Error = errors.New(fmt.Sprintf("Error unmarshaling Body:%e\n", err))
		return
	}
	res.ID = req.ID

	// reply with expected value
	var i int
	var buf []byte = []byte(*res.ID)
	for j := 0; j < len(buf); j++ {
		i = i*10 + int(buf[j]-0x30)
	}
	res.Result = mock[i]
	return
}

// TestManageWalletRequests is a component test!!
func TestManageWalletRequest(t *testing.T) {
	// connect to DB
	dbType := db.MONGODB
	dbUri := "mongodb://localhost:27017"
	s, _ := db.New(dbType, dbUri)
	defer db.Close(dbType, s)

	// connect to msg broker
	mb, err := amqp.New("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Errorf("Error creating broker:%e", err)
		return
	}
	defer mb.Close() // closing the message broker will stop the go routine launched by the function
	mb.Setup(nil)    // setup the exchanges

	// define a chain
	net := "ropsten"
	bc, err := block.Init([]config.BlockConfig{config.BlockConfig{Name: net, Node: "https://localhost:8545", Secret: "", MaxBlocks: 4}})
	block.End(bc)

	// instantiate an explorer and setup netExplorer
	e := New(dbType, s, mb, bc)
	if e.nem[net], err = netexplorer.New(net, e.bc[net].MaxBlocks(), nil, e.db); err != nil {
		t.Errorf("[%s] netexplorer.New failed:%e", net, err)
		return
	}

	// run ManageWalletRequests
	if err = e.ManageWalletRequests(net); err != nil {
		t.Errorf("ManageWalletRequests err:%e", err)
	}

	// test steps: a WalletReq is sent. Then check the map's len, and if an object is specified, we check the existence of that object in the map
	ts := []interface{}{
		[]interface{}{mtype.WalletReq{Net: net, Type: mtype.ADDRESS, Obj: "addr1", Act: mtype.UNLISTEN}, 0, ""},
		[]interface{}{mtype.WalletReq{Net: net, Type: mtype.ADDRESS, Obj: "addr1", Act: mtype.LISTEN}, 1, "addr1", true},
		[]interface{}{mtype.WalletReq{Net: net, Type: mtype.ADDRESS, Obj: "addr2", Act: mtype.LISTEN}, 2, "addr2", true},
		[]interface{}{mtype.WalletReq{Net: net, Type: mtype.ADDRESS, Obj: "addr1", Act: mtype.LISTEN}, 2, "addr1", true},
		[]interface{}{mtype.WalletReq{Net: net, Type: mtype.ADDRESS, Obj: "addr1", Act: mtype.UNLISTEN}, 1, "addr1", false},
	}

	// run test
	for _, step := range ts {
		// send request
		if err = mb.SendRequest(net, step.([]interface{})[0].(mtype.WalletReq)); err != nil {
			t.Errorf("SendRequest 1 err:%e", err)
		}
		time.Sleep(50 * time.Millisecond) // let the go routine finish managing the request
		// check result
		if len(e.nem[net].Map) != step.([]interface{})[1].(int) {
			t.Errorf("Map len (%d) does not match in step:%+v", len(e.nem[net].Map), step)
		}
		if step.([]interface{})[2].(string) != "" {
			if _, ok := e.nem[net].Map[step.([]interface{})[2].(string)]; ok != step.([]interface{})[3].(bool) {
				t.Errorf("Object in Map (%v) does not match in step:%+v", e.nem[net].Map, step)
			}
		}
	}
}

// TestExplorer is an end-to-end test: TODO
func TestExplorer(t *testing.T) {
	// connect to DB
	dbType := db.MONGODB
	dbUri := "mongodb://localhost:27017"
	s, _ := db.New(dbType, dbUri)
	defer db.Close(dbType, s)

	// connect to msg broker
	mb, err := amqp.New("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Errorf("Error creating broker:%e", err)
	}
	defer mb.Close()

	// define a chain
	bc, err := block.Init([]config.BlockConfig{config.BlockConfig{Name: "ropsten", Node: "https://localhost:8545", Secret: "", MaxBlocks: 4}})
	block.End(bc)

	// instantiate an explorer
	exp := New(dbType, s, mb, bc)

	ret := exp.Explore()

	// go func() {
	time.Sleep(5 * time.Second)
	exp.StopExplorer()
	// }()

	t.Logf("ret:%s", <-ret)
}

// mock contains the data used by the mock server in TestExploreChain.
var mock []interface{} = []interface{}{
	// block 1: an ether receive transaction
	map[string]interface{}{"difficulty": "0x7ee56684", "extraData": "0x414952412f7630", "gasLimit": "0x47b784", "gasUsed": "0x47addd", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "logsBloom": "0x0000000001400004002008000002000080000000000120200120002400208220000040000001000000000004804800000104000000000c0000000008201000005000200000010000140000084000000000000000100010400000080000040080100082000000000000000000004000021000800400802000000000501000000200000400000200020040010040000010105000000000040120000008000800200801000008004000000400004040000100000000000400000d005000020000008000004280010000000000000000000020010180100000140000000000020000000000000000008008000000000040000040100004001002c040000000000000", "miner": "0x00d8ae40d9a06d0e7a2877b62e32eb959afbe16d", "mixHash": "0xd93c06ec00e2c653b7958114ba8224aad8749caf8de6aee2c2f465c5f09cc0cc", "nonce": "0x34b98c94071402d8", "number": "0x29bf9b", "parentHash": "0x25e2e6cfc2f49ef320c652d91a7bea99a2d115d29ea832631e5f11911a463158", "receiptsRoot": "0x0506189cdc814f4440690b43aaf7cf278a9b346b8ef3174c03dde2d23aa820ea", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x299a", "stateRoot": "0xf8be81979f9a92cd123f8e6295dca2660184df4f58e275c6c9fe7adee0016e7c", "timestamp": "0x5a952da9", "totalDifficulty": "0x1bd6b7e3c7b473", "transactions": []map[string]interface{}{{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0xc4581843a8dacd100c7d435bb00b2a20d038e31d", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0x4bdb8ab50804004410241002040000c60890801000000000000000000000000000000000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}}, "transactionsRoot": "0x08e95959ada5ebbe3aae1a4b9179f811c326c0969b7a5fea75b4e427c2870f96", "uncles": []string{}},
	// block 2: a token send transaction
	map[string]interface{}{"difficulty": "0x7ee56684", "extraData": "0x414952412f7630", "gasLimit": "0x47b784", "gasUsed": "0x47addd", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed7", "logsBloom": "0x0000000001400004002008000002000080000000000120200120002400208220000040000001000000000004804800000104000000000c0000000008201000005000200000010000140000084000000000000000100010400000080000040080100082000000000000000000004000021000800400802000000000501000000200000400000200020040010040000010105000000000040120000008000800200801000008004000000400004040000100000000000400000d005000020000008000004280010000000000000000000020010180100000140000000000020000000000000000008008000000000040000040100004001002c040000000000000", "miner": "0x00d8ae40d9a06d0e7a2877b62e32eb959afbe16d", "mixHash": "0xd93c06ec00e2c653b7958114ba8224aad8749caf8de6aee2c2f465c5f09cc0cc", "nonce": "0x34b98c94071402d8", "number": "0x29bf9c", "parentHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "receiptsRoot": "0x0506189cdc814f4440690b43aaf7cf278a9b346b8ef3174c03dde2d23aa820ea", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x299a", "stateRoot": "0xf8be81979f9a92cd123f8e6295dca2660184df4f58e275c6c9fe7adee0016e7c", "timestamp": "0x5a952da9", "totalDifficulty": "0x1bd6b7e3c7b473", "transactions": []map[string]interface{}{{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9c", "from": "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0xa9059cbb0000000000000000000000001ee49d37ab544a0068d0bb8dc7b76ee8e7e4ec83000000000000000000000000000000000000000000000000000012309ce54000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0xc4581843a8dacd100c7d435bb00b2a20d038e31d", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0x4bdb8ab50804004410241002040000c60890801000000000000000000000000000000000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}}, "transactionsRoot": "0x08e95959ada5ebbe3aae1a4b9179f811c326c0969b7a5fea75b4e427c2870f96", "uncles": []string{}},
	// block 3: a token transferTo send transaction
	map[string]interface{}{"difficulty": "0x7ee56684", "extraData": "0x414952412f7630", "gasLimit": "0x47b784", "gasUsed": "0x47addd", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed8", "logsBloom": "0x0000000001400004002008000002000080000000000120200120002400208220000040000001000000000004804800000104000000000c0000000008201000005000200000010000140000084000000000000000100010400000080000040080100082000000000000000000004000021000800400802000000000501000000200000400000200020040010040000010105000000000040120000008000800200801000008004000000400004040000100000000000400000d005000020000008000004280010000000000000000000020010180100000140000000000020000000000000000008008000000000040000040100004001002c040000000000000", "miner": "0x00d8ae40d9a06d0e7a2877b62e32eb959afbe16d", "mixHash": "0xd93c06ec00e2c653b7958114ba8224aad8749caf8de6aee2c2f465c5f09cc0cc", "nonce": "0x34b98c94071402d8", "number": "0x29bf9d", "parentHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed7", "receiptsRoot": "0x0506189cdc814f4440690b43aaf7cf278a9b346b8ef3174c03dde2d23aa820ea", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x299a", "stateRoot": "0xf8be81979f9a92cd123f8e6295dca2660184df4f58e275c6c9fe7adee0016e7c", "timestamp": "0x5a952da9", "totalDifficulty": "0x1bd6b7e3c7b473", "transactions": []map[string]interface{}{{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9d", "from": "0xc4581843a8dacd100c7d435bb00b2a20d038e31d", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0x4bdb8ab50804004410241002040000c60890801000000000000000000000000000000000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9d", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x23b872dd000000000000000000000000357dd3856d856197c1a000bbAb4aBCB97Dfc92c4000000000000000000000000c4581843a8dacd100c7d435bb00b2a20d038e31d000000000000000000000000000000000000000000000000000012309ce54000", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}}, "transactionsRoot": "0x08e95959ada5ebbe3aae1a4b9179f811c326c0969b7a5fea75b4e427c2870f96", "uncles": []string{}},
	// block 4: a token transferTo receive transaction
	map[string]interface{}{"difficulty": "0x7ee56684", "extraData": "0x414952412f7630", "gasLimit": "0x47b784", "gasUsed": "0x47addd", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed9", "logsBloom": "0x0000000001400004002008000002000080000000000120200120002400208220000040000001000000000004804800000104000000000c0000000008201000005000200000010000140000084000000000000000100010400000080000040080100082000000000000000000004000021000800400802000000000501000000200000400000200020040010040000010105000000000040120000008000800200801000008004000000400004040000100000000000400000d005000020000008000004280010000000000000000000020010180100000140000000000020000000000000000008008000000000040000040100004001002c040000000000000", "miner": "0x00d8ae40d9a06d0e7a2877b62e32eb959afbe16d", "mixHash": "0xd93c06ec00e2c653b7958114ba8224aad8749caf8de6aee2c2f465c5f09cc0cc", "nonce": "0x34b98c94071402d8", "number": "0x29bf9e", "parentHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed8", "receiptsRoot": "0x0506189cdc814f4440690b43aaf7cf278a9b346b8ef3174c03dde2d23aa820ea", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x299a", "stateRoot": "0xf8be81979f9a92cd123f8e6295dca2660184df4f58e275c6c9fe7adee0016e7c", "timestamp": "0x5a952da9", "totalDifficulty": "0x1bd6b7e3c7b473", "transactions": []map[string]interface{}{{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9e", "from": "0xc4581843a8dacd100c7d435bb00b2a20d038e31d", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0x4bdb8ab50804004410241002040000c60890801000000000000000000000000000000000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9e", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf61", "input": "0x23b872dd000000000000000000000000c4581843a8dacd100c7d435bb00b2a20d038e31d000000000000000000000000357dd3856d856197c1a000bbAb4aBCB97Dfc92c4000000000000000000000000000000000000000000000000000012309ce54000", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9e", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}}, "transactionsRoot": "0x08e95959ada5ebbe3aae1a4b9179f811c326c0969b7a5fea75b4e427c2870f96", "uncles": []string{}},
	// block 5 - it is not chained
	map[string]interface{}{"difficulty": "0x7ee56684", "extraData": "0x414952412f7630", "gasLimit": "0x47b784", "gasUsed": "0x47addd", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed9", "logsBloom": "0x0000000001400004002008000002000080000000000120200120002400208220000040000001000000000004804800000104000000000c0000000008201000005000200000010000140000084000000000000000100010400000080000040080100082000000000000000000004000021000800400802000000000501000000200000400000200020040010040000010105000000000040120000008000800200801000008004000000400004040000100000000000400000d005000020000008000004280010000000000000000000020010180100000140000000000020000000000000000008008000000000040000040100004001002c040000000000000", "miner": "0x00d8ae40d9a06d0e7a2877b62e32eb959afbe16d", "mixHash": "0xd93c06ec00e2c653b7958114ba8224aad8749caf8de6aee2c2f465c5f09cc0cc", "nonce": "0x34b98c94071402d8", "number": "0x29bf9f", "parentHash": "0x25e2e6cfc2f49ef320c652d91a7bea99a2d115d29ea832631e5f11911a463158", "receiptsRoot": "0x0506189cdc814f4440690b43aaf7cf278a9b346b8ef3174c03dde2d23aa820ea", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x299a", "stateRoot": "0xf8be81979f9a92cd123f8e6295dca2660184df4f58e275c6c9fe7adee0016e7c", "timestamp": "0x5a952da9", "totalDifficulty": "0x1bd6b7e3c7b473", "transactions": []map[string]interface{}{{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0xc4581843a8dacd100c7d435bb00b2a20d038e31d", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0x4bdb8ab50804004410241002040000c60890801000000000000000000000000000000000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, {"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}}, "transactionsRoot": "0x08e95959ada5ebbe3aae1a4b9179f811c326c0969b7a5fea75b4e427c2870f96", "uncles": []string{}},
}
