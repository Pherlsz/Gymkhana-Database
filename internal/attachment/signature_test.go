package attachment

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestVerifyObjectPreservesSizeHashAndDetectedMIME(t *testing.T) {
	payload := []byte("%PDF-1.7\nprivate document")
	verified, err := VerifyObject(bytes.NewReader(payload), "application/pdf", int64(len(payload)), 1024)
	if err != nil {
		t.Fatalf("VerifyObject() error = %v", err)
	}
	if verified.DetectedMIME != "application/pdf" {
		t.Fatalf("DetectedMIME = %q", verified.DetectedMIME)
	}
	if verified.ByteSize != int64(len(payload)) {
		t.Fatalf("ByteSize = %d", verified.ByteSize)
	}
	expected := sha256.Sum256(payload)
	if verified.SHA256 != expected {
		t.Fatalf("SHA256 = %x, want %x", verified.SHA256, expected)
	}
}

func TestVerifyObjectRejectsDeclaredMIMEMismatch(t *testing.T) {
	payload := []byte("%PDF-1.7\nprivate document")
	_, err := VerifyObject(bytes.NewReader(payload), "image/png", int64(len(payload)), 1024)
	if !errors.Is(err, ErrMIMEMismatch) {
		t.Fatalf("error = %v, want ErrMIMEMismatch", err)
	}
}

func TestVerifyObjectRejectsSizeMismatchAndOversize(t *testing.T) {
	payload := []byte("%PDF-1.7\nprivate document")
	if _, err := VerifyObject(bytes.NewReader(payload), "application/pdf", int64(len(payload)-1), 1024); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("size mismatch error = %v, want ErrInvalidSize", err)
	}
	if _, err := VerifyObject(bytes.NewReader(payload), "application/pdf", int64(len(payload)), 4); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("oversize error = %v, want ErrInvalidSize", err)
	}
}

func TestVerifyObjectValidatesCompleteTextPayload(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), signaturePrefixBytes+16)
	payload[len(payload)-1] = 0
	_, err := VerifyObject(bytes.NewReader(payload), "text/plain", int64(len(payload)), int64(len(payload)))
	if !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("error = %v, want ErrUnsupportedFile", err)
	}
}

func TestVerifyObjectDetectsDOCXContainer(t *testing.T) {
	var payload bytes.Buffer
	archive := zip.NewWriter(&payload)
	for name, body := range map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   "<document/>",
	} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	mime := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	verified, err := VerifyObject(bytes.NewReader(payload.Bytes()), mime, int64(payload.Len()), int64(payload.Len()))
	if err != nil {
		t.Fatalf("VerifyObject() error = %v", err)
	}
	if verified.DetectedMIME != mime {
		t.Fatalf("DetectedMIME = %q, want %q", verified.DetectedMIME, mime)
	}
}

func TestVerifyObjectAcceptsWebMDocTypeAndRejectsGenericEBML(t *testing.T) {
	webm := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x87, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'}
	verified, err := VerifyObject(bytes.NewReader(webm), "video/webm", int64(len(webm)), 1024)
	if err != nil || verified.DetectedMIME != "video/webm" {
		t.Fatalf("WebM VerifyObject() = %#v, %v", verified, err)
	}

	matroska := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x8b, 0x42, 0x82, 0x88, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a'}
	if _, err := VerifyObject(bytes.NewReader(matroska), "video/webm", int64(len(matroska)), 1024); !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("Matroska VerifyObject() error = %v, want ErrUnsupportedFile", err)
	}

	generic := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x80}
	if _, err := VerifyObject(bytes.NewReader(generic), "video/webm", int64(len(generic)), 1024); !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("generic EBML VerifyObject() error = %v, want ErrUnsupportedFile", err)
	}
}
