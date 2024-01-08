package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

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
	r.Mount("/api", apiHandler(&cfg))
	r.Mount("/admin", adminHandler(&cfg))

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

func apiHandler(cfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.HandleFunc("/reset", cfg.resetMetric)
	r.Get("/metrics", cfg.printIt)
	r.Get("/healthz", responder)
	r.Post("/validate_chirp", validateChirp)
	return r
}

func adminHandler(cfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.Get("/metrics", cfg.handlerMetrics)
	return r
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

func cleanBody(message string) (cleanedmessage string) {
	array := strings.Split(message, " ")
	for i := 0; i < len(array); i++ {
		temp := array[i]
		temp = strings.ToLower(temp)
		if temp == "kerfuffle" || temp == "sharbert" || temp == "fornax" {
			temp = "****"
			array[i] = temp
		}
	}
	return strings.Join(array, " ")
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
	type incoming struct {
		Body string `json:"body"`
	}
	type undesired struct {
		Error string `json:"error"`
	}
	type desired struct {
		Msg string `json:"cleaned_body"`
	}

	respBody := desired{}

	urespBody := undesired{}

	decoder := json.NewDecoder(r.Body)
	params := incoming{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if utf8.RuneCountInString(params.Body) <= 140 {
		respBody.Msg = cleanBody(params.Body)
		dat, err := json.Marshal(respBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(dat)
		return
	} else if utf8.RuneCountInString(params.Body) > 140 {
		urespBody.Error = "Chirp is too long"
		dat, err := json.Marshal(urespBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		return
	}
	urespBody.Error = "Something went wrong"
	dat, err := json.Marshal(urespBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	w.Write(dat)
	return
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
