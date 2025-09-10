package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
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
	godotenv.Load(".env")

	// set log output to a file if specified
	if log_dir := os.Getenv("LOGS"); log_dir != "" {
		log_file_path := fmt.Sprintf("%s/beansack-%s.log", log_dir, time.Now().Format("2006-01-02-15-04-05"))
		log_file, err := os.OpenFile(log_file_path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		noerror(err, "LOG FILE ERROR")
		log.SetOutput(log_file)
	}

	// if nothing is mentioned then treat this as in memory db
	// else if the directory is mentioned but directory does not exist then create one
	db_path := os.Getenv("DATA")
	if db_path == "" {
		db_path = DEFAULT_DB_PATH
	} else {
		if _, err := os.Stat(db_path); os.IsNotExist(err) {
			noerror(os.MkdirAll(db_path, 0755), "CREATE DB DIR ERROR")
		}
		db_path = fmt.Sprintf("%s/%s", db_path, DB_NAME)
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

	throttle_max, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_QUERIES"))
	if err != nil || throttle_max <= 0 {
		throttle_max = DEFAULT_MAX_CONCURRENT_QUERIES
	}

	refresh_time, err := strconv.Atoi(os.Getenv("REFRESH_TIME"))
	if err != nil || refresh_time <= 0 {
		refresh_time = DEFAULT_REFRESH_TIME // default refresh time in minutes
	}

	// server port
	port := os.Getenv("PORT")
	if port == "" {
		port = DEFAULT_PORT
	}

	// initialize database
	init, err := os.ReadFile("./factory/init.sql")
	noerror(err, "READ SQL ERROR")

	engine := NewEngine(db_path, string(init), dim, related_eps, throttle_max, refresh_time)
	defer engine.Close()

	noerror(engine.Run("0.0.0.0:"+port), "SERVER ERROR")
}
