// Package main — точка входа MTG Stats API (пользователи, колоды, игры, статистика).
package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"mtg-stats-backend/database"
	"mtg-stats-backend/handlers"
	"mtg-stats-backend/middleware"

	"github.com/gin-gonic/gin"
)

// corsMiddleware — CORS: CORS_ALLOWED_ORIGINS (через запятую) или localhost при LOCAL_DSN; иначе "*".
// localhost всегда разрешён для разработки, даже если в списке только production-домен.
func corsMiddleware(isLocal bool) gin.HandlerFunc {
	allowedList := parseCORSOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowOrigin := ""

		if len(allowedList) > 0 {
			for _, o := range allowedList {
				if o == origin {
					allowOrigin = origin
					break
				}
			}
			// Разрешить localhost при разработке, даже если в списке только production
			if allowOrigin == "" && origin != "" && isLocalhostOrigin(origin) {
				allowOrigin = origin
			}
		} else if isLocal && origin != "" && isLocalhostOrigin(origin) {
			allowOrigin = origin
		} else {
			allowOrigin = "*"
		}

		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

func parseCORSOrigins(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if o := strings.TrimSpace(part); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func getJWTSecret() string {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if s != "" {
		return s
	}
	return os.Getenv("API_TOKEN")
}

func isLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1"
}

func main() {
	isLocal := os.Getenv("LOCAL_DSN") != ""

	if isLocal {
		log.Println("Запуск в локальном режиме (LOCAL_DSN)")
	} else {
		log.Println("Запуск в production (DATABASE_URL)")
		if os.Getenv("DATABASE_URL") == "" {
			log.Fatal("DATABASE_URL не задан")
		}
	}

	if err := database.InitDB(); err != nil {
		log.Fatalf("Ошибка БД: %v", err)
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}
	gin.SetMode(ginMode)

	router := gin.Default()
	router.SetTrustedProxies(nil)
	router.Use(corsMiddleware(isLocal))
	if !isLocal {
		router.Use(middleware.RequireHTTPS())
	}
	router.Static("/uploads", handlers.GetUploadDir())

	apiToken := strings.TrimSpace(os.Getenv("API_TOKEN"))
	if apiToken != "" {
		log.Println("API защищён Bearer-токеном (API_TOKEN)")
	} else {
		log.Println("API без авторизации (API_TOKEN не задан)")
	}

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message":   "MTG Stats API запущен",
			"status":    "OK",
			"version":   "1.0.0",
			"mode":      gin.Mode(),
			"auth":      apiToken != "",
			"auth_hint": "При auth=true все /api/* требуют заголовок: Authorization: Bearer <API_TOKEN>",
			"endpoints": gin.H{
				"POST /api/auth/login":          "Вход (name, password) → JWT",
				"GET /api/users":                "Список пользователей",
				"GET /api/users/:id":            "Пользователь по ID",
				"POST /api/users":               "Создать пользователя",
				"PUT /api/users/:id":            "Обновить пользователя",
				"DELETE /api/users/:id":         "Удалить пользователя",
				"GET /api/decks":                "Список колод",
				"GET /api/decks/:id":            "Колода по ID",
				"POST /api/decks":               "Создать колоду",
				"PUT /api/decks/:id":            "Обновить колоду",
				"POST /api/decks/:id/image":     "Загрузить изображение и аватар колоды (multipart: image, avatar)",
				"DELETE /api/decks/:id/image":   "Удалить изображение и аватар колоды",
				"DELETE /api/decks/:id":         "Удалить колоду",
				"GET /api/games":                "Список игр",
				"GET /api/games/active":         "Активная игра",
				"GET /api/games/:id":            "Игра по ID",
				"POST /api/games":               "Создать игру",
				"PUT /api/games/active":         "Обновить активную игру",
				"POST /api/games/active/finish": "Завершить активную игру",
				"GET /api/stats/players":        "Статистика игроков",
				"GET /api/stats/decks":          "Статистика колод",
				"GET /api/export/all":           "Экспорт всех данных (пользователи, колоды, игры, изображения в base64) в gzip-архиве JSON",
				"POST /api/import/all":          "Полная замена всех данных из gzip-архива JSON",
				"DELETE /api/games":             "Полная очистка игр и ходов",
				"GET /health":                   "Проверка состояния",
			},
		})
	})

	jwtSecret := getJWTSecret()

	// Вход — без авторизации (токена ещё нет).
	router.POST("/api/auth/login", handlers.Login)

	api := router.Group("/api")
	api.Use(middleware.BearerOrJWTAuth(apiToken, jwtSecret))
	{
		api.GET("/users", handlers.GetUsers)
		api.GET("/users/:id", handlers.GetUser)
		api.POST("/users", middleware.RequireAdmin(), handlers.CreateUser)
		api.PUT("/users/:id", middleware.RequireUser(), handlers.UpdateUser)
		api.DELETE("/users/:id", middleware.RequireAdmin(), handlers.DeleteUser)

		api.GET("/decks", handlers.GetDecks)
		api.GET("/decks/:id", handlers.GetDeck)
		api.POST("/decks", handlers.CreateDeck)
		api.PUT("/decks/:id", handlers.UpdateDeck)
		api.POST("/decks/:id/image", handlers.UploadDeckImage)
		api.DELETE("/decks/:id/image", handlers.DeleteDeckImage)
		api.DELETE("/decks/:id", handlers.DeleteDeck)

		api.POST("/games", handlers.CreateGame)
		api.GET("/games", handlers.GetGames)
		api.DELETE("/games", middleware.RequireAdmin(), handlers.ClearGamesAndTurns)
		api.GET("/games/active", handlers.GetActiveGame)
		api.GET("/games/:id", handlers.GetGame)
		api.PUT("/games/active", handlers.UpdateActiveGame)
		api.POST("/games/active/finish", handlers.FinishGame)

		api.GET("/stats/players", handlers.GetPlayerStats)
		api.GET("/stats/decks", handlers.GetDeckStats)

		api.GET("/export/all", middleware.RequireAdmin(), handlers.ExportAllData)
		api.POST("/import/all", middleware.RequireAdmin(), handlers.ImportAllData)
	}

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Не найден маршрут",
			"path":  c.Request.URL.Path,
			"hint":  "Используйте префикс /api, например POST /api/games",
		})
	})

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

	log.Printf("Сервер слушает порт %s (%s)", port, gin.Mode())
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Не удалось запустить сервер:", err)
	}
}
