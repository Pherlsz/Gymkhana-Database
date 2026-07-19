package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckLaunchAcceptsMatchingHealthyRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("X-Request-ID", "request-1")
		if request.URL.Path == "/health/ready" {
			_, _ = response.Write([]byte(`{"status":"ok","database":"ok","release":{"version":"sha-304b760a3fff","revision":"304b760a3fff"}}`))
			return
		}
		_, _ = response.Write([]byte(`{"status":"ok","release":{"version":"sha-304b760a3fff","revision":"304b760a3fff"}}`))
	}))
	defer server.Close()

	if err := checkLaunch(context.Background(), server.Client(), server.URL, "304b760a3fff"); err != nil {
		t.Fatalf("checkLaunch() error = %v", err)
	}
}

func TestCheckLaunchRejectsRevisionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("X-Request-ID", "request-1")
		_, _ = response.Write([]byte(`{"status":"ok","database":"ok","release":{"version":"sha-aaaaaaaaaaaa","revision":"aaaaaaaaaaaa"}}`))
	}))
	defer server.Close()

	err := checkLaunch(context.Background(), server.Client(), server.URL, "304b760a3fff")
	if err == nil || !strings.Contains(err.Error(), "revision") {
		t.Fatalf("checkLaunch() error = %v", err)
	}
}

func TestCheckLaunchRejectsInsecureRemoteURL(t *testing.T) {
	err := checkLaunch(context.Background(), http.DefaultClient, "http://api.example", "304b760a3fff")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("checkLaunch() error = %v", err)
	}
}

func TestCheckLaunchRequiresRequestIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"ok","release":{"version":"sha-304b760a3fff","revision":"304b760a3fff"}}`))
	}))
	defer server.Close()

	err := checkLaunch(context.Background(), server.Client(), server.URL, "304b760a3fff")
	if err == nil || !strings.Contains(err.Error(), "X-Request-ID") {
		t.Fatalf("checkLaunch() error = %v", err)
	}
}
