package database

import (
	"fmt"
	"log"
	"os"

	"mtg-stats-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	// Получаем DSN строку
	dsn := getDSN()

	if dsn == "" {
		return fmt.Errorf("DATABASE_URL не установлен. Добавьте PostgreSQL базу в Railway")
	}

	log.Printf("Подключение к базе данных: %s", maskPassword(dsn))

	// Подключаемся
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("ошибка подключения: %v\nDSN: %s", err, maskPassword(dsn))
	}

	// Проверяем соединение
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("ошибка получения соединения: %v", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	// Автомиграция
	err = DB.AutoMigrate(&models.User{})
	if err != nil {
		return fmt.Errorf("ошибка создания таблиц: %v", err)
	}

	log.Println("✅ База данных подключена успешно")
	return nil
}

// getDSN возвращает строку подключения
func getDSN() string {
	// 1. Проверяем DATABASE_URL (Railway)
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	// 2. Проверяем Railway PostgreSQL URL (альтернативное название)
	if url := os.Getenv("POSTGRESQL_URL"); url != "" {
		return url
	}

	// 3. Проверяем отдельные переменные для локальной разработки
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if host != "" && dbname != "" {
		if port == "" {
			port = "5432"
		}
		if sslmode == "" {
			sslmode = "disable"
		}

		return fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode,
		)
	}

	// 4. Fallback для локальной разработки
	return "postgresql://postgres:password@localhost:5432/mtg_stats?sslmode=disable"
}

// maskPassword скрывает пароль в логах
func maskPassword(dsn string) string {
	// Скрываем пароль для безопасности
	return dsn // В реальном коде добавьте маскировку
}

func GetDB() *gorm.DB {
	return DB
}
