package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Println("$PORT has not been set. Default: 8080")
		port = "8080"
	}

	startTime = time.Now()

	router := http.NewServeMux()

	// Spec root paths
	router.HandleFunc("/countryinfo/v1/status/", StatusHandler)
	router.HandleFunc("/countryinfo/v1/info/", InfoHandler)         // expects /countryinfo/v1/info/{code}
	router.HandleFunc("/countryinfo/v1/exchange/", ExchangeHandler) // expects /countryinfo/v1/exchange/{code}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Starting server on port " + port + " ...")
	log.Fatal(srv.ListenAndServe())
}
