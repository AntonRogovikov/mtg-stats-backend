// Package database — инициализация подключения к PostgreSQL через GORM.
// Настраивает пул (MaxIdleConns=5, MaxOpenConns=20) и выполняет AutoMigrate для User, Deck, Game, GamePlayer, GameTurn, AppSetting.
package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

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

	maxIdleConns := envInt("DB_MAX_IDLE_CONNS", 5)
	maxOpenConns := envInt("DB_MAX_OPEN_CONNS", 20)
	connMaxLifetimeMinutes := envInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)
	connMaxIdleTimeMinutes := envInt("DB_CONN_MAX_IDLE_TIME_MINUTES", 10)

	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(connMaxLifetimeMinutes) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(connMaxIdleTimeMinutes) * time.Minute)

	if err := DB.AutoMigrate(&models.User{}, &models.Deck{}, &models.Game{}, &models.GamePlayer{}, &models.GameTurn{}, &models.AppSetting{}, &models.Slice{}, &models.SliceDeck{}, &models.SlicePlayer{}); err != nil {
		return fmt.Errorf("миграции: %w", err)
	}

	if err := ensureDefaultSlice(DB); err != nil {
		return fmt.Errorf("сид глобального разреза: %w", err)
	}

	if err := syncSequences(DB); err != nil {
		return fmt.Errorf("синхронизация последовательностей: %w", err)
	}

	log.Println("БД подключена, таблицы проверены")
	return nil
}

// syncSequences выравнивает счётчики автонумерации по максимальному id в таблицах.
// Вставки с явным id (импорт бэкапа, восстановление дампа) счётчик не двигают,
// из-за чего следующий INSERT падает с duplicate key. Операция идемпотентна.
func syncSequences(db *gorm.DB) error {
	tables := []string{"users", "decks", "games", "game_players", "game_turns", "slices"}
	for _, table := range tables {
		query := fmt.Sprintf(
			"SELECT setval(pg_get_serial_sequence('%s','id'), GREATEST((SELECT COALESCE(MAX(id), 0) FROM %s), 1))",
			table, table,
		)
		if err := db.Exec(query).Error; err != nil {
			return fmt.Errorf("таблица %s: %w", table, err)
		}
	}
	return nil
}

// ensureDefaultSlice гарантирует существование глобального разреза (id=1)
// и привязывает к нему игры без разреза (например, созданные до миграции).
func ensureDefaultSlice(db *gorm.DB) error {
	var slice models.Slice
	err := db.First(&slice, 1).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		slice = models.Slice{ID: 1, Name: "Глобальный", IsDefault: true}
		if err := db.Create(&slice).Error; err != nil {
			return err
		}
		// Счётчик slices выровняет syncSequences сразу после сида.
		log.Println("Создан глобальный разрез (id=1)")
	} else if !slice.IsDefault {
		if err := db.Model(&slice).Update("is_default", true).Error; err != nil {
			return err
		}
	}
	return db.Exec("UPDATE games SET slice_id = 1 WHERE slice_id IS NULL OR slice_id = 0").Error
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

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		log.Printf("Некорректное значение %s=%q, использую %d", key, raw, fallback)
		return fallback
	}
	return v
}
