package health

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type Status string

const (
	StatusUp      Status = "up"
	StatusDown    Status = "down"
	StatusUnknown Status = "unknown"
)

type EndpointHealth struct {
	Status      Status    `json:"status"`
	LastChecked time.Time `json:"last_checked"`
}

type KnownHealth struct {
	Endpoints map[string]EndpointHealth `json:"endpoints"`
}

func (kh *KnownHealth) GetStatus(endpointName string) Status {
	if ep, ok := kh.Endpoints[endpointName]; ok {
		return ep.Status
	}
	return StatusUnknown
}

type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type Tracker struct {
	s3Client S3Client
	bucket   string
	key      string
}

func NewTracker(s3Client S3Client, bucket, key string) *Tracker {
	return &Tracker{
		s3Client: s3Client,
		bucket:   bucket,
		key:      key,
	}
}

func (t *Tracker) Load(ctx context.Context) (*KnownHealth, error) {
	output, err := t.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(t.bucket),
		Key:    aws.String(t.key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			return &KnownHealth{Endpoints: make(map[string]EndpointHealth)}, nil
		}
		return nil, fmt.Errorf("failed to get known health from S3: %w", err)
	}
	defer output.Body.Close()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read known health body: %w", err)
	}

	var kh KnownHealth
	if err := json.Unmarshal(body, &kh); err != nil {
		return nil, fmt.Errorf("failed to parse known health JSON: %w", err)
	}

	if kh.Endpoints == nil {
		kh.Endpoints = make(map[string]EndpointHealth)
	}

	return &kh, nil
}

func (t *Tracker) Save(ctx context.Context, kh *KnownHealth) error {
	data, err := json.Marshal(kh)
	if err != nil {
		return fmt.Errorf("failed to marshal known health: %w", err)
	}

	_, err = t.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(t.bucket),
		Key:    aws.String(t.key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to save known health to S3: %w", err)
	}

	return nil
}
