package audit_log

import (
	"net/http"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/modules/primitive"

	"github.com/gin-gonic/gin"
)

type Http struct {
	service ServiceInterface
}

type HttpInterface interface {
	GroupAuditLog(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupAuditLog(g *gin.RouterGroup) {
	g.GET("", h.List)
}

func (h *Http) List(c *gin.Context) {
	cursor, err := httplib.GetCursorPaginationFromCtx(c)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, "invalid cursor", nil)
		return
	}
	data, err := h.service.List(c.Request.Context(), primitive.ParameterFindAuditLog{
		ActorID:      c.Query("actor_id"),
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		ResourceID:   c.Query("resource_id"),
		Limit:        cursor.Limit + 1,
		ID:           cursor.ID,
	}, cursor.CreatedAt)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusInternalServerError, "failed to list audit logs", nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, data)
}
