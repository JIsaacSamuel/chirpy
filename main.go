package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type apiConfig struct {
	fileserverHits int
}

func main() {
	r := chi.NewRouter()
	cfg := apiConfig{
		fileserverHits: 0,
	}

	tempvar := http.FileServer(http.Dir("./static/"))
	r.Handle("/app", http.StripPrefix("/app", cfg.middlewareMetricsInc(tempvar)))
	r.Handle("/app/*", http.StripPrefix("/app", cfg.middlewareMetricsInc(tempvar)))
	r.HandleFunc("/reset", cfg.resetMetric)
	r.Get("/metrics", cfg.printIt)
	r.Get("/healthz", responder)

	corsMux := middlewareCors(r)
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
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func responder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) printIt(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits)))
}

func (cfg *apiConfig) resetMetric(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits = 0
	w.WriteHeader(http.StatusOK)
}
