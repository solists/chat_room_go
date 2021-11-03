// Includes all logic for establishing connection to microservices via grpc
package main

import (
	"chat_room_go/main/models"
	mongoconnector "chat_room_go/microservices/mongodb/pb"
	redisconnector "chat_room_go/microservices/redis/pb"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Adapters for grpc
var MongoAdapter grpcMongoAdapter
var RedisAdapter grpcRedisAdapter

// Init grpc to mongo
func init() {
	MongoAdapter = grpcMongoAdapter{}
	MongoAdapter.dbParms = dbParms{DbName: "test", CollectionName: "messages"}
	MongoAdapter.url = ":8082"
	MongoAdapter.InitMongoAdapter()
}

// Init grpc to redis
func init() {
	RedisAdapter = grpcRedisAdapter{}
	RedisAdapter.url = ":8083"
	RedisAdapter.recParms = recParms{ExpirationTime: strconv.Itoa(sessionLength)}
	RedisAdapter.initRedisAdapter()
}

// Db to write parameters
type dbParms struct {
	DbName         string
	CollectionName string
}

// Db to write parameters
type recParms struct {
	ExpirationTime string
}

// Struct, that implements grpc methods for mongodb microservice
type grpcMongoAdapter struct {
	writerClient mongoconnector.WriterClient
	readerClient mongoconnector.ReaderClient
	ctx          context.Context
	grpcConn     *grpc.ClientConn
	dbParms      dbParms
	url          string
}

// Struct, that implements grpc methods for redis microservice
type grpcRedisAdapter struct {
	writerClient        redisconnector.WriterClient
	readerClient        redisconnector.ReaderClient
	writerSessionClient redisconnector.WriterSessionClient
	getterSessionClient redisconnector.GetterSessionClient
	ctx                 context.Context
	grpcConn            *grpc.ClientConn
	recParms            recParms
	url                 string
}

// Writes user to redis
func (w *grpcRedisAdapter) Write(u models.User) (int, error) {
	_, err := w.writerClient.Write(
		w.ctx,
		&redisconnector.WriteRequest{Login: u.Login, Fname: u.Fname, Lname: u.Lname, Pass: string(u.Pass), Role: u.Role, LastActive: time.Now().Format("2006-01-02 15:04:05")},
	)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// Returns user from redis
func (w *grpcRedisAdapter) Read(login string) (*models.User, error) {
	toReturn, err := w.readerClient.Read(
		w.ctx,
		&redisconnector.ReadRequest{Login: login},
	)
	if err != nil {
		return nil, err
	}
	if toReturn.Result == nil {
		return nil, nil
	}
	r := toReturn.Result
	return &models.User{
		Login: r.Login,
		Fname: r.Fname,
		Lname: r.Lname,
		Pass:  []byte(r.Pass),
		Role:  r.Role}, nil
}

// Writes session to redis
func (w *grpcRedisAdapter) AddSession(sessionId, userName string) (int, error) {
	_, err := w.writerSessionClient.AddSession(
		w.ctx,
		&redisconnector.AddSessionRequest{SessionId: sessionId, UserName: userName},
	)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// Returns session from redis
func (w *grpcRedisAdapter) GetSession(sessionId string) (string, error) {
	toReturn, err := w.getterSessionClient.GetSession(
		w.ctx,
		&redisconnector.GetSessionRequest{SessionId: sessionId},
	)
	if err != nil {
		return "", err
	}

	return toReturn.UserName, nil
}

// Initializes TLS, grpc mappings, context for redis
func (w *grpcRedisAdapter) initRedisAdapter() {
	creds, err := loadTLSCredentialsRedis()
	if err != nil {
		log.Panicln(err)
	}

	w.grpcConn, err = grpc.Dial(
		w.url,
		grpc.WithPerRPCCredentials(&tokenAuth{"sometoken"}),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("cant connect to grpc")
	}

	w.writerClient = redisconnector.NewWriterClient(w.grpcConn)
	w.readerClient = redisconnector.NewReaderClient(w.grpcConn)
	w.getterSessionClient = redisconnector.NewGetterSessionClient(w.grpcConn)
	w.writerSessionClient = redisconnector.NewWriterSessionClient(w.grpcConn)

	w.ctx = context.Background()
	md := metadata.Pairs(
		"api-req-id", "123qwe",
		"expirationtime", w.recParms.ExpirationTime,
	)
	sHeader := metadata.Pairs("authorization", "val")
	grpc.SendHeader(w.ctx, sHeader)
	w.ctx = metadata.NewOutgoingContext(w.ctx, md)
}

// Writes message to mongodb storage
func (w *grpcMongoAdapter) Write(message, name, time string) (int, error) {
	_, err := w.writerClient.Write(
		w.ctx,
		&mongoconnector.WriteRequest{Message: message, Name: name, Time: time},
	)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// Returns messages from mongodb storage
func (w *grpcMongoAdapter) Read() ([]*mongoconnector.MessageInfo, error) {
	toReturn, err := w.readerClient.Read(
		w.ctx,
		&mongoconnector.ReadRequest{Time: time.Now().Format("2006-01-02 15:04:05"), Number: 100},
	)
	if err != nil {
		return nil, err
	}
	return toReturn.Results, nil
}

// Initializes TLS, grpc mappings, context for mongodb, might be different from redis
func (w *grpcMongoAdapter) InitMongoAdapter() {
	creds, err := loadTLSCredentialsMongo()
	if err != nil {
		log.Panicln(err)
	}

	w.grpcConn, err = grpc.Dial(
		w.url,
		grpc.WithPerRPCCredentials(&tokenAuth{"sometoken"}),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("cant connect to grpc")
	}

	w.writerClient = mongoconnector.NewWriterClient(w.grpcConn)
	w.readerClient = mongoconnector.NewReaderClient(w.grpcConn)

	w.ctx = context.Background()
	md := metadata.Pairs(
		"api-req-id", "123qwe",
		"dbname", w.dbParms.DbName,
		"collectionname", w.dbParms.CollectionName,
	)
	sHeader := metadata.Pairs("authorization", "val")
	grpc.SendHeader(w.ctx, sHeader)
	w.ctx = metadata.NewOutgoingContext(w.ctx, md)
}

// **********************************************
// Below security logic, establishing tls, tokens
// **********************************************
type tokenAuth struct {
	Token string
}

// Realization of PerRPCCredentials interface
func (t *tokenAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": t.Token,
	}, nil
}

// Realization of WithPerRPCCredentials interface
func (c *tokenAuth) RequireTransportSecurity() bool {
	return false
}

// Enables TLS and adds certificates for the mongo client
func loadTLSCredentialsMongo() (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("../microservices/mongodb/certs/ca-cert.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Load client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair("../microservices/mongodb/certs/client-cert.pem", "../microservices/mongodb/certs/client-key.pem")
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

// Enables TLS and adds certificates for the redis client
func loadTLSCredentialsRedis() (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("../microservices/redis/certs/ca-cert.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Load client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair("../microservices/redis/certs/client-cert.pem", "../microservices/redis/certs/client-key.pem")
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
