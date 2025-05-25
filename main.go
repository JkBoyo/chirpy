package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
)

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
}

type apiConfig struct {
	serverHits atomic.Int32
	db         *database.Queries
	secret     string
	polkaKey   string
}

func main() {
	godotenv.Load()
	polkaApiKey := os.Getenv("POLKA_KEY")
	secret := os.Getenv("SECRET")
	dbUrl := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		fmt.Println("Db opening err")
	}
	dbQueries := database.New(db)
	cfg := apiConfig{
		db:       dbQueries,
		secret:   secret,
		polkaKey: polkaApiKey,
	}
	serveMux := http.NewServeMux()
	handle := http.StripPrefix("/app", http.FileServer(http.Dir("./")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(handle))
	serveMux.HandleFunc("GET /admin/metrics", cfg.metrics)
	serveMux.HandleFunc("POST /admin/reset", cfg.resetDb)
	serveMux.HandleFunc("GET /api/healthz", readiness)
	serveMux.HandleFunc("POST /api/users", cfg.createUser)
	serveMux.HandleFunc("PUT /api/users", cfg.updateUserAuth)
	serveMux.HandleFunc("POST /api/login", cfg.loginUser)
	serveMux.HandleFunc("POST /api/chirps", cfg.postChirp)
	serveMux.HandleFunc("GET /api/chirps", cfg.fetchChirps)
	serveMux.HandleFunc("GET /api/chirps/{chirpId}", cfg.fetchChirp)
	serveMux.HandleFunc("DELETE /api/chirps/{chirpId}", cfg.deleteChirp)
	serveMux.HandleFunc("POST /api/refresh", cfg.refresh)
	serveMux.HandleFunc("POST /api/revoke", cfg.revoke)
	serveMux.HandleFunc("POST /api/polka/webhooks", cfg.upgradeUser)
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	server.ListenAndServe()
}

func (cfg *apiConfig) fetchChirp(w http.ResponseWriter, r *http.Request) {
	fmt.Println("fetch chirp")
	chirpIDStr := r.PathValue("chirpId")
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		respondWithError(w, 500, "Something went wrong.")
		return
	}
	dbChirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, 404, "Chirp not found")
		return
	}
	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserId:    dbChirp.UserID,
	}
	fmt.Println(chirp)
	err = respondWithJson(w, 200, chirp)
	if err != nil {
		respondWithError(w, 500, "something went wrong")
		return
	}
}

func (cfg *apiConfig) fetchChirps(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		log.Println("Error fetching chirps")
		log.Println(err)
		respondWithError(w, 500, "Something went wrong")
		return
	}
	sortOrder := r.URL.Query().Get("sort")
	fmt.Println("fetch chirps")
	authorId := uuid.Nil
	authorIdStr := r.URL.Query().Get("author_id")
	if authorIdStr != "" {
		authorId, err = uuid.Parse(authorIdStr)
		if err != nil {
			log.Println(err)
			respondWithError(w, 500, "Something went wrong")
			return
		}
	}
	fmt.Println(dbChirps)
	respChirps := []Chirp{}
	for _, chirp := range dbChirps {
		if authorId != uuid.Nil && chirp.UserID != authorId {
			continue
		}
		chirpStruct := Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserId:    chirp.UserID,
		}
		respChirps = append(respChirps, chirpStruct)
		fmt.Println(chirpStruct)
	}
	if sortOrder == "desc" {
		sort.Slice(respChirps, func(i, j int) bool { return respChirps[i].CreatedAt.After(respChirps[j].CreatedAt) })
	}
	err = respondWithJson(w, 200, respChirps)
	if err != nil {
		log.Println("Error responding")
		respondWithError(w, 500, "Something went wrong")
		return
	}
}

func (cfg *apiConfig) deleteChirp(w http.ResponseWriter, r *http.Request) {
	fmt.Println("delete chirp")
	token := r.Header.Get("Authorization")
	tokenSlice := strings.Split(token, " ")
	if len(tokenSlice) < 2 {
		log.Println("token is malformed")
		respondWithError(w, 401, "token not valid")
		return
	}
	userId, err := auth.ValidateJWT(tokenSlice[1], cfg.secret)
	if err != nil {
		log.Println("token validation failed")
		log.Println(err)
		respondWithError(w, 401, "Something went wrong")
		return
	}
	chirpIDStr := r.PathValue("chirpId")
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		log.Println("Chirp Id parsing failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		log.Println("Fetching chirp from db failed")
		respondWithError(w, 404, "Chirp not found")
		return
	}
	if chirp.UserID != userId {
		log.Println("Users don't match")
		respondWithError(w, 403, "Wrong user")
		return
	}
	err = cfg.db.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		log.Println("Chirp not found:", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}
	respondWithJson(w, 204, nil)
}

func (cfg *apiConfig) resetDb(w http.ResponseWriter, r *http.Request) {
	fmt.Println("reset db")
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
	fmt.Println("posting chirp")
	type post struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(r.Body)
	postStruct := post{}
	err := decoder.Decode(&postStruct)
	if err != nil {
		log.Printf("Error decoding params: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting token: %v", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		log.Printf("Token invalid: %v", err)
		respondWithError(w, 401, "Authentication Error")
	}
	chirpValid := len(postStruct.Body) < 140
	chirpWords := strings.Split(postStruct.Body, " ")
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	cleanedWords := []string{}
	for i, word := range chirpWords {
		if slices.Contains(badWords, strings.ToLower(word)) {
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
		UserID: userID,
	}
	dbChirp, err := cfg.db.CreateChirp(r.Context(), createChirpParams)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	respChirp := Chirp{
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
