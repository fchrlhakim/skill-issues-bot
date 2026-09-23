package membership

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
	GroupMembership(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface) HttpInterface {
	return &Http{service: service}
}

func (h *Http) GroupMembership(g *gin.RouterGroup) {
	g.POST("/verify", h.Verify)
	g.GET("/:discord_id", h.Get)
}

type verifyRequest struct {
	DiscordID      string   `json:"discord_id"`
	Roles          []string `json:"roles"`
	AccountAgeDays int      `json:"account_age_days"`
}

func (h *Http) Verify(c *gin.Context) {
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.DiscordID == "" {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	decision, err := h.service.Verify(c.Request.Context(), req.DiscordID, req.Roles, req.AccountAgeDays)
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusInternalServerError, "verify failed", nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, decision)
}

func (h *Http) Get(c *gin.Context) {
	member, err := h.service.Find(c.Request.Context(), c.Param("discord_id"))
	if err != nil {
		httplib.SetErrorResponse(c, http.StatusNotFound, "member not found", nil)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, member)
}
