package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"internal/auth"
	"internal/database"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type Chirp struct {
	Body string `json:"body"`
	ID   int    `json:"id"`
}

type User struct {
	EmailID string `json:"email"`
	ID      int    `json:"id"`
	// Token   string `json:"token"`
}

type apiConfig struct {
	fileserverHits int
	DB             *database.DB
	SecSig         string
	RevokeDB       map[string]time.Time
}

func main() {
	const filepathRoot = "./static/"
	const port = "8080"

	db, err := database.NewDB("database.json")
	if err != nil {
		log.Fatal(err)
	}
	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")

	req := make(map[string]time.Time)

	apiCfg := apiConfig{
		fileserverHits: 0,
		DB:             db,
		SecSig:         jwtSecret,
		RevokeDB:       req,
	}

	router := chi.NewRouter()
	fsHandler := apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot))))
	router.Handle("/app", fsHandler)
	router.Handle("/app/*", fsHandler)

	apiRouter := chi.NewRouter()
	apiRouter.Get("/healthz", handlerReadiness)
	apiRouter.Get("/reset", apiCfg.handlerReset)
	apiRouter.Post("/chirps", apiCfg.handlerChirpsCreate)
	apiRouter.Get("/chirps", apiCfg.handlerChirpsRetrieve)
	apiRouter.Get("/chirps/{chirpsID}", apiCfg.handlerChirpsRetrieveID)
	apiRouter.Post("/users", apiCfg.handlerUserCreate)
	apiRouter.Post("/login", apiCfg.handlerUserValidate)
	apiRouter.Post("/refresh", apiCfg.handlerRefresh)
	apiRouter.Post("/revoke", apiCfg.handlerRevoke)
	apiRouter.Put("/users", apiCfg.handlerUserUpdate)
	router.Mount("/api", apiRouter)

	adminRouter := chi.NewRouter()
	adminRouter.Get("/metrics", apiCfg.handlerMetrics)
	router.Mount("/admin", adminRouter)

	corsMux := middlewareCors(router)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: corsMux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
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

func respondWithError(w http.ResponseWriter, code int, msg string) {
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}

func (cfg *apiConfig) handlerChirpsRetrieve(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.DB.GetChirps()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve chirps")
		return
	}

	chirps := []Chirp{}
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, Chirp{
			ID:   dbChirp.ID,
			Body: dbChirp.Body,
		})
	}

	sort.Slice(chirps, func(i, j int) bool {
		return chirps[i].ID < chirps[j].ID
	})

	respondWithJSON(w, http.StatusOK, chirps)
}

func (cfg *apiConfig) handlerChirpsRetrieveID(w http.ResponseWriter, r *http.Request) {
	param := chi.URLParam(r, "chirpsID")
	v, err := strconv.Atoi(param)
	if err != nil {
		log.Fatal("Enter a valid chirp ID")
	}
	text, err := cfg.DB.GetChirpByID(v)
	if err != nil {
		respondWithError(w, 404, "No chirp found")
		return
	}

	chirp := Chirp{
		Body: text,
		ID:   v,
	}

	respondWithJSON(w, http.StatusOK, chirp)
}

func (cfg *apiConfig) handlerChirpsCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	cleaned, err := validateChirp(params.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	chirp, err := cfg.DB.CreateChirp(cleaned)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create chirp")
		return
	}

	respondWithJSON(w, http.StatusCreated, Chirp{
		ID:   chirp.ID,
		Body: chirp.Body,
	})
}

func validateChirp(body string) (string, error) {
	const maxChirpLength = 140
	if len(body) > maxChirpLength {
		return "", errors.New("Chirp is too long")
	}

	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	cleaned := getCleanedBody(body, badWords)
	return cleaned, nil
}

func getCleanedBody(body string, badWords map[string]struct{}) string {
	words := strings.Split(body, " ")
	for i, word := range words {
		loweredWord := strings.ToLower(word)
		if _, ok := badWords[loweredWord]; ok {
			words[i] = "****"
		}
	}
	cleaned := strings.Join(words, " ")
	return cleaned
}

