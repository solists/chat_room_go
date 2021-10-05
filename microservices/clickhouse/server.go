package main

import (
	grpcconnector "chat_room_go/microservices/clickhouse/pb"
	"fmt"
	"log"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatalln("cant listen port", err)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(logInterceptor, authInterceptor)),
		grpc.InTapHandle(rateLimiter),
	)

	grpcconnector.RegisterWriterServer(server, RPCWriter{})

	fmt.Println("starting server at :8081")
	logger.Debugf("Recieved request",
		"method", "hello",
	)
	//logger.Sync()
	server.Serve(lis)
}
