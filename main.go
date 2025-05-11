package main

import (
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
)

type User struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type apiConfig struct {
	serverHits atomic.Int32
	db         *database.Queries
}

func main() {
	godotenv.Load()
	dbUrl := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		fmt.Println("Db opening err")
	}
	dbQueries := database.New(db)
	cfg := apiConfig{
		db: dbQueries,
	}
	serveMux := http.NewServeMux()
	handle := http.StripPrefix("/app", http.FileServer(http.Dir("./")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(handle))
	serveMux.HandleFunc("GET /api/healthz", readiness)
	serveMux.HandleFunc("GET /admin/metrics", cfg.metrics)
	serveMux.HandleFunc("POST /admin/reset", cfg.resetDb)
	serveMux.HandleFunc("POST /api/chirps", cfg.postChirp)
	serveMux.HandleFunc("POST /api/users", cfg.createUser)
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	server.ListenAndServe()
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Email string `json:"email"`
	}{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	user, err := cfg.db.CreateUser(r.Context(), req.Email)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	resp := User{
		Id:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	err = respondWithJson(w, 201, resp)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
	}

}

func (cfg *apiConfig) resetDb(w http.ResponseWriter, r *http.Request) {
	godotenv.Load()
	if os.Getenv("PLATFORM") != "dev" {
		respondWithError(w, 403, "Forbidden")
	}
	cfg.db.ResetUsers(r.Context())
}

func readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	resp := []byte("OK")
	w.Write(resp)
}

func (cfg *apiConfig) postChirp(w http.ResponseWriter, r *http.Request) {
	type post struct {
		Body   string    `json:"body"`
		UserId uuid.UUID `json:"user_id"`
	}
	decoder := json.NewDecoder(r.Body)
	postStruct := post{}
	err := decoder.Decode(&postStruct)
	if err != nil {
		log.Printf("Error decoding params: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}
	chirpValid := len(postStruct.Body) < 140
	chirpWords := strings.Split(postStruct.Body, " ")
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	cleanedWords := []string{}
	for i, word := range chirpWords {
		if slices.Contains(badWords, word) {
			cleanedWords = append(cleanedWords, "****")
			fmt.Printf("bad word: %s, idx: %v\n", word, i)
			continue
		}
		cleanedWords = append(cleanedWords, word)
	}
	if !chirpValid {
		respondWithError(w, 400, "Chirp is too long")
		return
	}
	createChirpParams := database.CreateChirpParams{
		Body:   strings.Join(cleanedWords, " "),
		UserID: postStruct.UserId,
	}
	dbChirp, err := cfg.db.CreateChirp(r.Context(), createChirpParams)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	respChirp := struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserId    uuid.UUID `json:"user_id"`
	}{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserId:    dbChirp.UserID,
	}
	err = respondWithJson(w, 201, respChirp)
	if err != nil {
		log.Printf("Error marshaling json: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}
}

func (cfg *apiConfig) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
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
	w.Write(resp)
}

func (cfg *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	cfg.serverHits.Store(0)
	respStr := fmt.Sprintf("Hits: %d", cfg.serverHits.Load())
	resp := []byte(respStr)
	w.Write(resp)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.serverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func respondWithJson(w http.ResponseWriter, code int, payload any) error {
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
