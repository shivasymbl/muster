package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/db"
	"github.com/shivasymbl/muster/server/errs"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/responses"
)

func InitUsers(router *gin.RouterGroup) {
	usersRouter := router.Group("/users")

	usersRouter.GET("/:userId", getPublicUserProfile)
}

// @Summary Returns a minimal public user profile (safe for unauthenticated clients)
// @Tags users
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} models.User
// @Router /users/{userId} [get]
func getPublicUserProfile(c *gin.Context) {
	userId := c.Param("userId")
	user := db.GetUserById(userId)
	if user == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.UserDoesNotExist})
		return
	}

	public := models.User{
		Id:        user.Id,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Picture:   user.Picture,
	}
	c.JSON(http.StatusOK, public)
}
