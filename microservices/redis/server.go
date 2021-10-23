// Main service function, listens for requests

package main

import (
	"chat_room_go/utils/logs"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"

	mmw "chat_room_go/microservices"

	grpcconnector "chat_room_go/microservices/redis/pb"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var logger *zap.SugaredLogger
var wl logs.WriterToClickHouse

func init() {
	logger = logs.InitDirLogger("./logs/redislogs.json")
}

// Listen and serve grpc
func main() {
	defer logger.Sync()
	defer wl.GrpcConn.Close()

	creds, err := loadTLSCredentials()
	if err != nil {
		logger.Fatal("cannot load TLS credentials: ", err)
	}
	server := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(mmw.LogInterceptor, mmw.AuthInterceptor)),
		grpc.InTapHandle(mmw.RateLimiter),
	)
	grpcconnector.RegisterWriterServer(server, RPCWriter{})
	grpcconnector.RegisterReaderServer(server, RPCReader{})

	lis, err := net.Listen("tcp", ":8083")
	if err != nil {
		log.Fatalln("cant listen port", err)
	}
	defer lis.Close()

	fmt.Println("starting server at :8083")
	logger.Debugf("Recieved request")
	server.Serve(lis)
}

// Establish TLS
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed client's certificate
	pemClientCA, err := ioutil.ReadFile("certs/ca-cert.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, fmt.Errorf("failed to add client CA's certificate")
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair("certs/server-cert.pem", "certs/server-key.pem")
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(config), nil
}
