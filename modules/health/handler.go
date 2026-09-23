package health

import (
	"net/http"

	"go-starter-kit/infrastructure/httplib"

	"github.com/gin-gonic/gin"
)

type Http struct {
	service ServiceInterface
}

type HttpInterface interface {
	GroupHealth(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupHealth(g *gin.RouterGroup) {
	g.GET("/live", h.Live)
	g.GET("/ready", h.Ready)
}

func (h *Http) Live(c *gin.Context) {
	httplib.SetSuccessResponse(c, http.StatusOK, "alive", h.service.Live(c.Request.Context()))
}

func (h *Http) Ready(c *gin.Context) {
	if err := h.service.Ready(c.Request.Context()); err != nil {
		httplib.SetErrorResponse(c, http.StatusServiceUnavailable, "not ready", nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, "ready", gin.H{"database": "ok"})
}
