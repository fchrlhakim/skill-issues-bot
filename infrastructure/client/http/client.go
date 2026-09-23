package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type Client interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
	NewRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error)
}

type HTTPClient struct {
	client       *http.Client
	maxRetries   int
	circuitOpen  atomic.Bool
	failureCount atomic.Int64
	allowedHosts map[string]struct{}
}

type Option func(*HTTPClient)

func WithAllowedHosts(hosts ...string) Option {
	return func(h *HTTPClient) {
		if h.allowedHosts == nil {
			h.allowedHosts = make(map[string]struct{}, len(hosts))
		}
		for _, host := range hosts {
			normalized := strings.ToLower(strings.TrimSpace(host))
			if normalized != "" {
				h.allowedHosts[normalized] = struct{}{}
			}
		}
	}
}

func New(timeout time.Duration, options ...Option) Client {
	client := &HTTPClient{client: &http.Client{Timeout: timeout}, maxRetries: 2}
	for _, option := range options {
		option(client)
	}
	return client
}

func (h *HTTPClient) NewRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	if err := h.validateURL(url); err != nil {
		return nil, err
	}
	return http.NewRequestWithContext(ctx, method, url, body)
}

func (h *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if h.circuitOpen.Load() {
		return nil, errors.New("circuit breaker open")
	}
	if req == nil || req.URL == nil {
		return nil, errors.New("request URL is required")
	}
	if err := h.validateURL(req.URL.String()); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		resp, err := h.client.Do(req.Clone(ctx)) // #nosec G704 -- URL is validated above for scheme, host allowlist, and private IP literals.
		if err == nil && resp.StatusCode < http.StatusInternalServerError {
			h.failureCount.Store(0)
			return resp, nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		lastErr = err
		if req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
		}
	}
	if h.failureCount.Add(1) >= 5 {
		h.circuitOpen.Store(true)
		go func() {
			time.Sleep(30 * time.Second)
			h.failureCount.Store(0)
			h.circuitOpen.Store(false)
		}()
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, lastErr
}

func (h *HTTPClient) validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("URL host is required")
	}
	if len(h.allowedHosts) > 0 {
		if _, ok := h.allowedHosts[host]; !ok {
			return fmt.Errorf("URL host is not allowed")
		}
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return fmt.Errorf("private IP URL host is not allowed")
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
