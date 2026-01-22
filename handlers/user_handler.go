package handlers

import (
	"net/http"
	"strconv"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

// GetUsers возвращает всех пользователей
func GetUsers(c *gin.Context) {
	db := database.GetDB()
	var users []models.User

	// GORM делает всю работу за нас
	result := db.Order("id DESC").Find(&users)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch users",
		})
		return
	}

	c.JSON(http.StatusOK, users)
}

// GetUser возвращает пользователя по ID
func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	db := database.GetDB()
	var user models.User

	// Просто First - GORM сам добавит WHERE id = ?
	result := db.First(&user, id)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// CreateUser создает нового пользователя
func CreateUser(c *gin.Context) {
	var userReq models.UserRequest

	// Валидация входных данных
	if err := c.ShouldBindJSON(&userReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	// Проверяем длину имени
	if len(userReq.Name) < 2 || len(userReq.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name must be between 2 and 100 characters",
		})
		return
	}

	db := database.GetDB()

	// Создаем пользователя с помощью GORM
	user := models.User{
		Name: userReq.Name,
	}

	result := db.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create user",
		})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// UpdateUser обновляет пользователя
func UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	var userReq models.UserRequest

	if err := c.ShouldBindJSON(&userReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	// Проверяем длину имени
	if len(userReq.Name) < 2 || len(userReq.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name must be between 2 and 100 characters",
		})
		return
	}

	db := database.GetDB()

	// Проверяем, существует ли пользователь
	var existingUser models.User
	if err := db.First(&existingUser, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	// Обновляем пользователя
	existingUser.Name = userReq.Name
	result := db.Save(&existingUser)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update user",
		})
		return
	}

	c.JSON(http.StatusOK, existingUser)
}

// DeleteUser удаляет пользователя
func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	db := database.GetDB()

	// Удаляем пользователя с помощью GORM
	result := db.Delete(&models.User{}, id)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete user",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
		"id":      id,
	})
}