func (cfg *apiConfig) handlerUserCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
		Pass  string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	hashPass, err := bcrypt.GenerateFromPassword([]byte(params.Pass), 10)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save login details")
		return
	}

	user, err := cfg.DB.CreateUser(params.Email, hashPass)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, User{
		ID:      user.ID,
		EmailID: user.EmailID,
	})
}

func (cfg *apiConfig) handlerUserUpdate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
		Pass  string `json:"password"`
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		respondWithError(w, 401, "Error: Authorization not included")
		return
	}
	tempSlice := strings.Split(authHeader, " ")
	if len(tempSlice) < 2 || tempSlice[0] != "Bearer" {
		respondWithError(w, 401, "Error: Malformed Authorization header")
		return
	}
	subject, err := auth.ValidateJWT(tempSlice[1], cfg.SecSig, "chirpy-access")
	if err != nil {
		respondWithError(w, 401, "Error: Malformed Authorization header")
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Pass), 10)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't hash password")
		return
	}

	userIDInt, err := strconv.Atoi(subject)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse user ID")
		return
	}

	user, err := cfg.DB.UpdateUser(userIDInt, params.Email, hashedPassword)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create user")
		return
	}

	respondWithJSON(w, 200, User{
		ID:      user.ID,
		EmailID: user.EmailID,
	})
}

func (cfg *apiConfig) handlerUserValidate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email      string `json:"email"`
		Pass       string `json:"password"`
		ExpiryTime int    `json:"expires_in_seconds"`
	}

	type response struct {
		User
		Token  string `json:"token"`
		Token2 string `json:"refresh_token"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	pass, err := cfg.DB.GetUser(params.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Invalid user")
		return
	}

	if bcrypt.CompareHashAndPassword(pass.Password, []byte(params.Pass)) != nil {
		respondWithError(w, 401, "Incorrect password")
		return
	}

	access_time := 60 * 60 * time.Second
	ref_time := 60 * 24 * access_time

	token_access, err := auth.MakeJWT(pass.ID, cfg.SecSig, "chirpy-access", time.Duration(access_time))
	token_ref, err := auth.MakeJWT(pass.ID, cfg.SecSig, "chirpy-refresh", time.Duration(ref_time))

	respondWithJSON(w, http.StatusOK, response{
		User: User{
			ID:      pass.ID,
			EmailID: pass.EmailID,
		},
		Token:  token_access,
		Token2: token_ref,
	})
}
func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Token string `json:"token"`
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		respondWithError(w, 404, "Error: Authorization not included")
		return
	}
	tempSlice := strings.Split(authHeader, " ")
	if len(tempSlice) < 2 || tempSlice[0] != "Bearer" {
		respondWithError(w, 403, "Error: Malformed Authorization header")
		return
	}
	subID, err := auth.ValidateJWT(tempSlice[1], cfg.SecSig, "chirpy-refresh")
	if err != nil {
		respondWithError(w, 401, "Invalid token")
		return
	}

	if _, ok := cfg.RevokeDB[tempSlice[1]]; ok == true {
		respondWithError(w, 401, "Resfresh token revoked already")
		return
	}

	access_time := 60 * 60 * time.Second
	isubID, err := strconv.Atoi(subID)

	token_access, err := auth.MakeJWT(isubID, cfg.SecSig, "chirpy-access", time.Duration(access_time))
	// token_ref, err := auth.MakeJWT(pass.ID, cfg.SecSig, "chirpy-refresh", time.Duration(ref_time))

	respondWithJSON(w, http.StatusOK, response{
		Token: token_access,
	})
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		respondWithError(w, 401, "Error: Authorization not included")
		return
	}
	tempSlice := strings.Split(authHeader, " ")
	if len(tempSlice) < 2 || tempSlice[0] != "Bearer" {
		respondWithError(w, 401, "Error: Malformed Authorization header")
		return
	}
	_, err := auth.ValidateJWT(tempSlice[1], cfg.SecSig, "chirpy-refresh")
	if err != nil {
		respondWithError(w, 401, "Invalid token")
		return
	}
	cfg.RevokeDB[tempSlice[1]] = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`
<html>

<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visited %dtimes!</p>
</body>

</html>
	`, cfg.fileserverHits)))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits = 0
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}
