package wallet

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/config"
	"github.com/tarancss/adp/lib/msg/amqp"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/store/db"
	"github.com/tarancss/hd"
)

func TestAPI(t *testing.T) {
	// start a mock blockchain server
	mock := httptest.NewServer(http.HandlerFunc(mockHandler))
	t.Logf("Info: running tests against mock blockchain in %s", mock.URL)
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

	// load HD wallet
	var hdw *hd.HdWallet
	var seed []byte
	if seed, err = hex.DecodeString("642ce4e20f09c9f4d285c2b336063eaafbe4cb06dece8134f3a64bdd8f8c0c24df73e1a2e7056359b6db61e179ff45e5ada51d14f07b30becb6d92b961d35df4"); err != nil {
		t.Errorf("Error decoding seed:%e", err)
	}
	if hdw, err = hd.Init(seed); err != nil {
		t.Errorf("Error initialising HD wallet:%e", err)
	}

	// define a chain at the mock server URL
	net := "ropsten"
	bc, err := block.Init([]config.BlockConfig{config.BlockConfig{Name: net, Node: mock.URL, Secret: "", MaxBlocks: 4}})
	defer block.End(bc)

	// set up server for API
	w := New(db.MONGODB, s, mb, bc, hdw)
	go w.Init("", "3030", "", "", "")
	time.Sleep(200 * time.Millisecond) // let the server come up

	// define tests
	cases := []struct {
		name, method, uri string      // case name, http method to use and uri
		obj               interface{} // object for POST, PUT, DELETE
		err               error       // error in the http request call
		status            int         // http status code
		errExp            string      // error expected
		resExp            interface{} // body result expected
	}{
		{"homePage_1", http.MethodGet, "http://localhost:3030", nil, nil, http.StatusOK, "", "Hello, this is your multi-blockchain adaptor!"},
		{"homePage_2", http.MethodGet, "http://localhost:3030/", nil, nil, http.StatusOK, "", "Hello, this is your multi-blockchain adaptor!"},
		{"homePage_3", http.MethodPost, "http://localhost:3030/", nil, nil, http.StatusOK, "", "Hello, this is your multi-blockchain adaptor!"},
		{"networks_0", http.MethodPost, "http://localhost:3030/networks", nil, nil, 405, "", []string{}},
		{"networks_1", http.MethodGet, "http://localhost:3030/networks", nil, nil, 200, "", []string{"ropsten"}},
		{"addrbal_0", http.MethodPost, "http://localhost:3030/address/0x?tok=0x", nil, nil, 405, "", ""},
		{"addrbal_1", http.MethodGet, "http://localhost:3030/address/0x", nil, nil, 200, "", []addrBalance{{Net: "ropsten", Bal: "0", Tok: "0"}}},
		{"addrbal_2", http.MethodGet, "http://localhost:3030/address/0x?tok=0x", nil, nil, 200, "", []addrBalance{{Net: "ropsten", Bal: "0", Tok: "0"}}},
		{"addrbal_3", http.MethodGet, "http://localhost:3030/address/0xcba75F167B03e34B8a572c50273C082401b073Ed", nil, nil, 200, "", []addrBalance{{Net: "ropsten", Bal: "1615796230433485760", Tok: "0"}}},
		{"addrbal_4", http.MethodGet, "http://localhost:3030/address/0xcba75F167B03e34B8a572c50273C082401b073Ed?tok=0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", nil, nil, 200, "", []addrBalance{{Net: "ropsten", Bal: "1615796230433485760", Tok: "751000000000000000"}}},
		{"address_0", http.MethodGet, "http://localhost:3030/address?wallet=2&change=external&id=1", nil, nil, http.StatusOK, "", "0xf4cefc8d1afaa51d5a5e7f57d214b60429ca4378"},
		{"address_1", http.MethodPost, "http://localhost:3030/address?wallet=2&change=external&id=1", nil, nil, http.StatusMethodNotAllowed, "", ""},
		{"address_2", http.MethodGet, "http://localhost:3030/address?wallet=2&id=1", nil, nil, http.StatusBadRequest, "Bad request", ""},
		{"listen_0", http.MethodGet, "http://localhost:3030/listen/0xcba75F167B03e34B8a572c50273C082401b073Ed", nil, nil, http.StatusBadRequest, "Undefined blockchain - missing query: ?net=<blockchain>", ""},
		{"listen_1", http.MethodGet, "http://localhost:3030/listen/0xcba75F167B03e34B8a572c50273C082401b073Ed?net=ropsten", nil, nil, http.StatusBadRequest, "Bad method in request", ""},
		{"listen_2", http.MethodPost, "http://localhost:3030/listen/0xcba75F167B03e34B8a572c50273C082401b073Ed?net=ropsten&net=rinkeby", nil, nil, http.StatusBadRequest, "Undefined blockchain - missing query: ?net=<blockchain>", ""},
		{"listen_3", http.MethodPost, "http://localhost:3030/listen/0xcba75F167B03e34B8a572c50273C082401b073Ed?net=ropsten", nil, nil, http.StatusAccepted, "", ""},
		{"listen_4", http.MethodDelete, "http://localhost:3030/listen/0xcba75F167B03e34B8a572c50273C082401b073Ed?net=ropsten", nil, nil, http.StatusAccepted, "", ""},
		{"getAdr_0", http.MethodPost, "http://localhost:3030/listen", nil, nil, http.StatusMethodNotAllowed, "", ""},
		{"getAdr_1", http.MethodGet, "http://localhost:3030/listen?net=mainNet", nil, nil, http.StatusAccepted, "", []store.ListenedAddresses{}},
		{"getAdr_2", http.MethodGet, "http://localhost:3030/listen", nil, nil, http.StatusAccepted, "", []store.ListenedAddresses{{Net: "ropsten", Addr: []store.Address{}}}},
		{"getAdr_3", http.MethodGet, "http://localhost:3030/listen?net=ropsten", nil, nil, http.StatusAccepted, "", []store.ListenedAddresses{{Net: "ropsten", Addr: []store.Address{}}}},
		{"send_0", http.MethodPut, "http://localhost:3030/send", nil, nil, http.StatusMethodNotAllowed, "", ""},
		{"send_1", http.MethodPost, "http://localhost:3030/send", TxReq{Net: "rinkeby"}, nil, http.StatusOK, "Network not available", types.Trans{}},
		{"send_2", http.MethodPost, "http://localhost:3030/send", TxReq{Net: "ropsten", Wallet: 2, Change: 0, Id: 1, Tx: types.Trans{To: "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", Value: "0x565656"}}, nil, http.StatusAccepted, "", types.Trans{To: "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", Value: "0x565656", Hash: "0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872", Status: 0, From: "0xf4cefc8d1afaa51d5a5e7f57d214b60429ca4378"}},
		{"tx_0", http.MethodPut, "http://localhost:3030/tx/0x123456", nil, nil, http.StatusMethodNotAllowed, "", types.Trans{}},
		{"tx_1", http.MethodGet, "http://localhost:3030/tx/0x123456", nil, nil, http.StatusBadRequest, "You need to supply a 32-byte hash!", types.Trans{}},
		{"tx_2", http.MethodGet, "http://localhost:3030/tx/0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872", nil, nil, http.StatusBadRequest, "Network not available", types.Trans{}},
		{"tx_3", http.MethodGet, "http://localhost:3030/tx/0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872?net=mainNet", nil, nil, http.StatusBadRequest, "Network not available", types.Trans{}},
		{"tx_3", http.MethodGet, "http://localhost:3030/tx/0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872?net=ropsten", nil, nil, http.StatusBadRequest, "Hash of transaction does not match with requested hash", types.Trans{}},
		{"tx_3", http.MethodGet, "http://localhost:3030/tx/0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872?net=ropsten", nil, nil, http.StatusOK, "", types.Trans{To: "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", Value: "0x565656", Hash: "0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872", Status: 0, From: "0xf4cefc8d1afaa51d5a5e7f57d214b60429ca4378"}},
	}

	// run tests
	for _, c := range cases {
		// make http request to API
		s, b, e, err := makeRequest(c.method, c.uri, c.obj)
		// check error in call, StatusCode and error response
		if err != c.err {
			t.Errorf("[%s] Error in response:%e expected:%e", c.name, err, c.err)
		} else if s != c.status {
			t.Errorf("[%s] Error in StatusCode:%d expected:%d", c.name, s, c.status)
		} else if e != c.errExp {
			t.Errorf("[%s] Error in response:%s expected:%s", c.name, e, c.errExp)
		} else {
			// unmarshal and test body response
			if s < http.StatusBadRequest {
				switch c.name[:len(c.name)-2] {
				case "homepage", "address":
					if b != c.resExp {
						t.Errorf("[%s] Error in response:%s expected:%s", c.name, b, c.resExp)
					}
				case "network":
					var got []string
					if err = json.Unmarshal([]byte(b), &got); err != nil {
						t.Errorf("[%s] Error unmarshaling body:%s error:%s", c.name, b, err)
					}
					sort.Strings(got)
					exp := c.resExp.([]string)
					sort.Strings(exp)
					if len(got) != len(exp) {
						t.Errorf("[%s] Error in response:%v expected:%v", c.name, got, exp)
					} else {
						for i := 0; i < len(exp); i++ {
							if got[i] != exp[i] {
								t.Errorf("[%s] Error in response:%v expected:%v", c.name, got, exp)
							}
						}
					}
				case "addrbal":
					var sab []addrBalance = []addrBalance{}
					if err = json.Unmarshal([]byte(b), &sab); err != nil {
						t.Errorf("[%s] Error unmarshaling body:%s error:%s", c.name, b, err)
					}
					// check results
					if len(sab) != len(c.resExp.([]addrBalance)) || (len(sab) > 0 && len(c.resExp.([]addrBalance)) > 0 && (sab[0].Net != c.resExp.([]addrBalance)[0].Net || sab[0].Bal != c.resExp.([]addrBalance)[0].Bal || sab[0].Tok != c.resExp.([]addrBalance)[0].Tok)) {
						t.Errorf("[%s] Error in response:%v expected:%v", c.name, sab, c.resExp.([]addrBalance))
					}
				case "listen":
					if b != "" && b != c.resExp.(string) {
						t.Errorf("[%s] Error in response:%s expected:%s", c.name, b, c.resExp.(string))
					}
				case "getAdr":
					var sla []store.ListenedAddresses = []store.ListenedAddresses{}
					if b != "" {
						if err = json.Unmarshal([]byte(b), &sla); err != nil {
							t.Errorf("[%s] Error unmarshaling body:%s error:%s", c.name, b, err)
						}
					}
					// check results
					if len(sla) != len(c.resExp.([]store.ListenedAddresses)) || (len(sla) > 0 && len(c.resExp.([]store.ListenedAddresses)) > 0 && (sla[0].Net != c.resExp.([]store.ListenedAddresses)[0].Net)) {
						t.Errorf("[%s] Error in response:%v expected:%v", c.name, sla, c.resExp.([]store.ListenedAddresses))
					}
				case "send", "tx":
					var tx types.Trans
					if b != "" {
						if err = json.Unmarshal([]byte(b), &tx); err != nil {
							t.Errorf("[%s] Error unmarshaling body:%s error:%s", c.name, b, err)
						}
					}
					// check result
					if tx.Hash != c.resExp.(types.Trans).Hash || tx.To != c.resExp.(types.Trans).To || tx.From != c.resExp.(types.Trans).From || tx.Value != c.resExp.(types.Trans).Value {
						t.Errorf("[%s] Error in response:%v expected:%v", c.name, tx, c.resExp.(types.Trans))
					}
				}
			}
		}
	}
	w.Stop()
}

