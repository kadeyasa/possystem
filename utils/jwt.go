package utils

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func JWTSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_KEY"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("POSSYSTEM_JWT_KEY"))
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_KEY must be at least 32 characters")
	}
	return []byte(secret), nil
}

func GenerateToken(userID string, role string) (string, error) {
	secret, err := JWTSecret()
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // expired 24 jam
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
