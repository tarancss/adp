# adp

adp enables users to interact with multiple blockchains via a single, easy-to-use interface. Other non-blockchain networks can also be added by adapting their functionality to the interface. 

[![GoDoc](https://godoc.org/github.com/tarancss/adp?status.svg)](https://godoc.org/github.com/tarancss/adp)
[![Go Report Card](https://goreportcard.com/badge/github.com/tarancss/adp)](https://goreportcard.com/report/github.com/tarancss/adp)


#### Table of Contents

- [What is adp?](#what-is-adp?)
- [Wallet API documentation](#wallet-api-documentation)
- [Usage](#usage)
- [Configuring the microservices](#configuring-the-microservices)
- [Using the API from another golang program](#using-the-api-from-another-golang-program)
- [Testing adp](#testing-adp)
- [Deploying adp to Kubernetes](#deploying-adp-to-kubernetes)

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

#### Testing adp
###### Approach
Testing microservices requires a somehow different approach than monolithic applications. For adp, I have tried to test all functionality avoiding over testing. Except for unit testing, most tests involve setting up mock server for input from a blockchain and starting dependency services such as MongoDB and RabbitMQ.

Tests are included within the testable packages and are run in isolation. Each test will setup the required scenario - more for component and contract tests, less for unit tests. Test cases are presented with the expected results and are run. Be aware that tests may leave the dependency services Mongo and RabbitMQ statusses changed. Please run tests from the specific package folder with: `go test`.

End to end tests are out of scope and they should be run against a real blockchain with real data. Testing persistence of data by the microservices and message sending/receiving when microservices do not terminate gracefully is extremely important so that services can get up again from a stable status. In particular, this is important for the explorer and so once a block is fully explored, the status is updated to DB so that if the service failed, it would resume at the right block.

Also, load testing, availability testing, etc, are out of scope but are key tests to be considered and done once you plan to move to production.

###### Unit testing
package **lib/block/ethereum**: includes specific tests to decode transactions from received blocks.

package **lib/config**: tests reading microservice configuration from files.

package **lib/msg/amqp**: tests the setup of exchanges and queues.

package **lib/store/mongo**: tests persistence functionality for both wallet and explorer microservice.

###### Component testing
package **explorer/netexplorer**: tests block chaining and adding/removing addresses for exploring.

package **explorer**: tests blockchain exploration and detection of transactions for monitored addresses.


###### Integration testing
package **lib/msg/amqp**: tests sending and receiving messages for both microservices.


###### Contract testing
package **explorer**: tests that wallet requests are managed properly.

package **wallet**: tests exposed API.

#### Deploying adp to Kubernetes
Running adp in a Kubernetes cluster is a much better alternative than Docker if you want adp for a real production use case. Kubernetes offers loads of functionality to ensure your microservices keep running and enable you to scale your infrastructure to manage workload.

Here I provide 3 manifests to deploy adp in a kubernetes cluster.

###### adp_ingress.yaml
This manifest creates an Ingress object in your kubernetes installation. It exposes your backend service (wallet microservice) on http default port 80, so that you can access your adp API from your host. If you require HTTPS then add the https port 443 in the manifest.

Apply the manifest: `kubectl apply -f adp_ingress.yaml`

Once Kubernetes has set up the Ingress, obtain the IP address with `kubectl get ingress`. There you will have the IP address leading to the wallet service.
Edit your /etc/hosts file and add a line such as this:
`<ip address> www.my-adp.com` so that your host resolves the url to the ingress point for Kubernetes. Kubernetes will then route the requests to the wallet service in port 3030 as specified in the wallet microservice configuration. Then you can use adp using for example: `curl http://www.my-adp.com`

###### adp_ser.yaml
This manifest contains three service definitions. These allow the microservices resolve their MongoDB and RabbitMQ dependencies as well as exposing the wallet microservice to port 3030 externally to the ingress object.

Apply the manifest: `kubectl apply -f adp_ser.yaml`

###### adp_dep.yaml
This manifest declares the following objects:

- mongo: An object (pod) that runs the mongoDB image with a 10Mb storage volume so that data is persisted. You may update the storage line in the volumeClaimTemplate to increase the volume's capacity according to your needs.
- rmq: A pod that runs the rabbitMQ image also with a 10Mb storage volume to ensure exchanges, queues and messages are persisted.
- wallet: Two replicated pods that run the image for the wallet microservice both listening on port 3030. The mongoDB and rabbitMQ hostnames are passed as environment variables in the definition so the kubernetes DNS will resolve them to the corresponding IP address of the containers running them.
- explorer: A pod that runs the image for the explorer microservice. Like the wallet, MongoDB and RabbitMQ hostnames are passed via env variables.

###### Scaling adp
You will have noted that we have set up two replicas for the wallet microservice but just one (the default) for the mongo, rabbitMQ and explorer services.

The rationale is that the wallet it is the most likely service to require scaling. If many users send API requests, then you may need to provide further replicas to cope with the workload. You could then update and re-apply the manifest and Kubernetes will automatcally start new containers running the wallet. Kubernetes will loadbalance work across all the replicas to ensure application stability. 

The explorer deployment just specifies one replica, this is because the explorer's design does not allow more than one container/process per blockchain. In fact, one explorer microservice should be able to be configured to explore many networks as most of them only issue new blocks every few seconds. So scaling the explorer service is about having different explorers configured with fewer blockchains rather than increasing the number of replicas. For this, we would add a new deployment to the adp_dep.yaml manifest and pass an env variable with the required blockchain config. Automating this task is out of scope of this document.

The mongo and rabbitMQ services are defined as StatefulSets so that the data is persisted in the storage volumes. Both the wallet and explorer services make a rather reduced use of both services so just one replica is enough. However, if needed, these could be configured to have more replicas. This is also out of the scope of this document.
