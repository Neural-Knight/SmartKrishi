package chat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/files"
	chathandler "github.com/smartkrishi/backend/internal/api/chat"
	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
	fileservice "github.com/smartkrishi/backend/internal/service/file"
)

// fakeFileStore implements files.Store without network (records uploads).
type fakeFileStore struct{ uploads int }

func (s *fakeFileStore) Upload(_ context.Context, _ string, _ []byte, mime, name string) (files.Uploaded, error) {
	s.uploads++
	return files.Uploaded{FileID: "files/fake-" + name, URI: "https://gen/fake", MIMEType: mime}, nil
}
func (s *fakeFileStore) Ask(_ context.Context, _, _, _, _, _, _ string) (string, error) {
	return "fake answer", nil
}

// routerWithAgentAndFiles wires chat + a fake agent runner + a real file service
// (backed by a fake Gemini store and a temp uploads dir) against the live pool.
func (e *testEnv) routerWithAgentAndFiles(t *testing.T, runner chathandler.AgentRunner) (*chi.Mux, *fakeFileStore) {
	t.Helper()
	authSvc := authservice.NewService(e.users, e.tokens)
	fileRepo := postgres.NewFileRepository(e.pool)
	chatRepo := postgres.NewChatRepository(e.pool).WithFiles(fileRepo)
	chatSvc := chatservice.NewService(chatRepo)
	store := &fakeFileStore{}
	fileSvc := fileservice.NewService(fileRepo, store, t.TempDir())
	handler := chathandler.NewHandler(chatSvc, authSvc).WithAgent(runner).WithFiles(fileSvc)
	r := chi.NewRouter()
	r.Route("/api/v1/chat", handler.Routes)
	return r, store
}

// buildMultipart builds a multipart body with a file part and extra form fields.
func buildMultipart(t *testing.T, fieldFile, filename string, content []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	fw, err := w.CreateFormFile(fieldFile, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_ = w.Close()
	return &body, w.FormDataContentType()
}

func TestUploadFile_JSON(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	router, store := e.routerWithAgentAndFiles(t, &fakeRunner{})

	// Create a chat to upload into.
	chat, err := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(context.Background(), userID, domain.CreateChatRequest{Title: "Files"})
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}

	body, ct := buildMultipart(t, "file", "soil.pdf", []byte("%PDF-1.4 fake"), map[string]string{"chat_id": chat.ID.String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/upload-file", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp domain.FileUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if resp.FileID == uuid.Nil || resp.OriginalFilename != "soil.pdf" || resp.FileType != "pdf" {
		t.Fatalf("bad response: %+v", resp)
	}
	if resp.ProcessingStatus != "processed" {
		t.Errorf("processing_status = %q, want processed", resp.ProcessingStatus)
	}
	if resp.AgentFileID == nil || !strings.Contains(*resp.AgentFileID, "fake") {
		t.Errorf("agent_file_id = %v, want fake gemini id", resp.AgentFileID)
	}
	if store.uploads != 1 {
		t.Errorf("gemini uploads = %d, want 1", store.uploads)
	}

	// DB row exists.
	var count int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM uploaded_files WHERE chat_id=$1`, chat.ID).Scan(&count); err != nil {
		t.Fatalf("count files: %v", err)
	}
	if count != 1 {
		t.Fatalf("uploaded_files rows = %d, want 1", count)
	}
}

func TestUploadFile_UnsupportedType(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	router, _ := e.routerWithAgentAndFiles(t, &fakeRunner{})
	chat, _ := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(context.Background(), userID, domain.CreateChatRequest{Title: "Files"})

	body, ct := buildMultipart(t, "file", "evil.exe", []byte("MZ"), map[string]string{"chat_id": chat.ID.String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/upload-file", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUploadFile_DocxRecordedFailed(t *testing.T) {
	// docx is an allowed upload type but NOT sent to Gemini in this build; it
	// should be stored with processing_status=failed + a user-visible message,
	// not silently dropped.
	e := setup(t)
	userID, token := e.createUser(t)
	router, store := e.routerWithAgentAndFiles(t, &fakeRunner{})
	chat, _ := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(context.Background(), userID, domain.CreateChatRequest{Title: "Files"})

	body, ct := buildMultipart(t, "file", "notes.docx", []byte("PK fake docx"), map[string]string{"chat_id": chat.ID.String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/upload-file", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp domain.FileUploadResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ProcessingStatus != "failed" {
		t.Errorf("docx processing_status = %q, want failed", resp.ProcessingStatus)
	}
	if store.uploads != 0 {
		t.Errorf("docx should NOT be uploaded to gemini, uploads = %d", store.uploads)
	}
}

func TestUploadAndAnalyzeStream_FileUploadedThenStream(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	runner := &fakeRunner{
		events: []agent.Event{
			agent.ResponseChunkEventValue("This soil report shows pH 6.5."),
			agent.ResponseEventValue("This soil report shows pH 6.5.", nil),
			agent.EndEventValue(),
		},
		draft: "This soil report shows pH 6.5.",
	}
	router, store := e.routerWithAgentAndFiles(t, runner)

	body, ct := buildMultipart(t, "file", "soil.pdf", []byte("%PDF fake"), map[string]string{"message": "analyze this soil report"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/upload-and-analyze-stream", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}

	events := parseSSEEvents(t, rec.Body.String())
	if len(events) == 0 {
		t.Fatal("no SSE events")
	}
	// First event must be file_uploaded with the LOCAL file id + chat id + message id.
	first := events[0]
	if first.Type != "file_uploaded" {
		t.Fatalf("first event = %s, want file_uploaded", first.Type)
	}
	if first.FileID == "" || first.Filename != "soil.pdf" {
		t.Fatalf("file_uploaded payload = %+v", first)
	}
	if _, err := uuid.Parse(first.FileID); err != nil {
		t.Errorf("file_id should be a local UUID, got %q", first.FileID)
	}
	if first.ChatID == "" || first.MessageID == "" {
		t.Errorf("file_uploaded missing chat_id/message_id: %+v", first)
	}
	// Then the normal stream: response_chunk … response … end.
	types := eventTypes(events)
	if types[len(types)-1] != "end" {
		t.Fatalf("last event = %s, want end (types=%v)", types[len(types)-1], types)
	}
	if store.uploads != 1 {
		t.Errorf("gemini uploads = %d, want 1", store.uploads)
	}

	// The uploaded file is persisted and linked to the USER message (Fix 1), so
	// it reloads on that message's files[] array.
	chatID := first.ChatID
	ctx := context.Background()
	var count int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM uploaded_files WHERE chat_id=$1`, chatID).Scan(&count)
	if count != 1 {
		t.Fatalf("uploaded_files rows = %d, want 1", count)
	}
	// message_id must be set to the user message (not NULL).
	var linkedMsg *string
	if err := e.pool.QueryRow(ctx, `SELECT message_id::text FROM uploaded_files WHERE chat_id=$1`, chatID).Scan(&linkedMsg); err != nil {
		t.Fatalf("select message_id: %v", err)
	}
	if linkedMsg == nil {
		t.Fatal("uploaded file message_id is NULL; should be linked to the user message (Fix 1)")
	}
	// The linked message must be the user message in this chat.
	var role string
	if err := e.pool.QueryRow(ctx, `SELECT role FROM chat_messages WHERE id=$1`, *linkedMsg).Scan(&role); err != nil {
		t.Fatalf("select linked message: %v", err)
	}
	if role != "user" {
		t.Errorf("file linked to a %q message, want user", role)
	}
}

