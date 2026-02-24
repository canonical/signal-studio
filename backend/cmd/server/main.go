package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/simskij/otel-signal-lens/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := api.NewRouter()

	log.Printf("otel-signal-lens listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
