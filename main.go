package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

func main() {
	cfg := apiConfig{}
	serveMux := http.NewServeMux()
	handle := http.StripPrefix("/app", http.FileServer(http.Dir("./")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(handle))
	serveMux.HandleFunc("GET /api/healthz", readiness)
	serveMux.HandleFunc("GET /admin/metrics", cfg.metrics)
	serveMux.HandleFunc("POST /admin/reset", cfg.reset)
	serveMux.HandleFunc("POST /api/validate_chirp", validateChirp)
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	server.ListenAndServe()
}

func readiness(rW http.ResponseWriter, req *http.Request) {
	rW.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rW.WriteHeader(200)
	resp := []byte("OK")
	rW.Write(resp)
}

func validateChirp(rW http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding params: %s", err)
		respondWithError(rW, 500, "Something went wrong")
		rW.WriteHeader(500)
		return
	}
	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}
	chirpValid := len(params.Body) < 140
	fmt.Println("chirp: ", params.Body)
	fmt.Println("lenght: ", len(params.Body))
	chirpWords := strings.Split(params.Body, " ")
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	cleanedWords := []string{}
	for i, word := range chirpWords {
		if stringInSlice(word, badWords) {
			cleanedWords = append(cleanedWords, "****")
			fmt.Printf("bad word: %s, idx: %v\n", word, i)
			continue
		}
		cleanedWords = append(cleanedWords, word)
	}
	fmt.Println(cleanedWords)
	if !chirpValid {
		respondWithError(rW, 400, "Chirp is too long")
		return
	}
	respBody := returnVals{
		CleanedBody: strings.Join(cleanedWords, " "),
	}
	err = respondWithJson(rW, 200, respBody)
	if err != nil {
		log.Printf("Error marshaling json: %s", err)
		respondWithError(rW, 500, "Something went wrong")
		return
	}
}

type apiConfig struct {
	serverHits atomic.Int32
}

func (cfg *apiConfig) metrics(rW http.ResponseWriter, req *http.Request) {
	rW.Header().Set("Content-Type", "text/html; charset=utf-8")
	rW.WriteHeader(200)
	respStr := fmt.Sprintf(
		`
		<html>
		  <body>
		    <h1>Welcome, Chirpy Admin</h1>
		    <p>Chirpy has been visited %d times!</p>
		  </body>
		</html>
		`,
		cfg.serverHits.Load())
	resp := []byte(respStr)
	rW.Write(resp)
}

func (cfg *apiConfig) reset(rW http.ResponseWriter, req *http.Request) {
	rW.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rW.WriteHeader(200)
	cfg.serverHits.Store(0)
	respStr := fmt.Sprintf("Hits: %d", cfg.serverHits.Load())
	resp := []byte(respStr)
	rW.Write(resp)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.serverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJson(w, code, map[string]string{"error": msg})
}

func stringInSlice(s string, sL []string) bool {
	for _, str := range sL {
		if strings.ToLower(s) == str {
			return true
		}
	}
	return false
}
