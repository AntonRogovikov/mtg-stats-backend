// Package database — подключение к PostgreSQL, пул соединений и миграции GORM.
package database

import (
	"fmt"
	"log"
	"os"
	"strings"

	"mtg-stats-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB подключается по LOCAL_DSN (локально) или DATABASE_URL (production), проверяет Ping, настраивает пул и выполняет AutoMigrate.
func InitDB() error {
	dsn := os.Getenv("LOCAL_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return fmt.Errorf("не задан LOCAL_DSN или DATABASE_URL")
	}

	log.Printf("Подключение к БД: %s", maskPassword(dsn))

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("подключение к PostgreSQL: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("получение соединения: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("PostgreSQL не отвечает: %w", err)
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)

	if err := DB.AutoMigrate(&models.User{}, &models.Deck{}, &models.Game{}, &models.GamePlayer{}, &models.GameTurn{}); err != nil {
		return fmt.Errorf("миграции: %w", err)
	}

	log.Println("БД подключена, таблицы проверены")
	return nil
}

// maskPassword скрывает пароль в DSN для логов (URL и key=value форматы).
func maskPassword(dsn string) string {
	if strings.Contains(dsn, "://") {
		parts := strings.SplitN(dsn, "://", 2)
		if len(parts) == 2 {
			schema := parts[0]
			rest := parts[1]
			if strings.Contains(rest, "@") {
				userPass := strings.SplitN(rest, "@", 2)[0]
				return fmt.Sprintf("%s://%s:*****@%s", schema, strings.Split(userPass, ":")[0], strings.SplitN(rest, "@", 2)[1])
			}
		}
	}
	if strings.Contains(dsn, "password=") {
		parts := strings.Split(dsn, " ")
		masked := make([]string, 0, len(parts))
		for _, part := range parts {
			if strings.HasPrefix(part, "password=") {
				masked = append(masked, "password=*****")
			} else {
				masked = append(masked, part)
			}
		}
		return strings.Join(masked, " ")
	}
	return dsn
}

func GetDB() *gorm.DB {
	return DB
}
