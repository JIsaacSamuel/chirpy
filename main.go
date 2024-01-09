package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"internal/chirp_validation"
	"internal/configFuncs"
)

func main() {
	// Initialising chi router
	r := chi.NewRouter()
	cfg := configFuncs.ApiConfig{}
	cfg.SetToZero()

	// Handles
	tempvar := http.FileServer(http.Dir("./static/"))
	r.Handle("/app", http.StripPrefix("/app", cfg.MiddlewareMetricsInc(tempvar)))
	r.Handle("/app/*", http.StripPrefix("/app", cfg.MiddlewareMetricsInc(tempvar)))
	r.Mount("/api", apiHandler(&cfg))
	r.Mount("/admin", adminHandler(&cfg))

	// Common middleware
	corsMux := middlewareCors(r)
	localServer := http.Server{
		Handler: corsMux,
		Addr:    "localhost:8080",
	}

	// Running server
	fmt.Println("Server is starting")
	err := localServer.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

// api sub-router
func apiHandler(cfg *configFuncs.ApiConfig) http.Handler {
	r := chi.NewRouter()
	r.HandleFunc("/reset", cfg.ResetMetric)
	r.Get("/metrics", cfg.PrintIt)
	r.Get("/healthz", configFuncs.Responder)
	r.Post("/validate_chirp", chirp_validation.ValidateChirp)
	return r
}

// admin sub-router
func adminHandler(cfg *configFuncs.ApiConfig) http.Handler {
	r := chi.NewRouter()
	r.Get("/metrics", cfg.HandlerMetrics)
	return r
}

// middleware function
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
