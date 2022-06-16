package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/dgrijalva/jwt-go/v4"
	"github.com/fortuna91/ya_praktikum_final/internal/db"
	"net/http"
	"strings"
	"time"
)

var tokenDuration = 1 * time.Hour
var mySigningKey = []byte("secret")

func SetToken(userRequest *db.UserData) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &db.UserData{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: jwt.At(time.Now().Add(tokenDuration)),
			IssuedAt:  jwt.At(time.Now()),
		},
		Login: userRequest.Login,
	})
	return token.SignedString(mySigningKey)
}

func ParseToken(tokenRequest string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenRequest, &db.UserData{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected singing method")
		}
		return mySigningKey, nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(*db.UserData); ok && token.Valid {
		return claims.Login, nil
	}
	return "", fmt.Errorf("invalid access token")
}

func CalcHash(key string, hashedString string) (hash string) {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(hashedString))
	return hex.EncodeToString(h.Sum(nil))
}

func GetTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("there is no authorization header")
	}
	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return "", fmt.Errorf("wrong authorization header")
	}
	return headerParts[1], nil
}