// makeRequest places a http request on uri. Depending on method it will include obj in the request (ie. for POST). Returns the status code, the body and error fields of the received JSON response.
func makeRequest(method, uri string, obj interface{}) (s int, b, e string, err error) {
	var resp *http.Response
	switch method {
	case http.MethodGet:
		if resp, err = http.Get(uri); err != nil {
			return
		}
	case http.MethodPost:
		var pl []byte
		switch v := obj.(type) {
		case TxReq:
			if pl, err = json.Marshal(v); err != nil {
				return
			}
		default:
			if pl, err = json.Marshal(v); err != nil {
				return
			}
		}
		if resp, err = http.Post(uri, "application/json;charset=utf8", bytes.NewBuffer(pl)); err != nil {
			return
		}
	case http.MethodDelete, http.MethodPut:
		client := &http.Client{}
		var req *http.Request
		if req, err = http.NewRequest(method, uri, nil); err != nil {
			return
		}
		if resp, err = client.Do(req); err != nil {
			return
		}
	default:
		err = errors.New("Method not found!!")
		return
	}

	s = resp.StatusCode
	var v struct {
		B string `json:"body"`
		E string `json:"error"`
	}
	if resp.ContentLength > 0 {
		var p []byte = make([]byte, int(resp.ContentLength))
		var n int
		n, _ = resp.Body.Read(p)
		resp.Body.Close()
		err = json.Unmarshal(p[:n], &v)
	}
	return s, v.B, v.E, err
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

// mockHandler defines the handler function for mock HTTP server
var mockHandler = func(w http.ResponseWriter, r *http.Request) {
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
		// fmt.Printf("[Mock server] Request Body:%s", body)  // (un)comment this line for less/more verbosity
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

// mock contains the data used by the mock server.
var mock []interface{} = []interface{}{
	// addrBal_1
	"0x00",
	// addrBal_2
	"0x00",
	"0x0000000000000000000000000000000000000000000000000000000000000000",
	// addrBal_3
	"0x166c761c586733c0",
	// addrBal_4
	"0x166c761c586733c0",
	"0x0000000000000000000000000000000000000000000000000a6c168562518000",
	// send_2
	"0x10",     // getTransactionCount
	"0x100000", // getGasPrice
	"0x5208",   // estimateGas
	"0x",       // sendRawTransaction
	// tx_3
	map[string]interface{}{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"},
	// tx_4 (3 calls to node)
	map[string]interface{}{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0xf4cefc8d1afaa51d5a5e7f57d214b60429ca4378", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", "transactionIndex": "0x1", "v": "0x29", "value": "0x565656"},
	map[string]interface{}{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "contractAddress": nil, "cumulativeGasUsed": "0x4fa3d", "from": "0xf4cefc8d1afaa51d5a5e7f57d214b60429ca4378", "gas": "0xff59", "gasPrice": "0x98bca5a00", "gasUsed": "0xf67f", "hash": "0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872", "input": "0x", "logs": map[string]interface{}{"address": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "data": "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000", "logIndex": "0x2", "removed": false, "topics": []string{"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef", "0x0000000000000000000000008bac1770a2826111c0e773f39827c1cfa031a21e", "0x0000000000000000000000001cd434711fbae1f2d9c70001409fd82d71fdccaa"}, "transactionHash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "transactionIndex": "0x1"}, "logsBloom": "0x00000000000000000000000000000000800000000000000000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000008000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000010000000020000000000000000000000000000000000000000000000002000000000000000000100000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "status": "0x1", "to": "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4", "transactionHash": "0x2ba030485e79b5a98275b45d940e6fdd07b40dea593ef3b2a69b0a02a68a5872", "transactionIndex": "0x1", "v": "0x29", "value": "0x565656"},
	map[string]interface{}{"difficulty": "0x73622046", "extraData": "0xde8302050c8f5061726974792d457468657265756d86312e33382e30826c69", "gasLimit": "0x7a121d", "gasUsed": "0x2120e6", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "logsBloom": "0x00000008000000000000200000000000020000000040000000000000000000000000000020008000000000001000000000000200000000004000800080000000000000000000000000000008000000000000004000000000000000000000000000000400005000880000000000400020000004000000000008000010000000000000000000000000000000000800002000000000000000000000100000000000000000000000000020000000000000000000000000000000020000000000000000000002400000000402000000200000000000000000004000400000000401000000000000000040000000000000000001000108000000000000001000000000", "miner": "0x635b4764d1939dfacd3a8014726159abc277becc", "mixHash": "0x0d186ce62b77e466e4f66b30d1bbeff71b210f3bce72a6f7210a34edf84d9d98", "nonce": "0x87cce426abc7bcd5", "number": "0x29bf9b", "parentHash": "0x89cde9ba035de527c0fc03dd816e8205cb9c52bd9b7dc79567e72adce2460686", "receiptsRoot": "0x572216203b3b24631ea63c2f366f4d15612b6b120590350f3b8dffb69c6549bc", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x1936", "stateRoot": "0xd6805cc98512c0f1f0086761f15a5bcbeb45db4c4b30997d08ca50511b127d72", "timestamp": "0x5dfeaab3", "totalDifficulty": "0x6912173ecef77f", "transactions": []string{"0x8f138401bff60dbd947ccdf9eceef46c8e0ccd97043027de566c42f022a8abcc", "0x8b2db064cdeacff34f18eb16c74298f6b5692b095c678759a39e16682c98ea7a", "0xe6ee32f5df6a617bb965df1356842cb9b34c3ccc11049cb9cf2f8a6a6afcc0f9", "0x795aad22918d79ccfe1ef7522b4c147a0fc650014e5f240ed04c16334ab7240c", "0xb8afe6057b31c7c63c5382175da72bd026e0f5b7e05800c5801cc41fb43db798", "0x4411c92db0d7b4b8d77fe53f4111712fc7cdecaa78725076544fbc6abdf2d019", "0xf536f7479394bd241be973677cbffe5fec6d28b536d5f30979281de5c34ceebf", "0x8bafdabef7b99aefeafbd8539e2b7072f4cf56a3c92e20dc2c201f5855ff39b3", "0x5c1e8a3b59838a2b961fcd548ab06fea00273f23e0174dbcf0196213f0da4265", "0x0ef5e2aec0545017fa27a65eeeb45f587098f38ede31a57589d3f5d6f3043bdf", "0x9bcfe01a8242a0d2b422527cd6abc704fab21424bf8c0e7d759b639eff1f8f69", "0xcdbb0dc7d6c661fddfd9c1a728fed43fe65c2eea3380f9688a6fc30e85e20d24", "0x48633a32b409defbd805b07418827c647a2eb29d3f53ccdd22a901a304f7c9c5", "0xb92ebb3c76e8e24950560d370b233d0ab14061be41f830392cf0ceee6f73cc2c", "0x7eef4a0785fe1bf02e8a8c727c4b956f1abf0096e2186b64e8be50f504ad1447", "0x366f653f97a3d2705ece8b76f6df9b0243afbacb838fb80c96e24957ab17b5bc", "0x36e20c01159d2aab0100b00ca2758a4b066c947b051cf71f29a9efea594e57a0", "0x194a774856c85facc98cea05681aea3078b0cbd9ff908b5a7cbe1d70b9d28751", "0xdab4600e32a7f839b47f8d9e6e081b22b4ab744c462f75414f4d43100203e01b", "0xfb2ffb9c643dca812999a3fa4d14c7f78e0b80333789a33bd7ed54521b82ecc5", "0xc4f0690dd1e1c0e3c99418529da10463c46942979d905abfaf2eb126aeeedc21", "0x85564c0b32204909c77f73c23df7e8484655dde3e62b06f887e6095c79f67f3e", "0xe309317b02143e481a98174c1cfe2a5cfac6b0662f5b1f370fc3f34ee9d8da91", "0x096760b5183b7e53d7e4e74f5e82d92616850eb307e81d9c94fdc5a18e93f670", "0x0bd588bd04af3acb6b5d4bfc699d715aadb3c7084da4ccb21dd1b830d214edd0", "0x0e3875750c6292529ce4e1a8d407f478fb78bdfc20ee328f5a88558e0ccc3de3", "0x2e476d344e24108e215f3f110b2315c53ef47552f434b3746e1df9ae42fc65be", "0xa26bf3698db9b0664fcd91296642e31918faafe94794bf59f3a251196d3d06a3", "0x8eabd6f7894194cc6e523a30ffd4ccb41b87219f52460a93aac31114dd54c1bc", "0xba414f298d033d570db3fa83611f3f9be4e91a01d4dfcdcb2fb06fee02332cfd", "0xeb2dee95e748feca129635bef1ab2b22ff6695306ddd653185382861db1c7f51", "0x2fb5cca0931f44bb5a1169f7b3aad7defbe1d78e4b24bdc4da19313193000c81", "0x38e67f95ef6b730bc04da132df3b2530118f9d2963cbd6bb4f750779aa3e5653", "0x33764f5c8e687b7841c50861fac9770f0f4f3fe09ef9f713137dbe806516449c", "0x83cf99e79755a0604bfee68daa7ab81b2f14e5ba1c8f320b13b24abe4a6517d1", "0x9bc276f828200578c9ee5fcc4ba7459bbf01cc5192ed6aa8aacc34b22a7fd896", "0x18385aea472253973fc2b1266a0b22ff53052107c6345681e74df48f05b2bf6a", "0xce978f61e64a174ee56d3414887270598f574eb5a2038f51685981bafc8c78d7", "0x3f3f895f532d7aab86a0a25f6df799f673d35e27dd48ecb73c76e824fb63d302"}, "transactionsRoot": "0x0ba49975aecff1120685561471fcc58c87d3c270361d56367fd5206cb8957687", "uncles": []string{}},
}
