// Package config provides helper functionality to read microservice configurations from JSON config files or OS ENV variables.
// The default configuration can be overriden first by:
//
// - a valid JSON config file (see cmd/conf.json for a sample) and then by
//
// - OS ENV variables: prefixed with ADP_ (ie. ADP_DBTYPE, ADP_DBCONN, ...). All OS ENV variables should be valid strings, except for ADP_BLOCKCHAINS which should be a string with a valid JSON format. For example:
// # export ADP_BLOCKCHAINS='[{"name":"ropsten","node":"https://ropsten.infura.io/NoPSZJipdt0sqtNlaJq5","secret":"","maxBlocks":8}]'
package config

import (
	"encoding/json"
	"log"
	"os"
)

// Default configuration variables
var (
	DBTypeDefault    = "mongodb"
	DbConnDefault    = "mongodb://localhost" // DbConnDefault = "mongodb://127.0.0.1"
	RestfulEPDefault = ""
	PortDefault      = "3030"
	SSLPortDefault   = ""
	SSLCertDefault   = ""
	SSLKeyDefault    = ""
	MbTypeDefault    = "amqp"
	MbConnDefault    = "amqp://guest:guest@localhost:5672"
	BcDefault        = []BlockConfig{
		BlockConfig{Name: "ropsten", Node: "https://ropsten.infura.io/NoPSZJipdt0sqtNlaJq5", Secret: "", MaxBlocks: 8},
		BlockConfig{Name: "rinkeby", Node: "https://rinkeby.infura.io/NoPSZJipdt0sqtNlaJq5", Secret: "", MaxBlocks: 8},
		BlockConfig{Name: "mainNet", Node: "https://mainnet.infura.io/NoPSZJipdt0sqtNlaJq5", Secret: "", MaxBlocks: 16},
	}
	SeedDefault = "642ce4e20f09c9f4d285c2b336063eaafbe4cb06dece8134f3a64bdd8f8c0c24df73e1a2e7056359b6db61e179ff45e5ada51d14f07b30becb6d92b961d35df4"
)

// BlockConfig defines the required fields for blockchain/network connection configuration. Node contains the url (ie. https://localhost:8545) and Secret is an optional field when Basic Authentication is required by the blockchain server.
type BlockConfig struct {
	Name      string `json:"name"`
	Node      string `json:"node"`
	Secret    string `json:"secret"`
	MaxBlocks int    `json:"maxBlocks"`
}

// ServiceConfig contains the required fields for the wallet and explorer microservices. Database, API endpoint, ports, SSL cert and key, message broker type and url, a slice for blockchain configs and the seed for the HD wallet.
type ServiceConfig struct {
	DbType          string        `json:"dbtype"`
	DbConn          string        `json:"dbconn"`
	RestfulEndpoint string        `json:"endpoint"`
	Port            string        `json:"port"`
	SSLPort         string        `json:"sslport"`
	SSLCert         string        `json:"sslcert"`
	SSLKey          string        `json:"sslkey"`
	MbType          string        `json:"mbtype"`
	MbConn          string        `json:"mbconn"`
	Bc              []BlockConfig `json:"blockchains"`
	Seed            string        `json:"hdseed"`
}

// ExtractConfiguration reads from the given JSON filename and returns the ServiceConfig or an error otherwise.
func ExtractConfiguration(filename string) (ServiceConfig, error) {
	conf := ServiceConfig{
		DBTypeDefault,
		DbConnDefault,
		RestfulEPDefault,
		PortDefault,
		SSLPortDefault,
		SSLCertDefault,
		SSLKeyDefault,
		MbTypeDefault,
		MbConnDefault,
		BcDefault,
		SeedDefault,
	}
	// read from config file first
	if filename != "" {
		file, err := os.Open(filename)
		if err != nil {
			log.Println("Configuration file not found.")
			return conf, err
		}
		if err = json.NewDecoder(file).Decode(&conf); err != nil {
			return conf, err
		}
	}
	// then override config values with OS ENV variables
	var tmp string
	if tmp = os.Getenv("ADP_DBTYPE"); tmp != "" {
		conf.DbType = tmp
	}
	if tmp = os.Getenv("ADP_DBCONN"); tmp != "" {
		conf.DbConn = tmp
	}
	if tmp = os.Getenv("ADP_ENDPOINT"); tmp != "" {
		conf.RestfulEndpoint = tmp
	}
	if tmp = os.Getenv("ADP_PORT"); tmp != "" {
		conf.Port = tmp
	}
	if tmp = os.Getenv("ADP_SSLPORT"); tmp != "" {
		conf.SSLPort = tmp
	}
	if tmp = os.Getenv("ADP_SSLCERT"); tmp != "" {
		conf.SSLCert = tmp
	}
	if tmp = os.Getenv("ADP_SSLKEY"); tmp != "" {
		conf.SSLKey = tmp
	}
	if tmp = os.Getenv("ADP_MBTYPE"); tmp != "" {
		conf.MbType = tmp
	}
	if tmp = os.Getenv("ADP_MBCONN"); tmp != "" {
		conf.MbConn = tmp
	}
	if tmp = os.Getenv("ADP_BLOCKCHAINS"); tmp != "" {
		if err := json.Unmarshal([]byte(tmp), &conf.Bc); err != nil {
			log.Println("Error reading blockchains from OS ENV ADP_BLOCKCHAINS.")
			return conf, err
		}

	}
	if tmp = os.Getenv("ADP_SEED"); tmp != "" {
		conf.Seed = tmp
	}
	return conf, nil
}
