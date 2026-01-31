package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	DefaultTimeoutSeconds = 30
)

type Configuration struct {
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
}

func (e *Endpoint) GetTimeout() int {
	if e.TimeoutSeconds > 0 {
		return e.TimeoutSeconds
	}
	return DefaultTimeoutSeconds
}

type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type Loader struct {
	s3Client S3Client
	bucket   string
	key      string
}

func NewLoader(s3Client S3Client, bucket, key string) *Loader {
	return &Loader{
		s3Client: s3Client,
		bucket:   bucket,
		key:      key,
	}
}

func (l *Loader) Load(ctx context.Context) (*Configuration, error) {
	output, err := l.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(l.bucket),
		Key:    aws.String(l.key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get config from S3: %w", err)
	}
	defer output.Body.Close()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read config body: %w", err)
	}

	var cfg Configuration
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (c *Configuration) Validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("no endpoints configured")
	}

	for i, ep := range c.Endpoints {
		if ep.Name == "" {
			return fmt.Errorf("endpoint %d: name is required", i)
		}
		if ep.URL == "" {
			return fmt.Errorf("endpoint %d (%s): url is required", i, ep.Name)
		}
		if len(ep.URL) < 8 || ep.URL[:8] != "https://" {
			return fmt.Errorf("endpoint %d (%s): url must start with https://", i, ep.Name)
		}
		if ep.TimeoutSeconds < 0 || ep.TimeoutSeconds > 60 {
			return fmt.Errorf("endpoint %d (%s): timeout_seconds must be between 0 and 60", i, ep.Name)
		}
	}

	return nil
}
