package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent/files"
	"github.com/smartkrishi/backend/internal/agent/tools"
	"github.com/smartkrishi/backend/internal/domain"
)

// fakeFileReader implements tools.FileReader.
type fakeFileReader struct {
	list []domain.UploadedFile
	err  error
}

func (f *fakeFileReader) ListByChat(_ context.Context, _ uuid.UUID, _ int32) ([]domain.UploadedFile, error) {
	return f.list, f.err
}
func (f *fakeFileReader) GetByID(_ context.Context, id uuid.UUID, _ int32) (*domain.UploadedFile, error) {
	for i := range f.list {
		if f.list[i].ID == id {
			return &f.list[i], nil
		}
	}
	return nil, errors.New("not found")
}

// fakeStore implements files.Store.
type fakeStore struct {
	answer   string
	lastFile string
}

func (s *fakeStore) Upload(_ context.Context, _ string, _ []byte, _, _ string) (files.Uploaded, error) {
	return files.Uploaded{FileID: "files/x", URI: "u", MIMEType: "application/pdf"}, nil
}
func (s *fakeStore) Ask(_ context.Context, _, fileID, _, _, _, _ string) (string, error) {
	s.lastFile = fileID
	return s.answer, nil
}

func strp(s string) *string { return &s }

func filesRegistry(list []domain.UploadedFile, store files.Store) *tools.Registry {
	r := tools.NewRegistry(nil, tools.Config{}, nil)
	return r.WithFiles(&fakeFileReader{list: list}, store, "gemini-2.5-flash")
}

func TestFileTools_UnavailableWithoutWiring(t *testing.T) {
	r := tools.NewRegistry(nil, tools.Config{}, nil)
	if r.Has(tools.NameListFiles) {
		t.Error("file tools should be unavailable without WithFiles")
	}
	out := r.ListUploadedFiles(context.Background(), tools.FileToolArgs{UserID: "1", ChatID: uuid.NewString()})
	if out["error"] == nil {
		t.Error("expected error when file tools not wired")
	}
}

func TestFileTools_HasWhenWired(t *testing.T) {
	r := filesRegistry(nil, &fakeStore{})
	for _, n := range []string{tools.NameListFiles, tools.NameSearchFiles, tools.NameGetPDFContent, tools.NameAskAboutFiles, tools.NameGetImageAnalysis} {
		if !r.Has(n) {
			t.Errorf("Has(%q) = false, want true when wired", n)
		}
	}
}

func TestListUploadedFiles(t *testing.T) {
	chatID := uuid.New()
	list := []domain.UploadedFile{
		{ID: uuid.New(), OriginalFilename: "soil.pdf", FileType: "pdf", ProcessingStatus: "processed"},
		{ID: uuid.New(), OriginalFilename: "crop.png", FileType: "png", ProcessingStatus: "processed"},
	}
	r := filesRegistry(list, &fakeStore{})
	out := r.ListUploadedFiles(context.Background(), tools.FileToolArgs{UserID: "1", ChatID: chatID.String()})
	if out["type"] != "uploaded_files" || out["count"] != 2 {
		t.Fatalf("unexpected: %v", out)
	}
}

func TestSearchUserFiles(t *testing.T) {
	list := []domain.UploadedFile{
		{ID: uuid.New(), OriginalFilename: "wheat_report.pdf", Summary: strp("wheat yield analysis")},
		{ID: uuid.New(), OriginalFilename: "rice.csv", Summary: strp("rice irrigation data")},
	}
	r := filesRegistry(list, &fakeStore{})
	out := r.SearchUserFiles(context.Background(), tools.FileToolArgs{UserID: "1", ChatID: uuid.NewString(), Question: "wheat"})
	if out["type"] != "file_search_results" {
		t.Fatalf("type = %v", out["type"])
	}
	if out["count"] != 1 {
		t.Fatalf("count = %v, want 1 (only wheat match)", out["count"])
	}
}

func TestAskAboutFile_UsesLatestProcessed(t *testing.T) {
	store := &fakeStore{answer: "The soil pH is 6.5."}
	f1 := domain.UploadedFile{ID: uuid.New(), OriginalFilename: "old.pdf", ProcessingStatus: "processed", AgentFileID: strp("files/old"), MimeType: strp("application/pdf")}
	f2 := domain.UploadedFile{ID: uuid.New(), OriginalFilename: "new.pdf", ProcessingStatus: "processed", AgentFileID: strp("files/new"), MimeType: strp("application/pdf")}
	r := filesRegistry([]domain.UploadedFile{f1, f2}, store)

	out := r.AskAboutFile(context.Background(), tools.FileToolArgs{UserID: "1", ChatID: uuid.NewString(), Question: "what is the soil pH?"})
	if out["type"] != "file_answer" {
		t.Fatalf("type = %v (%v)", out["type"], out)
	}
	if out["answer"] != "The soil pH is 6.5." {
		t.Fatalf("answer = %v", out["answer"])
	}
	// Should have used the most-recent processed file.
	if store.lastFile != "files/new" {
		t.Errorf("asked about %q, want files/new (latest)", store.lastFile)
	}
}

func TestAskAboutFile_NoAnalyzableFile(t *testing.T) {
	// A file with no agent_file_id is not analyzable.
	list := []domain.UploadedFile{{ID: uuid.New(), OriginalFilename: "bad.docx", ProcessingStatus: "failed"}}
	r := filesRegistry(list, &fakeStore{})
	out := r.AskAboutFile(context.Background(), tools.FileToolArgs{UserID: "1", ChatID: uuid.NewString(), Question: "?"})
	if out["error"] == nil {
		t.Fatalf("expected error for no analyzable file, got %v", out)
	}
}

func TestAskAboutFile_SpecificFileNotAnalyzable(t *testing.T) {
	fid := uuid.New()
	list := []domain.UploadedFile{{ID: fid, OriginalFilename: "bad.docx", ProcessingStatus: "failed"}}
	r := filesRegistry(list, &fakeStore{})
	out := r.AskAboutFile(context.Background(), tools.FileToolArgs{UserID: "1", ChatID: uuid.NewString(), FileID: fid.String(), Question: "?"})
	if out["error"] == nil {
		t.Fatal("expected not-available error for a failed file")
	}
}
