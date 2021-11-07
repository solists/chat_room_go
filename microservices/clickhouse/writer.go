// Implements Write function
// Write is used by zap logger, as io.Writer
// Every log goes by grpc to this microservice and comes to clickhouse
// Every cache write we establish new connection to database. TODO: remove this feature

package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	grpcconnector "chat_room_go/microservices/clickhouse/pb"
	config "chat_room_go/utils/conf"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var dbURL string = config.Config.ClickhouseAdapter.DbName

const cacheSize int = 20

// Stores values which will be written to database
type item struct {
	Log        string    `db:"log"`
	ActionTime time.Time `db:"action_time"`
	dbName     string
	tableName  string
}

// TODO: must not be global inside main package
type Cache struct {
	v []item
	m *sync.RWMutex
}

var cache Cache

func init() {
	cache = Cache{make([]item, 0, cacheSize), &sync.RWMutex{}}
}

type RPCWriter struct{}

// grpc Write implementation
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
	tableNames, ok := md["tablename"]
	if !ok || len(tableNames) != 1 {
		return &grpcconnector.WriteResponse{Status: 404, Desription: "tableName is not supplied"}, status.Errorf(codes.NotFound, "tableName is not supplied")
	}
	tableName := tableNames[0]
	err := writeToDB(dbName, tableName, i.Log)
	if err != nil {
		logger.Errorf("Error during table insertion \"%s\"", err)
		return &grpcconnector.WriteResponse{Status: 500, Desription: "Error during table insertion"}, status.Errorf(codes.NotFound, "Error during table insertion: %s", err)
	}

	fmt.Println("Request: ", i.Log)

	return &grpcconnector.WriteResponse{Status: 0, Desription: "Ok"}, nil
}

// Creates database if not already exists
func createDB(dbName string, connect *sqlx.DB) error {
	_, err := connect.Exec("CREATE DATABASE IF NOT EXISTS " + dbName)
	if err != nil {
		return status.Errorf(codes.Internal, "Error during database creation \"%s\"", dbName)
	}
	logger.Infof("Success! Database \"%s\" created or already exists", dbName)
	return nil
}

// Creates database if not already exists
func createTable(dbName string, tableName string, connect *sqlx.DB) error {
	_, err := connect.Exec(`
	CREATE TABLE IF NOT EXISTS ` + dbName + `.` + tableName + ` (
		log          String,
		action_time  DateTime
	) engine=Memory
    `)
	if err != nil {
		return status.Errorf(codes.Internal, "Error during table creation \"%s.%s\"", dbName, tableName)
	}
	logger.Infof("Success! Table \"%s.%s\" created or already exists", dbName, tableName)
	return nil
}

// Writes to db
func writeToDB(dbName string, tableName string, log string) error {
	// If it is not full, then we do not write to DB
	cache.m.RLock()
	if len(cache.v) < cacheSize {
		cache.v = append(cache.v, item{ActionTime: time.Now(), Log: log, dbName: dbName, tableName: tableName})
		cache.m.RUnlock()
	} else if len(cache.v) == cacheSize {
		err := WriteCache()
		if err != nil {
			return err
		}
		// Unlock from read, then lock to write
		cache.m.RUnlock()
		cache.m.Lock()
		cache.v = make([]item, 0, cacheSize)
		cache.m.Unlock()
	} else {
		cache.m.RUnlock()
		return status.Errorf(codes.Internal, "Cache overflowed")
	}

	return nil
}

// Writes cache to database, should defer in main
func WriteCache() error {
	cache.m.RLock()
	defer cache.m.RUnlock()

	if len(cache.v) <= 0 {
		return status.Errorf(codes.Internal, "Cache error")
	}

	// TODO: work with multiple databases
	dbName := cache.v[0].dbName
	tableName := cache.v[0].tableName

	conn, err := prepareDB()
	if err != nil {
		return status.Errorf(codes.NotFound, "Error during initialising db connection: %s", err)
	}
	err = createDB(dbName, conn)
	if err != nil {
		return status.Errorf(codes.Internal, "Error during db creation: %s", err)
	}
	err = createTable(dbName, tableName, conn)
	if err != nil {
		return status.Errorf(codes.Internal, "Error during table creation: %s", err)
	}
	var (
		tx, _        = conn.Begin()
		statement, _ = tx.Prepare("INSERT INTO " + dbName + "." + tableName + " (log, action_time) VALUES (?, ?)")
	)
	defer statement.Close()

	for _, v := range cache.v {
		if _, err := statement.Exec(
			v.Log,
			v.ActionTime,
		); err != nil {
			return status.Errorf(codes.Internal, "Error during database insertion \"%s.%s\": %s", dbName, tableName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return status.Errorf(codes.Internal, "Error during database insertion \"%s.%s\": %s", dbName, tableName, err)
	}

	return nil
}

// Checks connection to db url and returns connection to it
func prepareDB() (*sqlx.DB, error) {
	wg := &sync.WaitGroup{}

	rc := make(chan *sqlx.DB)
	ec := make(chan error, 10)

	func() {
		wg.Add(1)
		go getConnectionToDB(rc, ec, wg)
	}()

	// CAUTION! CHECK LOCK
	for {
		select {
		case res := <-rc:
			return res, nil
		case err := <-ec:
			return nil, err
		}
	}
}

// Returns connection to database through channel
func getConnectionToDB(rc chan<- *sqlx.DB, ec chan<- error, wg *sync.WaitGroup) {
	connect, err := sqlx.Open("clickhouse", dbURL)
	if err != nil {
		wg.Done()
		ec <- status.Errorf(codes.NotFound, "Error during connecting to database : Error = %s", err)
	}

	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			logger.Infof("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			logger.Errorf("Unknown error during connecting to database : Error = %s", err)
		}
		wg.Done()
		ec <- status.Errorf(codes.NotFound, "Error during connecting to database : Error = %s", err)
	}
	wg.Done()
	rc <- connect
}
