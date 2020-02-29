package mongo

import (
	"fmt"
	"testing"

	"github.com/tarancss/adp/lib/store"
)

var m store.DB
var uri string = "mongodb://localhost:27017"

func TestNewMongo(t *testing.T) {
	var err error
	m, err = New(uri)
	if err != nil {
		t.Errorf("err:%e", err)
	}
	return
}

func TestCloseMongo(t *testing.T) {
	var err error
	m, err = New(uri)
	err = m.(*Mongo).CloseMongo()
	if err != nil {
		t.Errorf("err:%e", err)
	}
	return
}

func TestAddAddress(t *testing.T) {
	var err error
	var id []byte
	var net, address string = "ropsten", "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4"

	if m, err = New(uri); err != nil {
		t.Errorf("err:%e", err)
	}

	id, err = m.AddAddress(store.Address{Addr: address}, net)
	if err != nil {
		t.Errorf("err:%e", err)
	} else {
		fmt.Printf("Address added id:%x\n", id)
	}
	m.(*Mongo).CloseMongo()
	return

}

func TestGetAddresses(t *testing.T) {
	var err error
	if m, err = New(uri); err != nil {
		t.Errorf("err:%e", err)
	}

	var c []store.ListenedAddresses
	c, err = m.GetAddresses([]string{})
	if err != nil {
		t.Errorf("err:%e", err)
	} else if len(c) != 1 && c[0].Net != "ropsten" && len(c[0].Addr) != 1 {
		t.Errorf("expected one address but got:%+v\n", c)
	}
	m.(*Mongo).CloseMongo()
	return

}

func TestRemoveAddress(t *testing.T) {
	var err error
	var net, address string = "ropsten", "0x357dd3856d856197c1a000bbAb4aBCB97Dfc92c4"

	if m, err = New(uri); err != nil {
		t.Errorf("err:%e", err)
	}
	err = m.RemoveAddress(store.Address{Addr: address}, net)
	if err != nil {
		t.Errorf("err:%e", err)
	}
	m.(*Mongo).CloseMongo()
	return
}

func TestExplorer(t *testing.T) {
	var err error
	var ne store.NetExplorer = store.NetExplorer{
		Block: 208,
		Bh:    []string{"first", "second", "third"},
		Bhi:   0,
	}

	if m, err = New(uri); err != nil {
		t.Errorf("err:%e", err)
	}

	if err := m.SaveExplorer("ropsten", ne); err != nil {
		t.Errorf("SaveExplorer - err:%e", err)
	}

	if ne2, err2 := m.LoadExplorer("ropsten"); err2 != nil || ne2.Block != 208 || ne2.Bhi != 0 {
		t.Errorf("LoadExplorer - err:%e, ne2.Bh:%+v", err2, ne2.Bh)
	}

	m.(*Mongo).CloseMongo()
	return
}
