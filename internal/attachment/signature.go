package attachment

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const signaturePrefixBytes = 64 * 1024

func VerifyObject(reader io.Reader, declaredMIME string, expectedSize, maximumSize int64) (VerifiedObject, error) {
	declaredMIME = normalizeMIME(declaredMIME)
	if declaredMIME == "" {
		return VerifiedObject{}, ErrInvalidMIME
	}
	if expectedSize <= 0 || maximumSize <= 0 || expectedSize > maximumSize {
		return VerifiedObject{}, ErrInvalidSize
	}

	temporary, err := os.CreateTemp("", "gymkhana-attachment-*")
	if err != nil {
		return VerifiedObject{}, fmt.Errorf("create attachment verification file: %w", err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	defer temporary.Close()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(reader, maximumSize+1))
	if err != nil {
		return VerifiedObject{}, fmt.Errorf("stream attachment object: %w", err)
	}
	if written > maximumSize || written != expectedSize {
		return VerifiedObject{}, ErrInvalidSize
	}
	if err := temporary.Sync(); err != nil {
		return VerifiedObject{}, fmt.Errorf("sync attachment verification file: %w", err)
	}

	prefixSize := written
	if prefixSize > signaturePrefixBytes {
		prefixSize = signaturePrefixBytes
	}
	prefix := make([]byte, prefixSize)
	if _, err := temporary.ReadAt(prefix, 0); err != nil && err != io.EOF {
		return VerifiedObject{}, fmt.Errorf("read attachment signature: %w", err)
	}

	detected, err := detectMIME(path, prefix, declaredMIME)
	if err != nil {
		return VerifiedObject{}, err
	}
	if (detected == "text/plain" || detected == "text/csv") && !validTextFile(path) {
		return VerifiedObject{}, ErrUnsupportedFile
	}
	if !mimeCompatible(declaredMIME, detected) {
		return VerifiedObject{}, ErrMIMEMismatch
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return VerifiedObject{DetectedMIME: detected, ByteSize: written, SHA256: digest}, nil
}

func detectMIME(path string, prefix []byte, declaredMIME string) (string, error) {
	switch {
	case bytes.HasPrefix(prefix, []byte("%PDF-")):
		return "application/pdf", nil
	case len(prefix) >= 3 && prefix[0] == 0xff && prefix[1] == 0xd8 && prefix[2] == 0xff:
		return "image/jpeg", nil
	case bytes.HasPrefix(prefix, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "image/png", nil
	case len(prefix) >= 12 && bytes.Equal(prefix[0:4], []byte("RIFF")) && bytes.Equal(prefix[8:12], []byte("WEBP")):
		return "image/webp", nil
	case bytes.HasPrefix(prefix, []byte("GIF87a")) || bytes.HasPrefix(prefix, []byte("GIF89a")):
		return "image/gif", nil
	case len(prefix) >= 12 && bytes.Equal(prefix[0:4], []byte("RIFF")) && bytes.Equal(prefix[8:12], []byte("WAVE")):
		return "audio/wav", nil
	case bytes.HasPrefix(prefix, []byte("ID3")) || isMP3Frame(prefix):
		return "audio/mpeg", nil
	case len(prefix) >= 12 && bytes.Equal(prefix[4:8], []byte("ftyp")):
		return "video/mp4", nil
	case bytes.HasPrefix(prefix, []byte{0x1a, 0x45, 0xdf, 0xa3}):
		return "video/webm", nil
	case bytes.HasPrefix(prefix, []byte{'P', 'K', 0x03, 0x04}):
		return detectOfficeOpenXML(path)
	case validText(prefix):
		if declaredMIME == "text/csv" || declaredMIME == "application/csv" || declaredMIME == "application/vnd.ms-excel" {
			return "text/csv", nil
		}
		return "text/plain", nil
	default:
		return "", ErrUnsupportedFile
	}
}

func detectOfficeOpenXML(path string) (string, error) {
	archive, err := zip.OpenReader(filepath.Clean(path))
	if err != nil {
		return "", ErrUnsupportedFile
	}
	defer archive.Close()
	contentTypes := false
	word := false
	xl := false
	for _, file := range archive.File {
		name := strings.TrimPrefix(filepath.ToSlash(file.Name), "/")
		switch {
		case name == "[Content_Types].xml":
			contentTypes = true
		case strings.HasPrefix(name, "word/"):
			word = true
		case strings.HasPrefix(name, "xl/"):
			xl = true
		}
	}
	if !contentTypes {
		return "", ErrUnsupportedFile
	}
	if word && !xl {
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
	}
	if xl && !word {
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
	}
	return "", ErrUnsupportedFile
}

func validText(value []byte) bool {
	if len(value) == 0 || !utf8.Valid(value) || bytes.IndexByte(value, 0) >= 0 {
		return false
	}
	control := 0
	for _, current := range value {
		if current < 0x20 && current != '\n' && current != '\r' && current != '\t' {
			control++
		}
	}
	return control*100 <= len(value)
}

func validTextFile(path string) bool {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return false
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64*1024)
	total := 0
	control := 0
	for {
		current, size, err := reader.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil || current == utf8.RuneError && size == 1 || current == 0 {
			return false
		}
		total++
		if current < 0x20 && current != '\n' && current != '\r' && current != '\t' {
			control++
		}
	}
	return total > 0 && control*100 <= total
}

func isMP3Frame(value []byte) bool {
	return len(value) >= 2 && value[0] == 0xff && value[1]&0xe0 == 0xe0
}

func normalizeMIME(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if index := strings.IndexByte(value, ';'); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	return value
}

func mimeCompatible(declared, detected string) bool {
	declared = normalizeMIME(declared)
	aliases := map[string][]string{
		"application/pdf": {"application/pdf"},
		"image/jpeg":      {"image/jpeg", "image/jpg"},
		"image/png":       {"image/png"},
		"image/webp":      {"image/webp"},
		"image/gif":       {"image/gif"},
		"text/plain":      {"text/plain"},
		"text/csv":        {"text/csv", "application/csv", "application/vnd.ms-excel"},
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		},
		"audio/mpeg": {"audio/mpeg", "audio/mp3"},
		"audio/wav":  {"audio/wav", "audio/x-wav", "audio/wave"},
		"video/mp4":  {"video/mp4", "audio/mp4"},
		"video/webm": {"video/webm", "audio/webm"},
	}
	for _, candidate := range aliases[detected] {
		if declared == candidate {
			return true
		}
	}
	return false
}
