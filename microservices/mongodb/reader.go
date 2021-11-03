// implementation of grpc read, reads data from mongodb
// Everytime we establish new connection to database. TODO: remove this feature

package main

import (
	"context"
	"log"
	grpcconnector "chat_room_go/microservices/mongodb/pb"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type RPCReader struct{}

// grpc Read implementation
func (w RPCReader) Read(ctx context.Context, i *grpcconnector.ReadRequest) (*grpcconnector.ReadResponse, error) {
	logger.Info(ctx, i)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &grpcconnector.ReadResponse{Status: 404, Desription: "Metadata was not found"}, status.Errorf(codes.NotFound, "Metadata was not found")
	}
	dbNames, ok := md["dbname"]
	if !ok || len(dbNames) != 1 {
		return &grpcconnector.ReadResponse{Status: 404, Desription: "dbName is not supplied"}, status.Errorf(codes.NotFound, "dbName is not supplied")
	}
	dbName := dbNames[0]
	collectionNames, ok := md["collectionname"]
	if !ok || len(collectionNames) != 1 {
		return &grpcconnector.ReadResponse{Status: 404, Desription: "collection name is not supplied"}, status.Errorf(codes.NotFound, "collection name is not supplied")
	}
	collectionName := collectionNames[0]
	toReturn, err := readFromDB(dbName, collectionName, i.Number)
	if err != nil {
		logger.Errorf("Error during table reading \"%s\"", err)
		return &grpcconnector.ReadResponse{Status: 500, Desription: "Error during table insertion"}, status.Errorf(codes.NotFound, "Error during table insertion: %s", err)
	}

	logger.Info(toReturn)
	return &grpcconnector.ReadResponse{Results: toReturn, Status: 0, Desription: "Ok"}, nil
}

// TODO: return only numberToRecieve values, also sorted
func readFromDB(dbName, collectionName string, numberToRecieve int32) ([]*grpcconnector.MessageInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(dbURL))
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
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = client.Ping(ctx, readpref.Primary())

	collection := client.Database(dbName).Collection(collectionName)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := collection.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	toReturn := make([]*grpcconnector.MessageInfo, 0, numberToRecieve)
	for cur.Next(ctx) {
		var result grpcconnector.MessageInfo
		err := cur.Decode(&result)
		if err != nil {
			return nil, err
		}
		toReturn = append(toReturn, &result)
	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	return toReturn, nil
}
