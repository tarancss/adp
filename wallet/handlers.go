package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/tarancss/adp/lib/block"
	"github.com/tarancss/adp/lib/block/types"
	"github.com/tarancss/adp/lib/msg"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/util"
	"github.com/tarancss/ethcli"
	"github.com/tarancss/hd"
)

// TxReq transaction request data required to send transactions to the networks. Wallet, Change and Id correspond to
// the HDwallet address from which the transaction will be sent.
type TxReq struct {
	Wallet uint32      `json:"wallet"`
	Change uint8       `json:"change"`
	ID     uint32      `json:"id"`
	Net    string      `json:"net"` // blockchain to submit the transaction to
	Tx     types.Trans `json:"tx"`  // transaction details
}

// DryRun is a bool used to control sending transactions to the blockchain. When true, it will not send transactions
// but just do a dry run.
var DryRun bool = false //nolint:gochecknoglobals // consider adding this to config

// Errors returned to client requests.
var (
	ErrBadMethod  = errors.New("bad method in request")
	ErrBadrequest = errors.New("bad request")
	ErrChange     = errors.New("invalid change: has to be either 0 /1 or external / change")
	ErrMissingNet = errors.New("undefined blockchain - missing query: ?net=<blockchain>")
	ErrNoAddr     = errors.New("undefined address - missing in uri")
	ErrNoHash     = errors.New("a 32-byte hash is required")
	ErrNoNet      = errors.New("network not available")
)

// Response defines the data structure returned to the client making the http request.
type Response struct {
	Body  string `json:"body"`
	Error string `json:"error,omitempty"`
}

// homeHandler just replies a welcome message to the client.
func (w *Wallet) homeHandler(rw http.ResponseWriter, r *http.Request) {
	var res Response
	// log request
	log.Printf("httpreq from %v %s\n", r.RemoteAddr, r.RequestURI)
	// just reply a welcome message
	res.Body = "Hello, this is your multi-blockchain adaptor!"
	// reply
	rw.Header().Set("Content-Type", "application/json;charset=utf8")
	_ = json.NewEncoder(rw).Encode(res)
}

// networksHandler replies the networks available to the wallet.
func (w *Wallet) networksHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	pl := make([]string, 0, len(w.bc))

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			rw.WriteHeader(http.StatusBadRequest)
		} else {
			rw.WriteHeader(http.StatusOK)
			tmp, _ := json.Marshal(pl)
			res.Body = string(tmp)
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s res:%+v err:%e\n", r.RemoteAddr, r.RequestURI, pl, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()

	for net := range w.bc {
		pl = append(pl, net)
	}
}

// addrBalance struct used to get balances of addresses from the networks.
type addrBalance struct {
	Net string `json:"net"`           // blockchain name
	Bal string `json:"bal"`           // balance of blockchain currency of address
	Tok string `json:"tok,omitempty"` // balance of token of address
}

// addrBalHandler replies the balance of the address requested. If a token is specified, it will also reply the
// balance of the address in tokens for all the networks specified in the query.
func (w *Wallet) addrBalHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	var bals []addrBalance

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			rw.WriteHeader(http.StatusBadRequest)
		} else {
			rw.WriteHeader(http.StatusOK)
			tmp, _ := json.Marshal(bals)
			res.Body = string(tmp)
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s bals:%+v err:%e\n", r.RemoteAddr, r.RequestURI, bals, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()

	// parse request
	if err = r.ParseForm(); err != nil {
		log.Print("Error parsing request URL")

		return
	}

	v := mux.Vars(r)
	if address, ok := v["address"]; ok {
		var tok string = ""

		var nets []string

		if r.Form != nil {
			// get token
			if stok, okT := r.Form["tok"]; okT {
				tok = stok[0]
			}
			// get blockchains
			nets = r.Form["blk"]
		}
		// call all the clients
		for name, client := range w.bc {
			if len(nets) == 0 || util.In(nets, name) {
				ethBal, tokBal, err := client.Balance(address, tok)
				if err != nil {
					if tok != "" && errors.Is(err, ethcli.ErrBadAmt) {
						// this case happens when the token does not exist for the given blockchain
						tokBal = tokBal.SetInt64(0)
						err = nil
					} else {
						log.Printf("error getting balance for blockchain %s:%e\n", name, err)

						return
					}
				}

				bals = append(bals, addrBalance{Net: name, Bal: ethBal.String(), Tok: tokBal.String()})
			}
		}
	} else {
		err = ErrNoAddr
	}
}

