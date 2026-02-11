// Package middleware — проверка Bearer-токена для защиты API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const bearerPrefix = "Bearer "

// BearerAuth возвращает middleware, проверяющий заголовок Authorization: Bearer <token>.
// Если apiToken пустой — авторизация отключена (все запросы проходят).
// Если apiToken задан — требуется точное совпадение токена.
func BearerAuth(apiToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiToken == "" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Отсутствует заголовок Authorization",
			})
			return
		}

		if !strings.HasPrefix(auth, bearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Используйте формат: Authorization: Bearer <token>",
			})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(auth, bearerPrefix))
		if token != apiToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Неверный токен",
			})
			return
		}

		c.Next()
	}
}
