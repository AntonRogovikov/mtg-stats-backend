package main

import (
	"log"
	"net/http"
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
				"GET /api/users":           "Get all users",
				"GET /api/users/:id":       "Get user by ID",
				"POST /api/users":         "Create new user",
				"PUT /api/users/:id":      "Update user",
				"DELETE /api/users/:id":   "Delete user",
				"GET /api/decks":          "Get all decks",
				"GET /api/decks/:id":      "Get deck by ID",
				"POST /api/decks":         "Create new deck",
				"PUT /api/decks/:id":      "Update deck",
				"DELETE /api/decks/:id":   "Delete deck",
				"GET /api/games":          "Get all games",
				"GET /api/games/active":   "Get active game",
				"GET /api/games/:id":      "Get game by ID",
				"POST /api/games":         "Create game",
				"PUT /api/games/active":   "Update active game",
				"POST /api/games/active/finish": "Finish active game",
				"GET /api/stats/players":  "Get player stats",
				"GET /api/stats/decks":    "Get deck stats",
				"GET /health":             "Health check",
			},
		})
	})

	// Группа маршрутов для API
	api := router.Group("/api")
	{
		// User
		api.GET("/users", handlers.GetUsers)
		api.GET("/users/:id", handlers.GetUser)
		api.POST("/users", handlers.CreateUser)
		api.PUT("/users/:id", handlers.UpdateUser)
		api.DELETE("/users/:id", handlers.DeleteUser)

		// Deck
		api.GET("/decks", handlers.GetDecks)
		api.GET("/decks/:id", handlers.GetDeck)
		api.POST("/decks", handlers.CreateDeck)
		api.PUT("/decks/:id", handlers.UpdateDeck)
		api.DELETE("/decks/:id", handlers.DeleteDeck)

		// Games — POST регистрируем до GET /:id, плюс с trailing slash
		api.POST("/games", handlers.CreateGame)
		api.POST("/games/", handlers.CreateGame)
		api.GET("/games", handlers.GetGames)
		api.GET("/games/active", handlers.GetActiveGame)
		api.GET("/games/:id", handlers.GetGame)
		api.PUT("/games/active", handlers.UpdateActiveGame)
		api.POST("/games/active/finish", handlers.FinishGame)

		// Stats
		api.GET("/stats/players", handlers.GetPlayerStats)
		api.GET("/stats/decks", handlers.GetDeckStats)
	}

	// 404 — подсказка по URL (частая ошибка: запрос без /api)
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not found",
			"path":    c.Request.URL.Path,
			"hint":    "Проверьте URL: игры создаются через POST /api/games (обязателен префикс /api)",
		})
	})

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
