package upload

import (
	"net/http"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/infrastructure/middleware"

	"github.com/gin-gonic/gin"
)

type Http struct {
	service ServiceInterface
}

type HttpInterface interface {
	GroupUpload(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupUpload(g *gin.RouterGroup) {
	g.POST("", h.Upload)
}

func (h *Http) Upload(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		httplib.SetErrorResponse(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, "file is required", nil)
		return
	}
	data, err := h.service.Upload(c.Request.Context(), userID, file)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httplib.SetCreatedResponse(c, "uploaded", data)
}
