package attachment

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestR2PresignUploadUsesOpaquePrivateObjectPath(t *testing.T) {
	now := time.Date(2026, time.July, 17, 3, 0, 0, 0, time.UTC)
	store, err := NewR2Store(R2Options{
		Endpoint:        "https://account.r2.cloudflarestorage.com",
		Bucket:          "private-files",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewR2Store() error = %v", err)
	}
	signed, err := store.PresignUpload(context.Background(), "attachments/2026/07/object-id", 10*time.Minute)
	if err != nil {
		t.Fatalf("PresignUpload() error = %v", err)
	}
	if signed.Method != http.MethodPut {
		t.Fatalf("Method = %q, want PUT", signed.Method)
	}
	parsed, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Path != "/private-files/attachments/2026/07/object-id" {
		t.Fatalf("Path = %q", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || query.Get("X-Amz-Signature") == "" {
		t.Fatalf("missing SigV4 query: %v", query)
	}
	if query.Get("X-Amz-Expires") != "600" {
		t.Fatalf("X-Amz-Expires = %q", query.Get("X-Amz-Expires"))
	}
	if strings.Contains(signed.URL, "secret-key") {
		t.Fatal("presigned URL contains the secret access key")
	}
}

func TestR2PresignDownloadSetsResponseMetadata(t *testing.T) {
	store, err := NewR2Store(R2Options{
		Endpoint:        "https://account.r2.cloudflarestorage.com",
		Bucket:          "private-files",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		Now:             func() time.Time { return time.Date(2026, time.July, 17, 3, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewR2Store() error = %v", err)
	}
	signed, err := store.PresignDownload(context.Background(), "attachments/file", "Documento 01.pdf", "application/pdf", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignDownload() error = %v", err)
	}
	parsed, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Query().Get("response-content-type") != "application/pdf" {
		t.Fatalf("response-content-type = %q", parsed.Query().Get("response-content-type"))
	}
	if !strings.Contains(parsed.Query().Get("response-content-disposition"), "Documento%2001.pdf") {
		t.Fatalf("response-content-disposition = %q", parsed.Query().Get("response-content-disposition"))
	}
}

func TestR2OpenAndDeleteUseSignedPrivateRequests(t *testing.T) {
	const secret = "do-not-expose"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if strings.Contains(r.Header.Get("Authorization"), secret) {
			t.Error("Authorization header contains secret")
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, "%PDF-1.7\nbody")
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	store, err := NewR2Store(R2Options{
		Endpoint:        server.URL,
		Bucket:          "private-files",
		AccessKeyID:     "access-key",
		SecretAccessKey: secret,
		Client:          server.Client(),
		Now:             func() time.Time { return time.Date(2026, time.July, 17, 3, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewR2Store() error = %v", err)
	}
	reader, err := store.Open(context.Background(), "attachments/file")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if string(body) != "%PDF-1.7\nbody" {
		t.Fatalf("body = %q", body)
	}
	if err := store.Delete(context.Background(), "attachments/file"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
