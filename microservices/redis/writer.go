// Implements Write function
// We establish new connection everytime.

package main

import (
	grpcconnector "chat_room_go/microservices/redis/pb"
	"context"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	dbURL     string = "redis://user:@localhost:6379/0"
	cacheSize int    = 20
)

// Stores values which will be written to database
type item struct {
	messageTime time.Time
	name        string
	message     string
	actionTime  time.Time
	dbName      string
	tableName   string
}

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
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", "localhost:6379") },
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

func (w RPCWriter) Write(ctx context.Context, i *grpcconnector.WriteRequest) (*grpcconnector.WriteResponse, error) {
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

	fmt.Println("Request: ", i.Login)

	return &grpcconnector.WriteResponse{Status: 0, Desription: "Ok"}, nil
}

// Writes message to mongo
func writeToDB(dbName, collectionName string, i *grpcconnector.WriteRequest) error {
	conn := pool.Get()
	defer conn.Close()
	_, err := conn.Do("HSET", redis.Args{}.Add(i.Login).AddFlat(i)...)
	if err != nil {
		return err
	}

	return nil
}
