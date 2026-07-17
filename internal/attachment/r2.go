package attachment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	r2Region          = "auto"
	r2Service         = "s3"
	unsignedPayload   = "UNSIGNED-PAYLOAD"
	maximumPresignTTL = 7 * 24 * time.Hour
)

type R2Options struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Client          *http.Client
	Now             func() time.Time
}

type R2Store struct {
	endpoint        *url.URL
	bucket          string
	accessKeyID     string
	secretAccessKey string
	client          *http.Client
	now             func() time.Time
}

func NewR2Store(options R2Options) (*R2Store, error) {
	endpoint, err := url.Parse(strings.TrimSpace(options.Endpoint))
	if err != nil || !endpoint.IsAbs() || endpoint.Host == "" {
		return nil, fmt.Errorf("%w: invalid R2 endpoint", ErrInvalidServiceSetup)
	}
	if endpoint.Scheme != "https" && endpoint.Scheme != "http" {
		return nil, fmt.Errorf("%w: unsupported R2 endpoint scheme", ErrInvalidServiceSetup)
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, fmt.Errorf("%w: R2 endpoint cannot contain credentials, query, or fragment", ErrInvalidServiceSetup)
	}
	bucket := strings.Trim(strings.TrimSpace(options.Bucket), "/")
	if bucket == "" || strings.Contains(bucket, "/") {
		return nil, fmt.Errorf("%w: invalid R2 bucket", ErrInvalidServiceSetup)
	}
	if strings.TrimSpace(options.AccessKeyID) == "" || strings.TrimSpace(options.SecretAccessKey) == "" {
		return nil, fmt.Errorf("%w: missing R2 credentials", ErrInvalidServiceSetup)
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/")
	return &R2Store{
		endpoint:        endpoint,
		bucket:          bucket,
		accessKeyID:     strings.TrimSpace(options.AccessKeyID),
		secretAccessKey: strings.TrimSpace(options.SecretAccessKey),
		client:          client,
		now:             now,
	}, nil
}

func (store *R2Store) PresignUpload(_ context.Context, objectKey string, ttl time.Duration) (SignedRequest, error) {
	return store.presign(http.MethodPut, objectKey, nil, nil, ttl)
}

// PresignOperationUpload binds the browser upload signature to the declared
// workbook size. The browser supplies Content-Length automatically for a File;
// R2 rejects a body whose actual length differs from the signed value.
func (store *R2Store) PresignOperationUpload(_ context.Context, objectKey string, size int64, ttl time.Duration) (SignedRequest, error) {
	if size <= 0 {
		return SignedRequest{}, ErrInvalidInput
	}
	return store.presign(http.MethodPut, objectKey, nil, map[string]string{"content-length": strconv.FormatInt(size, 10)}, ttl)
}

func (store *R2Store) PresignDownload(_ context.Context, objectKey, fileName, mime string, ttl time.Duration) (SignedRequest, error) {
	query := url.Values{}
	query.Set("response-content-disposition", contentDisposition(fileName))
	if normalized := normalizeMIME(mime); normalized != "" {
		query.Set("response-content-type", normalized)
	}
	return store.presign(http.MethodGet, objectKey, query, nil, ttl)
}

func (store *R2Store) Open(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	response, err := store.do(ctx, http.MethodGet, objectKey)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return nil, ErrUploadObjectNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("%w: R2 read returned status %d", ErrStorageUnavailable, response.StatusCode)
	}
	return response.Body, nil
}

func (store *R2Store) Delete(ctx context.Context, objectKey string) error {
	response, err := store.do(ctx, http.MethodDelete, objectKey)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("%w: R2 delete returned status %d", ErrStorageUnavailable, response.StatusCode)
}

// Put stores a server-generated object without exposing permanent credentials or
// an object key outside the trusted backend boundary.
func (store *R2Store) Put(ctx context.Context, objectKey string, body io.Reader, size int64, mime string) error {
	if body == nil || size <= 0 {
		return ErrInvalidInput
	}
	signed, err := store.PresignUpload(ctx, objectKey, 15*time.Minute)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, signed.Method, signed.URL, body)
	if err != nil {
		return fmt.Errorf("create R2 upload request: %w", err)
	}
	request.ContentLength = size
	if normalized := normalizeMIME(mime); normalized != "" {
		request.Header.Set("Content-Type", normalized)
	}
	response, err := store.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("%w: R2 upload failed", ErrStorageUnavailable)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: R2 upload returned status %d", ErrStorageUnavailable, response.StatusCode)
	}
	return nil
}

