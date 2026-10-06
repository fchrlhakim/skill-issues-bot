package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRepo struct{ err error }

func (s stubRepo) PingDatabase(ctx context.Context) error { return s.err }

type stubGateway struct {
	connected bool
	lastAck   time.Time
}

func (s stubGateway) GatewayStatus() (bool, time.Time) { return s.connected, s.lastAck }

func TestReadyWithoutGatewayIsOK(t *testing.T) {
	svc := NewService(stubRepo{})
	status, err := svc.Ready(context.Background())
	if err != nil {
		t.Fatalf("no bot configured must be ready, got err=%v", err)
	}
	if status["discord_gateway"] != "disabled" {
		t.Fatalf("discord_gateway = %q, want disabled", status["discord_gateway"])
	}
}

func TestReadyFailsWhenGatewayDisconnected(t *testing.T) {
	svc := NewService(stubRepo{})
	svc.SetGateway(stubGateway{connected: false, lastAck: time.Now()})
	status, err := svc.Ready(context.Background())
	if !errors.Is(err, errGatewayDown) {
		t.Fatalf("disconnected gateway must fail readiness, got err=%v", err)
	}
	if status["discord_gateway"] != "disconnected" {
		t.Fatalf("discord_gateway = %q, want disconnected", status["discord_gateway"])
	}
}

func TestReadyFailsWhenHeartbeatStale(t *testing.T) {
	svc := NewService(stubRepo{})
	svc.SetGateway(stubGateway{connected: true, lastAck: time.Now().Add(-10 * time.Minute)})
	status, err := svc.Ready(context.Background())
	if !errors.Is(err, errGatewayDown) {
		t.Fatalf("stale heartbeat must fail readiness, got err=%v", err)
	}
	if status["discord_gateway"] != "stale" {
		t.Fatalf("discord_gateway = %q, want stale", status["discord_gateway"])
	}
}

func TestReadyOKWhenGatewayFresh(t *testing.T) {
	svc := NewService(stubRepo{})
	svc.SetGateway(stubGateway{connected: true, lastAck: time.Now()})
	status, err := svc.Ready(context.Background())
	if err != nil {
		t.Fatalf("fresh gateway must be ready, got err=%v", err)
	}
	if status["discord_gateway"] != "ok" || status["database"] != "ok" {
		t.Fatalf("status = %v, want database=ok discord_gateway=ok", status)
	}
}

func TestReadyReportsDatabaseError(t *testing.T) {
	dbErr := errors.New("connection refused")
	svc := NewService(stubRepo{err: dbErr})
	status, err := svc.Ready(context.Background())
	if !errors.Is(err, dbErr) {
		t.Fatalf("database error must fail readiness, got err=%v", err)
	}
	if status["database"] != dbErr.Error() {
		t.Fatalf("status[database] = %q, want %q", status["database"], dbErr.Error())
	}
}
