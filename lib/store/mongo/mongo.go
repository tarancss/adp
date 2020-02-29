// Implements the interface for MongoDB
package mongo

import (
	"context"
	"encoding/hex"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mgo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/util"
)

// Mongo implements a connection to a MongoDB database
type Mongo struct {
	c *mgo.Client
}

// MongoAddress implements a store address to MongoDB
type MongoAddress struct {
	ID   primitive.ObjectID `json:"_id" bson:"_id"`
	Name string             `json:"name,omitempty" bson:"name,omitempty"`
	Addr string             `json:"address" bson:"address"`
}

// Address converts a MongoAddress to store.Address type
func (a MongoAddress) Address() store.Address {
	return store.Address{ID: a.ID[:], Addr: a.Addr, Name: a.Name}
}

// New returns a Mongo client connection to the specified MongoDB database uri
func New(uri string) (store.DB, error) {
	// get a client
	c, err := mgo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return &Mongo{}, err
	}
	// connect client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = c.Connect(ctx)
	return &Mongo{c: c}, err
}

// CloseMongo will close a database connection. Must be called at termination time.
func (m *Mongo) CloseMongo() error {
	return m.c.Disconnect(context.Background())
}

// AddAddress saves an address if the address does not already exist.
func (m *Mongo) AddAddress(a store.Address, net string) (id []byte, err error) {
	var ma MongoAddress
	ma.Addr = a.Addr

	col := m.c.Database("addr").Collection(net)

	// try and find it
	filter := bson.M{"address": a.Addr}
	sr := col.FindOne(context.Background(), filter)
	err = sr.Decode(&ma)
	if err != nil {
		// if not found, do insert it!!
		if err == mgo.ErrNoDocuments {
			var res *mgo.InsertOneResult
			if res, err = col.InsertOne(context.Background(), bson.M{"name": ma.Name, "address": ma.Addr}); err == nil {
				ma.ID = res.InsertedID.(primitive.ObjectID)
			}
		}
	} else {
		log.Printf("[%s] Address was already listened:%+v\n", net, ma)
	}
	id, _ = hex.DecodeString(ma.ID.Hex())
	return
}

// RemoveAddress deletes an address from the database.
func (m *Mongo) RemoveAddress(a store.Address, net string) (err error) {
	var ma MongoAddress
	ma.Addr = a.Addr

	col := m.c.Database("addr").Collection(net)

	filter := bson.M{"address": a.Addr}
	var res *mgo.DeleteResult
	if res, err = col.DeleteOne(context.Background(), filter); err == nil && res.DeletedCount != 1 {
		err = store.ErrAddrNotFound
	}
	return
}

// GetAddresses returns the addresses or objects monitored for the network or blockchains indicated in the net slice.
func (m *Mongo) GetAddresses(net []string) (addrs []store.ListenedAddresses, err error) {
	var cols, docs *mgo.Cursor
	cols, err = m.c.Database("addr").ListCollections(context.Background(), bson.D{})
	if err == nil {
		for cols.Next(context.Background()) == true {
			col := cols.Current.Lookup("name").String()
			col = col[1 : len(col)-1]
			if len(net) == 0 || util.In(net, col) {
				// get the addresses
				docs, err = m.c.Database("addr").Collection(col).Find(nil, bson.M{})
				var addr store.ListenedAddresses
				if err == nil {
					addr.Net = col
					for docs.Next(context.Background()) == true {
						var a MongoAddress
						if err = bson.Unmarshal(docs.Current, &a); err == nil {
							addr.Addr = append(addr.Addr, a.Address())
						}
					}
				}
				addrs = append(addrs, addr)
			}
		}
	}
	return
}

// LoadExplorer loads from db the NetExplorer type for the indicated blockchain.
func (m *Mongo) LoadExplorer(net string) (ne store.NetExplorer, err error) {
	mongoSingleResult := m.c.Database("expl").Collection(net).FindOne(nil, bson.D{})
	if err = mongoSingleResult.Decode(&ne); err == mgo.ErrNoDocuments {
		err = store.ErrDataNotFound
	}
	return
}

// SaveExplorer saves to db the NetExplorer for the indicated blockchain.
func (m *Mongo) SaveExplorer(net string, ne store.NetExplorer) (err error) {
	_, err = m.c.Database("expl").Collection(net).UpdateOne(context.Background(), bson.D{}, bson.D{{"$set", bson.D{{"block", ne.Block}, {"bh", ne.Bh}, {"bhi", ne.Bhi}, {"map", ne.Map}}}}, options.Update().SetUpsert(true))
	return
}