// hdAddrHandler replies the HD wallet address requested to the client.
func (w *Wallet) hdAddrHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	var addr []byte

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			rw.WriteHeader(http.StatusBadRequest)
		} else {
			rw.WriteHeader(http.StatusOK)
			res.Body = "0x" + hex.EncodeToString(addr)
		}
		// log request and address
		log.Printf("httpreq from %v %s addr:0x%x err:%e\n", r.RemoteAddr, r.RequestURI, addr, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()

	// decode query, it must contain wallet, change and id
	var wallet, id uint64

	var change uint8

	if err = r.ParseForm(); err != nil {
		log.Print("Error parsing request URL")

		return
	}
	// get wallet
	tmp, ok := r.Form["wallet"]
	if ok {
		if wallet, err = strconv.ParseUint(tmp[0], 0, 32); err != nil {
			log.Printf("Wallet %s could not be decoded into a valid wallet number", tmp[0])

			return
		}
	} else {
		err = ErrBadrequest

		return
	}
	// get change
	tmp, ok = r.Form["change"]
	if ok {
		switch tmp[0] {
		case "0":
			change = hd.External
		case "external":
			change = hd.External
		case "1":
			change = hd.Change
		case "change":
			change = hd.Change
		default:
			log.Printf("Change %s could not be decoded into a valid change value", tmp[0])

			err = ErrChange

			return
		}
	} else {
		err = ErrBadrequest

		return
	}
	// get id
	tmp, ok = r.Form["id"]
	if ok {
		if id, err = strconv.ParseUint(tmp[0], 0, 32); err != nil {
			log.Printf("Id %s could not be decoded into a valid id number", tmp[0])

			return
		}
	} else {
		err = ErrBadrequest

		return
	}
	// get HD address
	if addr, _, _, err = w.hd.Address(uint32(wallet), change, uint32(id)); err != nil {
		log.Printf("Error obtaining HD wallet address for :%d %d %d\n", wallet, change, id)
	}
}

// listenHandler sends a wallet request message to the broker to start or stop monitoring an address or account. A
// request accepted status will be replied or an error otherwise.
func (w *Wallet) listenHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	var id []byte

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			rw.WriteHeader(http.StatusBadRequest)
		} else {
			res.Body = string(id)

			rw.WriteHeader(http.StatusAccepted)
		}
		// log request
		log.Printf("httpreq from %v %s id:%x err:%e\n", r.RemoteAddr, r.RequestURI, id, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()

	v := mux.Vars(r)
	if address, ok := v["address"]; ok {
		address = strings.ToLower(address) // keep everything in lowercase to avoid issues
		// get network
		if err = r.ParseForm(); err != nil {
			log.Print("Error parsing request URL")

			return
		}

		net, okN := r.Form["net"]
		if !okN || len(net) != 1 { // we only allow 1 net per request
			err = ErrMissingNet

			return
		}

		var wr msg.WalletReq = msg.WalletReq{Net: net[0], Type: msg.ADDRESS, Obj: address}

		switch r.Method {
		case "POST":
			wr.Act = msg.LISTEN
		case "DELETE":
			wr.Act = msg.UNLISTEN
		default:
			err = ErrBadMethod
		}
		// send message to broker
		if err == nil {
			err = w.mb.SendRequest(net[0], wr)
		}
	} else {
		err = ErrNoAddr
	}
}

