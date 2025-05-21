package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type UserInfo struct {
	Id           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	RefreshToken string    `json:"refresh_tok"`
	Token        string    `json:"token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

type UserReq struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}

func (cfg *apiConfig) upgradeUser(w http.ResponseWriter, r *http.Request) {
	apiKey, err := auth.GetApiKey(r.Header)
	if err != nil {
		log.Println(err)
		respondWithError(w, 401, "Authorization failed")
		return
	}
	if apiKey != cfg.polkaKey {
		log.Println("Apikey doesn't match")
		respondWithError(w, 401, "Authorization failed")
	}
	req := struct {
		Event string `json:"event"`
		Data  struct {
			UserId uuid.UUID `json:"user_id"`
		} `json:"data"`
	}{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&req)
	if err != nil {
		log.Println("Error decoding reqest")
		respondWithError(w, 500, "Malformed request")
		return
	}
	if req.Event != "user.upgraded" {
		log.Println("user not upgraded")
		respondWithError(w, 204, "user not upgraded")
		return
	}
	err = cfg.db.AddChirpyRed(r.Context(), req.Data.UserId)
	if err != nil {
		log.Println("User not found in db")
		respondWithError(w, 404, "User not found")
		return
	}
	err = respondWithJson(w, 204, "")
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	fmt.Println("create user")
	req := UserReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	userParams := database.CreateUserParams{
		Email:          req.Email,
		HashedPassword: hashedPw,
	}
	user, err := cfg.db.CreateUser(r.Context(), userParams)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	resp := UserInfo{
		Id:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed.Bool,
	}
	err = respondWithJson(w, 201, resp)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
	}

}

func (cfg *apiConfig) loginUser(w http.ResponseWriter, r *http.Request) {
	fmt.Println("login user")
	req := UserReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	user, err := cfg.db.FetchUser(r.Context(), req.Email)
	accToken, err := auth.MakeJWT(user.ID, cfg.secret, time.Hour)
	if err != nil {
		log.Println("Access token creation failed")
		respondWithError(w, 500, "something went wrong")
		return
	}
	refreshToken, err := auth.MakeRefreshToken()
	refTokParams := database.CreateRefTokParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(60 * time.Hour * 24),
	}
	respRefTok, err := cfg.db.CreateRefTok(r.Context(), refTokParams)
	if err != nil {
		log.Println("Refresh token creation failed")
		respondWithError(w, 500, "something went wrong")
		return
	}
	err = auth.CheckPasswordHash(user.HashedPassword, req.Password)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	resp := UserInfo{
		Id:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        accToken,
		RefreshToken: respRefTok.Token,
		IsChirpyRed:  user.IsChirpyRed.Bool,
	}
	fmt.Println(resp)
	respondWithJson(w, 200, resp)
}

func (cfg *apiConfig) updateUserAuth(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Password update")
	authTok := r.Header.Get("Authorization")
	if authTok == "" {
		log.Println("No authorization token found")
		respondWithError(w, 401, "Authorization failed")
		return
	}
	userId, err := auth.ValidateJWT(strings.Split(authTok, " ")[1], cfg.secret)
	if err != nil {
		log.Println("Token Invalid")
		respondWithError(w, 401, "Invalid token")
		return
	}
	req := UserReq{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&req)
	if err != nil {
		log.Println("Decoding error")
		respondWithJson(w, 500, "Something went wrong")
		return
	} else {
		if req.Email == "" || req.Password == "" {
			respondWithError(w, 401, "Please enter both username or password")
			return
		}
	}
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Println("Password hashing failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
	updateUserParams := database.UpdateUserParams{
		Email:          req.Email,
		HashedPassword: hashedPw,
		ID:             userId,
	}
	user, err := cfg.db.UpdateUser(r.Context(), updateUserParams)
	if err != nil {
		log.Println("User update failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
	resp := UserInfo{
		Id:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed.Bool,
	}
	respondWithJson(w, 200, resp)
}

func (cfg *apiConfig) refresh(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Refresh token")
	authHead := r.Header.Get("Authorization")
	token := strings.Split(authHead, " ")[1]
	refTok, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		log.Printf("Token not found: %v", err)
		respondWithError(w, 401, "Authorization failed")
		return
	} else if time.Since(refTok.ExpiresAt) > time.Duration(0) || time.Since(refTok.RevokedAt.Time) > time.Duration(0) && refTok.RevokedAt.Valid {
		log.Println("Token expired or revoked")
		log.Printf("Time since expired: %v", time.Since(refTok.ExpiresAt))
		log.Printf("Revoked time: %v", refTok.RevokedAt.Time)
		log.Printf("Time since revoked: %v", time.Since(refTok.RevokedAt.Time))
		respondWithError(w, 401, "Authorization failed")
		return
	}
	newAccTok, err := auth.MakeJWT(refTok.UserID, cfg.secret, time.Hour)
	if err != nil {
		log.Println("Access token creation failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
	resp := struct {
		Token string `json:"token"`
	}{Token: newAccTok}
	err = respondWithJson(w, 200, resp)
	if err != nil {
		log.Println("Response failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
}

func (cfg *apiConfig) revoke(w http.ResponseWriter, r *http.Request) {
	fmt.Println("revoke tok")
	authHead := r.Header.Get("Authorization")
	tok := strings.Split(authHead, " ")[1]
	fmt.Println(tok)
	err := cfg.db.RevokeTok(r.Context(), tok)
	if err != nil {
		log.Println("Revoking token failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
	err = respondWithJson(w, 204, "Token revoked")
	if err != nil {
		log.Println("Response failed")
		respondWithError(w, 500, "Something went wrong")
		return
	}
}