func (store *R2Store) presign(method, objectKey string, additional url.Values, headers map[string]string, ttl time.Duration) (SignedRequest, error) {
	if ttl <= 0 || ttl > maximumPresignTTL {
		return SignedRequest{}, ErrInvalidInput
	}
	requestURL, err := store.objectURL(objectKey)
	if err != nil {
		return SignedRequest{}, err
	}
	now := store.now().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	scope := date + "/" + r2Region + "/" + r2Service + "/aws4_request"
	query := cloneValues(additional)
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Content-Sha256", unsignedPayload)
	query.Set("X-Amz-Credential", store.accessKeyID+"/"+scope)
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(ttl/time.Second), 10))

	canonicalHeaders, signedHeaders, responseHeaders := presignedHeaders(requestURL.Host, headers)
	query.Set("X-Amz-SignedHeaders", signedHeaders)
	encodedQuery := canonicalQuery(query)
	canonicalRequest := strings.Join([]string{
		method,
		requestURL.EscapedPath(),
		encodedQuery,
		canonicalHeaders,
		signedHeaders,
		unsignedPayload,
	}, "\n")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hexSHA256(canonicalRequest),
	}, "\n")
	query.Set("X-Amz-Signature", hex.EncodeToString(store.signature(date, stringToSign)))
	requestURL.RawQuery = canonicalQuery(query)
	return SignedRequest{
		URL:       requestURL.String(),
		Method:    method,
		Headers:   responseHeaders,
		ExpiresAt: now.Add(ttl),
	}, nil
}

func presignedHeaders(host string, headers map[string]string) (string, string, map[string]string) {
	normalized := map[string]string{"host": strings.ToLower(strings.TrimSpace(host))}
	responseHeaders := map[string]string{}
	for name, value := range headers {
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.Join(strings.Fields(value), " ")
		if name == "" || name == "host" || value == "" {
			continue
		}
		normalized[name] = value
		responseHeaders[http.CanonicalHeaderKey(name)] = value
	}
	names := make([]string, 0, len(normalized))
	for name := range normalized {
		names = append(names, name)
	}
	sort.Strings(names)
	var canonical strings.Builder
	for _, name := range names {
		canonical.WriteString(name)
		canonical.WriteByte(':')
		canonical.WriteString(normalized[name])
		canonical.WriteByte('\n')
	}
	return canonical.String(), strings.Join(names, ";"), responseHeaders
}

func (store *R2Store) do(ctx context.Context, method, objectKey string) (*http.Response, error) {
	requestURL, err := store.objectURL(objectKey)
	if err != nil {
		return nil, err
	}
	now := store.now().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	scope := date + "/" + r2Region + "/" + r2Service + "/aws4_request"
	canonicalHeaders := "host:" + strings.ToLower(requestURL.Host) + "\n" +
		"x-amz-content-sha256:" + unsignedPayload + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		method,
		requestURL.EscapedPath(),
		"",
		canonicalHeaders,
		signedHeaders,
		unsignedPayload,
	}, "\n")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hexSHA256(canonicalRequest),
	}, "\n")
	authorization := "AWS4-HMAC-SHA256 Credential=" + store.accessKeyID + "/" + scope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + hex.EncodeToString(store.signature(date, stringToSign))
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create R2 request: %w", err)
	}
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
	request.Header.Set("Authorization", authorization)
	response, err := store.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: R2 request failed", ErrStorageUnavailable)
	}
	return response, nil
}

func (store *R2Store) objectURL(objectKey string) (*url.URL, error) {
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	if objectKey == "" || strings.Contains(objectKey, "..") {
		return nil, ErrInvalidInput
	}
	result := *store.endpoint
	segments := []string{strings.Trim(result.Path, "/"), store.bucket, objectKey}
	plain := "/" + path.Join(segments...)
	result.Path = plain
	escapedParts := make([]string, 0)
	for _, segment := range strings.Split(strings.TrimPrefix(plain, "/"), "/") {
		escapedParts = append(escapedParts, url.PathEscape(segment))
	}
	result.RawPath = "/" + strings.Join(escapedParts, "/")
	return &result, nil
}

func (store *R2Store) signature(date, stringToSign string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+store.secretAccessKey), date)
	regionKey := hmacSHA256(dateKey, r2Region)
	serviceKey := hmacSHA256(regionKey, r2Service)
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	return hmacSHA256(signingKey, stringToSign)
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func hexSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func canonicalQuery(values url.Values) string {
	return strings.ReplaceAll(values.Encode(), "+", "%20")
}

func cloneValues(values url.Values) url.Values {
	result := url.Values{}
	for key, entries := range values {
		result[key] = append([]string(nil), entries...)
	}
	return result
}

func contentDisposition(fileName string) string {
	fileName = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(fileName, "\r", ""), "\n", ""))
	if fileName == "" {
		fileName = "attachment"
	}
	return "attachment; filename*=UTF-8''" + url.PathEscape(fileName)
}
