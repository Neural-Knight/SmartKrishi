// Package tools ports the SmartKrishi agent tools (weather / market / soil /
// chat_history) from Agentic-AI/app/tools into Go.
//
// Design notes vs the Python original:
//   - Per-tool argument fix: the Python main_agent called every tool as
//     fn(plan.location), so market_api received the *location* as its `crop`
//     argument (a bug). Here each tool has a typed signature and the executor
//     routes plan.Crop to market and plan.Location to weather/soil. Documented
//     as an intentional improvement in MIGRATION.md.
//   - chat_history reads from Postgres via the MessageReader interface
//     (satisfied by the existing chat repository), not the Python SQLite.
//   - File tools (get_pdf_content, get_image_analysis, ...) are deferred to
//     Step 9; the planner/executor skip unavailable file tools gracefully.
//
// All HTTP tools accept an injected *http.Client and Config so they are
// unit-testable against httptest servers with no real network.
package tools

import (
	"context"
	"net/http"
	"time"
)

// Tool name constants match the identifiers the planner emits in
// Plan.ToolsNeeded and the Python TOOLS registry keys.
const (
	NameWeather     = "weather_api"
	NameMarket      = "market_api"
	NameSoil        = "soil_api"
	NameChatHistory = "chat_history"
)

// FileToolNames are the file-related tools deferred to Step 9. The executor
// recognizes these as known-but-unavailable and skips them without error.
var FileToolNames = map[string]struct{}{
	"get_pdf_content":          {},
	"get_image_analysis":       {},
	"list_uploaded_files":      {},
	"search_user_files":        {},
	"ask_question_about_files": {},
}

// IsDeferredFileTool reports whether name is a Step 9 file tool.
func IsDeferredFileTool(name string) bool {
	_, ok := FileToolNames[name]
	return ok
}

// Config carries the external API credentials/endpoints the tools need. It is
// populated from internal/config. Base URLs are overridable so tests can point
// tools at an httptest server.
type Config struct {
	WeatherAPIKey  string
	WeatherBaseURL string // default weatherAPICurrentURL

	DataGovKey    string
	AgmarknetID   string
	MarketBaseURL string // default dataGovBaseURL (resource id appended)
}

// Registry holds the tool dependencies (HTTP client, config, chat message
// reader) and exposes the individual tools. Nodes/executor call the registry.
type Registry struct {
	http   *http.Client
	cfg    Config
	reader MessageReader
}

// NewRegistry builds a Registry. If httpClient is nil a client with a sane
// timeout is used (matching the Python 5s weather timeout order of magnitude).
// reader may be nil, in which case the chat_history tool returns an error
// result rather than panicking (e.g. when no DB is configured).
func NewRegistry(httpClient *http.Client, cfg Config, reader MessageReader) *Registry {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Registry{http: httpClient, cfg: cfg, reader: reader}
}

// Has reports whether the registry can execute a tool by this name (i.e. it is
// an implemented non-file tool).
func (r *Registry) Has(name string) bool {
	switch name {
	case NameWeather, NameMarket, NameSoil, NameChatHistory:
		return true
	default:
		return false
	}
}

// ChatHistoryArgs are the parameters for the chat_history tool.
type ChatHistoryArgs struct {
	Query  string
	UserID string
	ChatID string
	Limit  int
}

// call is a small helper to run an HTTP GET and decode JSON, honoring ctx.
func (r *Registry) getJSON(ctx context.Context, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpStatusError{Code: resp.StatusCode, URL: url}
	}
	return decodeJSON(resp.Body, into)
}
