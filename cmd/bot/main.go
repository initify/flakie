package main

import (
	"log"
	"net/http"
	"os"

	"github.com/initify/flakie/internal/app"
)

func main() {
	cfg, err := app.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	srv := app.NewServer(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/webhook", srv)

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	log.Printf("flakie bot listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
