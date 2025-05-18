package auth

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordHashing(t *testing.T) {
	wrongPassword := "NOT THIS ONE"
	testPassword := "ThisIsForATest"
	hashedPassword, _ := HashPassword(testPassword)
	err := CheckPasswordHash(hashedPassword, testPassword)
	if err != nil {
		t.Fatal("Expected passwords to match")
	}
	err = CheckPasswordHash(hashedPassword, wrongPassword)
	if err == nil {
		t.Fatal("Expected passwords not to match and throw an err")
	}
}

func TestTokens(t *testing.T) {
	testUserUuid := uuid.New()
	sumTestSecret := "seek and ye shall find"
	expiration := time.Duration(time.Millisecond * 500)
	timeOut := time.Duration(800 * time.Millisecond)
	testToken, err := MakeJWT(testUserUuid, sumTestSecret, expiration)
	if err != nil {
		t.Fatal("Token generation failed.")
	}
	testTicker := time.NewTicker(300 * time.Millisecond)
	initTime := time.Now()
	uuID, err := ValidateJWT(testToken, sumTestSecret)
	if err != nil {
		t.Errorf("Validation error: %q", err)
	}
	if uuID != testUserUuid {
		t.Logf("uuID: %s, testUserUuid: %s", uuID, testUserUuid)
		t.Fatal("Incorrect user id")
	}
	for {
		tick := <-testTicker.C
		fmt.Println(tick.Sub(initTime))
		_, err = ValidateJWT(testToken, sumTestSecret)
		if err != nil && tick.Sub(initTime) < expiration {
			t.Fatal("Validation failed before expiration")
		} else if err == nil && tick.Sub(initTime) > expiration {
			t.Fatal("Validation didn't fail when token expired")
		}
		if time.Since(initTime) > timeOut {
			testTicker.Stop()
			break
		}
	}
}

func TestGetBearerToken(t *testing.T) {
	testUserUuid := uuid.New()
	sumTestSecret := "seek and ye shall find"
	expiration := time.Duration(time.Second * 5)
	testToken, err := MakeJWT(testUserUuid, sumTestSecret, expiration)
	if err != nil {
		t.Fatal("Token generation failed.")
	}
	goodAuthHeader := http.Header(make(map[string][]string))
	bearerToken := "Bearer " + testToken
	goodAuthHeader.Add("Authorization", bearerToken)
	retrievedTok, err := GetBearerToken(goodAuthHeader)
	if err != nil {
		t.Errorf("token retrieval failed: %v", err)
	} else if retrievedTok != testToken {
		t.Errorf("token not correctly retrieved: %s", retrievedTok)
	}
}
