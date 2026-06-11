package main

import (
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/kadeyasa/possystem/utils"

	"github.com/kadeyasa/possystem/routes"

	"github.com/kadeyasa/possystem/middleware"

	"github.com/kadeyasa/possystem/database"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("❌ Failed to load .env file")
	}

	database.Connect()
	utils.InitLogger()
	utils.Log.Info("📦 Starting app...")

	r := gin.Default()
	if err := r.SetTrustedProxies(trustedProxies()); err != nil {
		utils.Log.Fatal("failed to configure trusted proxies: ", err)
	}

	// Gunakan CORS middleware
	r.Use(middleware.SetupCors())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.SanitizeJSONRequests())
	r.Use(middleware.SanitizeJSONResponses())

	routes.SetupRoutes(r)

	utils.Log.Info("🚀 App running at :8081")
	if err := r.Run(":8081"); err != nil {
		utils.Log.Fatal("failed to start server: ", err)
	}
	log.Println("Server started on port 8081")
}

func trustedProxies() []string {
	value := os.Getenv("TRUSTED_PROXIES")
	if strings.TrimSpace(value) == "" {
		return []string{"127.0.0.1", "::1"}
	}

	proxies := strings.Split(value, ",")
	result := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy != "" {
			result = append(result, proxy)
		}
	}
	if len(result) == 0 {
		return []string{"127.0.0.1", "::1"}
	}
	return result
}
