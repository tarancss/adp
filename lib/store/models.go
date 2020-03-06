package store

// Address contains the fields for an address save to DB.
type Address struct {
	ID   []byte `json:"id"`
	Name string `json:"name"`
	Addr string `json:"addr"`
}

// ListenedAddresses contains the fields of monitored objects saved to DB.
type ListenedAddresses struct {
	Net  string    `json:"net"`
	Addr []Address `json:"addresses"`
}

// NetExplorer contains the fields for a NetExplorer type saved to DB.
type NetExplorer struct {
	Block uint64                 `json:"block" bson:"block"`
	Bh    []string               `json:"bh" bson:"bh"`
	Bhi   int                    `json:"bhi" bson:"bhi"`
	Map   map[string]interface{} `json:"map" bson:"map"`
}
