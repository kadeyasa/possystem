package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("abcd123456yukdkidkdk") // JWT_KEY

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		// Ambil token string
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// pastikan algoritma HS256
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Ambil claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			actorType := ""
			if rawActorType, ok := claims["actor_type"].(string); ok && strings.TrimSpace(rawActorType) != "" {
				actorType = strings.TrimSpace(rawActorType)
				c.Set("actor_type", actorType)
			}

			if userID, ok := claims["user_id"].(string); ok && strings.TrimSpace(userID) != "" {
				c.Set("user_id", strings.TrimSpace(userID))
			}
			if userID, ok := claims["userid"].(string); ok && strings.TrimSpace(userID) != "" {
				c.Set("user_id", strings.TrimSpace(userID))
			}
			if userID, ok := claims["userid"].(float64); ok && userID > 0 {
				c.Set("user_id", int64(userID))
			}
			if memberID, ok := claims["member_id"].(string); ok && strings.TrimSpace(memberID) != "" {
				c.Set("user_id", strings.TrimSpace(memberID))
				if actorType == "" {
					actorType = "karyawan"
					c.Set("actor_type", actorType)
				}
			}
			if memberID, ok := claims["member_id"].(float64); ok && memberID > 0 {
				c.Set("user_id", int64(memberID))
				if actorType == "" {
					actorType = "karyawan"
					c.Set("actor_type", actorType)
				}
			}
			if role, ok := claims["role"].(string); ok && strings.TrimSpace(role) != "" {
				c.Set("role", strings.TrimSpace(role))
			}
			if roleCode, ok := claims["role_code"].(float64); ok {
				c.Set("role_code", int(roleCode))
			}
			if roleCode, ok := claims["role_code"].(string); ok && strings.TrimSpace(roleCode) != "" {
				c.Set("role_code", strings.TrimSpace(roleCode))
			}
			if jabatan, ok := claims["jabatan"].(string); ok && strings.TrimSpace(jabatan) != "" {
				c.Set("jabatan", strings.TrimSpace(jabatan))
				if actorType == "" {
					c.Set("role", strings.TrimSpace(jabatan))
				}
			}

			displayName := ""
			for _, key := range []string{"full_name", "fullname", "name", "nama", "outlet_name", "username", "email"} {
				if value, ok := claims[key].(string); ok && strings.TrimSpace(value) != "" {
					trimmedValue := strings.TrimSpace(value)
					c.Set(key, trimmedValue)
					if displayName == "" {
						displayName = trimmedValue
					}
				}
			}
			if displayName != "" {
				c.Set("display_name", displayName)
			}
		}

		// lanjut ke handler berikutnya
		c.Next()
	}
}
