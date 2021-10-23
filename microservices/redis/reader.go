// implementation of grpc read, reads data from redis
// We establish new connection everytime.

package main

import (
	grpcconnector "chat_room_go/microservices/redis/pb"
	"context"
	"fmt"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type RPCReader struct{}

func (w RPCReader) Read(ctx context.Context, i *grpcconnector.ReadRequest) (*grpcconnector.ReadResponse, error) {
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
	toReturn, err := readFromDB(dbName, collectionName, i.Login)
	if err != nil {
		logger.Errorf("Error during table reading \"%s\"", err)
		return &grpcconnector.ReadResponse{Status: 500, Desription: "Error during table insertion"}, status.Errorf(codes.NotFound, "Error during table insertion: %s", err)
	}

	fmt.Println("Request: ", i.Login)

	return &grpcconnector.ReadResponse{Result: toReturn, Status: 0, Desription: "Ok"}, nil
}

// TODO: return only numberToRecieve values, also sorted
func readFromDB(dbName, collectionName, login string) (*grpcconnector.UserInfo, error) {
	conn := pool.Get()
	defer conn.Close()

	values, err := redis.Values(conn.Do("HGETALL", login))
	if err != nil {
		return nil, err
	}

	toReturn := grpcconnector.UserInfo{}
	redis.ScanStruct(values, &toReturn)
	if toReturn.Login == "" {
		return nil, nil
	}

	return &toReturn, nil
}
