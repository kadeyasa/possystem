package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("abcd123456yukdkidkdk")

func GenerateToken(userID string, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // expired 24 jam
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
