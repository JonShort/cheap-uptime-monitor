package checker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/cheap-uptime-monitor/internal/config"
)

type Result struct {
	Success    bool
	StatusCode int
	Reason     string
	Timestamp  time.Time
}

type Checker struct {
	logger *slog.Logger
}

func NewChecker(logger *slog.Logger) *Checker {
	return &Checker{
		logger: logger,
	}
}

func (c *Checker) Check(ctx context.Context, endpoint config.Endpoint) Result {
	timeout := time.Duration(endpoint.GetTimeout()) * time.Second

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	if err != nil {
		return Result{
			Success:   false,
			Reason:    fmt.Sprintf("failed to create request: %v", err),
			Timestamp: time.Now().UTC(),
		}
	}

	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	req.Header.Set("User-Agent", "cheap-uptime-monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Result{
			Success:   false,
			Reason:    fmt.Sprintf("request failed: %v", err),
			Timestamp: time.Now().UTC(),
		}
	}
	defer resp.Body.Close()

	// Drain body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return Result{
			Success:    false,
			StatusCode: resp.StatusCode,
			Reason:     fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
			Timestamp:  time.Now().UTC(),
		}
	}

	return Result{
		Success:    true,
		StatusCode: resp.StatusCode,
		Timestamp:  time.Now().UTC(),
	}
}
