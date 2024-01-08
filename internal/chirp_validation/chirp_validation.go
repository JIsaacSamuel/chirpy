package chirp_validation

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"
)

var foulwords = []string{"fuck", "ass", "asshole", "bitch", "nigga", "kerfuffle", "sharbert", "fornax"}

type incoming struct {
	Body string `json:"body"`
}
type undesired struct {
	Error string `json:"error"`
}
type desired struct {
	Msg string `json:"cleaned_body"`
}

// Function to clean the message
func cleanBody(message string) (cleanedmessage string) {
	array := strings.Split(message, " ")
	for i := 0; i < len(array); i++ {
		temp := array[i]
		temp = strings.ToLower(temp)
		for j := 0; j < len(foulwords); j++ {
			if temp == foulwords[j] {
				array[i] = "****"
			}
		}
	}
	return strings.Join(array, " ")
}

// Function to validate the message
func ValidateChirp(w http.ResponseWriter, r *http.Request) {
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
