package gemini

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/smartkrishi/backend/internal/agent/files"
)

// FileStore implements files.Store on top of the Gemini File API, reusing the
// provider's per-chat client pool so uploads and reads for a chat go through the
// same client (Gemini file access is client-scoped).
//
// This is the only document/image analysis path: files are uploaded via the
// File API and there is no vector store.
type FileStore struct {
	pool *clientPool
}

var _ files.Store = (*FileStore)(nil)

// FileStore returns a files.Store backed by this provider's client pool.
func (p *Provider) FileStore() *FileStore { return &FileStore{pool: p.pool} }

// Upload sends bytes to the Gemini File API via the chat's client and records
// the result in the registry (with localPath empty — the caller keeps the local
// copy; re-upload wiring can supply the path later).
func (s *FileStore) Upload(ctx context.Context, chatID string, data []byte, mime, displayName string) (files.Uploaded, error) {
	client, err := s.pool.get(ctx, chatID)
	if err != nil {
		return files.Uploaded{}, err
	}
	f, err := client.Files.Upload(ctx, bytes.NewReader(data), &genai.UploadFileConfig{
		MIMEType:    mime,
		DisplayName: displayName,
	})
	if err != nil {
		return files.Uploaded{}, fmt.Errorf("gemini: upload file: %w", err)
	}
	up := files.Uploaded{FileID: f.Name, URI: f.URI, MIMEType: f.MIMEType}
	s.pool.recordUpload(fileRegistryEntry{
		FileID:   f.Name,
		ChatID:   chatID,
		MIMEType: f.MIMEType,
		URI:      f.URI,
	})
	return up, nil
}

// Ask answers a question about an uploaded file by referencing it via its URI +
// mime and prompting the model.
func (s *FileStore) Ask(ctx context.Context, chatID, fileID, uri, mime, model, question string) (string, error) {
	client, err := s.pool.get(ctx, chatID)
	if err != nil {
		return "", err
	}
	if model == "" {
		model = "gemini-2.5-flash"
	}

	// Prefer the passed URI/mime; fall back to the registry, then to a live
	// Files.Get (handles a caller that only has the file id).
	if uri == "" || mime == "" {
		if e, ok := s.pool.lookup(fileID); ok {
			if uri == "" {
				uri = e.URI
			}
			if mime == "" {
				mime = e.MIMEType
			}
		}
	}
	if uri == "" {
		got, gErr := client.Files.Get(ctx, fileID, nil)
		if gErr != nil {
			return "", fmt.Errorf("gemini: get file %q: %w", fileID, gErr)
		}
		uri, mime = got.URI, got.MIMEType
	}

	prompt := "Based on the content of this document, answer the following question:\n\n" + question +
		"\n\nProvide a comprehensive answer based on the document content. If the information is not in the document, say so clearly."

	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromURI(uri, mime),
			genai.NewPartFromText(prompt),
		}, genai.Role(genai.RoleUser)),
	}

	resp, err := client.Models.GenerateContent(ctx, model, contents, nil)
	if err != nil {
		return "", fmt.Errorf("gemini: file Q&A: %w", err)
	}
	return resp.Text(), nil
}
