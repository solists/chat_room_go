// Implements Write function
// Everytime we establish new connection to database. TODO: remove this feature

package main

import (
	grpcconnector "chat_room_go/microservices/mongodb/pb"
	config "chat_room_go/utils/conf"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var dbURL string = config.Config.MongoAdapter.DbURL

//cacheSize int    = 20

// TODO: Add chache
//type cache struct {
//	v []item
//	m *sync.RWMutex
//}
//
//var cache Cache
//
//func init() {
//	cache = Cache{make([]item, 0, cacheSize), &sync.RWMutex{}}
//}

type RPCWriter struct{}

// grpc Write implementation
func (w RPCWriter) Write(ctx context.Context, i *grpcconnector.WriteRequest) (*grpcconnector.WriteResponse, error) {
	logger.Info(ctx, i)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &grpcconnector.WriteResponse{Status: 404, Desription: "Metadata was not found"}, status.Errorf(codes.NotFound, "Metadata was not found")
	}
	dbNames, ok := md["dbname"]
	if !ok || len(dbNames) != 1 {
		return &grpcconnector.WriteResponse{Status: 404, Desription: "dbName is not supplied"}, status.Errorf(codes.NotFound, "dbName is not supplied")
	}
	dbName := dbNames[0]
	collectionNames, ok := md["collectionname"]
	if !ok || len(collectionNames) != 1 {
		return &grpcconnector.WriteResponse{Status: 404, Desription: "collection name is not supplied"}, status.Errorf(codes.NotFound, "collection name is not supplied")
	}
	collectionName := collectionNames[0]

	err := writeToDB(dbName, collectionName, i)
	if err != nil {
		logger.Errorf("Error during table insertion \"%s\"", err)
		return &grpcconnector.WriteResponse{Status: 500, Desription: "Error during table insertion"}, status.Errorf(codes.NotFound, "Error during table insertion: %s", err)
	}

	logger.Info("Response ok")
	return &grpcconnector.WriteResponse{Status: 0, Desription: "Ok"}, nil
}

// Writes message to mongo
func writeToDB(dbName, collectionName string, i *grpcconnector.WriteRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Creates connection
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(dbURL))
	if err != nil {
		return err
	}
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Recovered in writeToDB", r)
		}
	}()

	// Checking connection
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return err
	}

	// Retrieve collection and write to it
	collection := client.Database(dbName).Collection(collectionName)
	res, err := collection.InsertOne(ctx, bson.D{{"time", i.Time}, {"name", i.Name}, {"message", i.Message}})
	if err != nil {
		return err
	}
	id := res.InsertedID
	logger.Info(id)

	return nil
}
