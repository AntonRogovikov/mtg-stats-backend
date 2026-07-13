// Утилита смены пароля пользователя: хеширует пароль (bcrypt) и сразу
// обновляет его в БД.
//
// Использование:
//
//	go run ./cmd/setpass                                  # спросит пароль для «Антон Роговиков»
//	go run ./cmd/setpass -user "Имя Фамилия"              # спросит пароль для указанного игрока
//	go run ./cmd/setpass -user "Антон Роговиков" -password "секрет"
//
// DSN берётся из LOCAL_DSN или DATABASE_URL (как основной бэкенд). Если рядом
// есть файл .env — переменные подхватываются из него.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// loadDotEnv подхватывает KEY=VALUE из .env, не перетирая уже заданные переменные.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if _, ok := os.LookupEnv(key); !ok {
			os.Setenv(key, val)
		}
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	userName := flag.String("user", "Антон Роговиков", "имя пользователя")
	password := flag.String("password", "", "новый пароль (если пусто — спросит в консоли)")
	envPath := flag.String("env", ".env", "путь к .env с DSN (необязательно)")
	flag.Parse()

	loadDotEnv(*envPath)
	dsn := os.Getenv("LOCAL_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fail("Не задан LOCAL_DSN или DATABASE_URL (проверьте .env рядом с утилитой)")
	}

	pass := *password
	if pass == "" {
		fmt.Printf("Новый пароль для «%s»: ", *userName)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		pass = strings.TrimSpace(line)
	}
	if len([]rune(pass)) < 4 {
		fail("Пароль слишком короткий (минимум 4 символа)")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		fail("Ошибка хеширования: %v", err)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fail("Ошибка подключения к БД: %v", err)
	}

	res := db.Table("users").
		Where("name = ?", *userName).
		Updates(map[string]any{
			"password_hash": string(hash),
			"updated_at":    time.Now(),
		})
	if res.Error != nil {
		fail("Ошибка обновления: %v", res.Error)
	}
	if res.RowsAffected == 0 {
		fail("Пользователь «%s» не найден", *userName)
	}

	fmt.Printf("Пароль для «%s» обновлён.\n", *userName)
}
