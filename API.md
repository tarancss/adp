**Blockchain adaptor API**
----
  All addresses, tokens, hashes and amounts/values are expressed in 0x-hexadecimal format. Addresses, tokens require a 40 hex digit length whilst hashes require 64 hex digit length. Amounts are recommemded to have an even number of digits, for example if you want to express a 1280 amount use `0x0500` instead of `0x500`. 

* **URL:** /<br/>
  Prints a welcoming message.
  * **Method:** Any.
  * **Params:** None.
  * **Success Response:**<br/>
      * **Code:** 200 <br/>
    **ContentType:** `application/json;charset=utf8` <br/>
    **Content:** `Hello, this is your multi-blockchain adaptor!` <br/>

* **URL:** /networks<br/>
  Lists the blockchains available to the adaptor.
  * **Method:** `GET`
  * **Params:** None. 
  * **Success Response:** <br/>
        All other methods yield error code _405 Method not allowed_.
      * **Code:** 200 <br />
      **ContentType:** `application/json;charset=utf8` <br/>
      **Content:** `["mainNet","ropsten","rinkeby"]`<br/>
  
* **URL:** /address/{address}?tok={token}<br/>
  For each blockchain, returns the balance of the given address. If a token is specified in the query, the balance of that token is also returned.
  * **Method:** `GET`
  * **URL Params:**<br/> 
     **Required:** `address=[string]`<br/>
     **Optional:** `tok=[string]`
  * **Data Params:** None.
  * **Success Response:**
      * **Code:** 200 <br/>
    **ContentType:** `application/json;charset=utf8` <br/>
    **Content:** `{"bals":[{"net":"ropsten","bal":"1615795130433485760","tok":"8859520000000000"},{"net":"rinkeby","bal":"18128874093010005000","tok":"0"},{"net":"mainNet","bal":"0","tok":"0"}]}`
  * **Error Response:**
      All other methods return `405 Method not allowed`.

      * **Code:** 400 Bad request <br />
    **Content:** `{ error : "rpc.ServerError={"code":-32602,"message":"invalid argument 0: hex string has length 38, want 40 for common.Address"}" }`<br />
    **Content:** `{%!e(string=You need to supply a 20-byte address with format 0x<20-byte address>!)}`

  * **Notes:** If the address or token does not exist, a zero balance is returned.
  
* **URL:** /address?wallet={wallet}&change={change}&id={id}<br/>
  Returns the address requested from the HD wallet (hierarchical deterministic wallet).
  * **Method:** `GET`
  * **URL Params:**
     **Required:** <br/>
           `wallet=[integer]`<br/>
           `change=[integer]`<br/>
         `id=[integer]`
  * **Data Params:** None.
  * **Success Response:**
    * **Code:** 200 <br />
    **ContentType:** `application/json;charset=utf8` <br/>
    **Content:** `"0xfda36ac6df73422b7224ebd34b25b1a99c1c1c62"`
 
  * **Error Response:**
    * **Code:** 400 Bad request<br/>
    **Content:** `{%!e(string=Invalid change: has to be either 0 /1 or external / change)}`

  
* **URL:** /listen/{address}?net={blockchain}<br/>
  Requests the explorer to monitor (method POST) or stop monitoring (method DELETE) an address.
  * **Method:** `POST` or `DELETE`
  * **URL Params:**
     **Required:** <br/>
           `net=[string]`
  * **Data Params:** None.
  * **Success Response:**
    * **Code:** 201 <br />
    **ContentType:** `application/json;charset=utf8` <br/>
    **Content:** none
 
  * **Error Response:**
    * **Code:** 400 Bad request<br/>
    **Content:** `{%!e(string=Undefined blockchain - missing query: ?net=<blockchain>)}`

  
* **URL:** /send
<br/>Sends a transaction to the specified blockchain returning the hash and other transaction details.
  * **Method:** `POST`<br/>
  * **URL Params: None.**
  * **Data Params:**
      * **Required:**<br/>
      `wallet=[integer]`<br/>
      `change=[integer]`<br/>
      `id=[integer]`<br/>
      `net=[string]`<br/>
    `tx=[string]`<br/>

  * **Success Response:**
      * **Code:** 200<br/>
    **ContentType:** `application/json;charset=utf8` <br/>
    **Content:** `{"block":"","status":1,"hash":"0x","from":"0xf4cefc8d1afaa51d5a5e7f57d214b60429ca4378","to":"0x454545","value":"0x565656","gas":"","price":0,"fee":0,"ts":0}`<br/>
 
  * **Error Response:**

  <TODO>To do.

  * **Sample Call:**<br/>
From a terminal:<br/>
`curl -X POST -H "application/json" -d '{"wallet":2, "change":0, "id":1, "net":"ropsten", "tx":{"to":"0x454545","value":"0x565656"}}' localhost:3030/send`<br/>
From a go program:
```
// transaction to send
var txr rest.TxReq = rest.TxReq{
  Wallet: 2,
  Change: 0,
  Id:     1,
  Net:    "ropsten",
  Tx: types.Trans{
    To:    "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4",
    Value: "0x500000",
  },
}
var resp *http.Response
pl, err := json.Marshal(&txr)
if err != nil {
  panic(err)
}
resp, err = http.Post("http://localhost:3030/send", "application/json;charset=utf8", bytes.NewBuffer(pl))
if err != nil || resp.StatusCode != http.StatusAccepted {
  // yield error
} else {
  var p []byte = make([]byte, 512)
  var n int
  n, _ = resp.Body.Read(p)
  resp.Body.Close()
  fmt.Printf("response:%s err:%e\n", string(p[:n]), err)
  err = json.Unmarshal(p[:n], &txr.Tx)
  fmt.Printf("err:%e\ntrx:%+v\n", err, txr)
}
```

  
* **URL:** /tx/{hash}?net={blockchain}<br/>
  Returns the transaction data for the given hash and network.
  * **Method:** `GET`
  *  **URL Params:**<br/> 
    **Required:**<br/>`hash=[string]`<br/>
    `net=[string]`
  * **Data Params:** None.
  * **Success Response:**
      * **Code:** 200 <br/>
  **ContentType:** `application/json;charset=utf8` <br/>
    **Content:** `{"block":"7024699","status":2,"hash":"0x9626a3677e30331fc29a6e24d4e2c1693cd287c3588031ca43e18a27cedf3a6d","from":"0xcba75f167b03e34b8a572c50273c082401b073ed","to":"0x357dd3856d856197c1a000bbab4abcb97dfc92c4","token":"0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f","value":"0x038d7ea4c68000","data":"a9059cbb000000000000000000000000357dd3856d856197c1a000bbab4abcb97dfc92c400000000000000000000000000000000000000000000000000038d7ea4c68000","gas":"","price":0,"fee":36772000000,"ts":1577201600}`
 
  * **Error Response:**

      All other methods return `405 Method not allowed`.

      * **Code:** 400 Bad request <br />
    **Content:** `{%!e(string=Network client not found)}`<br/>
    **Reason:** The network specified is not available.<br /><br/>
    **Content:** `{%!e(string=You need to supply a 32-byte hash!)}`
     **Reason:** The hash specified is invalid.<br />

  * **Sample Call:**<br/>
From a terminal:<br/>
`curl "localhost:3030/tx/0x9626a3677e30331fc29a6e24d4e2c1693cd287c358ss8031ca43e18a27cedf3a6d?net=ropsten"` 
<br/>


