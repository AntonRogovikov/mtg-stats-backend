// Package handlers — HTTP-обработчики API.
package handlers

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

// LoginRequest — запрос на вход.
type LoginRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse — ответ с JWT.
type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

const jwtExpireHours = 24 * 7 // 7 дней

// loginRateLimiter — 5 попыток в минуту с одного IP.
var loginRateLimiter = newLoginRateLimiter(5, time.Minute)

const loginLimiterCleanupThreshold = 1000
const loginLimiterMaxAge = 10 * time.Minute

func newLoginRateLimiter(r int, per time.Duration) *loginLimiter {
	return &loginLimiter{
		limiters:  make(map[string]*rate.Limiter),
		lastAccess: make(map[string]time.Time),
		rate:       rate.Every(per / time.Duration(r)),
		burst:      r,
	}
}

type loginLimiter struct {
	mu         sync.Mutex
	limiters   map[string]*rate.Limiter
	lastAccess map[string]time.Time
	rate       rate.Limit
	burst      int
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.limiters) > loginLimiterCleanupThreshold {
		cutoff := time.Now().Add(-loginLimiterMaxAge)
		for k, t := range l.lastAccess {
			if t.Before(cutoff) {
				delete(l.limiters, k)
				delete(l.lastAccess, k)
			}
		}
	}

	lim, ok := l.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(l.rate, l.burst)
		l.limiters[ip] = lim
	}
	l.lastAccess[ip] = time.Now()
	return lim.Allow()
}

// Login — вход по имени и паролю, возвращает JWT.
func Login(c *gin.Context) {
	ip := c.ClientIP()
	if !loginRateLimiter.allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много попыток входа. Подождите минуту."})
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}

	db := database.GetDB()
	var user models.User
	if err := db.Where("name = ?", strings.TrimSpace(req.Name)).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверное имя или пароль"})
		return
	}

	if user.PasswordHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "У пользователя не задан пароль"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверное имя или пароль"})
		return
	}

	secret := getJWTSecret()
	claims := models.UserClaims{
		UserID:  user.ID,
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpireHours * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать токен"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: tokenStr,
		User:  user,
	})
}

func getJWTSecret() string {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if s != "" {
		return s
	}
	return os.Getenv("API_TOKEN") // fallback
}
