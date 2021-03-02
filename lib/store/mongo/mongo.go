// Package mongo implements the interface for MongoDB.
package mongo

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mgo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/tarancss/adp/lib/store"
	"github.com/tarancss/adp/lib/util"
)

// Mongo implements a connection to a MongoDB database.
type Mongo struct {
	c *mgo.Client
}

// MongoAddress implements a store address to MongoDB.
type MongoAddress struct {
	ID   primitive.ObjectID `json:"_id" bson:"_id"`
	Name string             `json:"name,omitempty" bson:"name,omitempty"`
	Addr string             `json:"address" bson:"address"`
}

// Address converts a MongoAddress to store.Address type.
func (a MongoAddress) Address() store.Address {
	return store.Address{ID: a.ID[:], Addr: a.Addr, Name: a.Name}
}

// New returns a Mongo client connection to the specified MongoDB database uri.
func New(uri string) (*Mongo, error) {
	// get a client
	c, err := mgo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to mongo DB in %s: %w", uri, err)
	}
	// connect client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:gomnd // 5 seconds timeout
	defer cancel()

	err = c.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("error connecting to mongo DB: %w", err)
	}

	return &Mongo{c: c}, nil
}

// CloseMongo will close a database connection. Must be called at termination time.
func (m *Mongo) CloseMongo() error {
	return m.c.Disconnect(context.Background())
}

// AddAddress saves an address if the address does not already exist.
func (m *Mongo) AddAddress(a store.Address, net string) ([]byte, error) {
	var ma MongoAddress
	ma.Addr = a.Addr

	col := m.c.Database("addr").Collection(net)

	// try and find it
	filter := bson.M{"address": a.Addr}
	sr := col.FindOne(context.Background(), filter)

	err := sr.Decode(&ma)
	if errors.Is(err, mgo.ErrNoDocuments) { // if not found, do insert it!!
		res, errIns := col.InsertOne(context.Background(), bson.M{"name": ma.Name, "address": ma.Addr})
		if errIns != nil {
			return nil, fmt.Errorf("could not insert address in db: %w", errIns)
		}

		return hex.DecodeString(res.InsertedID.(primitive.ObjectID).Hex())
	}

	if err != nil {
		return nil, fmt.Errorf("could not insert address in db: %w", err)
	}

	log.Printf("[%s] Address was already listened:%+v\n", net, ma)

	return hex.DecodeString(ma.ID.Hex())
}

// RemoveAddress deletes an address from the database.
func (m *Mongo) RemoveAddress(a store.Address, net string) error {
	var ma MongoAddress
	ma.Addr = a.Addr

	col := m.c.Database("addr").Collection(net)

	filter := bson.M{"address": a.Addr}

	res, err := col.DeleteOne(context.Background(), filter)
	if err == nil && res.DeletedCount != 1 {
		err = store.ErrAddrNotFound
	}

	return err
}

// GetAddresses returns the addresses or objects monitored for the network or blockchains indicated in the net slice.
func (m *Mongo) GetAddresses(net []string) ([]store.ListenedAddresses, error) {
	cols, err := m.c.Database("addr").ListCollections(context.Background(), bson.D{})
	if err != nil {
		return nil, fmt.Errorf("error getting mongo DB object: %w", err)
	}

	addrs := []store.ListenedAddresses{}

	for cols.Next(context.Background()) {
		col := cols.Current.Lookup("name").String()
		col = col[1 : len(col)-1]

		if len(net) == 0 || util.In(net, col) {
			var addr store.ListenedAddresses
			// get the addresses
			docs, err := m.c.Database("addr").Collection(col).Find(context.TODO(), bson.M{})
			if err == nil {
				addr.Net = col

				for docs.Next(context.Background()) {
					var a MongoAddress
					if err = bson.Unmarshal(docs.Current, &a); err == nil {
						addr.Addr = append(addr.Addr, a.Address())
					}
				}
			}

			addrs = append(addrs, addr)
		}
	}

	return addrs, nil
}

// LoadExplorer loads from db the NetExplorer type for the indicated blockchain.
func (m *Mongo) LoadExplorer(net string) (ne store.NetExplorer, err error) {
	mongoSingleResult := m.c.Database("expl").Collection(net).FindOne(context.TODO(), bson.D{})
	if err = mongoSingleResult.Decode(&ne); errors.Is(err, mgo.ErrNoDocuments) {
		err = store.ErrDataNotFound
	}

	return
}

// SaveExplorer saves to db the NetExplorer for the indicated blockchain.
func (m *Mongo) SaveExplorer(net string, ne store.NetExplorer) (err error) {
	_, err = m.c.Database("expl").Collection(net).UpdateOne(context.Background(),
		bson.D{}, // filter
		bson.D{ // update
			{
				Key: "$set", Value: bson.D{
					{Key: "block", Value: ne.Block},
					{Key: "bh", Value: ne.Bh},
					{Key: "bhi", Value: ne.Bhi},
					{Key: "map", Value: ne.Map},
				},
			},
		},
		options.Update().SetUpsert(true))

	return
}

// DeleteExplorer deletes from db the NetExplorer for the indicated blockchain.
func (m *Mongo) DeleteExplorer(net string) (err error) {
	_, err = m.c.Database("expl").Collection(net).DeleteOne(context.Background(), bson.D{}, options.Delete())

	return
}
