package config

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type mockS3Client struct {
	response []byte
	err      error
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(m.response)),
	}, nil
}

func TestLoader_Load_Success(t *testing.T) {
	configJSON := `{
		"endpoints": [
			{
				"name": "Test API",
				"url": "https://api.example.com/health",
				"timeout_seconds": 10,
				"headers": {
					"X-Test": "value"
				}
			}
		]
	}`

	client := &mockS3Client{response: []byte(configJSON)}
	loader := NewLoader(client, "test-bucket", "config.json")

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(cfg.Endpoints))
	}

	ep := cfg.Endpoints[0]
	if ep.Name != "Test API" {
		t.Errorf("expected name 'Test API', got '%s'", ep.Name)
	}
	if ep.URL != "https://api.example.com/health" {
		t.Errorf("expected url 'https://api.example.com/health', got '%s'", ep.URL)
	}
	if ep.TimeoutSeconds != 10 {
		t.Errorf("expected timeout 10, got %d", ep.TimeoutSeconds)
	}
	if ep.Headers["X-Test"] != "value" {
		t.Errorf("expected header X-Test=value, got %s", ep.Headers["X-Test"])
	}
}

func TestLoader_Load_S3Error(t *testing.T) {
	client := &mockS3Client{err: errors.New("access denied")}
	loader := NewLoader(client, "test-bucket", "config.json")

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoader_Load_InvalidJSON(t *testing.T) {
	client := &mockS3Client{response: []byte("not valid json")}
	loader := NewLoader(client, "test-bucket", "config.json")

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConfiguration_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Configuration
		wantErr bool
	}{
		{
			name: "valid config",
			config: Configuration{
				Endpoints: []Endpoint{
					{Name: "Test", URL: "https://example.com"},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty endpoints",
			config:  Configuration{Endpoints: []Endpoint{}},
			wantErr: true,
		},
		{
			name: "missing name",
			config: Configuration{
				Endpoints: []Endpoint{
					{URL: "https://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing url",
			config: Configuration{
				Endpoints: []Endpoint{
					{Name: "Test"},
				},
			},
			wantErr: true,
		},
		{
			name: "http url not allowed",
			config: Configuration{
				Endpoints: []Endpoint{
					{Name: "Test", URL: "http://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			config: Configuration{
				Endpoints: []Endpoint{
					{Name: "Test", URL: "https://example.com", TimeoutSeconds: 100},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEndpoint_GetTimeout(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		want     int
	}{
		{
			name:     "default timeout",
			endpoint: Endpoint{},
			want:     DefaultTimeoutSeconds,
		},
		{
			name:     "custom timeout",
			endpoint: Endpoint{TimeoutSeconds: 10},
			want:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.endpoint.GetTimeout(); got != tt.want {
				t.Errorf("GetTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

