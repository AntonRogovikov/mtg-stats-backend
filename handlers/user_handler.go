// Package handlers — HTTP-обработчики для пользователей, колод, игр и статистики.
package handlers

import (
	"net/http"
	"strconv"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

// GetUsers возвращает список всех пользователей (сортировка по id DESC).
func GetUsers(c *gin.Context) {
	db := database.GetDB()
	var users []models.User

	result := db.Order("id DESC").Find(&users)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось загрузить список пользователей",
		})
		return
	}

	c.JSON(http.StatusOK, users)
}

// GetUser возвращает одного пользователя по id (404 при отсутствии).
func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID пользователя",
		})
		return
	}

	db := database.GetDB()
	var user models.User

	result := db.First(&user, id)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Пользователь не найден",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// CreateUser создаёт пользователя по телу запроса (имя 2–100 символов).
func CreateUser(c *gin.Context) {
	var userReq models.UserRequest

	if err := c.ShouldBindJSON(&userReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Некорректные данные запроса",
			"details": err.Error(),
		})
		return
	}

	if len(userReq.Name) < 2 || len(userReq.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Имя должно быть от 2 до 100 символов",
		})
		return
	}

	db := database.GetDB()

	user := models.User{
		Name: userReq.Name,
	}

	result := db.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось создать пользователя",
		})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// UpdateUser обновляет имя пользователя по id (404 при отсутствии).
func UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID пользователя",
		})
		return
	}

	var userReq models.UserRequest

	if err := c.ShouldBindJSON(&userReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Некорректные данные запроса",
			"details": err.Error(),
		})
		return
	}

	if len(userReq.Name) < 2 || len(userReq.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Имя должно быть от 2 до 100 символов",
		})
		return
	}

	db := database.GetDB()

	var existingUser models.User
	if err := db.First(&existingUser, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Пользователь не найден",
		})
		return
	}

	existingUser.Name = userReq.Name
	result := db.Save(&existingUser)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось обновить пользователя",
		})
		return
	}

	c.JSON(http.StatusOK, existingUser)
}

// DeleteUser удаляет пользователя по id (404 если не найден).
func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID пользователя",
		})
		return
	}

	db := database.GetDB()

	result := db.Delete(&models.User{}, id)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось удалить пользователя",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Пользователь не найден",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Пользователь успешно удалён",
		"id":      id,
	})
}
