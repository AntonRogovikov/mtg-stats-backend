// Package middleware — JWT и проверка прав.
package middleware

import (
	"net/http"
	"strings"

	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const bearerPrefix = "Bearer "

// ContextKey — ключ для хранения пользователя в контексте.
type ContextKey string

const UserContextKey ContextKey = "user"

// UserInfo — данные пользователя из JWT (в контексте).
type UserInfo struct {
	ID      uint
	Name    string
	IsAdmin bool
}

// BearerOrJWTAuth — обязательная авторизация: API_TOKEN или JWT. API_TOKEN задаётся всегда; Bearer подтверждает запрос от приложения.
func BearerOrJWTAuth(apiToken string, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Пустой токен"})
			return
		}

		if apiToken != "" && token == apiToken {
			c.Set(string(UserContextKey), UserInfo{IsAdmin: true})
			c.Next()
			return
		}

		if jwtSecret != "" {
			var claims models.UserClaims
			tok, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err == nil && tok != nil && tok.Valid {
				c.Set(string(UserContextKey), UserInfo{
					ID:      claims.UserID,
					Name:    claims.Name,
					IsAdmin: claims.IsAdmin,
				})
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Неверный токен"})
	}
}

// RequireUser — требует, чтобы в контексте был пользователь (JWT).
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := c.Get(string(UserContextKey))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Требуется вход под учётной записью (JWT). Используйте POST /api/auth/login.",
			})
			return
		}
		c.Next()
	}
}

// RequireAdmin — требует, чтобы в контексте был пользователь с is_admin.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(string(UserContextKey))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Требуется вход под учётной записью пользователя (JWT). Используйте POST /api/auth/login.",
			})
			return
		}
		u := v.(UserInfo)
		if !u.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Требуются права администратора",
			})
			return
		}
		c.Next()
	}
}

// GetUserInfo возвращает пользователя из контекста (если есть).
func GetUserInfo(c *gin.Context) (UserInfo, bool) {
	v, ok := c.Get(string(UserContextKey))
	if !ok {
		return UserInfo{}, false
	}
	u, ok := v.(UserInfo)
	return u, ok
}
