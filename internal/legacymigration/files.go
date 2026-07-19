package legacymigration

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read legacy manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode legacy manifest: %w", err)
	}
	return manifest, nil
}

func validateDigests(directory string, manifest Manifest, report *Report) error {
	fingerprint := sha256.New()
	for _, name := range bundleFiles {
		expected, exists := manifest.Files[name]
		if !exists {
			return fmt.Errorf("legacy manifest is missing %s", name)
		}
		file, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			return fmt.Errorf("open %s: %w", name, err)
		}
		digest := sha256.New()
		rows, copyErr := countLines(io.TeeReader(file, digest))
		closeErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf("hash %s: %w", name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", name, closeErr)
		}
		actualHash := hex.EncodeToString(digest.Sum(nil))
		if actualHash != expected.SHA256 || rows != expected.Rows {
			report.addError("manifest_file_mismatch", "file", name)
		}
		_, _ = fmt.Fprintf(fingerprint, "%s:%s\n", name, actualHash)
	}
	if hex.EncodeToString(fingerprint.Sum(nil)) != manifest.Fingerprint {
		report.addError("manifest_fingerprint_mismatch", "manifest", "manifest.json")
	}
	return nil
}

func countLines(reader io.Reader) (int, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func readJSONLines[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var values []T
	previousID := ""
	line := 0
	for scanner.Scan() {
		line++
		var value T
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			return nil, fmt.Errorf("decode %s line %d: %w", filepath.Base(path), line, err)
		}
		encoded, _ := json.Marshal(value)
		var identity struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(encoded, &identity)
		if identity.ID != "" && previousID != "" && identity.ID <= previousID {
			return nil, fmt.Errorf("%s is not strictly sorted by id at line %d", filepath.Base(path), line)
		}
		previousID = identity.ID
		values = append(values, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}
