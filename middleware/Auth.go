package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kadeyasa/possystem/utils"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		jwtSecret, secretErr := utils.JWTSecret()
		if secretErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
			c.Abort()
			return
		}

		// Ambil Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		// Ambil token string
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse token: HS256 only, expiry required.
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !isAcceptedPOSTokenType(claims) {
			// Member, PT and untyped legacy tokens share the signing key with
			// the gym API but must never act as cashier/staff on the POS.
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Ambil claims
		{
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
			if karyawanID, ok := claims["karyawan_id"].(float64); ok && karyawanID > 0 {
				if _, exists := c.Get("user_id"); !exists {
					c.Set("user_id", int64(karyawanID))
				}
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

// Token types issued by the gym API (apifitness) that may use the POS:
// "staff" (admin/outlet/karyawan back-office login) and "karyawan_app"
// (cashier login). Member ("member"), PT ("pt") and untyped tokens are refused.
var acceptedPOSTokenTypes = map[string]bool{
	"staff":        true,
	"karyawan_app": true,
}

func isAcceptedPOSTokenType(claims jwt.MapClaims) bool {
	typ, _ := claims["typ"].(string)
	return acceptedPOSTokenTypes[strings.TrimSpace(typ)]
}
