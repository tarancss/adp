# adp

adp enables users to interact with multiple blockchains via a single, easy-to-use interface. Other non-blockchain networks can also be added by adapting their functionality to the interface. 

[![GoDoc](https://godoc.org/github.com/tarancss/adp?status.svg)](https://godoc.org/github.com/tarancss/adp)
[![Go Report Card](https://goreportcard.com/badge/github.com/tarancss/adp)](https://goreportcard.com/report/github.com/tarancss/adp)

#### What is adp?

adp provides two backend microservices:

1) a wallet, that implements a RESTful [API](https://github.com/tarancss/adp/blob/master/API.md) for user requests such as checking the balance of an address or account, sending transactions to execute in the blockchain, getting details of transactions and monitoring addresses.

2) an explorer that provides real-time events for those addresses or accounts that monitoring has been requested for. If your use case does not require real-time eventing, you may opt to ignore this microservice. The explorer detects transfers of funds and/or tokens to the monitored addresses, sending one event per transaction detected.

Initially, I have built the interface for Ethereum type blockchains (mainNet, ropsten, rinkeby, etc). I am generally open to collaboration of any kind, one being adding more blockchain interfaces to adp.

Apart from expanding the available blockchains and add extra functionality, future plans go about building a front-end for end users.

#### Wallet API documentation
[API reference](https://github.com/tarancss/adp/blob/master/API.md)

#### Usage
###### Docker
If you dont have a golang installation, using a container environment like Docker is the easiest way to run adp. A docker compose file is provided for easy setup. Just run: `# docker-compose up`.

Alternatively, you can manually create and run the microservice containers. Use these steps from the directory adp:
```
# docker network create adp
# docker container create --name adp_rmq --network adp rabbitmq:latest
# docker container create --name adp_mongo --network adp mongo:latest
# docker container start adp_mongo
# docker container start adp_rmq

# docker image build -f cmd/wallet/Dockerfile -t wallet:1.0 .
# docker container create --name wallet --network=adp -p 3030:3030 wallet:1.0
# docker container start wallet
# docker image build -f cmd/explorer/Dockerfile -t explorer:1.0 .
# docker container create --name explorer --network=adp explorer:1.0
# docker container start explorer
```

If you want to set up the management UI for your rabbitMQ container follow these steps:
```
# docker exec -ti adp_rmq bash
root@e57f92d03137:/# rabbitmq-plugins enable rabbitmq_management
root@e57f92d03137:/# exit
# docker container restart adp_rmq
```

###### golang run
If you have a golang installation, you can simply run the services directly. You can provide a configuration file by specifying the -c flag at the command line. Like using docker, you can optionally enable Prometheus to monitor the microservices by using the -m flag. 

`go run main.go -c <config_file> [-m]`

###### Dependencies
Both wallet and explorer microservices require the use of a database for persistence and a message broker for communication. Whilst the architecture provides a product-agnostic interface, only MongoDB and RabbitMQ have currently been developed and tested. 

#### Configuring the microservices
Both wallet and explorer microservice have a configuration by default. However, you should provide your own if you plan to use adp seriously, especially the HD wallet seed. The default configuration can be overridden first by:

- a valid JSON config file (see cmd/conf.json for a sample) and then by
- OS ENV variables capitalised and prefixed with ADP_ (ie. ADP_DBTYPE, ADP_DBCONN, ...). All OS ENV variables should be valid strings, except for ADP_BLOCKCHAINS which should be a string with a valid JSON format. For example:
`# export ADP_BLOCKCHAINS='[{"name":"ropsten","node":"https://ropsten.infura.io/NoPSZJipdt0sqtNlaJq5","secret":"","maxBlocks":8}]'`

 A valid JSON configuration file should be provided with the following contents:

**Required:**
- endpoint: (only wallet) the url endpoint for the API service.
- port: (only wallet) the port if any
- blockchains: an array of blockchain definitions containing at least the following:
	- name: name of the blockchain
	- node: url or endpoiont of the blockchain node to connect to
	- secret: key used to connect to the blockchain [use "" if not required]
	- maxBlocks: the number of blocks to keep in memory in order to ensure new mined blocks are chained.
- hdseed: (only wallet) seed for the Hierearchical deterministic wallet to be used to send transactions.
- dbtype: database type, available "mongodb" and "postgres".
- dbconn: connection (uri) to the DB
- mbtype: messsage broker type, available "amqp"
- mbconn: connection (uri) to the message broker

**Optional (only wallet):**
- SSLport: if specified, the wallet will also listen HTTPS requests. Then, SSLcert and SSLkey must be specified and valid.
- SSLcert: path to the certificate file for HTTPS.
- SSLkey: path to the key file for HTTPS.

Config file sample:
```
{
	"endpoint": "",
	"port": "3030",
	"SSLport": "",
	"SSLcert": "",
	"SSLkey": "",

	"dbtype":"mongodb",
	"dbconn":"mongodb://127.0.0.1",
	"mbtype": "amqp",
	"mbconn": "amqp://guest:guest@localhost:5672",

	"blockchains": [
		{"name":"ropsten","node":"https://ropsten.infura.io/NoPSZJipdt0sqtNlaJq5", "secret":"", "maxBlocks": 8},
		{"name":"rinkeby","node":"https://rinkeby.infura.io/NoPSZJipdt0sqtNlaJq5", "secret":"", "maxBlocks": 8},
		{"name":"mainNet","node":"https://mainnet.infura.io/NoPSZJipdt0sqtNlaJq5", "secret":"", "maxBlocks": 16}
	],

	"hdseed": "642ce4e20f09c9f4d285c2b336063eaafbe4cb06dece8134f3a64bdd8f8c0c24df73e1a2e7056359b6db61e179ff45e5ada51d14f07b30becb6d92b961d35df4",

	"end": "end"
}
```

###### Default configuration
The default is for the wallet API to be at http://localhost:3030, MongoDB at mongodb://127.0.0.1 and RabbitMQ at amqp://guest:guest@localhost:5672.

Please ensure you **change and use your own hdseed value** if you are using adp for real blockchain transactions. Keep your seed secure at all times and do not share it as it is the key to your funds!

#### Using the API from another golang program
You can use adp to further build your own programs, applications or just learn with it. In fact, if you plan to use adp to manage your crypto assets, I encourage you to first become familiar with the usage of the wallet's API and even better, to explore and familiarise yourself with the code. Here follows a couple of examples of how to use adp within your own code:

###### Sending a transaction
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

###### Getting an HD wallet address
```
var resp *http.Response
var err error
if resp, err = http.Get("http://localhost:3030/address?wallet=2&change=external&id=1"); err!=nil {
	// yield error
}
var p []byte = make([]byte, 64)
var n int
n, _ = resp.Body.Read(p)
resp.Body.Close()
var address string = string(p[:n])
```
