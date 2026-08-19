// Package files defines a provider-agnostic abstraction for uploading files to
// an LLM's file API and asking questions about them. The single production
// implementation is Gemini's File API (internal/agent/gemini); tests use a fake.
//
// The Gemini File API is the only document/image analysis path: files are
// uploaded via the File API and there is no vector store. File search is
// keyword search over Postgres file summaries/filenames.
package files

import "context"

// Uploaded is the result of uploading a file to the LLM file API.
type Uploaded struct {
	// FileID is the provider's file identifier (e.g. Gemini "files/abc123").
	// Persisted as uploaded_files.agent_file_id.
	FileID string
	// URI is the provider file URI, used to reference the file in generation.
	URI string
	// MIMEType as understood by the provider.
	MIMEType string
}

// Store uploads files to the LLM file API (per chat, for access isolation) and
// answers questions about a previously uploaded file. All methods are scoped by
// chatID so a chat's files are read back through the same client (Gemini's file
// access is client-scoped).
type Store interface {
	// Upload sends the file bytes to the provider and returns its handle. mime
	// is the detected MIME type; displayName is the original filename.
	Upload(ctx context.Context, chatID string, data []byte, mime, displayName string) (Uploaded, error)

	// Ask answers a question about an already-uploaded file, referenced by its
	// provider fileID + URI + mime, using the given model. Returns the answer
	// text. Used by the get_pdf_content / get_image_analysis /
	// ask_question_about_files tools.
	Ask(ctx context.Context, chatID, fileID, uri, mime, model, question string) (string, error)
}
