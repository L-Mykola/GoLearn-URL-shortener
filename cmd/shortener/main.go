package main

import (
	"log"
	"net/http"
	"time"

	"golearn/internal/httpapi"
	"golearn/internal/link"
	"golearn/internal/memstore"
)

func main() {
	store := memstore.New()
	svc := link.NewService(store)
	handler := httpapi.New(svc, "http://localhost:8090")

	srv := &http.Server{
		Addr:              ":8090",
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
