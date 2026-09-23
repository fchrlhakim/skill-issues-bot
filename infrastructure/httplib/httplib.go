package httplib

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type DefaultResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func SetSuccessResponse(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, DefaultResponse{Success: true, Message: message, Data: data})
}

func SetCreatedResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, DefaultResponse{Success: true, Message: message, Data: data})
}

func SetErrorResponse(c *gin.Context, status int, message string, details interface{}) {
	c.JSON(status, DefaultResponse{Success: false, Message: message, Error: details})
}
