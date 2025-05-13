package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type UserInfo struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type UserReq struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
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

func (cfg *apiConfig) loginUser(w http.ResponseWriter, r *http.Request) {
	req := UserReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	user, err := cfg.db.FetchUser(r.Context(), req.Email)
	err = auth.CheckPasswordHash(user.HashedPassword, req.Password)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	resp := UserInfo{
		Id:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	respondWithJson(w, 200, resp)
}
