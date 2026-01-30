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

func InitDB() error {
	log.Println("🚀 Инициализация базы данных...")

	// В Railway ДОЛЖНА быть эта переменная
	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		// Если нет DATABASE_URL - значит PostgreSQL не добавлен
		log.Println("⚠️ ВНИМАНИЕ: DATABASE_URL не найден!")
		log.Println("👉 Действия:")
		log.Println("1. В Railway Dashboard нажмите '+'")
		log.Println("2. Выберите 'Database' → 'PostgreSQL'")
		log.Println("3. Дождитесь создания")
		log.Println("4. Передеплойте приложение")
		return fmt.Errorf("PostgreSQL база не добавлена в Railway. Добавьте базу через интерфейс Railway")
	}

	// Логируем безопасную версию (без пароля)
	safeDSN := dsn
	if strings.Contains(safeDSN, "://") {
		parts := strings.SplitN(safeDSN, "://", 2)
		if len(parts) == 2 {
			schema := parts[0]
			rest := parts[1]
			if strings.Contains(rest, "@") {
				userPass := strings.SplitN(rest, "@", 2)[0]
				safeDSN = fmt.Sprintf("%s://%s:*****@%s", schema, strings.Split(userPass, ":")[0], strings.SplitN(rest, "@", 2)[1])
			}
		}
	}
	log.Printf("📡 Подключение к Railway PostgreSQL: %s", safeDSN)

	// Подключаемся
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("❌ Ошибка подключения к Railway PostgreSQL: %v", err)
	}

	// Проверяем соединение
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("❌ Ошибка получения соединения: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("❌ PostgreSQL не отвечает: %v", err)
	}

	// Настраиваем пул соединений
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)

	// Создаем таблицы
	if err := DB.AutoMigrate(&models.User{}, &models.Deck{}, &models.Game{}, &models.GamePlayer{}, &models.GameTurn{}); err != nil {
		return fmt.Errorf("❌ Ошибка создания таблиц: %v", err)
	}

	log.Println("✅ База данных Railway PostgreSQL подключена успешно!")
	log.Printf("✅ Таблицы users, decks, games созданы/проверены")
	return nil
}

func GetDB() *gorm.DB {
	return DB
}
