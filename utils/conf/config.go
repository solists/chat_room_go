// Includes config initialization and parsing from json

package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var Config *Configuration

func init() {
	Config = InitConf()
}

type Configuration struct {
	ChatServeURL          string `json:"chatServeURL"`
	SessionExpirationTime int    `json:"sessionExpirationTime"`
	NumChatMessages       int    `json:"numChatMessages"`
	MongoAdapter          struct {
		URL            string `json:"url"`
		IntURL         string `json:"intURL"`
		DbURL          string `json:"dbURL"`
		DbName         string `json:"dbName"`
		CollectionName string `json:"collectionName"`
		TokenAuth      string `json:"tokenAuth"`
		PathToLogs     string `json:"pathToLogs"`
	} `json:"mongoAdapter"`
	RedisAdapter struct {
		URL        string `json:"url"`
		IntURL     string `json:"intURL"`
		TokenAuth  string `json:"tokenAuth"`
		DbURL      string `json:"dbURL"`
		PathToLogs string `json:"pathToLogs"`
	} `json:"redisAdapter"`
	ClickhouseAdapter struct {
		URL        string `json:"url"`
		IntURL     string `json:"intURL"`
		DbName     string `json:"dbName"`
		TableName  string `json:"tableName"`
		TokenAuth  string `json:"tokenAuth"`
		DbURL      string `json:"dbURL"`
		PathToLogs string `json:"pathToLogs"`
	} `json:"clickhouseAdapter"`
	MicroserviceMiddleware struct {
		PathToLogs string `json:"pathToLogs"`
	} `json:"microserviceMiddleware"`
}

// Initialises configuration. File IO operations
func InitConf() *Configuration {
	currentPath, err := filepath.Abs(".")
	if err != nil {
		log.Panic("Error during config processing: ", err)
	}
	pathToConf := filepath.Join(strings.SplitAfterN(currentPath, "chat_room_go", 2)[0], "utils/conf/config.json")
	file, _ := os.Open(pathToConf)
	defer file.Close()
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		log.Panic("Error during config processing: ", err)
	}
	return &configuration
}
