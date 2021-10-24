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

type RPCReader struct{}

// grpc Read implementation
func (w RPCReader) Read(ctx context.Context, i *grpcconnector.ReadRequest) (*grpcconnector.ReadResponse, error) {
	logger.Info(ctx, i)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &grpcconnector.ReadResponse{Status: 404, Desription: "Metadata was not found"}, status.Errorf(codes.NotFound, "Metadata was not found")
	}
	expirationTimes, ok := md["expirationtime"]
	if !ok || len(expirationTimes) != 1 {
		return &grpcconnector.ReadResponse{Status: 404, Desription: "expirationtime is not supplied"}, status.Errorf(codes.NotFound, "expirationtime is not supplied")
	}
	expirationTime := expirationTimes[0]

	toReturn, err := readFromDB(expirationTime, i.Login)
	if err != nil {
		logger.Errorf("Error during table reading \"%s\"", err)
		return &grpcconnector.ReadResponse{Status: 500, Desription: "Error during table reading"}, status.Errorf(codes.NotFound, "Error during table reading: %s", err)
	}

	logger.Info(toReturn)
	return &grpcconnector.ReadResponse{Result: toReturn, Status: 0, Desription: "Ok"}, nil
}

// Reads user info from db, if user not found - empty struct
func readFromDB(expirationTime, login string) (*grpcconnector.UserInfo, error) {
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
