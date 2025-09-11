package main

import (
	"log"
	"net/http"
	"os"

	"github.com/initify/flakie/pkg/app"
)

func main() {
	router, err := app.RouterFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	log.Printf("flakie bot listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
