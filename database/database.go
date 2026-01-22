package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"mtg-stats-backend/models"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB инициализирует подключение к базе данных
func InitDB() error {
	_ = godotenv.Load()

	dbName := getEnv("DB_NAME", "mtg_stats")

	// 1. Сначала убедимся, что база существует
	if err := ensureDatabaseExists(dbName); err != nil {
		return err
	}

	// 2. Теперь подключаемся к базе
	dsn := getDSN(dbName)
	log.Printf("Подключение к базе '%s'...", dbName)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("не удалось подключиться: %w", err)
	}

	// 3. AutoMigrate создает таблицы
	log.Println("Создание/проверка таблиц...")
	err = DB.AutoMigrate(&models.User{})
	if err != nil {
		return fmt.Errorf("не удалось создать таблицы: %w", err)
	}

	log.Println("✅ База данных готова")
	return nil
}

// ensureDatabaseExists создает базу если её нет
func ensureDatabaseExists(dbName string) error {
	// Подключаемся к базе postgres (системная база)
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к PostgreSQL: %w", err)
	}
	defer db.Close()

	// Проверяем, существует ли база
	var exists bool
	err = db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbName,
	).Scan(&exists)

	if err != nil {
		return fmt.Errorf("ошибка проверки базы: %w", err)
	}

	if !exists {
		log.Printf("База '%s' не существует, создаем...", dbName)

		// Создаем базу
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return fmt.Errorf("не удалось создать базу '%s': %w", dbName, err)
		}

		log.Printf("✅ База '%s' создана", dbName)
	}

	return nil
}

// getDSN возвращает строку подключения
func getDSN(dbName string) string {
	// 1. Проверяем DATABASE_URL (Railway)
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		log.Println("Using DATABASE_URL from environment")
		return databaseURL
	}

	// 2. Проверяем отдельные переменные
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "")
	name := getEnv("DB_NAME", "mtg_stats")
	sslMode := getEnv("DB_SSLMODE", "disable")

	// 3. Формируем DSN
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, name, sslMode,
	)
}

// hidePasswordInDSN скрывает пароль в DSN для логов
func hidePasswordInDSN(dsn string) string {
	if strings.Contains(dsn, "password=") {
		parts := strings.Split(dsn, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "password=") {
				parts[i] = "password=*****"
				break
			}
		}
		return strings.Join(parts, " ")
	}
	return dsn
}

// getEnv возвращает значение переменной окружения
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDB возвращает экземпляр GORM
func GetDB() *gorm.DB {
	return DB
}

// CloseDB закрывает соединение с базой данных
func CloseDB() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			sqlDB.Close()
			log.Println("Database connection closed")
		}
	}
}
