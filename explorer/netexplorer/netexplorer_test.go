package netexplorer

import (
	"testing"

	"github.com/tarancss/adp/lib/store/db"
)

// TestNE unit tests the netexplorer package.
// Requires a MongoDB connection
// Covers tests for:
// - UpdateChain / Chained: make sure the revolving slice Bh and index Bhi behave correctly.
// - Add/Del objects to the monitoring map: test the monitoring map.
func TestNE(t *testing.T) {

	// connect to DB
	dbUri := "mongodb://localhost:27017"
	s, err := db.New(db.MONGODB, dbUri)

	// create a netexplorer
	var ne *NetExplorer
	var maxBlocks int = 4
	if ne, err = New("net", maxBlocks, nil, s); err != nil { // listenmap = nil
		t.Errorf("Error creating NetExplorer: %e", err)
	}

	// Test UpdateChain/Chained
	var tsChained []interface{} = []interface{}{
		// steps contain a previous hash to check, the expected boolean and a hash to update chain
		[]interface{}{"hash0", true, "hash1"},
		[]interface{}{"hash1", true, "hash2"},
		[]interface{}{"hash2", true, "hash3"},
		[]interface{}{"hash3", true, "hash4"},
		[]interface{}{"hash4", true, "hash5"},
		[]interface{}{"hash5", true, "hash6"},
		[]interface{}{"hash6bis", false, "hash6bis"},
		[]interface{}{"hash6", true, "hash7"},
		[]interface{}{"hash7", true, "hash8"},
		[]interface{}{"hash8", true, "hash9"},
	}
	for _, ts := range tsChained {
		if ne.Chained(ts.([]interface{})[0].(string)) != ts.([]interface{})[1].(bool) {
			t.Errorf("Previous hash error at %+v", ts)
		}
		if ts.([]interface{})[1].(bool) {
			ne.UpdateChain(ts.(([]interface{}))[2].(string), maxBlocks)
		}
	}
	// check the final result
	if ne.Block != 9 || ne.Bhi != 1 || ne.Bh[0] != "hash8" || ne.Bh[1] != "hash9" || ne.Bh[2] != "hash6" || ne.Bh[3] != "hash7" {
		t.Errorf("error ne:%+v", ne)
	}

	// Test Add/Del functionality
	var tsAddGet []interface{} = []interface{}{
		// steps contain a previous hash to check, the expected boolean and a hash to update chain
		[]interface{}{"del", "object1", "", false},
		[]interface{}{"add", "object1", "value1"},
		[]interface{}{"add", "object2", "value2"},
		[]interface{}{"del", "object3", "", false},
		[]interface{}{"del", "object1", "value1", true},
		[]interface{}{"add", "object1", "value1"},
		[]interface{}{"add", "object2", "value2-again"},
		[]interface{}{"add", "object4", "value4"},
		[]interface{}{"del", "object5", "", false},
	}
	var val interface{}
	var ok bool
	for _, ts := range tsAddGet {
		if ts.([]interface{})[0] == "add" {
			ne.Add(ts.([]interface{})[1].(string), ts.([]interface{})[2])
		} else {
			if val, ok = ne.Del(ts.([]interface{})[1].(string)); !ok {
				val = ""
			}
			if val.(string) != ts.([]interface{})[2].(string) || ok != ts.([]interface{})[3].(bool) {
				t.Errorf("Error with %+v", ts)
			}
		}
	}
	// check final result
	if len(ne.Map) != 3 {
		t.Errorf("Error with the Map:%v", ne.Map)
	}
}
