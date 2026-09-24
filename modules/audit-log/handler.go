package audit_log

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
	GroupAuditLog(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupAuditLog(g *gin.RouterGroup) {
	g.GET("", h.List)
}

// List returns the audit trail. Operator-only: it is the record of who did what,
// including every actor's path, IP address and user agent, so it must not be
// readable by an ordinary authenticated caller. Fails closed with no allowlist.
func (h *Http) List(c *gin.Context) {
	if !middleware.IsOperator(c) {
		httplib.SetErrorResponse(c, http.StatusForbidden, primitive.MessageForbidden, nil)
		return
	}
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
