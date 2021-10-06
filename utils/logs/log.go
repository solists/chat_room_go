// Includes zap logging logic:
// config, methods, initializers
// Here implemented io.Writer interface for clickhouse logging microservice
package logs

import (
	grpcconnector "chat_room_go/microservices/clickhouse/pb"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

func InitDirLogger(dirName string) *zap.SugaredLogger {
	writeSyncer := getLogWriter(dirName)
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)

	logger := zap.New(core)

	return logger.Sugar()
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(dirName string) zapcore.WriteSyncer {
	file, err := os.OpenFile(dirName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		log.Panicln(err)
	}
	return zapcore.AddSync(file)
}

// Returns zap logger
func (w *WriterToClickHouse) GetCLickHouseLogger() *zap.SugaredLogger {
	writeSyncer := w.getLogWriterToCLickHouse()
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)

	logger := zap.New(core)

	return logger.Sugar()
}

// Adds zap sync
func (w *WriterToClickHouse) getLogWriterToCLickHouse() zapcore.WriteSyncer {
	w.InitClickHouseLogger()
	return zapcore.AddSync(w)
}

// Struct, that implements io.Writer, keeps all data for grpc request TODO: GrpcConn closel ogic move inside
type WriterToClickHouse struct {
	writerClient grpcconnector.WriterClient
	ctx          context.Context
	GrpcConn     *grpc.ClientConn
	DbParms      ClickHouseDBParms
}

type ClickHouseDBParms struct {
	DbName    string
	TableName string
}

// Initializes TLS, grpc mappings, context for logwriter
func (w *WriterToClickHouse) InitClickHouseLogger() {
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Panicln(err)
	}

	w.GrpcConn, err = grpc.Dial(
		"127.0.0.1:8081",
		grpc.WithPerRPCCredentials(&tokenAuth{"sometoken"}),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("cant connect to grpc")
	}

	w.writerClient = grpcconnector.NewWriterClient(w.GrpcConn)

	w.ctx = context.Background()
	md := metadata.Pairs(
		"api-req-id", "123qwe",
		"dbName", w.DbParms.DbName,
		"tableName", w.DbParms.TableName,
	)
	sHeader := metadata.Pairs("authorization", "val")
	grpc.SendHeader(w.ctx, sHeader)
	w.ctx = metadata.NewOutgoingContext(w.ctx, md)
}

type tokenAuth struct {
	Token string
}

func (t *tokenAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": t.Token,
	}, nil
}

func (c *tokenAuth) RequireTransportSecurity() bool {
	return false
}

// Enables TLS and adds certificates for the client
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("microservices/clickhouse/certs/ca-cert.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Load client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair("microservices/clickhouse/certs/client-cert.pem", "microservices/clickhouse/certs/client-key.pem")
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		// Self signed certificate, TODO: Let`s Encrypt
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{clientCert},
		RootCAs:            certPool,
	}

	return credentials.NewTLS(config), nil
}

// io.Writer interface implementation for zap logger sync
func (w *WriterToClickHouse) Write(p []byte) (int, error) {
	_, err := w.writerClient.Write(
		w.ctx,
		&grpcconnector.WriteRequest{Log: string(p)},
	)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
