package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Configuration struct {
	DbURL string `json:"dbURL"`
}

// Initialises configuration. File IO operations
func InitConf() *Configuration {
	file, _ := os.Open("conf.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		log.Panicln("error:", err)
	}
	fmt.Println(configuration.DbURL)

	return &configuration
}
