package ticket

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	GroupTicket(g *gin.RouterGroup)
}

func NewHttp(service ServiceInterface, validate *validator.Validator) HttpInterface {
	return &Http{service: service, validator: validate}
}

func (h *Http) GroupTicket(g *gin.RouterGroup) {
	g.GET("/types", h.Types)
	g.POST("", h.Create)
	g.GET("", h.Queue)
	g.GET("/mutasi", h.Mutasi)
	g.GET("/whoami", h.Whoami)
	g.GET("/:id", h.Get)
	g.POST("/:id/claim", h.Claim)
	g.POST("/:id/handoff", h.Handoff)
	g.POST("/:id/resolution", h.Resolution)
	g.POST("/:id/status", h.Status)
	g.POST("/:id/withdraw-approve", h.WithdrawApprove)
	g.POST("/:id/withdraw-paid", h.WithdrawPaid)
	g.POST("/:id/seller-approve", h.SellerApprove)
}

type actorRequest struct {
	ID              string   `json:"id" validate:"required"`
	Username        string   `json:"username"`
	Tag             string   `json:"tag"`
	Roles           []string `json:"roles"`
	IsAdmin         bool     `json:"is_admin"`
	OwnerID         string   `json:"owner_id"`
	AvailableAdmins []string `json:"available_admins"`
}

type createRequest struct {
	Actor    actorRequest      `json:"actor"`
	Type     string            `json:"type" validate:"required"`
	Data     map[string]string `json:"data"`
	ThreadID string            `json:"thread_id"`
}

type mutateRequest struct {
	Actor     actorRequest `json:"actor"`
	Reason    string       `json:"reason"`
	Note      string       `json:"note"`
	TargetID  string       `json:"target_id"`
	Status    string       `json:"status"`
	Reference string       `json:"reference"`
	Roles     []string     `json:"opener_roles"`
}

func (h *Http) Types(c *gin.Context) {
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, Types)
}

func (h *Http) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	rec, err := h.service.Create(c.Request.Context(), toActor(req.Actor), CreateInput{Type: req.Type, Data: req.Data, ThreadID: req.ThreadID})
	writeTicket(c, rec, err, http.StatusCreated)
}

func (h *Http) Get(c *gin.Context) {
	rec, err := h.service.Get(c.Request.Context(), c.Param("id"))
	writeTicket(c, rec, err, http.StatusOK)
}

func (h *Http) Queue(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	actor := Actor{ID: c.Query("actor_id"), IsAdmin: c.Query("is_admin") == "true", OwnerID: c.Query("owner_id")}
	data, err := h.service.Queue(c.Request.Context(), actor, page)
	if err != nil {
		writeTicket(c, Record{}, err, http.StatusOK)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, data)
}

func (h *Http) Claim(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.Claim(c.Request.Context(), actor, c.Param("id"))
	})
}

func (h *Http) Handoff(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.Handoff(c.Request.Context(), actor, c.Param("id"), req.TargetID, req.Reason)
	})
}

func (h *Http) Resolution(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.SetResolution(c.Request.Context(), actor, c.Param("id"), req.Note)
	})
}

func (h *Http) Status(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.Transition(c.Request.Context(), actor, c.Param("id"), req.Status)
	})
}

func (h *Http) WithdrawApprove(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.ApproveWithdrawal(c.Request.Context(), actor, c.Param("id"))
	})
}

func (h *Http) WithdrawPaid(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.RecordPayment(c.Request.Context(), actor, c.Param("id"), req.Reference)
	})
}

func (h *Http) SellerApprove(c *gin.Context) {
	h.mutate(c, func(actor Actor, req mutateRequest) (Record, error) {
		return h.service.ApproveSeller(c.Request.Context(), actor, c.Param("id"), req.Reason, req.Roles)
	})
}

func (h *Http) Mutasi(c *gin.Context) {
	data, err := h.service.Mutasi(c.Request.Context(), Actor{ID: c.Query("seller_id")})
	if err != nil {
		writeTicket(c, Record{}, err, http.StatusOK)
		return
	}
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, data)
}

func (h *Http) Whoami(c *gin.Context) {
	roles := strings.Split(c.Query("roles"), ",")
	httplib.SetSuccessResponse(c, http.StatusOK, primitive.MessageOK, h.service.Whoami(Actor{
		ID: c.Query("id"), Roles: roles, IsAdmin: c.Query("is_admin") == "true", OwnerID: c.Query("owner_id"),
	}))
}

func (h *Http) mutate(c *gin.Context, fn func(Actor, mutateRequest) (Record, error)) {
	var req mutateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httplib.SetErrorResponse(c, http.StatusBadRequest, primitive.MessageInvalidBody, nil)
		return
	}
	rec, err := fn(toActor(req.Actor), req)
	writeTicket(c, rec, err, http.StatusOK)
}

func toActor(req actorRequest) Actor {
	return Actor{ID: req.ID, Username: req.Username, Tag: req.Tag, Roles: req.Roles, IsAdmin: req.IsAdmin, OwnerID: req.OwnerID, AvailableAdmins: req.AvailableAdmins}
}

func writeTicket(c *gin.Context, rec Record, err error, okStatus int) {
	if err == nil {
		if okStatus == http.StatusCreated {
			httplib.SetCreatedResponse(c, primitive.MessageOK, rec)
			return
		}
		httplib.SetSuccessResponse(c, okStatus, primitive.MessageOK, rec)
		return
	}
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrNotAdmin), errors.Is(err, ErrOpenerRecusal):
		status = http.StatusForbidden
	}
	httplib.SetErrorResponse(c, status, err.Error(), nil)
}
