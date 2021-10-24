// Implements Write function
// We establish new connection everytime.

package main

import (
	grpcconnector "chat_room_go/microservices/redis/pb"
	"context"
	"time"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	dbURL string = "localhost:6379"
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

// Connection pool for redis
var pool *redis.Pool

func init() {
	pool = &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 60 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", dbURL) },
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
}

type RPCWriter struct{}

// grpc Write implementation
func (w RPCWriter) Write(ctx context.Context, i *grpcconnector.WriteRequest) (*grpcconnector.WriteResponse, error) {
	logger.Info(ctx, i)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &grpcconnector.WriteResponse{Status: 404, Desription: "Metadata was not found"}, status.Errorf(codes.NotFound, "Metadata was not found")
	}
	expirationTimes, ok := md["expirationtime"]
	if !ok || len(expirationTimes) != 1 {
		return &grpcconnector.WriteResponse{Status: 404, Desription: "expirationtime is not supplied"}, status.Errorf(codes.NotFound, "expirationtime is not supplied")
	}
	expirationTime := expirationTimes[0]

	err := writeToDB(expirationTime, i)
	if err != nil {
		logger.Errorf("Error during table insertion \"%s\"", err)
		return &grpcconnector.WriteResponse{Status: 500, Desription: "Error during table insertion"}, status.Errorf(codes.NotFound, "Error during table insertion: %s", err)
	}

	logger.Info("Response ok")
	return &grpcconnector.WriteResponse{Status: 0, Desription: "Ok"}, nil
}

// Writes message to redis
func writeToDB(expirationTime string, i *grpcconnector.WriteRequest) error {
	conn := pool.Get()
	defer conn.Close()
	_, err := conn.Do("HSET", redis.Args{}.Add(i.Login).AddFlat(i)...)
	if err != nil {
		return err
	}

	return nil
}
