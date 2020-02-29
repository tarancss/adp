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
	mtype "github.com/tarancss/adp/lib/msg/types"
	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/util"
	"github.com/tarancss/ethcli"
	"github.com/tarancss/hd"
)

// TxReq transaction request data required to send transactions to the networks. Wallet, Change and Id correspond to the HDwallet address fro which the transaction will be sent.
type TxReq struct {
	Wallet uint32      `json:"wallet"`
	Change uint8       `json:"change"`
	Id     uint32      `json:"id"`
	Net    string      `json:"net"` // blockchain to submit the transaction to
	Tx     types.Trans `json:"tx"`  // transaction details
}

// DryRun is a bool used to control sending transactions to the blockchain. When true, it will not send transactions but just do a dry run
var DryRun bool = false

// Errors returned to client requests
var (
	ErrBadMethod  = errors.New("Bad method in request")
	ErrChange     = errors.New("Invalid change: has to be either 0 /1 or external / change")
	ErrMissingNet = errors.New("Undefined blockchain - missing query: ?net=<blockchain>")
	ErrNoAddr     = errors.New("Undefined address - missing in uri")
	ErrNoHash     = errors.New("You need to supply a 32-byte hash!")
	ErrNoNet      = errors.New("Network not available")
)

// Response defines the data structure returned to the client making the http request.
type Response struct {
	Body  interface{} `json:"body"`
	Error string      `json:"error,omitempty"`
}

// homeHandler just replies a welcome message to the client.
func (h *Wallet) homeHandler(w http.ResponseWriter, r *http.Request) {
	var res Response
	// log request
	log.Printf("httpreq from %v %s\n", r.RemoteAddr, r.RequestURI)
	// just reply a welcome message
	res.Body = "Hello, this is your multi-blockchain adaptor!"
	// reply
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(res)
}

// networksHandler replies the networks available to the wallet.
func (h *Wallet) networksHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var pl []string

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			res.Body = pl
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s res:%+v err:%e\n", r.RemoteAddr, r.RequestURI, pl, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
	}()

	for name, _ := range h.bc {
		pl = append(pl, name)
	}
}

// addrBalance struct used to get balances of addresses from the networks
type addrBalance struct {
	Net string `json:"net"`          // blockchain name
	Bal string `json:"bal"`          // balance of blockchain currency of address
	Tok string `json"tok,omitempty"` // balance of token of address
}

// addrBalHandler replies the balance of the address requested. If a token is specified, it will also reply the balance of the address in tokens for all the networks specified in the query.
func (h *Wallet) addrBalHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var bals []addrBalance

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			res.Body = bals
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s bals:%+v err:%e\n", r.RemoteAddr, r.RequestURI, bals, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
	}()

	// parse request
	if err = r.ParseForm(); err != nil {
		log.Print("Error parsing request URL")
		return
	}
	v := mux.Vars(r)
	if address, ok := v["address"]; ok {
		var ethBal, tokBal *big.Int = new(big.Int), new(big.Int)
		var tok string = ""
		var nets []string
		// var ok bool
		if r.Form != nil {
			// get token
			if stok, ok := r.Form["tok"]; ok {
				tok = stok[0]
			}
			// get blockchains
			nets, ok = r.Form["blk"]
		}
		// call all the clients
		for name, client := range h.bc {
			if len(nets) == 0 || util.In(nets, name) {
				err = client.Balance(address, tok, ethBal, tokBal)
				if err != nil {
					if tok != "" && err == ethcli.ErrBadAmt {
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

// hdAddrHandler replies the HD wallet address requested to the client
func (h *Wallet) hdAddrHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var addr []byte

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			res.Body = "0x" + hex.EncodeToString(addr)
		}
		// log request and address
		log.Printf("httpreq from %v %s addr:0x%x err:%e\n", r.RemoteAddr, r.RequestURI, addr, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
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
	}
	// get change
	tmp, ok = r.Form["change"]
	if ok {
		if tmp[0] == "0" || tmp[0] == "external" {
			change = hd.External
		} else if tmp[0] == "1" || tmp[0] == "change" {
			change = hd.Change
		} else {
			log.Printf("Change %s could not be decoded into a valid change value", tmp[0])
			err = ErrChange
			return
		}
	}
	// get id
	tmp, ok = r.Form["id"]
	if ok {
		if id, err = strconv.ParseUint(tmp[0], 0, 32); err != nil {
			log.Printf("Id %s could not be decoded into a valid id number", tmp[0])
			return
		}
	}
	// get HD address
	if addr, _, _, err = h.hd.Address(uint32(wallet), change, uint32(id)); err != nil {
		log.Printf("Error obtaining HD wallet address for :%d %d %d\n", wallet, change, id)
	}
}

// listenHandler sends a wallet request message to the broker to start or stop monitoring an address or account. A request accepted status will be replied or an error otherwise.
func (h *Wallet) listenHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var id []byte

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			res.Body = id
			w.WriteHeader(http.StatusAccepted)
		}
		// log request
		log.Printf("httpreq from %v %s id:%x err:%e\n", r.RemoteAddr, r.RequestURI, id, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
	}()

	v := mux.Vars(r)
	if address, ok := v["address"]; ok {
		address = strings.ToLower(address) // keep everything in lowercase to avoid issues
		// get network
		if err = r.ParseForm(); err != nil {
			log.Print("Error parsing request URL")
			return
		}
		var net []string
		net, ok = r.Form["net"]
		if !ok || len(net) != 1 { // we only allow 1 net per request
			err = ErrMissingNet
			return
		}
		var wr mtype.WalletReq = mtype.WalletReq{Net: net[0], Type: mtype.ADDRESS, Obj: address}
		switch r.Method {
		case "POST":
			wr.Act = mtype.LISTEN
		case "DELETE":
			wr.Act = mtype.UNLISTEN
		default:
			err = ErrBadMethod
		}
		// send message to broker
		if err == nil {
			err = h.mb.SendRequest(net[0], wr)
		}
	} else {
		err = ErrNoAddr
	}
	return
}

// getAddrHandler replies the client with the addresses being monitored for the specified network. If not network is queried, addresses from all the networks are returned.
func (h *Wallet) getAddrHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var addrs []store.ListenedAddresses

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			res.Body = addrs
			w.WriteHeader(http.StatusAccepted)
		}
		// log request
		log.Printf("httpreq from %v %s addrs:%v err:%e\n", r.RemoteAddr, r.RequestURI, addrs, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
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
	addrs, err = h.db.GetAddresses(net)
}

