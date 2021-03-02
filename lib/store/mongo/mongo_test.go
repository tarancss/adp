package mongo

import (
	"testing"

	"github.com/tarancss/adp/lib/store"
)

var uri string = "mongodb://localhost:27017" //nolint:gochecknoglobals // default mongo uri for tests

func TestNewMongo(t *testing.T) {
	_, err := New(uri)
	if err != nil {
		t.Errorf("err:%e", err)
	}
}

func TestCloseMongo(t *testing.T) {
	m, err := New(uri)
	if err != nil {
		t.Errorf("err:%e", err)

		return
	}

	err = m.CloseMongo()
	if err != nil {
		t.Errorf("err:%e", err)
	}
}

func TestAddAddress(t *testing.T) {
	m, err := New(uri)
	if err != nil {
		t.Errorf("err:%e", err)

		return
	}

	defer m.CloseMongo()

	var net, address string = "ropsten", "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4"

	_, err = m.AddAddress(store.Address{ID: nil, Addr: address}, net)
	if err != nil {
		t.Errorf("err:%e", err)
	}
}

func TestGetAddresses(t *testing.T) {
	m, err := New(uri)
	if err != nil {
		t.Errorf("err:%e", err)

		return
	}

	defer m.CloseMongo()

	c, err := m.GetAddresses([]string{})
	if err != nil {
		t.Errorf("err:%e", err)
	} else if len(c) != 1 && c[0].Net != "ropsten" && len(c[0].Addr) != 1 {
		t.Errorf("expected one address but got:%+v\n", c)
	}
}

func TestRemoveAddress(t *testing.T) {
	m, err := New(uri)
	if err != nil {
		t.Errorf("err:%e", err)

		return
	}

	defer m.CloseMongo()

	var net, address string = "ropsten", "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4"

	err = m.RemoveAddress(store.Address{Addr: address}, net)
	if err != nil {
		t.Errorf("err:%e", err)
	}
}

func TestExplorer(t *testing.T) {
	m, err := New(uri)
	if err != nil {
		t.Errorf("err:%e", err)

		return
	}

	defer m.CloseMongo()

	var ne store.NetExplorer = store.NetExplorer{
		Block: 208,
		Bh:    []string{"first", "second", "third"},
		Bhi:   0,
		Map:   nil,
	}

	if err := m.SaveExplorer("ropsten", ne); err != nil {
		t.Errorf("SaveExplorer - err:%e", err)
	}

	if ne2, err2 := m.LoadExplorer("ropsten"); err2 != nil || ne2.Block != 208 || ne2.Bhi != 0 {
		t.Errorf("LoadExplorer - err:%e, ne2.Bh:%+v", err2, ne2.Bh)
	}
}
