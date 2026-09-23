package discordbot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaasRevenueFailureNamesTheEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	bot := &Bot{revenue: server.Client(), revenueURL: server.URL, revenueAuth: "token"}
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

	bot := &Bot{revenue: server.Client(), revenueURL: server.URL, revenueAuth: "token"}
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
