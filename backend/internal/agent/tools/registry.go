// Package tools implements the SmartKrishi agent tools (weather / market / soil /
// chat_history) plus the optional file tools.
//
// Each tool has a typed signature and the executor routes plan.Crop to market
// and plan.Location to weather/soil. chat_history reads from Postgres via the
// MessageReader interface (satisfied by the existing chat repository). The file
// tools (get_pdf_content, get_image_analysis, ...) are only available once
// WithFiles is wired; otherwise the executor skips them gracefully.
//
// All HTTP tools accept an injected *http.Client and Config so they are
// unit-testable against httptest servers with no real network.
package tools

import (
	"context"
	"net/http"
	"time"

	"github.com/smartkrishi/backend/internal/agent/files"
)

// Tool name constants match the identifiers the planner emits in
// Plan.ToolsNeeded.
const (
	NameWeather     = "weather_api"
	NameMarket      = "market_api"
	NameSoil        = "soil_api"
	NameChatHistory = "chat_history"

	// File tools: available only when the registry has WithFiles wired.
	NameGetPDFContent    = "get_pdf_content"
	NameAskAboutFiles    = "ask_question_about_files"
	NameGetImageAnalysis = "get_image_analysis"
	NameListFiles        = "list_uploaded_files"
	NameSearchFiles      = "search_user_files"
)

// FileToolNames is the set of file-related tools, used for the planner catalog.
// They become executable once WithFiles is wired.
var FileToolNames = map[string]struct{}{
	NameGetPDFContent:    {},
	NameGetImageAnalysis: {},
	NameListFiles:        {},
	NameSearchFiles:      {},
	NameAskAboutFiles:    {},
}

// IsDeferredFileTool reports whether name is a file tool.
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

	// File subsystem, wired via WithFiles. When both are set the file tools
	// become available; otherwise they are treated as unavailable and the
	// executor skips them gracefully.
	fileReader FileReader
	fileStore  files.Store
	agentModel string
}

// NewRegistry builds a Registry. If httpClient is nil a client with a sane
// timeout is used. reader may be nil, in which case the chat_history tool
// returns an error result rather than panicking (e.g. when no DB is configured).
func NewRegistry(httpClient *http.Client, cfg Config, reader MessageReader) *Registry {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Registry{http: httpClient, cfg: cfg, reader: reader}
}

// WithFiles enables the file tools (get_pdf_content, ask_question_about_files,
// get_image_analysis, list_uploaded_files, search_user_files). fileReader lists
// a chat's files from Postgres; store answers questions via the Gemini File API.
// agentModel is the model used for file Q&A. Returns the registry for chaining.
func (r *Registry) WithFiles(fileReader FileReader, store files.Store, agentModel string) *Registry {
	r.fileReader = fileReader
	r.fileStore = store
	r.agentModel = agentModel
	return r
}

// filesEnabled reports whether the file tools can run.
func (r *Registry) filesEnabled() bool {
	return r.fileReader != nil && r.fileStore != nil
}

// ToolInfo describes a tool for the agent-tools endpoint: its name, a short
// human description, and whether it is currently available in this deployment
// (file tools require the file subsystem to be wired).
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
}

// toolCatalog is the static description of every known tool, in a stable order.
var toolCatalog = []struct {
	name, desc string
}{
	{NameWeather, "Current weather conditions and forecast for a location."},
	{NameSoil, "Soil analysis: pH, nutrient levels, and recommendations."},
	{NameMarket, "Crop market prices and trends (Agmarknet)."},
	{NameChatHistory, "Prior conversation history for context."},
	{NameGetPDFContent, "Answer questions about an uploaded PDF."},
	{NameAskAboutFiles, "Answer questions about uploaded files."},
	{NameGetImageAnalysis, "Analyze an uploaded image (crop/plant/equipment)."},
	{NameListFiles, "List the chat's uploaded files."},
	{NameSearchFiles, "Search the chat's uploaded files by keyword."},
}

// AvailableTools returns the tool catalog with a per-tool availability flag,
// reflecting how this server is currently wired.
func (r *Registry) AvailableTools() []ToolInfo {
	out := make([]ToolInfo, 0, len(toolCatalog))
	for _, t := range toolCatalog {
		out = append(out, ToolInfo{Name: t.name, Description: t.desc, Available: r.Has(t.name)})
	}
	return out
}

// Has reports whether the registry can execute a tool by this name (i.e. it is
// an implemented tool). File tools are available only when WithFiles was called.
func (r *Registry) Has(name string) bool {
	switch name {
	case NameWeather, NameMarket, NameSoil, NameChatHistory:
		return true
	case NameGetPDFContent, NameAskAboutFiles, NameGetImageAnalysis, NameListFiles, NameSearchFiles:
		return r.filesEnabled()
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
