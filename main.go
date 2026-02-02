// Package main — точка входа MTG Stats API.
// REST API для учёта игр Magic: The Gathering: пользователи, колоды, игры и статистика.
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
	// Проверяем окружение (Railway или локальный запуск)
	if os.Getenv("RAILWAY_ENVIRONMENT") == "" {
		log.Println("⚠️ Запуск в локальном режиме")
	} else {
		log.Println("🚀 Запуск в Railway")

		// Проверяем наличие DATABASE_URL
		if os.Getenv("DATABASE_URL") == "" {
			log.Fatal("❌ DATABASE_URL не найден! Добавьте PostgreSQL базу в Railway Dashboard")
		}
	}

	// Инициализация подключения к PostgreSQL и миграции таблиц
	err := database.InitDB()
	if err != nil {
		log.Fatalf("❌ Ошибка базы данных: %v", err)
	}

	// Режим Gin: debug / release / test
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}
	gin.SetMode(ginMode)

	router := gin.Default()

	// Статика: раздача загруженных изображений колод по URL /uploads/...
	router.Static("/uploads", "./uploads")

	// CORS: разрешаем запросы с любых источников (для SPA/мобильных клиентов)
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

	// Корневой маршрут: информация об API и список эндпоинтов
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "MTG Stats API запущен",
			"status":  "OK",
			"version": "1.0.0",
			"mode":    gin.Mode(),
			"endpoints": gin.H{
				"GET /api/users":           "Список пользователей",
				"GET /api/users/:id":       "Пользователь по ID",
				"POST /api/users":         "Создать пользователя",
				"PUT /api/users/:id":      "Обновить пользователя",
				"DELETE /api/users/:id":   "Удалить пользователя",
				"GET /api/decks":          "Список колод",
				"GET /api/decks/:id":      "Колода по ID",
				"POST /api/decks":         "Создать колоду",
				"PUT /api/decks/:id":      "Обновить колоду",
				"POST /api/decks/:id/image": "Загрузить изображение колоды",
				"DELETE /api/decks/:id":   "Удалить колоду",
				"GET /api/games":          "Список игр",
				"GET /api/games/active":   "Активная игра",
				"GET /api/games/:id":      "Игра по ID",
				"POST /api/games":         "Создать игру",
				"PUT /api/games/active":   "Обновить активную игру",
				"POST /api/games/active/finish": "Завершить активную игру",
				"GET /api/stats/players":  "Статистика игроков",
				"GET /api/stats/decks":    "Статистика колод",
				"GET /health":             "Проверка состояния",
			},
		})
	})

	// Группа /api — CRUD пользователей, колод, игр и статистика
	api := router.Group("/api")
	{
		// Пользователи
		api.GET("/users", handlers.GetUsers)
		api.GET("/users/:id", handlers.GetUser)
		api.POST("/users", handlers.CreateUser)
		api.PUT("/users/:id", handlers.UpdateUser)
		api.DELETE("/users/:id", handlers.DeleteUser)

		// Колоды
		api.GET("/decks", handlers.GetDecks)
		api.GET("/decks/:id", handlers.GetDeck)
		api.POST("/decks", handlers.CreateDeck)
		api.PUT("/decks/:id", handlers.UpdateDeck)
		api.POST("/decks/:id/image", handlers.UploadDeckImage)
		api.DELETE("/decks/:id", handlers.DeleteDeck)

		// Игры (POST до GET /:id, чтобы /active и /:id не конфликтовали; поддержка trailing slash)
		api.POST("/games", handlers.CreateGame)
		api.POST("/games/", handlers.CreateGame)
		api.GET("/games", handlers.GetGames)
		api.GET("/games/active", handlers.GetActiveGame)
		api.GET("/games/:id", handlers.GetGame)
		api.PUT("/games/active", handlers.UpdateActiveGame)
		api.POST("/games/active/finish", handlers.FinishGame)

		// Статистика игроков и колод
		api.GET("/stats/players", handlers.GetPlayerStats)
		api.GET("/stats/decks", handlers.GetDeckStats)
	}

	// 404: подсказка по корректному URL (частая ошибка — запрос без префикса /api)
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Не найдено",
			"path":    c.Request.URL.Path,
			"hint":    "Проверьте URL: игры создаются через POST /api/games (обязателен префикс /api)",
		})
	})

	// Health check: проверка доступности сервиса и подключения к БД
	router.GET("/health", func(c *gin.Context) {
		db := database.GetDB()

		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(500, gin.H{
				"status": "unhealthy",
				"error":  "Ошибка подключения к базе данных",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(500, gin.H{
				"status": "unhealthy",
				"error":  "База данных не отвечает",
			})
			return
		}

		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": "connected",
			"mode":     gin.Mode(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Server starting on port %s in %s mode", port, gin.Mode())

	if err := router.Run(":" + port); err != nil {
		log.Fatal("Не удалось запустить сервер:", err)
	}
}
