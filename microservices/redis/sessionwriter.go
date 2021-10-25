// Implements Write function
// We establish new connection everytime.

package main

import (
	grpcconnector "chat_room_go/microservices/redis/pb"
	"context"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
//cacheSize int    = 20
)

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

// grpc Write implementation
func (w RPCWriter) AddSession(ctx context.Context, i *grpcconnector.AddSessionRequest) (*grpcconnector.AddSessionResponse, error) {
	logger.Info(ctx, i)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &grpcconnector.AddSessionResponse{Status: 404, Desription: "Metadata was not found"}, status.Errorf(codes.NotFound, "Metadata was not found")
	}
	expirationTimes, ok := md["expirationtime"]
	if !ok || len(expirationTimes) != 1 {
		return &grpcconnector.AddSessionResponse{Status: 404, Desription: "expirationtime is not supplied"}, status.Errorf(codes.NotFound, "expirationtime is not supplied")
	}
	expirationTime := expirationTimes[0]

	err := writeSessionToDB(expirationTime, i)
	if err != nil {
		logger.Errorf("Error during table insertion \"%s\"", err)
		return &grpcconnector.AddSessionResponse{Status: 500, Desription: "Error during table insertion"}, status.Errorf(codes.NotFound, "Error during table insertion: %s", err)
	}

	logger.Info("Response ok")
	return &grpcconnector.AddSessionResponse{Status: 0, Desription: "Ok"}, nil
}

// Writes message to redis
func writeSessionToDB(expirationTime string, i *grpcconnector.AddSessionRequest) error {
	expTime, err := strconv.Atoi(expirationTime)
	if err != nil {
		return err
	}
	conn := pool.Get()
	defer conn.Close()
	_, err = conn.Do("SET", i.SessionId, i.UserName, "EX", expTime)
	if err != nil {
		return err
	}

	return nil
}
