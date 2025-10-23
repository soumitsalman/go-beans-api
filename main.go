package main

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/k0kubun/pp"
	log "github.com/sirupsen/logrus"
	bs "github.com/soumitsalman/gobeansack/beansack"
	r "github.com/soumitsalman/gobeansack/router"
)

const (
	DEFAULT_DB_PATH                 = ""
	DEFAULT_PORT                    = "8080"
	DEFAULT_MAX_CONCURRENT_REQUESTS = 2
)

func main() {
	// load .env file into environment (if present)
	bs.LogWarning(godotenv.Load(), "No .env file found, continuing with system environment")

	// set logging stuff
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	catalog_path, ok := os.LookupEnv("PG_CONNECTION_STRING")
	if !ok {
		catalog_path = DEFAULT_DB_PATH
	}
	pp.Println(catalog_path)
	storage_path, ok := os.LookupEnv("STORAGE_DATAPATH")
	if !ok {
		storage_path = DEFAULT_DB_PATH
	}
	// enumerate up to 5 entries in storage_path/main/bean_cores (no IsDir check, no slice)
	beanCoresPath := storage_path + "/main/bean_cores"
	entries, err := os.ReadDir(beanCoresPath)
	bs.LogWarning(err, "Unable to read bean_cores directory, continuing")

	pp.Println("bean_cores (up to 5):")
	count := 0
	for _, e := range entries {
		pp.Println(e.Name())
		count++
		if count == 5 {
			break
		}
	}
	db, err := bs.NewReadonlyBeansack(catalog_path, storage_path)
	bs.NoError(err)
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = DEFAULT_PORT
	}
	max_concurrent_requests, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_REQUESTS"))
	if err != nil {
		max_concurrent_requests = DEFAULT_MAX_CONCURRENT_REQUESTS
	}

	engine := r.InitializeRoutes(db, max_concurrent_requests)
	bs.NoError(engine.Run("0.0.0.0:"+port), "SERVER ERROR")
}
