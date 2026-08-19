package domain

import (
	"time"

	"github.com/google/uuid"
)

// Chat mirrors the Python `chats` table / Pydantic `Chat` schema.
type Chat struct {
	ID                  uuid.UUID     `json:"id"`
	UserID              int32         `json:"user_id"`
	Title               string        `json:"title"`
	AgentChatID         *string       `json:"agent_chat_id,omitempty"`
	IsFallbackChat      bool          `json:"is_fallback_chat"`
	FallbackPhoneNumber *string       `json:"fallback_phone_number"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
	IsDeleted           bool          `json:"is_deleted"`
	Messages            []ChatMessage `json:"messages"`
}

// ChatMessage mirrors the Python `chat_messages` table / Pydantic `ChatMessage` schema.
// ReasoningSteps and Files are always serialized as arrays (never null) for strict
// parity with the Python Pydantic model, which defaults both to []. ReasoningSteps
// is populated from the reasoning_steps table (Step 8); Files remains empty until
// the file-upload step (Step 9).
type ChatMessage struct {
	ID                  uuid.UUID       `json:"id"`
	ChatID              uuid.UUID       `json:"chat_id"`
	UserID              int32           `json:"user_id"`
	Role                string          `json:"role"`
	Content             string          `json:"content"`
	MessageType         string          `json:"message_type"`
	FileURL             *string         `json:"file_url"`
	IsEdited            bool            `json:"is_edited"`
	OriginalContent     *string         `json:"original_content"`
	FallbackType        *string         `json:"fallback_type"`
	FallbackPhoneNumber *string         `json:"fallback_phone_number"`
	CreatedAt           time.Time       `json:"created_at"`
	EditedAt            *time.Time      `json:"edited_at"`
	ReasoningSteps      []ReasoningStep `json:"reasoning_steps"`
	Files               []UploadedFile  `json:"files"`
}

// ReasoningStepInput is the write-side payload for persisting one reasoning
// step (Step 8). It mirrors the Python ReasoningStepCreate fields the streaming
// flow fills from each agent event.
type ReasoningStepInput struct {
	MessageID    uuid.UUID
	ChatID       uuid.UUID
	UserID       int32
	StepType     string
	StepOrder    int32
	Stage        *string
	Content      *string
	ToolName     *string
	ToolArgs     *string
	ToolResult   any // stored as JSONB
	StepMetadata any // stored as JSONB
}

// ReasoningStep is one persisted reasoning step returned with a message's
// reasoning_steps array (Step 8).
type ReasoningStep struct {
	ID           uuid.UUID `json:"id"`
	StepType     string    `json:"step_type"`
	StepOrder    int32     `json:"step_order"`
	Stage        *string   `json:"stage,omitempty"`
	Content      *string   `json:"content,omitempty"`
	ToolName     *string   `json:"tool_name,omitempty"`
	ToolArgs     *string   `json:"tool_args,omitempty"`
	ToolResult   any       `json:"tool_result,omitempty"`
	StepMetadata any       `json:"step_metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// UploadedFile mirrors the `uploaded_files` table / Python Pydantic UploadedFile
// (Step 9). Populated on a message's files array by GetMessages and returned by
// the file endpoints.
type UploadedFile struct {
	ID               uuid.UUID  `json:"id"`
	UserID           int32      `json:"user_id"`
	ChatID           uuid.UUID  `json:"chat_id"`
	MessageID        *uuid.UUID `json:"message_id,omitempty"`
	OriginalFilename string     `json:"original_filename"`
	FileType         string     `json:"file_type"`
	FileSize         int64      `json:"file_size"`
	MimeType         *string    `json:"mime_type,omitempty"`
	AgentFileID      *string    `json:"agent_file_id,omitempty"`
	ProcessingStatus string     `json:"processing_status"`
	Summary          *string    `json:"summary,omitempty"`
	FileMetadata     *string    `json:"file_metadata,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	IsDeleted        bool       `json:"is_deleted"`
}

// UploadedFileInput is the write payload for inserting an uploaded-file row.
type UploadedFileInput struct {
	UserID           int32
	ChatID           uuid.UUID
	MessageID        *uuid.UUID
	OriginalFilename string
	FileType         string
	FileSize         int64
	MimeType         *string
	AgentFileID      *string
	ProcessingStatus string
	Summary          *string
	FileMetadata     *string
}

// FileUploadResponse is the JSON body returned by POST /chat/upload-file
// (mirrors the Python FileUploadResponse).
type FileUploadResponse struct {
	FileID           uuid.UUID `json:"file_id"`
	OriginalFilename string    `json:"original_filename"`
	FileType         string    `json:"file_type"`
	FileSize         int64     `json:"file_size"`
	ProcessingStatus string    `json:"processing_status"`
	AgentFileID      *string   `json:"agent_file_id,omitempty"`
	Message          string    `json:"message"`
}

// ChatSummary mirrors the Python `ChatSummary` schema used by the chat list endpoint.
type ChatSummary struct {
	ID                  uuid.UUID `json:"id"`
	Title               string    `json:"title"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	LastMessage         string    `json:"last_message"`
	MessageCount        int64     `json:"message_count"`
	IsFallbackChat      bool      `json:"is_fallback_chat"`
	FallbackPhoneNumber *string   `json:"fallback_phone_number"`
}

// CreateChatRequest is the body for POST /chat/chats.
type CreateChatRequest struct {
	Title               string  `json:"title"`
	IsFallbackChat      bool    `json:"is_fallback_chat"`
	FallbackPhoneNumber *string `json:"fallback_phone_number"`
}

// UpdateChatRequest is the body for PUT /chat/chats/{id}.
type UpdateChatRequest struct {
	Title string `json:"title"`
}
