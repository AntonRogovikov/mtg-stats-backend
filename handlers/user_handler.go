// Package handlers — HTTP-обработчики API (пользователи, колоды, игры, статистика).
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"mtg-stats-backend/database"
	"mtg-stats-backend/middleware"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// GetUsers — список пользователей, сортировка по id DESC.
func GetUsers(c *gin.Context) {
	db := database.GetDB()
	var users []models.User
	if err := db.Order("id DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список пользователей"})
		return
	}
	c.JSON(http.StatusOK, users)
}

// GetUser — пользователь по id; 404 если не найден.
func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID пользователя"})
		return
	}
	db := database.GetDB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// CreateUser — создание пользователя; только администратор; имя 2–100 символов.
func CreateUser(c *gin.Context) {
	var req models.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Имя от 2 до 100 символов"})
		return
	}
	user := models.User{Name: name}
	if req.Password != "" {
		hash, err := hashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обработать пароль"})
			return
		}
		user.PasswordHash = hash
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	db := database.GetDB()
	var existing models.User
	if err := db.Where("name = ?", name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Пользователь с таким именем уже существует"})
		return
	}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать пользователя"})
		return
	}
	c.JSON(http.StatusCreated, user)
}

// UpdateUser — обновление пользователя. Админ может менять любого; пользователь — только себя (имя, пароль). is_admin меняет только админ.
func UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID пользователя"})
		return
	}
	var req models.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Имя от 2 до 100 символов"})
		return
	}
	me, hasUser := middleware.GetUserInfo(c)
	db := database.GetDB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	isSelf := hasUser && me.ID == user.ID
	isAdmin := hasUser && me.IsAdmin

	if !isAdmin && !isSelf {
		c.JSON(http.StatusForbidden, gin.H{"error": "Можно изменять только свой профиль"})
		return
	}
	var existing models.User
	if err := db.Where("name = ? AND id != ?", name, id).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Пользователь с таким именем уже существует"})
		return
	}
	if req.IsAdmin != nil && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Только администратор может менять признак is_admin"})
		return
	}

	user.Name = name
	if req.Password != "" {
		hash, err := hashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обработать пароль"})
			return
		}
		user.PasswordHash = hash
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить пользователя"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// DeleteUser — удаление по id; только администратор; 404 если не найден.
func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID пользователя"})
		return
	}
	db := database.GetDB()
	result := db.Delete(&models.User{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить пользователя"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Пользователь удалён", "id": id})
}
