package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// AnalyzeImage runs a one-shot vision call: it sends the inline image bytes plus
// a text prompt and returns the model's text answer. It is stateless and uses
// the shared default client (no chat scoping needed), so it is suited to the
// non-persistent image-analysis endpoints.
func (p *Provider) AnalyzeImage(ctx context.Context, model, prompt string, image []byte, mime string) (string, error) {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	client, err := p.pool.get(ctx, "")
	if err != nil {
		return "", err
	}
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromBytes(image, mime),
			genai.NewPartFromText(prompt),
		}, genai.Role(genai.RoleUser)),
	}
	resp, err := client.Models.GenerateContent(ctx, model, contents, nil)
	if err != nil {
		return "", fmt.Errorf("gemini: analyze image: %w", err)
	}
	return resp.Text(), nil
}
