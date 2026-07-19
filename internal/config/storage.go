package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/releaseinfo"
)

const (
	defaultAttachmentMaxFileBytes  int64 = 50 << 20
	defaultAttachmentMaxTotalBytes int64 = 5 << 30
	defaultAttachmentUploadRate          = 12
)

type StorageConfig struct {
	Enabled           bool
	Endpoint          string
	Bucket            string
	AccessKeyID       string
	SecretAccessKey   string
	UploadTTL         time.Duration
	DownloadTTL       time.Duration
	TrashRetention    time.Duration
	MaximumFileSize   int64
	MaximumTotalBytes int64
	UploadRateLimit   int
	CleanupBatch      int
}

func LoadStorage() (StorageConfig, error) {
	environment := Environment(strings.ToLower(valueOrDefault("APP_ENV", string(EnvironmentLocal))))
	immutableRelease := environment == EnvironmentStaging || environment == EnvironmentProduction
	if err := releaseinfo.Current().Validate(immutableRelease); err != nil {
		return StorageConfig{}, fmt.Errorf("validate release identity: %w", err)
	}

	enabled, err := strconv.ParseBool(valueOrDefault("R2_ENABLED", "false"))
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse R2_ENABLED: %w", err)
	}
	uploadTTL, err := time.ParseDuration(valueOrDefault("ATTACHMENT_UPLOAD_TTL", "10m"))
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_UPLOAD_TTL: %w", err)
	}
	downloadTTL, err := time.ParseDuration(valueOrDefault("ATTACHMENT_DOWNLOAD_TTL", "5m"))
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_DOWNLOAD_TTL: %w", err)
	}
	trashRetention, err := time.ParseDuration(valueOrDefault("ATTACHMENT_TRASH_RETENTION", "168h"))
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_TRASH_RETENTION: %w", err)
	}
	maximumFileSize, err := strconv.ParseInt(valueOrDefault("ATTACHMENT_MAX_FILE_BYTES", strconv.FormatInt(defaultAttachmentMaxFileBytes, 10)), 10, 64)
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_MAX_FILE_BYTES: %w", err)
	}
	maximumTotalBytes, err := strconv.ParseInt(valueOrDefault("ATTACHMENT_MAX_TOTAL_BYTES", strconv.FormatInt(defaultAttachmentMaxTotalBytes, 10)), 10, 64)
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_MAX_TOTAL_BYTES: %w", err)
	}
	uploadRateLimit, err := strconv.Atoi(valueOrDefault("ATTACHMENT_UPLOAD_RATE_LIMIT", strconv.Itoa(defaultAttachmentUploadRate)))
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_UPLOAD_RATE_LIMIT: %w", err)
	}
	cleanupBatch, err := strconv.Atoi(valueOrDefault("ATTACHMENT_CLEANUP_BATCH", "100"))
	if err != nil {
		return StorageConfig{}, fmt.Errorf("parse ATTACHMENT_CLEANUP_BATCH: %w", err)
	}
	cfg := StorageConfig{
		Enabled:           enabled,
		Endpoint:          strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
		Bucket:            strings.TrimSpace(os.Getenv("R2_BUCKET")),
		AccessKeyID:       strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey:   strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		UploadTTL:         uploadTTL,
		DownloadTTL:       downloadTTL,
		TrashRetention:    trashRetention,
		MaximumFileSize:   maximumFileSize,
		MaximumTotalBytes: maximumTotalBytes,
		UploadRateLimit:   uploadRateLimit,
		CleanupBatch:      cleanupBatch,
	}
	if err := cfg.validate(); err != nil {
		return StorageConfig{}, err
	}
	return cfg, nil
}

func (cfg StorageConfig) validate() error {
	if cfg.UploadTTL <= 0 || cfg.UploadTTL > time.Hour {
		return errors.New("ATTACHMENT_UPLOAD_TTL must be between zero and one hour")
	}
	if cfg.DownloadTTL <= 0 || cfg.DownloadTTL > time.Hour {
		return errors.New("ATTACHMENT_DOWNLOAD_TTL must be between zero and one hour")
	}
	if cfg.TrashRetention < 24*time.Hour {
		return errors.New("ATTACHMENT_TRASH_RETENTION must be at least 24h")
	}
	if cfg.MaximumFileSize <= 0 {
		return errors.New("ATTACHMENT_MAX_FILE_BYTES must be positive")
	}
	if cfg.MaximumTotalBytes < cfg.MaximumFileSize {
		return errors.New("ATTACHMENT_MAX_TOTAL_BYTES must be at least ATTACHMENT_MAX_FILE_BYTES")
	}
	if cfg.UploadRateLimit < 1 || cfg.UploadRateLimit > 1000 {
		return errors.New("ATTACHMENT_UPLOAD_RATE_LIMIT must be between 1 and 1000")
	}
	if cfg.CleanupBatch <= 0 || cfg.CleanupBatch > 1000 {
		return errors.New("ATTACHMENT_CLEANUP_BATCH must be between 1 and 1000")
	}
	if !cfg.Enabled {
		return nil
	}
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return errors.New("R2 endpoint, bucket, access key, and secret are required when R2_ENABLED is true")
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || !endpoint.IsAbs() || endpoint.Host == "" {
		return errors.New("R2_ENDPOINT must be an absolute HTTP(S) URL")
	}
	if endpoint.Scheme != "https" && endpoint.Scheme != "http" {
		return errors.New("R2_ENDPOINT must use HTTP or HTTPS")
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("R2_ENDPOINT cannot contain credentials, query parameters, or a fragment")
	}
	if strings.Contains(cfg.Bucket, "/") {
		return errors.New("R2_BUCKET cannot contain a slash")
	}
	return nil
}
