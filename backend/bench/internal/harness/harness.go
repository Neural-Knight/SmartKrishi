// Package harness wires a benchmark run together: it waits for a target server
// to be healthy, bootstraps an auth token, and orchestrates load + resource
// sampling into schema Results. It supports two modes:
//
//	e2e         - drives a live server (env SMARTKRISHI_BENCH_URL or a started
//	              subprocess) with real Gemini/Postgres/tool APIs.
//	controlled  - starts an in-process httptest server whose chat pipeline uses a
//	              scripted LLM and stubbed tool HTTP endpoints (real Postgres),
//	              isolating our own overhead from external variance.
package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WaitHealthy polls {baseURL}/health until it returns 200 or the deadline
// passes. Returns the time taken to first healthy response (cold-start proxy).
func WaitHealthy(ctx context.Context, client *http.Client, baseURL string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	deadline := start.Add(timeout)
	for {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
		resp, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(string(body), "healthy") {
				return time.Since(start), nil
			}
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("server at %s not healthy within %s", baseURL, timeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// BootstrapToken creates a fresh benchmark user and returns a Bearer JWT. It
// signs up a uniquely-named email user via POST {apiV1}/auth/signup; if signup
// fails because the user exists, it logs in instead.
func BootstrapToken(ctx context.Context, client *http.Client, baseURL, apiV1 string) (string, error) {
	email := fmt.Sprintf("bench_%d@smartkrishi.local", time.Now().UnixNano())
	const password = "bench-password-123456"

	signup := map[string]string{"name": "Bench User", "email": email, "password": password}
	if tok, err := postToken(ctx, client, baseURL+apiV1+"/auth/signup", signup); err == nil && tok != "" {
		return tok, nil
	}
	// Fall back to login (e.g. if a fixed bench user is pre-seeded).
	login := map[string]string{"email": email, "password": password}
	tok, err := postToken(ctx, client, baseURL+apiV1+"/auth/login", login)
	if err != nil {
		return "", fmt.Errorf("bootstrap token: signup and login both failed: %w", err)
	}
	return tok, nil
}

func postToken(ctx context.Context, client *http.Client, url string, body map[string]string) (string, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("no access_token in response")
	}
	return tr.AccessToken, nil
}

// FetchPoolMaxConns reads /_bench/pool (when SMARTKRISHI_BENCH=1 on the server)
// and returns max_conns, or 0 if unavailable.
func FetchPoolMaxConns(ctx context.Context, client *http.Client, baseURL string) int32 {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/_bench/pool", nil)
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var m struct {
		MaxConns int32 `json:"max_conns"`
	}
	if json.NewDecoder(resp.Body).Decode(&m) != nil {
		return 0
	}
	return m.MaxConns
}
