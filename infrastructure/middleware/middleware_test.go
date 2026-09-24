package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go-starter-kit/infrastructure/limiter"
)

// A caller must not be able to choose its own rate-limit bucket. The header was
// read directly, so rotating X-Forwarded-For produced a fresh bucket per request
// and the limiter never engaged.
func TestForwardedHeaderCannotSpoofTheRateLimitKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	seen := make([]string, 0, 3)
	engine.GET("/x", RateLimiterMiddlewareWithPrefix(limiter.NewLocalRateLimiter(1, 1), "auth"), func(c *gin.Context) {
		seen = append(seen, clientKey(c))
		c.Status(http.StatusOK)
	})

	for _, spoofed := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		request := httptest.NewRequest(http.MethodGet, "/x", nil)
		request.RemoteAddr = "10.0.0.7:5555"
		request.Header.Set("X-Forwarded-For", spoofed)
		engine.ServeHTTP(httptest.NewRecorder(), request)
	}
	if len(seen) == 0 {
		t.Fatal("no requests reached the handler")
	}
	for _, key := range seen {
		if key != "ip:10.0.0.7" {
			t.Fatalf("client key %q followed the spoofed header; every request must key on the real peer", key)
		}
	}
}

// With no trusted proxies configured, a forwarded header must not change the key.
func TestClientKeyIgnoresForwardedHeaderWithoutTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	var got string
	engine.GET("/x", func(c *gin.Context) {
		got = clientKey(c)
		c.Status(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/x", nil)
	request.RemoteAddr = "192.0.2.9:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	engine.ServeHTTP(httptest.NewRecorder(), request)
	if got != "ip:192.0.2.9" {
		t.Fatalf("clientKey=%q, want the peer address", got)
	}
}
