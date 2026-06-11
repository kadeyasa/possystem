package main

import (
	"log"

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
