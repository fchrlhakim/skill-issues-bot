package discordbot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// staticToken builds a manager holding a token that never expires, so tests
// exercise the revenue client without a rotation server.
func staticToken(t *testing.T) *saasToken {
	t.Helper()
	return newSaaSToken("", "", "", "", http.DefaultClient)
}

// testLogger matches production, where New always sets the logger. Without it a
// warning on the rotation path would panic the test rather than the process.
func testLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func TestSaasRevenueFailureNamesTheEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	token := staticToken(t)
	token.access = "token"
	bot := &Bot{revenue: server.Client(), revenueURL: server.URL, saas: token, log: testLogger()}
	_, err := bot.saasRevenue(context.Background())
	if err == nil {
		t.Fatal("expected an error on 401")
	}
	// A bare status code cannot tell a wrong token from a wrong URL, which is
	// exactly how a silent failure here burns an afternoon.
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), server.URL) {
		t.Fatalf("err = %q, want it to name both the status and the endpoint", err)
	}
}

func TestSaasRevenueFormatsNanoUSD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q, want the configured bearer", got)
		}
		_, _ = w.Write([]byte(`{"data":{"summary":{` +
			`"today_platform_fee_nano_usd":"1408","today_requests":5,` +
			`"month_platform_fee_nano_usd":"1408","month_requests":5,` +
			`"all_time_platform_fee_nano_usd":"1408","all_time_requests":5}}}`))
	}))
	defer server.Close()

	token := staticToken(t)
	token.access = "token"
	bot := &Bot{revenue: server.Client(), revenueURL: server.URL, saas: token, log: testLogger()}
	got, err := bot.saasRevenue(context.Background())
	if err != nil {
		t.Fatalf("saasRevenue error = %v", err)
	}
	for _, want := range []string{
		"SaaS platform fee today: $0.000001408 (5 requests)",
		"SaaS platform fee this month: $0.000001408 (5 requests)",
		"SaaS platform fee all time: $0.000001408 (5 requests)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("got %q, want it to contain %q", got, want)
		}
	}
}

func TestSaasRevenueUnconfiguredIsAnError(t *testing.T) {
	bot := &Bot{}
	if _, err := bot.saasRevenue(context.Background()); err == nil {
		t.Fatal("expected an error when the URL and token are empty")
	}
}

// A rejected access token must cost exactly one rotation and one retry, and the
// replacement must be the one that finally answers.
func TestSaasGetRotatesOnceAfterUnauthorized(t *testing.T) {
	var fetches, rotations int
	var mu sync.Mutex

	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		rotations++
		mu.Unlock()
		if r.Method != http.MethodPost {
			t.Errorf("refresh method = %s, want POST", r.Method)
		}
		_, _ = w.Write([]byte(`{"data":{"access_token":"fresh-access","refresh_token":"refresh-2","expires_at":` +
			itoa64(time.Now().Add(time.Hour).Unix()) + `}}`))
	}))
	defer auth.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetches++
		mu.Unlock()
		if r.Header.Get("Authorization") == "Bearer stale-access" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"summary":{"all_time_requests":7}}}`))
	}))
	defer api.Close()

	token := newSaaSToken(auth.URL, "stale-access", "refresh-1", filepath.Join(t.TempDir(), "saas-token.json"), api.Client())
	token.expiresAt = time.Now().Add(time.Hour)
	bot := &Bot{revenue: api.Client(), revenueURL: api.URL, saas: token, log: testLogger()}

	body, err := bot.saasGet(context.Background())
	if err != nil {
		t.Fatalf("saasGet error = %v", err)
	}
	if !strings.Contains(string(body), `"all_time_requests":7`) {
		t.Fatalf("body = %s, want the rotated response", body)
	}
	if fetches != 2 || rotations != 1 {
		t.Fatalf("fetches = %d, rotations = %d; want 2 and 1", fetches, rotations)
	}
}

// The persisted chain outranks the environment: replaying a rotated-away token
// would revoke the whole family.
func TestSaasTokenPrefersPersistedChain(t *testing.T) {
	file := filepath.Join(t.TempDir(), "saas-token.json")
	if err := os.WriteFile(file, []byte(`{"access_token":"persisted-access","refresh_token":"persisted-refresh"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	token := newSaaSToken("", "env-access", "env-refresh", file, http.DefaultClient)
	if token.refresh != "persisted-refresh" {
		t.Fatalf("refresh = %q, want the persisted chain", token.refresh)
	}
}

// A rotation must land on disk before it is used, so a restart cannot strand the
// bot on a token the SaaS already rotated away.
func TestSaasRotationPersistsBeforeUse(t *testing.T) {
	file := filepath.Join(t.TempDir(), "saas-token.json")
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"access_token":"` + jwtWithExp(time.Now().Add(time.Hour)) +
			`","refresh_token":"refresh-2","expires_at":` + itoa64(time.Now().Add(time.Hour).Unix()) + `}}`))
	}))
	defer auth.Close()

	token := newSaaSToken(auth.URL, "", "refresh-1", file, auth.Client())
	if _, err := token.Access(context.Background()); err != nil {
		t.Fatalf("Access error = %v", err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("the chain was not persisted: %v", err)
	}
	if !strings.Contains(string(raw), "refresh-2") {
		t.Fatalf("persisted state = %s, want the new refresh token", raw)
	}
}

func TestSaasTokenInvalidateForcesRotation(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"access_token":"fresh","refresh_token":"refresh-2","expires_at":` +
			itoa64(time.Now().Add(time.Hour).Unix()) + `}}`))
	}))
	defer auth.Close()

	token := newSaaSToken(auth.URL, "stale", "refresh-1", "", auth.Client())
	token.expiresAt = time.Now().Add(time.Hour)
	token.Invalidate()
	got, err := token.Access(context.Background())
	if err != nil {
		t.Fatalf("Access error = %v", err)
	}
	if got != "fresh" {
		t.Fatalf("token = %q, want a rotated one", got)
	}
}

func TestSaasTokenFallsBackToEnvWhenFileAbsent(t *testing.T) {
	token := newSaaSToken("", "env-access", "env-refresh", filepath.Join(t.TempDir(), "missing.json"), http.DefaultClient)
	if token.refresh != "env-refresh" {
		t.Fatalf("refresh = %q, want the environment value", token.refresh)
	}
}

// jwtWithExp mints an unsigned JWT carrying only an exp claim. The SaaS verifies
// signatures; this only has to drive the local rotation clock.
func jwtWithExp(exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]int64{"exp": exp.Unix()})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
