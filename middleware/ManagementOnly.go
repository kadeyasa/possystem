package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ManagementOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		actorType := strings.ToLower(strings.TrimSpace(c.GetString("actor_type")))
		role := strings.ToLower(strings.TrimSpace(c.GetString("role")))

		if actorType == "admin" || actorType == "administrator" || actorType == "outlet" || role == "admin" || role == "administrator" {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Management access required"})
		c.Abort()
	}
}
