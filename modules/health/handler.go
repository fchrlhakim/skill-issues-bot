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
	// SetGateway attaches the Discord gateway prober so /health/ready can
	// gate on it. The bot process calls this before opening the session.
	SetGateway(p GatewayProber)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupHealth(g *gin.RouterGroup) {
	g.GET("/live", h.Live)
	g.GET("/ready", h.Ready)
}

// SetGateway forwards to the service. It lives on the handler because main
// only holds the Http layer, not the service behind it.
func (h *Http) SetGateway(p GatewayProber) {
	h.service.SetGateway(p)
}

func (h *Http) Live(c *gin.Context) {
	httplib.SetSuccessResponse(c, http.StatusOK, "alive", h.service.Live(c.Request.Context()))
}

func (h *Http) Ready(c *gin.Context) {
	status, err := h.service.Ready(c.Request.Context())
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusServiceUnavailable, "not ready", status)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, "ready", status)
}
