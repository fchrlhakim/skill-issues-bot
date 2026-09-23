package jwt

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceGenerateAndParseAccessToken(t *testing.T) {
	service := NewService(strings.Repeat("a", 32), time.Minute, time.Hour)
	userID := uuid.New()

	pair, err := service.GeneratePair(userID)
	if err != nil {
		t.Fatalf("GeneratePair error = %v", err)
	}

	claims, err := service.ParseAccessToken("Bearer " + pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken error = %v", err)
	}
	if claims.UserID != userID {
		t.Fatalf("UserID = %v, want %v", claims.UserID, userID)
	}
}

func TestServiceRejectsMalformedBearer(t *testing.T) {
	service := NewService(strings.Repeat("a", 32), time.Minute, time.Hour)
	if _, err := service.ParseAccessToken("bad"); err == nil {
		t.Fatal("expected malformed bearer to fail")
	}
}
