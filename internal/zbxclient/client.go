package zbxclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"zx/internal/config"
)

func normalizeRPCURL(rawURL string) string {
	rawURL = strings.TrimRight(rawURL, "/")
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL + "/api_jsonrpc.php"
	}
	if strings.HasSuffix(u.Path, ".php") {
		u.Path = path.Dir(u.Path)
		if u.Path == "/" || u.Path == "." {
			u.Path = ""
		}
	}
	u.Path = path.Join(u.Path, "api_jsonrpc.php")
	return u.String()
}

type Client struct {
	profile    *config.Profile
	httpClient *http.Client
	rpcURL     string
	auth       string
	authMu     sync.RWMutex
	apiVersion string
	isBearer   bool
	reqID      int
	reqIDMu    sync.Mutex
}

func NewClient(p *config.Profile, timeout time.Duration) *Client {
	rpcURL := normalizeRPCURL(p.URL)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !p.VerifySSL,
		},
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		profile: p,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
		rpcURL: rpcURL,
		auth:   p.Token,
	}
}

func (c *Client) Preflight(ctx context.Context) (int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.profile.URL, nil)
	if err != nil {
		return 0, 0, err
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		return 0, duration, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, duration, nil
}

func (c *Client) nextID() int {
	c.reqIDMu.Lock()
	defer c.reqIDMu.Unlock()
	c.reqID++
	return c.reqID
}

func (c *Client) Login(ctx context.Context) error {
	if c.profile.Token != "" {
		c.authMu.Lock()
		c.auth = c.profile.Token
		c.authMu.Unlock()
		return nil
	}

	if c.profile.User == "" {
		return nil
	}

	if c.profile.Name != "" {
		if cached := LoadSessionToken(c.profile.Name); cached != "" {
			c.authMu.Lock()
			c.auth = cached
			c.authMu.Unlock()
			return nil
		}
	}

	params := map[string]string{
		"username": c.profile.User,
		"password": c.profile.Password,
	}

	var token string
	err := c.rawCall(ctx, "user.login", params, "", &token)
	if err != nil {
		// Fallback for older Zabbix 5.x using "user" instead of "username"
		paramsOld := map[string]string{
			"user":     c.profile.User,
			"password": c.profile.Password,
		}
		if errOld := c.rawCall(ctx, "user.login", paramsOld, "", &token); errOld == nil {
			err = nil
		} else {
			return fmt.Errorf("zabbix login failed: %w", err)
		}
	}

	c.authMu.Lock()
	c.auth = token
	c.authMu.Unlock()

	if c.profile.Name != "" {
		_ = SaveSessionToken(c.profile.Name, token)
	}
	return nil
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "-32602") ||
		strings.Contains(msg, "Session terminated") ||
		strings.Contains(msg, "Not authorized") ||
		strings.Contains(msg, "re-login")
}

func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	c.authMu.RLock()
	currentAuth := c.auth
	c.authMu.RUnlock()

	if currentAuth == "" && c.profile.User != "" && method != "apiinfo.version" && method != "user.login" {
		if err := c.Login(ctx); err != nil {
			return err
		}
		c.authMu.RLock()
		currentAuth = c.auth
		c.authMu.RUnlock()
	}

	err := c.rawCall(ctx, method, params, currentAuth, result)
	if err != nil && c.profile.Name != "" && c.profile.User != "" && isAuthError(err) {
		_ = ClearSessionToken(c.profile.Name)
		c.authMu.Lock()
		c.auth = ""
		c.authMu.Unlock()

		if loginErr := c.Login(ctx); loginErr == nil {
			c.authMu.RLock()
			currentAuth = c.auth
			c.authMu.RUnlock()
			return c.rawCall(ctx, method, params, currentAuth, result)
		}
	}
	return err
}

func (c *Client) rawCall(ctx context.Context, method string, params any, auth string, result any) error {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID(),
	}

	// Legacy auth parameter in body for Zabbix 5.x
	if auth != "" && !c.isBearer && c.profile.Token == "" && method != "apiinfo.version" {
		reqBody.Auth = auth
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json-rpc")

	// Set Authorization header for static token or Zabbix 7.0+ bearer mode
	if c.profile.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.profile.Token)
	} else if auth != "" && c.isBearer && method != "apiinfo.version" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing rpc request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("zabbix rpc http status: %d", resp.StatusCode)
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decoding rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		// Auto-adapt for Zabbix 7.0+ which disallows "auth" parameter in body
		if !c.isBearer && auth != "" && strings.Contains(rpcResp.Error.Data, `unexpected parameter "auth"`) {
			c.isBearer = true
			return c.rawCall(ctx, method, params, auth, result)
		}
		return fmt.Errorf("zabbix api error (%d): %s - %s", rpcResp.Error.Code, rpcResp.Error.Message, rpcResp.Error.Data)
	}

	if result != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("unmarshaling rpc result: %w", err)
		}
	}

	return nil
}

// APIVersion returns the cached Zabbix API version from apiinfo.version.
func (c *Client) APIVersion(ctx context.Context) (string, error) {
	c.authMu.RLock()
	if c.apiVersion != "" {
		v := c.apiVersion
		c.authMu.RUnlock()
		return v, nil
	}
	c.authMu.RUnlock()

	var v string
	if err := c.Call(ctx, "apiinfo.version", map[string]any{}, &v); err != nil {
		return "", err
	}
	c.authMu.Lock()
	c.apiVersion = v
	c.authMu.Unlock()
	return v, nil
}
