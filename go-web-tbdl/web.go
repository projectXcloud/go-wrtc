package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("./go-web-tbdl"))
	http.Handle("/", fs)

	log.Printf("Starting server on https://localhost:8080")
	// err := http.ListenAndServeTLS(":8080", "./go-web-tbdl/ssl.pem", "./go-web-tbdl/ssl.pem", nil)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
