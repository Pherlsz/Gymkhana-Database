package config

import (
	"testing"
	"time"
)

func TestLoadStorageDefaultsToDisabledSafeConfiguration(t *testing.T) {
	clearStorageEnvironment(t)
	cfg, err := LoadStorage()
	if err != nil {
		t.Fatalf("LoadStorage() error = %v", err)
	}
	if cfg.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if cfg.UploadTTL != 10*time.Minute || cfg.DownloadTTL != 5*time.Minute {
		t.Fatalf("unexpected signing TTLs: upload=%s download=%s", cfg.UploadTTL, cfg.DownloadTTL)
	}
	if cfg.TrashRetention != 7*24*time.Hour {
		t.Fatalf("TrashRetention = %s", cfg.TrashRetention)
	}
	if cfg.MaximumFileSize != 50<<20 {
		t.Fatalf("MaximumFileSize = %d", cfg.MaximumFileSize)
	}
}

func TestLoadStorageRequiresPrivateR2SettingsWhenEnabled(t *testing.T) {
	clearStorageEnvironment(t)
	t.Setenv("R2_ENABLED", "true")
	if _, err := LoadStorage(); err == nil {
		t.Fatal("LoadStorage() error = nil, want required settings error")
	}
	t.Setenv("R2_ENDPOINT", "https://account.r2.cloudflarestorage.com")
	t.Setenv("R2_BUCKET", "private-files")
	t.Setenv("R2_ACCESS_KEY_ID", "access")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	cfg, err := LoadStorage()
	if err != nil {
		t.Fatalf("LoadStorage() error = %v", err)
	}
	if !cfg.Enabled || cfg.Bucket != "private-files" {
		t.Fatalf("configuration = %#v", cfg)
	}
}

func TestLoadStorageRejectsUnsafeOrUnboundedValues(t *testing.T) {
	clearStorageEnvironment(t)
	t.Setenv("ATTACHMENT_UPLOAD_TTL", "2h")
	if _, err := LoadStorage(); err == nil {
		t.Fatal("LoadStorage() error = nil, want upload TTL error")
	}
	clearStorageEnvironment(t)
	t.Setenv("ATTACHMENT_MAX_FILE_BYTES", "0")
	if _, err := LoadStorage(); err == nil {
		t.Fatal("LoadStorage() error = nil, want file-size error")
	}
	clearStorageEnvironment(t)
	t.Setenv("ATTACHMENT_CLEANUP_BATCH", "1001")
	if _, err := LoadStorage(); err == nil {
		t.Fatal("LoadStorage() error = nil, want cleanup batch error")
	}
}

func clearStorageEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"R2_ENABLED",
		"R2_ENDPOINT",
		"R2_BUCKET",
		"R2_ACCESS_KEY_ID",
		"R2_SECRET_ACCESS_KEY",
		"ATTACHMENT_UPLOAD_TTL",
		"ATTACHMENT_DOWNLOAD_TTL",
		"ATTACHMENT_TRASH_RETENTION",
		"ATTACHMENT_MAX_FILE_BYTES",
		"ATTACHMENT_CLEANUP_BATCH",
	} {
		t.Setenv(name, "")
	}
}
