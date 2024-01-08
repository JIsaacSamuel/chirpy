package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"internal/chirp_validation"
)

// Number of times /app is loaded
type apiConfig struct {
	fileserverHits int
}

func main() {
	// Initialising chi router
	r := chi.NewRouter()
	cfg := apiConfig{
		fileserverHits: 0,
	}

	// Handles
	tempvar := http.FileServer(http.Dir("./static/"))
	r.Handle("/app", http.StripPrefix("/app", cfg.middlewareMetricsInc(tempvar)))
	r.Handle("/app/*", http.StripPrefix("/app", cfg.middlewareMetricsInc(tempvar)))
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
func apiHandler(cfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.HandleFunc("/reset", cfg.resetMetric)
	r.Get("/metrics", cfg.printIt)
	r.Get("/healthz", responder)
	r.Post("/validate_chirp", chirp_validation.ValidateChirp)
	return r
}

// admin sub-router
func adminHandler(cfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.Get("/metrics", cfg.handlerMetrics)
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

// function to check if server is running
func responder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Increases hits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

// Prints hits
func (cfg *apiConfig) printIt(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits)))
}

// Resets hits to 0
func (cfg *apiConfig) resetMetric(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits = 0
	w.WriteHeader(http.StatusOK)
}

// Injects html to print number of hits
func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`
<html>

<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visited %d times!</p>
</body>

</html>
	`, cfg.fileserverHits)))
}