// TestGetMessages_PopulatesFiles verifies that a file linked to a message is
// reloaded on that message's files[] array (like reasoning_steps), when the
// chat repo has a FileRepository wired.
func TestGetMessages_PopulatesFiles(t *testing.T) {
	e := setup(t)
	userID, _ := e.createUser(t)
	ctx := context.Background()

	fileRepo := postgres.NewFileRepository(e.pool)
	chatRepo := postgres.NewChatRepository(e.pool).WithFiles(fileRepo)

	chat, err := chatRepo.Create(ctx, userID, domain.CreateChatRequest{Title: "With files"})
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}
	msg, err := chatRepo.AddMessage(ctx, chat.ID, userID, "user", "see attached")
	if err != nil {
		t.Fatalf("add message: %v", err)
	}
	mid := msg.ID
	if _, err := fileRepo.Insert(ctx, domain.UploadedFileInput{
		UserID:           userID,
		ChatID:           chat.ID,
		MessageID:        &mid,
		OriginalFilename: "attached.pdf",
		FileType:         "pdf",
		FileSize:         123,
		ProcessingStatus: "processed",
	}); err != nil {
		t.Fatalf("insert file: %v", err)
	}

	msgs, err := chatRepo.GetMessages(ctx, chat.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	if len(msgs[0].Files) != 1 {
		t.Fatalf("message files = %d, want 1", len(msgs[0].Files))
	}
	if msgs[0].Files[0].OriginalFilename != "attached.pdf" {
		t.Errorf("file name = %q", msgs[0].Files[0].OriginalFilename)
	}
}

func TestListChatFiles(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	router, _ := e.routerWithAgentAndFiles(t, &fakeRunner{})
	ctx := context.Background()

	fileRepo := postgres.NewFileRepository(e.pool)
	chat, _ := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(ctx, userID, domain.CreateChatRequest{Title: "Files"})
	for _, name := range []string{"a.pdf", "b.png"} {
		if _, err := fileRepo.Insert(ctx, domain.UploadedFileInput{
			UserID: userID, ChatID: chat.ID, OriginalFilename: name, FileType: extOf(name),
			FileSize: 10, ProcessingStatus: "processed",
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/chats/"+chat.ID.String()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Files []domain.UploadedFile `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(resp.Files))
	}
}

func TestListChatFiles_OtherUsersChatIs404(t *testing.T) {
	e := setup(t)
	ownerID, _ := e.createUser(t)
	_, otherToken := e.createUser(t)
	router, _ := e.routerWithAgentAndFiles(t, &fakeRunner{})

	chat, _ := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(context.Background(), ownerID, domain.CreateChatRequest{Title: "Owner's"})

	// A different user requests the owner's chat files → 404.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/chats/"+chat.ID.String()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for another user's chat", rec.Code)
	}
}

func extOf(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return ""
}

func TestUploadAndAnalyzeStream_FileOnlyNoMessage(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router, _ := e.routerWithAgentAndFiles(t, &fakeRunner{})

	body, ct := buildMultipart(t, "file", "crop.png", []byte("\x89PNG fake"), map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/upload-and-analyze-stream", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	events := parseSSEEvents(t, rec.Body.String())
	types := eventTypes(events)
	// No message → file_uploaded then end (no analysis).
	if len(types) != 2 || types[0] != "file_uploaded" || types[1] != "end" {
		t.Fatalf("types = %v, want [file_uploaded end]", types)
	}
}
