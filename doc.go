// Package adp and its sub-packages implement the backend services to interact with multiple blockchains or other types
// of similar networks.
/*
adp provides you with two microservices:

1) a wallet microservice (package wallet) that implements a RESTful API for user requests such as checking the balance
 of an address or account, sending and getting details of transactions and monitoring addresses.

2) an explorer microservice (package explorer) that provides real-time events for those addresses or accounts that
 monitoring has been requested for.

Architecture

The wallet and explorer services communicate via a message broker. The user can request the explorer to monitor
blockchain addresses or accounts channeling requests to the message broker. The explorer service consumes requests
and monitors addresses. When an address is involved in a transaction, the explorer will send an event to the message
broker. Wallet services can then listen to the broker to notify their users about these events in real-time. The
message broker is implemented as a product agnostic layer (package lib/msg) and is configured via a JSON config file at
service startup.

Both wallet and explorer have their own database used for persistence. Each microservice's database can be standalone
or shared by the microservices. It's layered implementation (package lib/store) provides a database product agnostic
interface.

A blockchain layer (package lib/block) is implemented so new blockchain interfaces can be developed and added. The
layer provides basic functionality to request account balance, send and get transactions, etc. Both the wallet and
explorer services will connect to the blockchains or networks indicated in the JSON config file provided at startup.

Depending on workload and resources, one or more instances of the microservices can be orchestrated in order to provide
the required service level to the users.

The microservices can also be monitored via a Prometheus API by setting the flag "-m" at startup.

Wallet

The wallet microservice (package wallet) can be started running cmd/wallet/main.go or using Dockerfile.wallet. The
wallet exposes an HTTP RESTful API that can be used by multiple clients.
The API provides basic functionality to get the available networks, request account balances, set accounts for
monitoring and send transactions to the blockchains. It also provides a hierarchical deterministic wallet (HD wallet)
which comes quite handy in a single-user configuration. Transaction events sent by the explorer service are logged and
can be read by clients. Any client front-end can also get the events by consuming the appropriate queues of the message
broker.

Explorer

The explorer microservice (package explorer) can be started running cmd/explorer/main.go or using Dockerfile.explorer.
The explorer scans mined blocks of the configured networks and sends transaction events to the message broker when an
account or address being monitored is involved. Wallet services can send requests for the explorer to start or stop
monitoring addresses so that real time eventing can be provided to the clients or front-end.

*/
package adp
