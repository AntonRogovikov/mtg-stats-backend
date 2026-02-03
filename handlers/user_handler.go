// Package handlers — HTTP-обработчики API (пользователи, колоды, игры, статистика).
package handlers

import (
	"net/http"
	"strconv"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

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

// CreateUser — создание пользователя; имя 2–100 символов.
func CreateUser(c *gin.Context) {
	var req models.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}
	if len(req.Name) < 2 || len(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Имя от 2 до 100 символов"})
		return
	}
	db := database.GetDB()
	user := models.User{Name: req.Name}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать пользователя"})
		return
	}
	c.JSON(http.StatusCreated, user)
}

// UpdateUser — обновление имени по id; 404 если не найден.
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
	if len(req.Name) < 2 || len(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Имя от 2 до 100 символов"})
		return
	}
	db := database.GetDB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	user.Name = req.Name
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить пользователя"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// DeleteUser — удаление по id; 404 если не найден.
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
