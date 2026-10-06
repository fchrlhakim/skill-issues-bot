package health

import (
	"context"
	"time"
)

// GatewayProber reports the live state of the Discord gateway websocket.
// The bot process registers one at startup; a nil prober means the gateway
// is not part of this process's readiness (token or guild not configured).
type GatewayProber interface {
	GatewayStatus() (connected bool, lastAck time.Time)
}

type ServiceInterface interface {
	Live(ctx context.Context) map[string]string
	Ready(ctx context.Context) (map[string]string, error)
	// SetGateway attaches the Discord gateway prober. It must be called
	// before the HTTP server starts serving /health/ready.
	SetGateway(p GatewayProber)
}

type Service struct {
	repository RepositoryInterface
	gateway    GatewayProber
}

func NewService(repository RepositoryInterface) ServiceInterface {
	return &Service{repository: repository}
}

func (s *Service) SetGateway(p GatewayProber) {
	s.gateway = p
}

func (s *Service) Live(ctx context.Context) map[string]string {
	return map[string]string{"status": "ok"}
}

// Ready reports what is actually usable. The gateway gate exists because the
// bot once ran "healthy" for four days while its token was revoked: the
// healthcheck only proved that HTTP and the database were up.
// gatewayStaleAfter bounds how old the last heartbeat ACK may be. Discord
// asks for a heartbeat every ~41s and drops the connection after five
// missed ACKs, so three intervals is comfortably past one hiccup.
const gatewayStaleAfter = 3 * time.Minute

func (s *Service) Ready(ctx context.Context) (map[string]string, error) {
	status := map[string]string{"database": "ok"}
	if err := s.repository.PingDatabase(ctx); err != nil {
		status["database"] = err.Error()
		return status, err
	}
	if s.gateway == nil {
		// No bot configured in this process: readiness is HTTP+database only.
		status["discord_gateway"] = "disabled"
		return status, nil
	}
	connected, lastAck := s.gateway.GatewayStatus()
	switch {
	case !connected:
		status["discord_gateway"] = "disconnected"
		return status, errGatewayDown
	case time.Since(lastAck) > gatewayStaleAfter:
		status["discord_gateway"] = "stale"
		return status, errGatewayDown
	default:
		status["discord_gateway"] = "ok"
		return status, nil
	}
}