// getAddrHandler replies the client with the addresses being monitored for the specified network. If no network is
// queried, addresses from all the networks are returned.
func (w *Wallet) getAddrHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	var addrs []store.ListenedAddresses

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			rw.WriteHeader(http.StatusBadRequest)
		} else {
			tmp, _ := json.Marshal(addrs)
			res.Body = string(tmp)

			rw.WriteHeader(http.StatusAccepted)
		}
		// log request
		log.Printf("httpreq from %v %s addrs:%v err:%e\n", r.RemoteAddr, r.RequestURI, addrs, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()
	// get network
	if err = r.ParseForm(); err != nil {
		log.Print("Error parsing request URL")

		return
	}

	net, ok := r.Form["net"]
	if ok && len(net) != 1 { // we only allow 1 net per request
		err = ErrNoNet

		return
	}
	// get addresses from DB
	addrs, err = w.db.GetAddresses(net) // ideally, this should be requested to the explorer!!
}

// sendHandler creates a send ether or ERC20 token transaction and sends it to the appropriate network for execution.
// A response is given to the client with the transaction hash or error.
func (w *Wallet) sendHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	var hash []byte

	var txReq TxReq

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			if !errors.Is(err, ErrNoNet) {
				rw.WriteHeader(http.StatusBadRequest)
			} else {
				rw.WriteHeader(http.StatusNotFound)
			}
		} else {
			rw.WriteHeader(http.StatusAccepted)
			tmp, _ := json.Marshal(txReq.Tx)
			res.Body = string(tmp)
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s hash:0x%x err:%e\n", r.RemoteAddr, r.RequestURI, hash, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()

	// get request
	if err = json.NewDecoder(r.Body).Decode(&txReq); err != nil {
		log.Printf("Error decoding transaction request %+v\n", r.Body)

		return
	}

	var data, addr, key []byte

	var fee *big.Int = new(big.Int)

	// get HD wallet address and key
	if addr, key, _, err = w.hd.Address(txReq.Wallet, txReq.Change, txReq.ID); err != nil {
		log.Printf("Error obtaining HD wallet address for :%d %d %d\n", txReq.Wallet, txReq.Change, txReq.ID)

		return
	}
	// send tx ...
	b, ok := w.bc[txReq.Net]
	if !ok {
		err = ErrNoNet

		return
	}

	if len(txReq.Tx.Data) > 0 {
		data = []byte(txReq.Tx.Data)
	} else {
		data = nil
	}

	fee, hash, err = b.Send("0x"+hex.EncodeToString(addr), txReq.Tx.To, txReq.Tx.Token, txReq.Tx.Value,
		data, hex.EncodeToString(key), txReq.Tx.Price, DryRun)
	// load return values
	txReq.Tx.Hash = "0x" + hex.EncodeToString(hash)
	txReq.Tx.From = "0x" + hex.EncodeToString(addr)
	txReq.Tx.Fee = fee.Uint64()

	if err == nil {
		txReq.Tx.Status = ethcli.TrxPending
	} else {
		txReq.Tx.Status = ethcli.TrxFailed
		log.Printf("httpreq from %v %s hash:0x%x err:%e\n", r.RemoteAddr, r.RequestURI, hash, err)
	}
}

// txHandler gets the details of the specified transaction and network and replies it to the client request.
func (w *Wallet) txHandler(rw http.ResponseWriter, r *http.Request) {
	var err error

	var res Response

	tx := &types.Trans{}

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%s", err)

			rw.WriteHeader(http.StatusBadRequest)
		} else {
			rw.WriteHeader(http.StatusOK)
			tmp, _ := json.Marshal(*tx)
			res.Body = string(tmp)
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s tx:%+v err:%e\n", r.RemoteAddr, r.RequestURI, tx, err)
		// reply
		rw.Header().Set("Content-Type", "application/json;charset=utf8")
		_ = json.NewEncoder(rw).Encode(&res)
	}()

	if err = r.ParseForm(); err != nil {
		log.Print("Error parsing request URL")

		return
	}

	v := mux.Vars(r)
	if hash, ok := v["hash"]; ok && len(hash) == 66 { // 42 = 0x + 32 bytes
		// get network
		var b block.Chain

		if tmp, ok := r.Form["net"]; ok {
			if b, ok = w.bc[tmp[0]]; !ok {
				log.Printf("Blockchain client for network %s could not be found", tmp[0])

				err = ErrNoNet

				return
			}
		} else {
			err = ErrNoNet

			return
		}

		tx, err = b.Get(hash)
	} else {
		err = ErrNoHash
	}
}
