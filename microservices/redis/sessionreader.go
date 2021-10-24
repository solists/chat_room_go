// implementation of grpc read, reads data from redis
// We establish new connection everytime.

package main

import (
	grpcconnector "chat_room_go/microservices/redis/pb"
	"context"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// grpc Read implementation
func (w RPCReader) GetSession(ctx context.Context, i *grpcconnector.GetSessionRequest) (*grpcconnector.GetSessionResponse, error) {
	logger.Info(ctx, i)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &grpcconnector.GetSessionResponse{Status: 404, Desription: "Metadata was not found"}, status.Errorf(codes.NotFound, "Metadata was not found")
	}
	dbNames, ok := md["dbname"]
	if !ok || len(dbNames) != 1 {
		return &grpcconnector.GetSessionResponse{Status: 404, Desription: "dbName is not supplied"}, status.Errorf(codes.NotFound, "dbName is not supplied")
	}
	dbName := dbNames[0]
	collectionNames, ok := md["collectionname"]
	if !ok || len(collectionNames) != 1 {
		return &grpcconnector.GetSessionResponse{Status: 404, Desription: "collection name is not supplied"}, status.Errorf(codes.NotFound, "collection name is not supplied")
	}
	collectionName := collectionNames[0]
	toReturn, err := readSessionFromDB(dbName, collectionName, i.SessionId)
	if err != nil {
		logger.Errorf("Error during table reading \"%s\"", err)
		return &grpcconnector.GetSessionResponse{Status: 500, Desription: "Error during table reading"}, status.Errorf(codes.NotFound, "Error during table reading: %s", err)
	}

	logger.Info(toReturn)
	return &grpcconnector.GetSessionResponse{UserName: toReturn, Status: 0, Desription: "Ok"}, nil
}

// Reads user info from db, if user not found - empty struct
func readSessionFromDB(dbName, collectionName, sessionId string) (string, error) {
	conn := pool.Get()
	defer conn.Close()

	userName, err := redis.String(conn.Do("GET", sessionId))
	if err != nil {
		return "", err
	}

	return userName, nil
}
