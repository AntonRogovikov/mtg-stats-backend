// Package middleware — проверка HTTPS в production.
package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireHTTPS — в production (без LOCAL_DSN) отклоняет запросы без HTTPS.
// Учитывает X-Forwarded-Proto при работе за reverse proxy.
func RequireHTTPS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("LOCAL_DSN") != "" {
			c.Next()
			return
		}
		proto := c.GetHeader("X-Forwarded-Proto")
		if proto == "" {
			proto = "http"
			if c.Request.TLS != nil {
				proto = "https"
			}
		}
		if strings.ToLower(proto) != "https" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "В production требуется HTTPS",
			})
			return
		}
		c.Next()
	}
}
