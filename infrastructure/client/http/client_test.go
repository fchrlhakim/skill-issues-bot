package httpclient

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewRequestRejectsPrivateIPLiteral(t *testing.T) {
	client := New(time.Second)

	_, err := client.NewRequest(context.Background(), http.MethodGet, "http://127.0.0.1/admin", nil)
	if err == nil {
		t.Fatal("expected private IP literal to be rejected")
	}
}

func TestNewRequestAllowsConfiguredHost(t *testing.T) {
	client := New(time.Second, WithAllowedHosts("api.example.com"))

	req, err := client.NewRequest(context.Background(), http.MethodPost, "https://api.example.com/v1", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if req.URL.Hostname() != "api.example.com" {
		t.Fatalf("unexpected hostname %q", req.URL.Hostname())
	}
}

func TestNewRequestRejectsHostOutsideAllowlist(t *testing.T) {
	client := New(time.Second, WithAllowedHosts("api.example.com"))

	_, err := client.NewRequest(context.Background(), http.MethodGet, "https://evil.example.com", nil)
	if err == nil {
		t.Fatal("expected host outside allowlist to be rejected")
	}
}

func TestDoRejectsManuallyConstructedUnsafeRequest(t *testing.T) {
	client := New(time.Second)
	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}

	resp, err := client.Do(context.Background(), req)
	if err == nil {
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		t.Fatal("expected unsafe request to be rejected")
	}
}

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip      string
		private bool
	}{
		{ip: "127.0.0.1", private: true},
		{ip: "10.0.0.1", private: true},
		{ip: "172.16.0.1", private: true},
		{ip: "192.168.1.1", private: true},
		{ip: "169.254.169.254", private: true},
		{ip: "::1", private: true},
		{ip: "8.8.8.8", private: false},
	}

	for _, tc := range cases {
		got := isPrivateIP(net.ParseIP(tc.ip))
		if got != tc.private {
			t.Fatalf("isPrivateIP(%s) = %v, want %v", tc.ip, got, tc.private)
		}
	}
}
