package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var revisionPattern = regexp.MustCompile(`^[a-f0-9]{12}$`)

type healthPayload struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
	Release  struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	} `json:"release"`
}

func main() {
	apiURL := flag.String("api-url", "", "absolute API base URL")
	expectedRevision := flag.String("expected-revision", "", "expected 12-character Git revision")
	timeout := flag.Duration("timeout", 20*time.Second, "overall smoke-check timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := checkLaunch(ctx, http.DefaultClient, *apiURL, *expectedRevision); err != nil {
		fmt.Fprintln(os.Stderr, "launch smoke check failed:", err)
		os.Exit(1)
	}
	fmt.Printf("launch smoke check passed: revision=%s\n", *expectedRevision)
}

func checkLaunch(ctx context.Context, client *http.Client, baseURL, expectedRevision string) error {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("api URL must be an absolute URL without query or fragment")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLocalHost(parsed.Hostname())) {
		return errors.New("api URL must use HTTPS outside localhost")
	}
	if !revisionPattern.MatchString(expectedRevision) {
		return errors.New("expected revision must be 12 lowercase hexadecimal characters")
	}
	base := strings.TrimRight(parsed.String(), "/")
	live, err := readHealth(ctx, client, base+"/health/live")
	if err != nil {
		return fmt.Errorf("liveness: %w", err)
	}
	if live.Status != "ok" {
		return fmt.Errorf("liveness status is %q", live.Status)
	}
	if live.Release.Revision != expectedRevision {
		return fmt.Errorf("liveness revision is %q, expected %q", live.Release.Revision, expectedRevision)
	}
	ready, err := readHealth(ctx, client, base+"/health/ready")
	if err != nil {
		return fmt.Errorf("readiness: %w", err)
	}
	if ready.Status != "ok" || ready.Database != "ok" {
		return fmt.Errorf("readiness returned status=%q database=%q", ready.Status, ready.Database)
	}
	if ready.Release.Revision != expectedRevision {
		return fmt.Errorf("readiness revision is %q, expected %q", ready.Release.Revision, expectedRevision)
	}
	return nil
}

func readHealth(ctx context.Context, client *http.Client, endpoint string) (healthPayload, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return healthPayload{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return healthPayload{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return healthPayload{}, fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/json") {
		return healthPayload{}, errors.New("response is not JSON")
	}
	if strings.TrimSpace(response.Header.Get("X-Request-ID")) == "" {
		return healthPayload{}, errors.New("response is missing X-Request-ID")
	}
	var payload healthPayload
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&payload); err != nil {
		return healthPayload{}, err
	}
	if payload.Release.Version == "" || payload.Release.Revision == "" {
		return healthPayload{}, errors.New("response is missing release metadata")
	}
	return payload, nil
}

func isLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
