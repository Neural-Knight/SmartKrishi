package gemini

import (
	"google.golang.org/genai"

	"github.com/smartkrishi/backend/internal/agent/llm"
)

// buildTools maps the provider-agnostic NativeTools toggles onto genai's
// server-side tool declarations. This is the Go equivalent of the Python
// `tools = [SEARCH, url_context, code_execution]` list in main.py's
// /ask_stream, but gated by explicit flags instead of always-on.
func buildTools(nt llm.NativeTools) []*genai.Tool {
	var tools []*genai.Tool
	if nt.GoogleSearch {
		tools = append(tools, &genai.Tool{GoogleSearch: &genai.GoogleSearch{}})
	}
	if nt.URLContext {
		tools = append(tools, &genai.Tool{URLContext: &genai.URLContext{}})
	}
	if nt.CodeExecution {
		tools = append(tools, &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}})
	}
	return tools
}

// buildConfig assembles a genai.GenerateContentConfig from llm.Opts. It wires
// system instruction, temperature, thinking (include_thoughts), native tools,
// and JSON response mode.
func buildConfig(opts llm.Opts) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{}

	if opts.System != "" {
		cfg.SystemInstruction = genai.NewContentFromText(opts.System, genai.Role(genai.RoleUser))
	}
	if opts.Temperature != nil {
		cfg.Temperature = opts.Temperature
	}
	if opts.Thinking {
		cfg.ThinkingConfig = &genai.ThinkingConfig{IncludeThoughts: true}
	}
	if tools := buildTools(opts.Tools); len(tools) > 0 {
		cfg.Tools = tools
	}
	if opts.JSON {
		cfg.ResponseMIMEType = "application/json"
	}
	return cfg
}

// buildContents converts an llm.Request into genai contents. When Messages is
// set it is used verbatim; otherwise Prompt becomes a single user message.
func buildContents(req llm.Request) []*genai.Content {
	if len(req.Messages) == 0 {
		return []*genai.Content{genai.NewContentFromText(req.Prompt, genai.Role(genai.RoleUser))}
	}
	contents := make([]*genai.Content, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := genai.Role(genai.RoleUser)
		if m.Role == llm.RoleModel {
			role = genai.Role(genai.RoleModel)
		}
		contents = append(contents, genai.NewContentFromText(m.Text, role))
	}
	return contents
}
