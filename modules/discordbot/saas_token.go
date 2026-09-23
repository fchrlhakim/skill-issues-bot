package discordbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// saasRefreshSkew renews slightly before the access token actually expires, so a
// request is never sent carrying a token that dies in flight.
const saasRefreshSkew = time.Minute

// saasToken owns the SaaS credential used by /revenue.
//
// The SaaS issues 15-minute access tokens and 30-day refresh tokens, and its
// rotation is strict: presenting a refresh token that was already rotated
// revokes the entire token family, logging the caller out for good. This type
// therefore keeps exactly one refresh token, serialises every rotation behind a
// mutex, and persists the replacement before it is used. Two rotations racing,
// or a restart replaying a stale token, would both be fatal.
type saasToken struct {
	authURL string
	file    string
	client  *http.Client

	mu        sync.Mutex
	access    string
	refresh   string
	expiresAt time.Time
}

// newSaaSToken builds the manager. An empty refresh token keeps the older
// static-token behaviour, so an existing deployment keeps working unchanged.
func newSaaSToken(authURL, access, refresh, file string, client *http.Client) *saasToken {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	token := &saasToken{authURL: authURL, file: file, client: client, access: access, refresh: refresh}
	if expiry, ok := jwtExpiry(access); ok {
		token.expiresAt = expiry
	}
	token.load()
	return token
}

// saasAuthURL resolves the refresh endpoint. SAAS_AUTH_URL wins; otherwise it is
// derived from the revenue URL, which points at the same SaaS host.
func saasAuthURL(revenueURL string) string {
	if override := strings.TrimSpace(os.Getenv("SAAS_AUTH_URL")); override != "" {
		return override
	}
	parsed, err := url.Parse(revenueURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/api/v1/auth/refresh"
}

// saasTokenFile is where the rotation chain lives. It defaults under the storage
// volume, which survives a container replace.
func saasTokenFile() string {
	if override := strings.TrimSpace(os.Getenv("SAAS_TOKEN_FILE")); override != "" {
		return override
	}
	return filepath.Join("storage", "saas-token.json")
}

// Access returns a usable access token, refreshing first when the cached one is
// missing or about to expire.
func (t *saasToken) Access(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.access != "" && t.freshLocked() {
		return t.access, nil
	}
	if t.refresh == "" {
		if t.access != "" {
			// A static token with no usable expiry: send it and let a 401 drive
			// the retry path.
			return t.access, nil
		}
		return "", fmt.Errorf("no SaaS credential configured")
	}
	if err := t.rotateLocked(ctx); err != nil {
		// A cached token that is still valid beats failing the command outright.
		if t.access != "" && time.Now().Before(t.expiresAt) {
			return t.access, nil
		}
		return "", err
	}
	return t.access, nil
}

// Invalidate drops the cached access token so the next Access rotates.
func (t *saasToken) Invalidate() {
	t.mu.Lock()
	t.expiresAt = time.Time{}
	t.mu.Unlock()
}

// canRotate reports whether a refresh token is available to rotate with.
func (t *saasToken) canRotate() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.refresh != ""
}

func (t *saasToken) freshLocked() bool {
	return !t.expiresAt.IsZero() && time.Now().Before(t.expiresAt.Add(-saasRefreshSkew))
}

func (t *saasToken) rotateLocked(ctx context.Context) error {
	if t.authURL == "" {
		return fmt.Errorf("cannot rotate: no SaaS auth URL")
	}
	payload, err := json.Marshal(map[string]string{"refresh_token": t.refresh})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.authURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh rejected with status %d from %s", res.StatusCode, t.authURL)
	}
	var body struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresAt    int64  `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return err
	}
	if body.Data.AccessToken == "" || body.Data.RefreshToken == "" {
		return fmt.Errorf("refresh returned an incomplete token pair")
	}
	t.access = body.Data.AccessToken
	t.refresh = body.Data.RefreshToken
	t.expiresAt = time.Unix(body.Data.ExpiresAt, 0)
	if expiry, ok := jwtExpiry(t.access); ok {
		t.expiresAt = expiry
	}
	return t.persist()
}

// persist writes the chain before the new token is used. A crash between the
// SaaS rotation and this write would strand the bot on a rotated-away token.
func (t *saasToken) persist() error {
	if t.file == "" {
		return nil
	}
	state, err := json.Marshal(struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		ExpiresAt    time.Time `json:"expires_at"`
	}{t.access, t.refresh, t.expiresAt})
	if err != nil {
		return err
	}
	if dir := filepath.Dir(t.file); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	tmp := t.file + ".tmp"
	if err := os.WriteFile(tmp, state, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.file)
}

// load adopts the persisted chain, which outranks the environment: the file
// holds the newest refresh token, and replaying an older one revokes the family.
func (t *saasToken) load() {
	if t.file == "" {
		return
	}
	raw, err := os.ReadFile(t.file)
	if err != nil {
		return
	}
	var state struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		ExpiresAt    time.Time `json:"expires_at"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return
	}
	if state.RefreshToken != "" {
		t.refresh = state.RefreshToken
	}
	if state.AccessToken != "" {
		t.access = state.AccessToken
		t.expiresAt = state.ExpiresAt
	}
}

// jwtExpiry reads the exp claim without verifying the signature: the SaaS
// validates it, and this only decides when to rotate.
func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}
