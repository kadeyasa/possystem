package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		actorType := strings.ToLower(strings.TrimSpace(c.GetString("actor_type")))
		role := strings.ToLower(strings.TrimSpace(c.GetString("role")))

		if actorType == "admin" || actorType == "administrator" || role == "admin" || role == "administrator" {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		c.Abort()
	}
}
