package main

import (
	"fmt"
	"net/http"
)

func main() {
	newMux := http.NewServeMux()
	corsMux := middlewareCors(newMux)
	localServer := http.Server{
		Handler: corsMux,
		Addr:    "localhost:8080",
	}
	fmt.Println("Server is starting")
	err := localServer.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
