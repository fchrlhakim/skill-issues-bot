package auth

import (
	"net/http"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/infrastructure/validator"
	"go-starter-kit/modules/primitive"

	"github.com/gin-gonic/gin"
)

type Http struct {
	service   ServiceInterface
	validator *validator.Validator
}

type HttpInterface interface {
	GroupAuth(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface, validator *validator.Validator) HttpInterface {
	return &Http{service: service, validator: validator}
}

func (h *Http) GroupAuth(g *gin.RouterGroup) {
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
}

func (h *Http) Register(c *gin.Context) {
	var req primitive.RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageValidationFailed, err.Error())
		return
	}

	data, err := h.service.Register(c.Request.Context(), primitive.RegisterUserInput{Name: req.Name, Email: req.Email, Password: req.Password})
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusConflict, "register failed", nil)
		return
	}
	httplib.SetCreatedResponse(c, primitive.MessageRegistered, primitive.NewUserResponse(data))
}

func (h *Http) Login(c *gin.Context) {
	var req primitive.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageValidationFailed, err.Error())
		return
	}

	data, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusUnauthorized, primitive.MessageInvalidCredential, nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageLoginSuccess, data)
}

func (h *Http) Refresh(c *gin.Context) {
	var req primitive.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageValidationFailed, err.Error())
		return
	}

	data, err := h.service.Refresh(c.Request.Context(), req)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusUnauthorized, primitive.MessageUnauthorized, nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageRefreshSuccess, data)
}

func (h *Http) Logout(c *gin.Context) {
	var req primitive.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageValidationFailed, err.Error())
		return
	}
	if err := h.service.Logout(c.Request.Context(), req); err != nil {
		httplib.SetErrorResponse(c, http.StatusUnauthorized, primitive.MessageUnauthorized, nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageLogoutSuccess, nil)
}
