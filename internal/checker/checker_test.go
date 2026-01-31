package checker

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cheap-uptime-monitor/internal/config"
)

func newTestChecker() *Checker {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewChecker(logger)
}

func TestChecker_Check_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            server.URL,
		TimeoutSeconds: 5,
	}

	result := checker.Check(context.Background(), endpoint)

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Reason)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

func TestChecker_Check_Non200Status(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"201 Created", http.StatusCreated},
		{"204 No Content", http.StatusNoContent},
		{"301 Moved Permanently", http.StatusMovedPermanently},
		{"302 Found", http.StatusFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			checker := newTestChecker()
			endpoint := config.Endpoint{
				Name:           "Test",
				URL:            server.URL,
				TimeoutSeconds: 5,
			}

			result := checker.Check(context.Background(), endpoint)

			if result.Success {
				t.Errorf("expected failure for status %d", tt.statusCode)
			}
			if result.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, result.StatusCode)
			}
		})
	}
}

func TestChecker_Check_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            server.URL,
		TimeoutSeconds: 1,
	}

	result := checker.Check(context.Background(), endpoint)

	if result.Success {
		t.Error("expected failure due to timeout")
	}
}

func TestChecker_Check_ConnectionRefused(t *testing.T) {
	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            "https://localhost:59999",
		TimeoutSeconds: 2,
	}

	result := checker.Check(context.Background(), endpoint)

	if result.Success {
		t.Error("expected failure due to connection refused")
	}
}

func TestChecker_Check_CustomHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            server.URL,
		TimeoutSeconds: 5,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	result := checker.Check(context.Background(), endpoint)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Reason)
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("expected X-Custom-Header=custom-value, got %s", receivedHeaders.Get("X-Custom-Header"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("expected Authorization header, got %s", receivedHeaders.Get("Authorization"))
	}
	if receivedHeaders.Get("User-Agent") != "cheap-uptime-monitor/1.0" {
		t.Errorf("expected User-Agent header, got %s", receivedHeaders.Get("User-Agent"))
	}
}

func TestChecker_Check_NoRedirectFollow(t *testing.T) {
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectServer.URL, http.StatusMovedPermanently)
	}))
	defer server.Close()

	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            server.URL,
		TimeoutSeconds: 5,
	}

	result := checker.Check(context.Background(), endpoint)

	if result.Success {
		t.Error("expected failure, redirects should not be followed")
	}
	if result.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected status 301, got %d", result.StatusCode)
	}
}

func TestChecker_Check_InvalidURL(t *testing.T) {
	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            "://invalid-url",
		TimeoutSeconds: 5,
	}

	result := checker.Check(context.Background(), endpoint)

	if result.Success {
		t.Error("expected failure for invalid URL")
	}
}

func TestChecker_Check_TimestampSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	before := time.Now().UTC()

	checker := newTestChecker()
	endpoint := config.Endpoint{
		Name:           "Test",
		URL:            server.URL,
		TimeoutSeconds: 5,
	}

	result := checker.Check(context.Background(), endpoint)

	after := time.Now().UTC()

	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("timestamp %v not within expected range [%v, %v]", result.Timestamp, before, after)
	}
}
