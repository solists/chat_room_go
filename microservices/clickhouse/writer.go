package main

import (
	"context"
	"fmt"

	grpcconnector "chat_room_go/microservices/clickhouse/pb"
)

type RPCWriter struct{}

func (w RPCWriter) Write(ctx context.Context, i *grpcconnector.WriteRequest) (*grpcconnector.WriteResponse, error) {
	fmt.Println("Request: ", i.Log)

	return &grpcconnector.WriteResponse{Status: 0, Desription: "Ok"}, nil
}
