package user

import (
	"net/http"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/infrastructure/middleware"
	"go-starter-kit/modules/primitive"

	"github.com/gin-gonic/gin"
)

type Http struct {
	service ServiceInterface
}

type HttpInterface interface {
	GroupUser(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupUser(g *gin.RouterGroup) {
	g.GET("", h.List)
	g.GET("/me", h.Me)
}

// List returns the user directory. Operator-only: it exposes every account's
// email address, and the HTTP surface has no other notion of an operator (there
// is no role column; admin is a Discord concept). The gate fails closed, so with
// no OPERATOR_USER_IDS configured this endpoint returns 403 to everyone rather
// than leaking the directory to any authenticated caller.
func (h *Http) List(c *gin.Context) {
	if !middleware.IsOperator(c) {
		httplib.SetErrorResponse(c, http.StatusForbidden, primitive.MessageForbidden, nil)
		return
	}
	query, err := httplib.GetCursorPaginationFromCtx(c)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, "invalid cursor", nil)
		return
	}
	data, err := h.service.ListUsers(c.Request.Context(), query.Limit, query.CreatedAt, query.ID)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusInternalServerError, "failed to list users", nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, data)
}

func (h *Http) Me(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		httplib.SetErrorResponse(c, http.StatusUnauthorized, primitive.MessageUnauthorized, nil)
		return
	}

	data, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusNotFound, primitive.MessageUserNotFound, nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, primitive.NewUserResponse(data))
}
