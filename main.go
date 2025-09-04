package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

const (
	DEFAULT_DB_PATH     = ""
	DEFAULT_VECTOR_DIM  = 384
	DEFAULT_RELATED_EPS = 0.43
	DEFAULT_PORT        = "8080"
	DB_NAME             = "beansack.db"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	// Load configuration from environment variables
	// Read the configuration parameters
	godotenv.Load(".env")

	// if nothing is mentioned then treat this as in memory db
	// else if the directory is mentioned but directory does not exist then create one
	dbpath, ok := os.LookupEnv("DATA")
	if !ok {
		dbpath = DEFAULT_DB_PATH
	} else {
		if _, err := os.Stat(dbpath); os.IsNotExist(err) {
			noerror(os.MkdirAll(dbpath, 0755), "CREATE DB DIR ERROR")
		}
		dbpath = fmt.Sprintf("%s/%s", dbpath, DB_NAME)
	}

	// vector dimension
	dim, err := strconv.Atoi(os.Getenv("VECTOR_DIMENSIONS"))
	if err != nil {
		dim = DEFAULT_VECTOR_DIM
	}

	// Get cluster epsilon from env or use default
	related_eps, err := strconv.ParseFloat(os.Getenv("RELATED_EPS"), 64)
	if err != nil {
		related_eps = DEFAULT_RELATED_EPS
	}

	// server port
	port := os.Getenv("PORT")
	if port == "" {
		port = DEFAULT_PORT
	}

	// initialize database
	init, err := os.ReadFile("./factory/init.sql")
	noerror(err, "READ SQL ERROR")
	ds := NewBeansack(dbpath, string(init), dim, related_eps)
	defer ds.Close()

	r := initRoutes(ds)
	noerror(r.Run(":"+port), "SERVER ERROR")
}
