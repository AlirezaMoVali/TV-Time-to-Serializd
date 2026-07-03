package main

import (
	"log"
	"os"

	"github.com/alireza/tvtime2serializd/internal/config"
	"github.com/alireza/tvtime2serializd/internal/platform/postgres"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "down" {
		if err := postgres.MigrateDown(config.Load().DatabaseURL); err != nil {
			log.Fatal(err)
		}
		log.Println("rolled back one migration")
		return
	}

	if err := postgres.Migrate(config.Load().DatabaseURL); err != nil {
		log.Fatal(err)
	}
	log.Println("migrations applied")
}
