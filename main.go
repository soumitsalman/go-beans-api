package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

const (
	DEFAULT_DB_PATH                = ""
	DEFAULT_VECTOR_DIM             = 384
	DEFAULT_RELATED_EPS            = 0.43
	DEFAULT_PORT                   = "8080"
	DEFAULT_MAX_CONCURRENT_QUERIES = 2
	DEFAULT_REFRESH_TIME           = 5 // in minutes
	DB_NAME                        = "beansack.db"
)

func main() {
	// set logging stuff
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	// Load configuration from environment variables
	// Read the configuration parameters
	catalog_path, ok := os.LookupEnv("CATALOG_PATH")
	if !ok {
		catalog_path = DEFAULT_DB_PATH
	}
	storage_path, ok := os.LookupEnv("STORAGE_PATH")
	if !ok {
		storage_path = DEFAULT_DB_PATH
	}
	ds := NewReadonlyBeansack(catalog_path, storage_path)
	defer ds.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = DEFAULT_PORT
	}

	noerror(engine.Run("0.0.0.0:"+port), "SERVER ERROR")
}
