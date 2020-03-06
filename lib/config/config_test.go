// config_test.go tests config files
package config

import (
	"testing"
)

// fileToTest is a relative path to the configuration file to test (ie. adp/cmd/conf.json)
var fileToTest string = "../../cmd/conf.json"

// TestConfig extracts config from a file and checks values loaded
func TestConfig(t *testing.T) {
	//extract configuration
	conf, err := ExtractConfiguration(fileToTest)
	if err != nil {
		t.Errorf("Error reading config file:%e\n", err)
	} else {
		// lets check the port
		if conf.Port != "3030" {
			t.Errorf("config port is not the expected %s", conf.Port)
		}
		// and the blockchains
		if len(conf.Bc) != 3 {
			t.Errorf("blockchains do not match the expected %v", conf.Bc)
		} else {
			if conf.Bc[0].Name != "ropsten" || conf.Bc[1].Name != "rinkeby" || conf.Bc[2].Name != "mainNet" {
				t.Errorf("blockchains do not match the expected %v", conf.Bc)
			}
		}
	}
}
