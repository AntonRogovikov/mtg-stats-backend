package database

import (
	"fmt"
	"log"
	"os"

	"mtg-stats-backend/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB инициализирует подключение к базе данных
func InitDB() error {
	// Только для локальной разработки
	_ = godotenv.Load()

	// Получаем DSN строку
	dsn := getDSN()

	log.Println("Подключение к базе данных...")

	// Подключаемся через GORM
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	// Создаем таблицы
	err = DB.AutoMigrate(&models.User{})
	if err != nil {
		return fmt.Errorf("ошибка создания таблиц: %w", err)
	}

	log.Println("✅ База данных подключена")
	return nil
}

// getDSN возвращает строку подключения
func getDSN() string {
	// 1. Проверяем DATABASE_URL (Railway автоматически устанавливает)
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	// 2. Для локальной разработки
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_NAME", "mtg_stats"),
		getEnv("DB_SSLMODE", "disable"),
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetDB() *gorm.DB {
	return DB
}
