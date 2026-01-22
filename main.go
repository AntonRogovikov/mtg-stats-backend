package main

import (
	"log"
	"os"

	"mtg-stats-backend/database"
	"mtg-stats-backend/handlers"

	"github.com/gin-gonic/gin"
)

func main() {

	// Проверяем, что мы в Railway
	if os.Getenv("RAILWAY_ENVIRONMENT") == "" {
		log.Println("⚠️ Запуск в локальном режиме")
	} else {
		log.Println("🚀 Запуск в Railway")

		// Проверяем наличие DATABASE_URL
		if os.Getenv("DATABASE_URL") == "" {
			log.Fatal("❌ DATABASE_URL не найден! Добавьте PostgreSQL базу в Railway Dashboard")
		}
	}

	// Инициализируем базу
	err := database.InitDB()
	if err != nil {
		log.Fatalf("❌ Ошибка базы данных: %v", err)
	}

	// Устанавливаем режим Gin
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}
	gin.SetMode(ginMode)

	// Создаем роутер
	router := gin.Default()

	// Настраиваем CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Основные маршруты
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "MTG Stats API is running",
			"status":  "OK",
			"version": "1.0.0",
			"mode":    gin.Mode(),
			"endpoints": gin.H{
				"GET /api/v1/users":        "Get all users",
				"GET /api/v1/users/:id":    "Get user by ID",
				"POST /api/v1/users":       "Create new user",
				"PUT /api/v1/users/:id":    "Update user",
				"DELETE /api/v1/users/:id": "Delete user",
				"GET /health":              "Health check",
			},
		})
	})

	// Группа маршрутов для API
	api := router.Group("/api/v1")
	{
		api.GET("/users", handlers.GetUsers)
		api.GET("/users/:id", handlers.GetUser)
		api.POST("/users", handlers.CreateUser)
		api.PUT("/users/:id", handlers.UpdateUser)
		api.DELETE("/users/:id", handlers.DeleteUser)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		db := database.GetDB()

		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(500, gin.H{
				"status": "unhealthy",
				"error":  "Database connection error",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(500, gin.H{
				"status": "unhealthy",
				"error":  "Database ping failed",
			})
			return
		}

		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": "connected",
			"mode":     gin.Mode(),
		})
	})

	// Получаем порт из переменных окружения
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Server starting on port %s in %s mode", port, gin.Mode())

	// Запускаем сервер
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
