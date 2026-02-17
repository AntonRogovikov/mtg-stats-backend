// Утилита для генерации bcrypt-хеша пароля (для ручного добавления в БД).
// Использование: go run ./cmd/hashpass <пароль>
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run ./cmd/hashpass <пароль>")
		fmt.Fprintln(os.Stderr, "Пример: go run ./cmd/hashpass admin123")
		os.Exit(1)
	}
	password := os.Args[1]
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