// sendHandler creates a send ether or ERC20 token transaction and sends it to the appropriate network for execution. A response is given to the client with the transaction hash or error.
func (h *Wallet) sendHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var hash []byte
	var txReq TxReq

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusAccepted)
			res.Body = txReq.Tx
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s hash:0x%x err:%e\n", r.RemoteAddr, r.RequestURI, hash, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
	}()

	// get request
	if err = json.NewDecoder(r.Body).Decode(&txReq); err != nil {
		log.Printf("Error decoding transaction request %+v\n", r.Body)
		panic(err)
	}

	var data, addr, key []byte
	var fee *big.Int = new(big.Int)

	// get HD wallet address and key
	if addr, key, _, err = h.hd.Address(txReq.Wallet, txReq.Change, txReq.Id); err != nil {
		log.Printf("Error obtaining HD wallet address for :%d %d %d\n", txReq.Wallet, txReq.Change, txReq.Id)
		return
	}
	// send tx ...
	b, ok := h.bc[txReq.Net]
	if !ok {
		err = ErrNoNet
		return
	}
	if len(txReq.Tx.Data) > 0 {
		// if data, err = hex.DecodeString(txReq.Tx.Data); err != nil {
		// 	log.Printf("Error parsing Data in transaction request:%s\n", txReq.Tx.Data)
		// 	return
		// }
		data = []byte(txReq.Tx.Data)
	} else {
		data = nil
	}
	fee, hash, err = b.Send("0x"+hex.EncodeToString(addr), txReq.Tx.To, txReq.Tx.Token, txReq.Tx.Value, data, hex.EncodeToString(key), txReq.Tx.Price, DryRun)
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

// txHandler gets the details of the specified transaction and network and replies it to the client request
func (h *Wallet) txHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var res Response
	var tx types.Trans

	defer func() {
		// reply to requester accordingly
		if err != nil {
			res.Error = fmt.Sprintf("%e", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			res.Body = tx
		}
		// log request and tx hash
		log.Printf("httpreq from %v %s tx:%+v err:%e\n", r.RemoteAddr, r.RequestURI, tx, err)
		// reply
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		json.NewEncoder(w).Encode(&res)
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
			if b, ok = h.bc[tmp[0]]; !ok {
				log.Printf("Blockchain client for network %s could not be found", tmp[0])
				err = ErrNoNet
				return
			}
		} else {
			err = ErrNoNet
			return
		}
		blk, ts, fee, status, token, data, to, from, amount, errGet := b.Get(hash)
		if errGet != nil {
			err = errGet
			return
		}
		tx = types.Trans{
			Block:  strconv.FormatUint(blk, 10),
			Status: status,
			Hash:   hash,
			From:   from,
			To:     to,
			Token:  "0x" + hex.EncodeToString(token),
			Value:  amount,
			Data:   hex.EncodeToString(data),
			Price:  0,
			Gas:    "",
			Fee:    fee,
			Ts:     uint32(ts),
		}

	} else {
		err = ErrNoHash
	}
}
