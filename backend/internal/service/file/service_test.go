package file_test

import (
	"errors"
	"testing"

	fileservice "github.com/smartkrishi/backend/internal/service/file"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		size    int64
		max     int64
		wantErr error
	}{
		{"ok pdf", "report.pdf", 1000, fileservice.MaxUploadFileBytes, nil},
		{"ok png", "photo.PNG", 500, fileservice.MaxUploadFileBytes, nil},
		{"ok csv", "data.csv", 10, fileservice.MaxUploadFileBytes, nil},
		{"empty", "x.pdf", 0, fileservice.MaxUploadFileBytes, fileservice.ErrEmpty},
		{"too large", "big.pdf", fileservice.MaxUploadFileBytes + 1, fileservice.MaxUploadFileBytes, fileservice.ErrTooLarge},
		{"bad type", "malware.exe", 10, fileservice.MaxUploadFileBytes, fileservice.ErrUnsupportedType},
		{"no ext", "noext", 10, fileservice.MaxUploadFileBytes, fileservice.ErrUnsupportedType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fileservice.Validate(tc.file, tc.size, tc.max)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestSizeLimits(t *testing.T) {
	if fileservice.MaxUploadFileBytes != 10<<20 {
		t.Errorf("upload-file limit = %d, want 10MB", fileservice.MaxUploadFileBytes)
	}
	if fileservice.MaxAnalyzeBytes != 20<<20 {
		t.Errorf("analyze limit = %d, want 20MB", fileservice.MaxAnalyzeBytes)
	}
}
