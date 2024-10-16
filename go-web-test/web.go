package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	fs := http.FileServer(http.Dir("./go-web-test"))
	http.Handle("/", noCache(fs))

	log.Printf("Starting server on http://localhost:8000")
	err := http.ListenAndServe("127.0.0.1:8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", time.Now().UTC().Format(http.TimeFormat))
		next.ServeHTTP(w, r)
	})
}
